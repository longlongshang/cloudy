package local

import (
	"bytes"
	"encoding/binary"
	"path/filepath"
	"testing"
	"time"
)

// wavChunk is a RIFF chunk: its id, size and body, padded to even.
func wavChunk(id string, b []byte) []byte {
	out := append([]byte(id), binary.LittleEndian.AppendUint32(nil, uint32(len(b)))...)
	out = append(out, b...)
	if len(b)%2 == 1 {
		out = append(out, 0)
	}
	return out
}

// wavOf is a WAV of two seconds of 16-bit stereo silence at 44.1 kHz, with
// chunks before its data.
func wavOf(chunks ...[]byte) []byte {
	var fmtc bytes.Buffer
	for _, v := range []any{uint16(1), uint16(2), uint32(44100), uint32(44100 * 4), uint16(4), uint16(16)} {
		binary.Write(&fmtc, binary.LittleEndian, v)
	}
	body := append([]byte("WAVE"), wavChunk("fmt ", fmtc.Bytes())...)
	for _, c := range chunks {
		body = append(body, c...)
	}
	body = append(body, wavChunk("data", make([]byte, 2*44100*4))...)
	return append(append([]byte("RIFF"), binary.LittleEndian.AppendUint32(nil, uint32(len(body)))...), body...)
}

// A WAV is listed among the folder's songs, its length read from its
// chunks and its tags from its INFO list, which Chinese Windows writes in
// GBK, or from an ID3 tag in its "id3 " chunk.
func TestReadWAV(t *testing.T) {
	dir := t.TempDir()
	// "雨夜" and "测试" in GBK.
	info := append([]byte("INFO"), wavChunk("INAM", []byte("\xd3\xea\xd2\xb9\x00"))...)
	info = append(info, wavChunk("IART", []byte("Cloudy Test\x00"))...)
	info = append(info, wavChunk("IPRD", []byte("\xb2\xe2\xca\xd4\x00"))...)
	info = append(info, wavChunk("ITRK", []byte("3\x00"))...)
	writeFile(t, filepath.Join(dir, "info.wav"), wavOf(wavChunk("LIST", info)))
	id3 := id3Tag(id3Frame("TIT2", append([]byte{3}, "晴天"...)), id3Frame("TPE1", append([]byte{3}, "周杰伦"...)))
	writeFile(t, filepath.Join(dir, "id3.WAV"), wavOf(wavChunk("id3 ", id3)))
	writeFile(t, filepath.Join(dir, "歌手 - 无标签.wav"), wavOf())
	songs, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 3 {
		t.Fatalf("%d songs", len(songs))
	}
	want := map[string]Song{
		"id3.WAV":      {Title: "晴天", Artist: "周杰伦"},
		"info.wav":     {Title: "雨夜", Artist: "Cloudy Test", Album: "测试", Track: 3},
		"歌手 - 无标签.wav": {Title: "无标签", Artist: "歌手"},
	}
	for _, s := range songs {
		w := want[filepath.Base(s.Path)]
		if s.Title != w.Title || s.Artist != w.Artist || s.Album != w.Album || s.Track != w.Track {
			t.Errorf("%s: %+v", filepath.Base(s.Path), s)
		}
		if s.Duration != 2*time.Second {
			t.Errorf("%s: duration %v", filepath.Base(s.Path), s.Duration)
		}
	}
}
