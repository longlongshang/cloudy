package local

import (
	"testing"
	"time"
)

// An M4A's iTunes tags and length are read, as an MP3's are.
func TestReadMP4Tags(t *testing.T) {
	s, err := Read("testdata/tagged.m4a")
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "雨夜" || s.Artist != "Cloudy Test" || s.Album != "测试专辑" || s.Track != 3 {
		t.Fatalf("tags %+v", s)
	}
	if d := s.Duration; d < 2900*time.Millisecond || d > 3100*time.Millisecond {
		t.Fatalf("duration %v", d)
	}
}

// The formats the player plays are the ones the folders list.
func TestPlayable(t *testing.T) {
	for ext, want := range map[string]bool{".mp3": true, ".FLAC": true, ".m4a": true, ".aac": true, ".dsf": true, ".dff": true, ".ogg": false, ".wav": true, ".WAV": true, ".txt": false} {
		if Playable(ext) != want {
			t.Errorf("%s: %v", ext, !want)
		}
	}
}
