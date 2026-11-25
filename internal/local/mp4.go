package local

import (
	"encoding/binary"
	"io"
	"strings"
	"time"
)

// Playable reports whether a file with extension ext (".mp3") is one the
// player plays: MP3 and FLAC, and since 1.3.3 AAC (in an M4A or bare, and
// ALAC in an M4A, which Windows decodes) and DSD (DSF and DSDIFF), and
// WAV (#54).
func Playable(ext string) bool {
	switch strings.ToLower(ext) {
	case ".mp3", ".flac", ".wav", ".m4a", ".m4b", ".aac", ".dsf", ".dff":
		return true
	}
	return false
}

// mp4Atom is a box of an MP4 file: its type, and where its body is.
type mp4Atom struct {
	typ        string
	body, size int64
}

// mp4Atoms lists the boxes in r from off to end.
func mp4Atoms(r io.ReaderAt, off, end int64) []mp4Atom {
	var out []mp4Atom
	for off+8 <= end && len(out) < 4096 {
		var h [16]byte
		if _, err := r.ReadAt(h[:8], off); err != nil {
			break
		}
		size := int64(binary.BigEndian.Uint32(h[:4]))
		head := int64(8)
		switch size {
		case 0:
			size = end - off
		case 1:
			if _, err := r.ReadAt(h[8:16], off+8); err != nil {
				return out
			}
			size, head = int64(binary.BigEndian.Uint64(h[8:16])), 16
		}
		if size < head || off+size > end {
			break
		}
		out = append(out, mp4Atom{typ: string(h[4:8]), body: off + head, size: size - head})
		off += size
	}
	return out
}

func mp4Find(atoms []mp4Atom, typ string) (mp4Atom, bool) {
	for _, a := range atoms {
		if a.typ == typ {
			return a, true
		}
	}
	return mp4Atom{}, false
}

// readMP4 reads an M4A's iTunes tags and its length.
func readMP4(r io.ReaderAt, size int64, s *Song) {
	moov, ok := mp4Find(mp4Atoms(r, 0, size), "moov")
	if !ok || moov.size > 64<<20 {
		return
	}
	b := make([]byte, moov.size)
	if _, err := r.ReadAt(b, moov.body); err != nil {
		return
	}
	m := bytesAt(b)
	inner := mp4Atoms(m, 0, int64(len(b)))
	if mvhd, ok := mp4Find(inner, "mvhd"); ok && mvhd.size >= 20 {
		h := b[mvhd.body : mvhd.body+mvhd.size]
		var scale, dur uint64
		if h[0] == 1 && len(h) >= 32 {
			scale, dur = uint64(binary.BigEndian.Uint32(h[20:24])), binary.BigEndian.Uint64(h[24:32])
		} else {
			scale, dur = uint64(binary.BigEndian.Uint32(h[12:16])), uint64(binary.BigEndian.Uint32(h[16:20]))
		}
		if scale > 0 {
			s.Duration = time.Duration(float64(dur) / float64(scale) * float64(time.Second))
		}
	}
	udta, ok := mp4Find(inner, "udta")
	if !ok {
		return
	}
	meta, ok := mp4Find(mp4Atoms(m, udta.body, udta.body+udta.size), "meta")
	if !ok || meta.size < 4 {
		return
	}
	// meta has a version and flags before its boxes.
	ilst, ok := mp4Find(mp4Atoms(m, meta.body+4, meta.body+meta.size), "ilst")
	if !ok {
		return
	}
	for _, item := range mp4Atoms(m, ilst.body, ilst.body+ilst.size) {
		data, ok := mp4Find(mp4Atoms(m, item.body, item.body+item.size), "data")
		if !ok || data.size < 8 {
			continue
		}
		kind := binary.BigEndian.Uint32(b[data.body:]) & 0xffffff
		v := b[data.body+8 : data.body+data.size]
		text := strings.TrimSpace(string(v))
		switch item.typ {
		case "\xa9nam":
			s.Title = first(s.Title, text)
		case "\xa9ART":
			s.Artist = first(s.Artist, text)
		case "aART":
			if s.Artist == "" {
				s.Artist = text
			}
		case "\xa9alb":
			s.Album = first(s.Album, text)
		case "\xa9lyr":
			s.Lyrics = first(s.Lyrics, string(v))
		case "trkn":
			if len(v) >= 4 {
				s.Track = int(binary.BigEndian.Uint16(v[2:4]))
			}
		case "disk":
			if len(v) >= 4 {
				s.Disc = int(binary.BigEndian.Uint16(v[2:4]))
			}
		case "covr":
			mime := "image/jpeg"
			if kind == 14 {
				mime = "image/png"
			}
			if s.Picture == nil {
				s.Picture, s.PictureMIME = append([]byte(nil), v...), mime
			}
		}
	}
}

// bytesAt reads a byte slice as an io.ReaderAt.
type bytesAt []byte

func (b bytesAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// readADTS reads a bare AAC's length from its frames' headers: each holds
// 1024 samples, at the rate the first says.
func readADTS(r io.ReaderAt, size int64, s *Song) {
	rates := []int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}
	off := id3TagSize(r)
	frames, rate := int64(0), 0
	var h [7]byte
	for off+7 <= size && frames < 1<<22 {
		if _, err := r.ReadAt(h[:], off); err != nil || h[0] != 0xff || h[1]&0xf6 != 0xf0 {
			break
		}
		if rate == 0 {
			if i := int(h[2]>>2) & 0xf; i < len(rates) {
				rate = rates[i]
			}
		}
		n := int64(h[3]&3)<<11 | int64(h[4])<<3 | int64(h[5]>>5)
		if n < 7 {
			break
		}
		off += n
		frames++
	}
	if rate > 0 {
		s.Duration = time.Duration(frames * 1024 * int64(time.Second) / int64(rate))
	}
}

// readDSF reads a DSF's length from its format chunk, and its tags from
// the ID3v2 tag its header points to.
func readDSF(r io.ReaderAt, size int64, s *Song) {
	var h [92]byte
	if _, err := r.ReadAt(h[:], 0); err != nil || string(h[:4]) != "DSD " || string(h[28:32]) != "fmt " {
		return
	}
	meta := int64(binary.LittleEndian.Uint64(h[20:28]))
	rate := binary.LittleEndian.Uint32(h[56:60])
	count := binary.LittleEndian.Uint64(h[64:72])
	if rate > 0 {
		s.Duration = time.Duration(float64(count) / float64(rate) * float64(time.Second))
	}
	if meta > 0 && meta+10 <= size {
		var th [10]byte
		if _, err := r.ReadAt(th[:], meta); err == nil && string(th[:3]) == "ID3" {
			n := syncsafe(th[6:10])
			if n > 0 && n < 64<<20 && meta+10+n <= size {
				b := make([]byte, 10+n)
				if _, err := r.ReadAt(b, meta); err == nil {
					readID3v2(b, s)
				}
			}
		}
	}
}

// readDFF reads a DSDIFF's length from its sample rate and sound data.
func readDFF(r io.ReaderAt, size int64, s *Song) {
	var h [16]byte
	if _, err := r.ReadAt(h[:16], 0); err != nil || string(h[:4]) != "FRM8" || string(h[12:16]) != "DSD " {
		return
	}
	var rate uint32
	channels := 2
	var data int64
	for off := int64(16); off+12 <= size; {
		var ch [12]byte
		if _, err := r.ReadAt(ch[:], off); err != nil {
			break
		}
		id, n := string(ch[:4]), int64(binary.BigEndian.Uint64(ch[4:12]))
		body := off + 12
		switch id {
		case "PROP":
			for p := body + 4; p+12 <= body+n; {
				var sub [16]byte
				if _, err := r.ReadAt(sub[:], p); err != nil {
					break
				}
				sid, sn := string(sub[:4]), int64(binary.BigEndian.Uint64(sub[4:12]))
				switch sid {
				case "FS  ":
					rate = binary.BigEndian.Uint32(sub[12:16])
				case "CHNL":
					channels = int(binary.BigEndian.Uint16(sub[12:14]))
				}
				p += 12 + sn + sn&1
			}
		case "DSD ":
			data = n
		}
		off = body + n + n&1
	}
	if rate > 0 && channels > 0 && data > 0 {
		s.Duration = time.Duration(float64(data*8/int64(channels)) / float64(rate) * float64(time.Second))
	}
}
