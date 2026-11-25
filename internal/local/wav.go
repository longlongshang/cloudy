package local

import (
	"encoding/binary"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// readWAV reads a WAV's length from its "fmt " and "data" chunks, and its
// tags from an ID3v2 tag in an "id3 " chunk, or else from the "INFO" list
// that most editors write.
func readWAV(r io.ReaderAt, size int64, s *Song) {
	le := binary.LittleEndian
	var h [12]byte
	if _, err := r.ReadAt(h[:], 0); err != nil || string(h[8:12]) != "WAVE" {
		return
	}
	if id := string(h[:4]); id != "RIFF" && id != "RF64" && id != "BW64" {
		return
	}
	var byteRate, big int64
	var info []byte
	for off := int64(12); off+8 <= size; {
		var c [8]byte
		if _, err := r.ReadAt(c[:], off); err != nil {
			break
		}
		id, n := string(c[:4]), int64(le.Uint32(c[4:8]))
		body := off + 8
		switch id {
		case "ds64":
			var b [16]byte
			if _, err := r.ReadAt(b[:], body); err == nil {
				big = int64(le.Uint64(b[8:16]))
			}
		case "fmt ":
			var b [16]byte
			if _, err := r.ReadAt(b[:], body); err == nil {
				byteRate = int64(le.Uint32(b[8:12]))
			}
		case "data":
			if n == 0xffffffff && big > 0 {
				n = big
			}
			if n == 0 || body+n > size {
				n = size - body
			}
			if byteRate > 0 {
				s.Duration = time.Duration(float64(n) / float64(byteRate) * float64(time.Second))
			}
		case "id3 ", "ID3 ":
			if n > 10 && n < 64<<20 && body+n <= size {
				b := make([]byte, n)
				if _, err := r.ReadAt(b, body); err == nil {
					readID3v2(b, s)
				}
			}
		case "LIST":
			if n > 4 && n < 1<<20 && body+n <= size {
				b := make([]byte, n)
				if _, err := r.ReadAt(b, body); err == nil && string(b[:4]) == "INFO" {
					info = b[4:]
				}
			}
		}
		off = body + n + n&1
	}
	readWAVInfo(info, s)
}

// readWAVInfo fills what an ID3 tag left empty from a RIFF "INFO" list:
// the title, artist, album and track. Its strings carry no encoding: UTF-8
// where they are that, else taken for GBK, as Chinese Windows writes them.
func readWAVInfo(b []byte, s *Song) {
	for len(b) >= 8 {
		id, n := string(b[:4]), int(binary.LittleEndian.Uint32(b[4:8]))
		if n > len(b)-8 {
			break
		}
		v := riffText(b[8 : 8+n])
		switch id {
		case "INAM":
			s.Title = first(s.Title, v)
		case "IART":
			s.Artist = first(s.Artist, v)
		case "IPRD":
			s.Album = first(s.Album, v)
		case "ITRK", "IPRT":
			if s.Track == 0 {
				s.Track = number(v)
			}
		}
		b = b[min(len(b), 8+n+n&1):]
	}
}

func riffText(b []byte) string {
	v := strings.TrimRight(string(b), "\x00")
	if !utf8.ValidString(v) {
		if d, err := simplifiedchinese.GBK.NewDecoder().String(v); err == nil {
			v = d
		}
	}
	return strings.TrimSpace(v)
}
