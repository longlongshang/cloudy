package player

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// wavFile is a second of a 1 kHz tone at amplitude 0.5 in a WAV, of
// channels and bits a sample, as floats or not, its format chunk extensible
// or plain, with a LIST chunk before its data as editors write one.
func wavFile(channels, bits int, float, extensible bool, rate int) []byte {
	le := binary.LittleEndian
	var data bytes.Buffer
	for n := 0; n < rate; n++ {
		x := 0.5 * math.Sin(2*math.Pi*1000*float64(n)/float64(rate))
		for c := 0; c < channels; c++ {
			switch {
			case float && bits == 32:
				binary.Write(&data, le, float32(x))
			case float:
				binary.Write(&data, le, x)
			case bits == 8:
				data.WriteByte(byte(int(math.Round(x*127)) + 128))
			case bits == 16:
				binary.Write(&data, le, int16(math.Round(x*32767)))
			case bits == 24:
				v := int32(math.Round(x * (1<<23 - 1)))
				data.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16)})
			default:
				binary.Write(&data, le, int32(math.Round(x*(1<<31-1))))
			}
		}
	}
	tag := uint16(wavPCM)
	if float {
		tag = wavFloat
	}
	var fmtc bytes.Buffer
	align := channels * bits / 8
	head := tag
	if extensible {
		head = wavExtensible
	}
	binary.Write(&fmtc, le, head)
	binary.Write(&fmtc, le, uint16(channels))
	binary.Write(&fmtc, le, uint32(rate))
	binary.Write(&fmtc, le, uint32(rate*align))
	binary.Write(&fmtc, le, uint16(align))
	binary.Write(&fmtc, le, uint16(bits))
	if extensible {
		binary.Write(&fmtc, le, uint16(22))
		binary.Write(&fmtc, le, uint16(bits))
		binary.Write(&fmtc, le, uint32(3))
		binary.Write(&fmtc, le, tag)
		fmtc.Write([]byte{0, 0, 0, 0, 0x10, 0, 0x80, 0, 0, 0xaa, 0, 0x38, 0x9b, 0x71})
	}
	var body bytes.Buffer
	body.WriteString("WAVE")
	chunk := func(id string, b []byte) {
		body.WriteString(id)
		binary.Write(&body, le, uint32(len(b)))
		body.Write(b)
		if len(b)%2 == 1 {
			body.WriteByte(0)
		}
	}
	chunk("fmt ", fmtc.Bytes())
	chunk("LIST", []byte("INFOINAM\x05\x00\x00\x00tone\x00\x00"))
	chunk("data", data.Bytes())
	var f bytes.Buffer
	f.WriteString("RIFF")
	binary.Write(&f, le, uint32(body.Len()))
	f.Write(body.Bytes())
	return f.Bytes()
}

func openWAVFile(t *testing.T, b []byte) stream {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tone.wav")
	os.WriteFile(path, b, 0o644)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	s, err := openStream(opened(f, int64(len(b))), 1000)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// A WAV plays, as the listener's folder and the cloud drive hold them
// (#54): it was taken for a format the player cannot decode, and skipped.
// Each kind of sample plays the tone at its pitch and loudness, a second
// of it, from the start and from half in.
func TestWAVDecodes(t *testing.T) {
	cases := map[string]struct {
		channels, bits    int
		float, extensible bool
	}{
		"16-bit":            {2, 16, false, false},
		"8-bit mono":        {1, 8, false, false},
		"24-bit extensible": {2, 24, false, true},
		"32-bit":            {2, 32, false, false},
		"float":             {2, 32, true, false},
		"double 6ch":        {6, 64, true, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := openWAVFile(t, wavFile(c.channels, c.bits, c.float, c.extensible, 48000))
			if s.SampleRate() != 48000 || s.Len() != 48000 {
				t.Fatalf("rate %d, length %d", s.SampleRate(), s.Len())
			}
			if _, bits := s.(formatOf).format(); bits != c.bits {
				t.Errorf("%d bits", bits)
			}
			frames, crossings, power := decodeTone(t, s)
			if frames != 48000 || crossings < 1990 || crossings > 2010 {
				t.Errorf("%d frames, %d crossings", frames, crossings)
			}
			// A sine of 0.5 has a power of 0.125.
			if math.Abs(power-0.125) > 0.005 {
				t.Errorf("power %v", power)
			}
			if err := s.SetPosition(24000); err != nil {
				t.Fatal(err)
			}
			if frames, crossings, _ := decodeTone(t, s); frames != 24000 || crossings < 990 || crossings > 1010 {
				t.Errorf("after seeking: %d frames, %d crossings", frames, crossings)
			}
		})
	}
}

// A WAV whose data chunk says more than the file holds, as a recording
// cut short or one written as it went (0xffffffff) does, plays to the
// file's end.
func TestWAVLongerThanItIs(t *testing.T) {
	b := wavFile(2, 16, false, false, 44100)
	at := bytes.Index(b, []byte("data"))
	binary.LittleEndian.PutUint32(b[at+4:], 0xffffffff)
	b = b[:len(b)-4*100]
	s := openWAVFile(t, b)
	if frames, _, _ := decodeTone(t, s); frames != 44100-100 {
		t.Errorf("%d frames", frames)
	}
}

// A WAV of compressed samples (ADPCM here) is one the player cannot
// decode: it is skipped, not fetched again and again.
func TestCompressedWAVSkipped(t *testing.T) {
	b := wavFile(2, 16, false, false, 8000)
	at := bytes.Index(b, []byte("fmt "))
	binary.LittleEndian.PutUint16(b[at+8:], 2)
	path := filepath.Join(t.TempDir(), "adpcm.wav")
	os.WriteFile(path, b, 0o644)
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = openStream(opened(f, int64(len(b))), 1000)
	var format *UnsupportedError
	if !errors.As(err, &format) || format.Format != "WAV" || !unplayable(err) {
		t.Fatalf("%v, want WAV skipped", err)
	}
}

// A WAV's second decoder, for loudness, leaves the one playing open.
func TestSharedWAVLeavesPlaybackOpen(t *testing.T) {
	checkSharedPlayback(t, wavFile(2, 16, false, false, 44100), 1000)
}

// A WAV of the listener's folder opens where it is, as lossless.
func TestOpenLocalWAV(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tone.WAV")
	os.WriteFile(path, wavFile(2, 24, false, false, 96000), 0o644)
	st, s, err := openLocal(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if st.Type != "wav" || st.Level != "lossless" || s.SampleRate() != 96000 {
		t.Fatalf("stream %+v, rate %d", st, s.SampleRate())
	}
}
