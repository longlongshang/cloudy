package player

import (
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sync"
)

// wavStream decodes a WAV, PCM as it is, for package audio: songs of the
// listener's folder and of the cloud drive (#54). It reads RIFF and RF64
// (WAVs of more than 4 GB), of 8, 16, 24 or 32-bit integers or 32 or
// 64-bit floats, plain or as WAVE_FORMAT_EXTENSIBLE. Its frames are all of
// one size, so a seek goes to the very byte.
type wavStream struct {
	dl        *download
	wavFormat       // what the header says of the sound
	frames    int64 // in all
	r         *bufio.Reader
	pos       int64 // the frame Read returns next
	raw       []byte
	spec      *spectrum
	drop      func()
	once      sync.Once
}

// wavFormat is what a WAV's "fmt " and "data" chunks say of its sound.
type wavFormat struct {
	float         bool
	channels      int
	rate          int
	bits          int // a sample's, of the container: 8, 16, 24, 32 or 64
	valid         int // the bits of them that hold the sound
	align         int // bytes of a frame, every channel's sample
	start, length int64
}

const (
	wavPCM        = 1
	wavFloat      = 3
	wavExtensible = 0xfffe
)

// readWAVHeader reads the chunks of the WAV at off, from "RIFF" (or
// "RF64") to the start of its "data".
func readWAVHeader(dl *download, off int64) (wavFormat, error) {
	le := binary.LittleEndian
	if !dl.waitAt(off, 12) {
		return wavFormat{}, io.ErrUnexpectedEOF
	}
	head := dl.peek(off, 12)
	riff := string(head[:4])
	if riff != "RIFF" && riff != "RF64" && riff != "BW64" || string(head[8:12]) != "WAVE" {
		return wavFormat{}, errors.New("wav: not a WAV file")
	}
	var f wavFormat
	var haveFmt bool
	var big int64 // the size of "data" from RF64's "ds64", which then says 0xffffffff
	for at := off + 12; ; {
		if !dl.waitAt(at, 8) {
			return wavFormat{}, errors.New("wav: no data chunk")
		}
		c := dl.peek(at, 8)
		id, n := string(c[:4]), int64(le.Uint32(c[4:8]))
		body := at + 8
		switch id {
		case "ds64":
			if n >= 16 && dl.waitAt(body, 16) {
				big = int64(le.Uint64(dl.peek(body+8, 8)))
			}
		case "fmt ":
			if n < 16 || n > 1<<10 || !dl.waitAt(body, n) {
				return wavFormat{}, errors.New("wav: bad format chunk")
			}
			b := dl.peek(body, int(n))
			tag := le.Uint16(b[0:2])
			f.channels = int(le.Uint16(b[2:4]))
			f.rate = int(le.Uint32(b[4:8]))
			f.align = int(le.Uint16(b[12:14]))
			f.bits = int(le.Uint16(b[14:16]))
			f.valid = f.bits
			if tag == wavExtensible {
				if len(b) < 40 {
					return wavFormat{}, errors.New("wav: bad format chunk")
				}
				if v := int(le.Uint16(b[18:20])); v > 0 && v <= f.bits {
					f.valid = v
				}
				// The sub-format's GUID begins with the tag it stands for.
				tag = le.Uint16(b[24:26])
			}
			switch {
			case tag == wavPCM && (f.bits == 8 || f.bits == 16 || f.bits == 24 || f.bits == 32):
			case tag == wavFloat && (f.bits == 32 || f.bits == 64):
				f.float = true
			default:
				return wavFormat{}, errors.New("wav: compressed or unknown samples")
			}
			if f.channels <= 0 || f.rate <= 0 || f.align != f.channels*f.bits/8 {
				return wavFormat{}, errors.New("wav: unexpected format")
			}
			haveFmt = true
		case "data":
			if !haveFmt {
				return wavFormat{}, errors.New("wav: data before its format")
			}
			if n == 0xffffffff && big > 0 {
				n = big
			}
			// A WAV written as it was recorded, or cut short, can say more
			// than there is (or 0xffffffff, or 0): the file's end is the
			// sound's.
			if size := dl.totalSize(); size > 0 && (n == 0 || body+n > size) {
				n = size - body
			}
			f.start, f.length = body, n
			return f, nil
		}
		at = body + n + n&1
	}
}

func openWAVStream(dl *download) (*wavStream, error) {
	f, err := readWAVHeader(dl, id3Size(dl))
	if err != nil {
		return nil, err
	}
	s := &wavStream{dl: dl, wavFormat: f, frames: f.length / int64(f.align), drop: dl.Close}
	s.spec = dl.spectrum(f.rate, s.frames)
	s.seek(0)
	return s, nil
}

func (s *wavStream) SampleRate() int { return s.rate }
func (s *wavStream) Channels() int   { return 2 }
func (s *wavStream) Len() int64      { return s.frames }

// format is the WAV's rate and the bits of a sample that hold sound.
func (s *wavStream) format() (rate, bits int) { return s.rate, s.valid }

func (s *wavStream) seek(frame int64) {
	s.pos = frame
	at := s.start + frame*int64(s.align)
	s.r = bufio.NewReaderSize(&downloadReader{d: s.dl, pos: at, rebuffer: s.dl.parent == nil}, 64<<10)
}

func (s *wavStream) Read(dst []float32) (int, error) {
	want := min(int64(len(dst)/2), s.frames-s.pos)
	if want <= 0 {
		return 0, io.EOF
	}
	need := int(want) * s.align
	if cap(s.raw) < need {
		s.raw = make([]byte, need)
	}
	raw := s.raw[:need]
	// A short read is the end: of the file, or of a download cut off.
	got, err := io.ReadFull(s.r, raw)
	n := got / s.align
	if n == 0 {
		if err == nil || err == io.ErrUnexpectedEOF {
			err = io.EOF
		}
		return 0, err
	}
	size := s.bits / 8
	second := 0
	if s.channels > 1 {
		second = size
	}
	for i := 0; i < n; i++ {
		frame := raw[i*s.align:]
		// More than two channels start with the front left and right.
		dst[2*i] = s.sample(frame[:size])
		dst[2*i+1] = s.sample(frame[second : second+size])
	}
	out := dst[:2*n]
	s.spec.feed(s.pos, out)
	s.pos += int64(n)
	return len(out), nil
}

// sample is the sound of a sample's bytes, from -1 to 1.
func (s *wavStream) sample(b []byte) float32 {
	le := binary.LittleEndian
	if s.float {
		if len(b) == 8 {
			return float32(math.Float64frombits(le.Uint64(b)))
		}
		return math.Float32frombits(le.Uint32(b))
	}
	switch len(b) {
	case 1:
		// Eight bits are unsigned, about 128.
		return float32(int(b[0])-128) / 128
	case 2:
		return float32(int16(le.Uint16(b))) / (1 << 15)
	case 3:
		v := int32(uint32(b[0])<<8|uint32(b[1])<<16|uint32(b[2])<<24) >> 8
		return float32(v) / (1 << 23)
	}
	return float32(float64(int32(le.Uint32(b))) / (1 << 31))
}

// SetPosition goes to frame, at its very byte.
func (s *wavStream) SetPosition(frame int64) error {
	s.seek(max(0, min(frame, s.frames)))
	return nil
}

func (s *wavStream) source() (*download, *spectrum) { return s.dl, s.spec }

func (s *wavStream) Close() error {
	s.once.Do(s.drop)
	return nil
}
