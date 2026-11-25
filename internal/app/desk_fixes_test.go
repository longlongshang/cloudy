package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// deskTestApp is an app playing a song whose first line is text, translated.
func deskTestApp(text string) *app {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 7, Name: "x", Duration: 200000}})
	a.pl.SetLyrics([]lyrics.Line{{Text: text, Translation: "The brightest star in the night sky"}, {At: 100 * time.Second, Text: "b"}})
	return a
}

// A line too long for the desktop lyrics shows smaller, or as small as it
// goes and sliding as it is sung: it was cut to its start and an ellipsis,
// then went on two lines.
func TestDeskLongLineFits(t *testing.T) {
	long := strings.Repeat("我曾经跨过山和大海也穿过人山人海", 2)
	longer := strings.Repeat("我曾经跨过山和大海也穿过人山人海", 4)
	for _, text := range []string{long, longer} {
		a := deskTestApp(text)
		main, _ := a.deskLines()
		if main.size >= 32 {
			t.Errorf("%d runes: still at %v", len([]rune(text)), main.size)
		}
		w, _ := a.deskShape(main, deskLook{}, 0, false).Size()
		if !main.scroll && w > deskRoom {
			t.Errorf("%d runes: on one line %v wide, in %v, not sliding", len([]rune(text)), w, deskRoom)
		}
		if main.scroll && main.size < 32*deskScrollShrink-0.01 {
			t.Errorf("%d runes: sliding at %v, smaller than it needs", len([]rune(text)), main.size)
		}
	}
	if main, _ := deskTestApp("夜空中最亮的星").deskLines(); main.size != 32 || main.scroll {
		t.Errorf("a short line changed: %+v", main)
	}
}

// Whatever color the app is, the translation reads against what it lies on
// or is edged with: in a white or a black one, it went into its edge or its
// capsule.
func TestDeskTranslationReadsInAnyAccent(t *testing.T) {
	for _, accent := range []string{"#ffffff", "#000000", "#fff6a0", "#e8423f"} {
		a := newTestApp()
		a.settings = defaultSettings()
		a.pal.accent = ui.Hex(accent)
		for _, st := range deskStyles {
			look := a.deskLookOf(st.id)
			ground := look.pill
			if look.edge.A > 0 {
				ground = look.edge
			}
			if ground.A == 0 {
				continue // on whatever is behind
			}
			for name, c := range map[string]ui.Color{"translation": look.second, "line": look.sung} {
				if d := luminance(c.Over(ui.RGB(128, 128, 128))) - luminance(ground.Over(ui.RGB(128, 128, 128))); d > -0.3 && d < 0.3 {
					t.Errorf("%s in %s: the %s is %v on %v", accent, st.id, name, c, ground)
				}
			}
		}
	}
}

// The desktop lyrics ask to be drawn again when what they show changes, not
// every 40ms: between words, as the next one starts; a line at once, as the
// next line comes.
func TestDeskDrawsWhenLyricsChange(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	lines := []lyrics.Line{
		{Text: "ab", Words: []lyrics.Word{{Text: "a", Start: 0, End: time.Second}, {Text: "b", Start: 3 * time.Second, End: 4 * time.Second}}},
		{At: 10 * time.Second, Text: "c"},
	}
	if got := a.deskNextChange(lines, 500*time.Millisecond); got > 50*time.Millisecond {
		t.Errorf("while a word lights: %v", got)
	}
	if got := a.deskNextChange(lines, 2*time.Second); got < 900*time.Millisecond || got > 1100*time.Millisecond {
		t.Errorf("a second before the next word: %v", got)
	}
	if got := a.deskNextChange(lines, 9500*time.Millisecond); got < 500*time.Millisecond || got > 600*time.Millisecond {
		t.Errorf("half a second before the next line: %v", got)
	}
	a.settings.DeskWords = false
	if got := a.deskNextChange(lines, 500*time.Millisecond); got < time.Second-time.Millisecond {
		t.Errorf("lit whole, while a word sings: %v", got)
	}
}

// With the pointer over locked lyrics, a button over them unlocks them: there
// was no way back but the settings, the tray or a shortcut.
func TestDeskUnlocksFromItsButton(t *testing.T) {
	a := deskTestApp("夜空中最亮的星")
	a.settings.DeskLyrics, a.settings.DeskLock = true, true
	tt := ui.NewTester(a.deskView, deskW, deskH)
	tt.Frame()
	if _, ok := tt.Find("解锁桌面歌词"); ok {
		t.Fatal("the unlock button shows with the pointer away")
	}
	a.desk.near = true
	tt.Frame()
	if err := tt.Click("解锁桌面歌词"); err != nil {
		t.Fatal(err)
	}
	if a.settings.DeskLock {
		t.Fatal("clicked the unlock button: still locked")
	}
}
