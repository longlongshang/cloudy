package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// The comments drawer narrows the lyrics beside the cover and widens them
// again: the lines wrap anew, and the regular (flat) lyrics kept the offset
// of the old wrapping, showing lines not sung yet until the next line came
// (#56). The line sung stays in sight through both.
func TestLyricsFollowTheDrawer(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.LyricPerspective = false
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一", Duration: 400000}})
	var lines []lyrics.Line
	for i := 0; i < 40; i++ {
		lines = append(lines, lyrics.Line{At: time.Duration(i) * 8 * time.Second,
			Text: fmt.Sprintf("第 %d 句 这是一句很长很长的歌词，在窄的时候会换成两行甚至三行来显示", i)})
	}
	a.pl.SetLyrics(lines)
	a.nowPlaying = true
	old := npCursor
	npCursor = func() mygo.Point { return mygo.Point{} }
	defer func() { npCursor = old }()
	tt := ui.NewTester(a.view, 1400, 860)
	settle := func() {
		for range 8 {
			tt.Advance(300 * time.Millisecond)
			tt.Frame()
		}
	}
	tt.Frame()
	a.pl.Seek(20*8*time.Second + time.Second)
	settle()
	inSight := func(when string) {
		t.Helper()
		r, ok := tt.Find("第 20 句 这是一句很长很长的歌词，在窄的时候会换成两行甚至三行来显示")
		if !ok {
			t.Fatalf("%s: the line sung is not built", when)
		}
		if r.Y < 0 || r.Y+r.H > 860 {
			t.Errorf("%s: the line sung is out of sight at %v", when, r)
		}
	}
	inSight("before")
	a.npComments = true
	settle()
	inSight("drawer out")
	a.npComments = false
	settle()
	inSight("drawer back")
}
