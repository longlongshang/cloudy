package player

import (
	"os"
	"path/filepath"
	"testing"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// A song NetEase has no lyrics of reads them from the .lrc beside its file,
// not only when NetEase cannot be reached.
func TestLyricsFallBackToFileWhenNetEaseHasNone(t *testing.T) {
	old := lyricsOf
	defer func() { lyricsOf = old }()
	lyricsOf = func(int64) (netease.LyricTexts, error) { return netease.LyricTexts{}, nil }
	dir := t.TempDir()
	song := filepath.Join(dir, "a.mp3")
	os.WriteFile(song, []byte("x"), 0o644)
	os.WriteFile(filepath.Join(dir, "a.lrc"), []byte("[00:01.00]第一句\n[00:03.00]第二句\n"), 0o644)
	lines := fetchLyrics(netease.Track{ID: 5, LocalPath: song})
	if len(lines) != 2 || lines[0].Text != "第一句" {
		t.Fatalf("lines %+v", lines)
	}
}

// A cloud drive song with no lyrics on NetEase is looked for elsewhere; a
// catalog song with none is not.
func TestCloudSongWithoutLyricsLooksElsewhere(t *testing.T) {
	old := MoreWords
	defer func() { MoreWords = old }()
	MoreWords = func(netease.Track) []lyrics.Line { return nil }
	cloud := netease.Track{ID: 9, Privilege: &netease.Privilege{Cs: true}}
	if !wantsFoundLyrics(cloud, nil) {
		t.Error("cloud song not looked for")
	}
	if wantsFoundLyrics(netease.Track{ID: 9}, nil) {
		t.Error("catalog song looked for")
	}
	if wantsFoundLyrics(cloud, []lyrics.Line{{Text: "x"}}) {
		t.Error("looked for though it has lyrics")
	}
}
