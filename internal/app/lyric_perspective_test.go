package app

import (
	"encoding/json"
	"fmt"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

func lyricFollowTester() (*app, *ui.Tester) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.NpCalm = false
	a.nowPlaying, a.npK, a.motion = true, 1, motionFull
	a.pl.Cue([]netease.Track{{ID: 1, Name: "lyrics", Duration: 400000}})
	var lines []lyrics.Line
	for i := range 30 {
		lines = append(lines, lyrics.Line{At: time.Duration(i) * 10 * time.Second, Text: fmt.Sprintf("Line %02d", i), Translation: "翻译"})
	}
	a.pl.SetLyrics(lines)
	a.pl.Seek(50 * time.Second)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() { a.lyricsPane(c, a.pl.Current(), 600) })
	}, 600, 600)
	tt.SetScale(0.3)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	return a, tt
}

// Wheel scrolling must resume following after 1.5 seconds even when the sung line has not changed.
func TestLyricManualScrollReturnsToSameLine(t *testing.T) {
	a, tt := lyricFollowTester()
	before := a.lyricScroll.Y
	tt.Wheel(300, 300, 0, 180)
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	if a.lyricScroll.Y <= before+20 || a.lyricGlide.took.IsZero() {
		t.Fatal("wheel did not take over the lyrics")
	}
	held := a.lyricScroll.Y
	tt.Advance(time.Second)
	tt.Frame()
	if abs32(a.lyricScroll.Y-held) > 1 {
		t.Fatal("lyrics followed before manual reading ended")
	}
	tt.Advance(600 * time.Millisecond)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	if abs32(a.lyricScroll.Y-before) > 2 {
		t.Fatalf("same line did not return after reading: got %v, want %v", a.lyricScroll.Y, before)
	}
}

// Clicking bilingual lyrics honors their time offset, and changing tracks discards a manual scroll hold.
func TestLyricFollowSeekAndTrackReset(t *testing.T) {
	a, tt := lyricFollowTester()
	a.settings.LyricShift = map[int64]int{1: 250}
	tt.Frame()
	if err := tt.Click("Line 06"); err != nil {
		t.Fatal(err)
	}
	if got := a.pl.Position(); got != 60*time.Second-250*time.Millisecond {
		t.Fatalf("lyric click sought to %v", got)
	}
	tt.Wheel(300, 300, 0, 120)
	tt.Frame()
	a.pl.Cue([]netease.Track{{ID: 2, Name: "next", Duration: 100000}})
	a.pl.SetLyrics(lyrics.Build("[00:00]Next track\n[00:10]Next line", "", ""))
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	if !a.lyricGlide.took.IsZero() || a.lyricTrack != 2 {
		t.Fatal("track change kept the old manual reading state")
	}
}

// Dragging lyrics scrolls the list without seeking, and a held pointer keeps manual reading active.
func TestLyricDragTakesOver(t *testing.T) {
	a, tt := lyricFollowTester()
	r, ok := tt.Find("Line 05")
	if !ok {
		t.Fatal("active lyric is missing")
	}
	before := a.lyricScroll.Y
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Move(x, y-70)
	tt.Frame()
	if a.lyricScroll.Y <= before+20 || a.pl.Position() != 50*time.Second {
		t.Fatal("drag did not scroll independently of seeking")
	}
	tt.Advance(2 * time.Second)
	tt.Frame()
	if a.lyricGlide.took.IsZero() {
		t.Fatal("held drag stopped manual reading")
	}
	tt.Release(x, y-70)
	tt.Advance(1600 * time.Millisecond)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	if abs32(a.lyricScroll.Y-before) > 2 {
		t.Fatal("lyrics did not follow after releasing the drag")
	}
}

// A wheel interrupting an automatic glide must start at the visible row, not at its hidden target.
func TestLyricWheelCatchesMovingLine(t *testing.T) {
	a, tt := lyricFollowTester()
	a.pl.Seek(60 * time.Second)
	tt.Frame()
	tt.Advance(120 * time.Millisecond)
	tt.Frame()
	before, _ := tt.Find("Line 06")
	tt.Wheel(300, 300, 0, 48)
	tt.Frame()
	after, _ := tt.Find("Line 06")
	if moved := before.Y - after.Y; abs32(moved-48) > 3 {
		t.Fatalf("interrupting the spring moved the row %v DIPs, want 48", moved)
	}
}

// Interrupting a stagger after rapid seeks must carry every row from where it shows, including delayed neighbors.
func TestLyricWheelCatchesStaggeredRows(t *testing.T) {
	a, tt := lyricFollowTester()
	a.pl.Seek(60 * time.Second)
	tt.Frame()
	tt.Advance(80 * time.Millisecond)
	tt.Frame()
	a.pl.Seek(70 * time.Second)
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	tt.Frame()
	before := map[string]ui.Rect{}
	for _, label := range []string{"Line 05", "Line 06", "Line 07", "Line 08"} {
		before[label], _ = tt.Find(label)
	}
	tt.Wheel(300, 300, 0, 48)
	tt.Frame()
	for label, r := range before {
		after, _ := tt.Find(label)
		if moved := r.Y - after.Y; abs32(moved-48) > 3 {
			t.Errorf("interrupting stagger moved %s by %v DIPs, want 48", label, moved)
		}
	}
}

// A drag during a stagger catches all rows before painting and must not apply the old parent lag again.
func TestLyricDragCatchesMovingRows(t *testing.T) {
	a, tt := lyricFollowTester()
	a.pl.Seek(60 * time.Second)
	tt.Frame()
	tt.Advance(120 * time.Millisecond)
	tt.Frame()
	before := map[string]ui.Rect{}
	for _, label := range []string{"Line 05", "Line 06", "Line 07"} {
		before[label], _ = tt.Find(label)
	}
	r := before["Line 06"]
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	tt.Move(x, y-48)
	tt.Frame()
	for label, r := range before {
		after, _ := tt.Find(label)
		if moved := r.Y - after.Y; abs32(moved-48) > 3 {
			t.Errorf("dragging the spring moved %s by %v DIPs, want 48", label, moved)
		}
	}
	tt.Release(x, y-48)
	if a.pl.Position() != 60*time.Second {
		t.Fatal("dragging a moving lyric also sought")
	}
}

// NM_FRAMES previews focus, stagger, bilingual wrapping, native word highlighting and manual reading using synthetic lyrics.
func TestLyricFollowFrames(t *testing.T) {
	dir := os.Getenv("NM_FRAMES")
	if dir == "" {
		t.Skip("set NM_FRAMES to a directory to save frames")
	}
	a := newTestApp()
	a.settings = defaultSettings()
	a.nowPlaying, a.npK, a.motion = true, 1, motionFull
	a.pl.Cue([]netease.Track{{ID: 1, Name: "Synthetic lyrics", Duration: 200000}})
	texts := []string{"The city lights are fading", "在晚风中轻轻唱起", "Every word finds its way home", "让时间停在这一刻", "And every little moment carries us along the winding road", "把所有温柔留在今晚", "We keep on singing softly", "直到星光落入眼底"}
	var lines []lyrics.Line
	for i, text := range texts {
		at := time.Duration(i) * 8 * time.Second
		lines = append(lines, lyrics.Line{At: at, End: at + 6*time.Second, Text: text, Translation: "那些微小的瞬间，沿着蜿蜒的路陪伴我们。", Timed: true,
			Words: lyrics.SpreadWords(text, at, at+6*time.Second)})
	}
	a.pl.SetLyrics(lines)
	a.pl.Seek(35 * time.Second)
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(24).Background(ui.Hex("#182235")).Children(func() {
			a.lyricsPane(c, a.pl.Current(), 760)
		})
	}, 620, 760)
	tt.SetDark(true)
	save := func(name string) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	for range 3 {
		tt.Frame()
		tt.Advance(2 * time.Second)
	}
	save("lyrics-focus-native-words")
	a.pl.Seek(41 * time.Second)
	tt.Frame()
	save("lyrics-newly-active")
	for _, ms := range []int{35, 120, 240, 600} {
		tt.Advance(time.Duration(ms) * time.Millisecond)
		tt.Frame()
		save(fmt.Sprintf("lyrics-wave-%03d", ms))
	}
	tt.Wheel(300, 400, 0, -120)
	tt.Advance(500 * time.Millisecond)
	tt.Frame()
	save("lyrics-manual")
	tt.Advance(2 * time.Second)
	tt.Frame()
	tt.Advance(2 * time.Second)
	tt.Frame()
	save("lyrics-resumed")
}

// The arc keeps the sung line in the upper focus band and tilted rows still seek after reflow.
func TestLyricPerspectiveSeeksAfterReflow(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.LyricPerspective = true
	// Left still it would calm, and a click would only wake it.
	a.settings.NpCalm = false
	a.nowPlaying = true
	a.pl.Cue([]netease.Track{{ID: 1, Name: "x", Duration: 60000}})
	a.pl.SetLyrics(lyrics.Build("[00:00]第一句歌词\n[00:05]第二句歌词\n[00:10]最后一句歌词", "[00:10]翻译", ""))
	tt := ui.NewTester(a.view, 1100, 760)
	tt.SetScale(0.25) // Geometry and input stay in DIPs; render fewer pixels.
	settle := func() {
		tt.Frame()
		tt.Advance(2 * time.Second)
		tt.Frame()
	}
	checkFocus := func(text string) {
		t.Helper()
		settle()
		line, ok := tt.Find(text)
		// Use the unprojected viewport: perspective expands its outer
		// bounding box, while its focus band stays fixed on screen.
		focus := a.lyricGlide.focus
		if !ok || abs32(line.Y+line.H/2-(focus.top+focus.height*0.38)) > 3 {
			t.Fatalf("%q did not settle in the lyric band: %+v", text, line)
		}
	}
	seek := func(text string, want time.Duration) {
		t.Helper()
		settle()
		if err := tt.Click(text); err != nil {
			t.Fatal(err)
		}
		if pos := a.pl.Position(); pos != want {
			t.Fatalf("clicking %q sought to %v, want %v", text, pos, want)
		}
	}
	checkFocus("第一句歌词")
	seek("最后一句歌词", 10*time.Second)
	tt.SetSize(1300, 860)
	checkFocus("最后一句歌词")
	seek("第二句歌词", 5*time.Second)
	a.settings.LyricScale = 1.5
	a.settings.LyricTrans = false
	seek("最后一句歌词", 10*time.Second)
	a.settings.LyricTilt = 0
	settle()
	checkFocus("最后一句歌词")
	a.settings.LyricTilt = 0.8
	checkFocus("最后一句歌词")
}

// Old settings keep the original arc, and setting either slider to zero
// must survive reopening the app.
func TestLyricPerspectiveSettingsKeepZero(t *testing.T) {
	s := defaultSettings()
	if err := json.Unmarshal([]byte(`{"lyricPerspective":true}`), &s); err != nil {
		t.Fatal(err)
	}
	if s.LyricTilt != 0.5 {
		t.Fatalf("the old arc loaded at %v tilt, want 0.5", s.LyricTilt)
	}
	s.LyricTilt, s.LyricBounce = 0, 0
	s.path = filepath.Join(t.TempDir(), "settings.json")
	if err := s.save(); err != nil {
		t.Fatal(err)
	}
	loaded := loadSettingsFrom(s.path)
	if loaded.LyricTilt != 0 || loaded.LyricBounce != 0 {
		t.Fatalf("zero settings reloaded as tilt %v, bounce %v", loaded.LyricTilt, loaded.LyricBounce)
	}
}

// The spring can overshoot the sung line, while zero strength approaches
// the line from the same side throughout the animation.
func TestLyricGlideBounce(t *testing.T) {
	overshoot := func(strength float32) float64 {
		var glide lyricGlide
		var scroll ui.ScrollState
		tt := ui.NewTester(func(c *ui.Context) {
			if glide.start.IsZero() {
				glide.to(c, &scroll, 100, strength)
			}
		}, 100, 100)
		tt.Frame()
		var peak float64
		for i := 0; i <= 100; i++ {
			lag, _ := glide.at(glide.start.Add(time.Duration(i) * 10 * time.Millisecond))
			peak = max(peak, lag)
		}
		return peak
	}
	if peak := overshoot(0); peak != 0 {
		t.Errorf("the smooth scroll overshot by %v DIPs", peak)
	}
	mid, full := overshoot(0.5), overshoot(1)
	if mid <= 1 || full <= mid || full > 20 {
		t.Errorf("unexpected overshoot at 50%% and 100%%: %v, %v DIPs", mid, full)
	}
}

// The wheel over the middle values makes fine adjustments, kept once it
// rests, while clicking them still resets what was set either way.
func TestLyricToolsWheelValues(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.path = filepath.Join(t.TempDir(), "settings.json")
	a.npPointer = true
	const id = 1
	tt := ui.NewTester(func(c *ui.Context) {
		a.lyricTools(c, id, ui.Rect{X: 20, Y: 60, W: 430, H: 32})
	}, 470, 160)
	tt.Frame()
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	wheel := func(label string, notches int) {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q button", label)
		}
		before := loadSettingsFrom(a.settings.path)
		for range max(notches, -notches) {
			dy := float32(-100)
			if notches < 0 {
				dy = 100
			}
			tt.Wheel(r.X+r.W/2, r.Y+r.H/2, 0, dy)
		}
		tt.Frame()
		during := loadSettingsFrom(a.settings.path)
		if during.LyricScale != before.LyricScale || during.LyricShift[id] != before.LyricShift[id] {
			t.Fatal("the wheel saved settings before it rested")
		}
		a.lyricWheelAt = time.Now().Add(-lyricWheelRest)
		tt.Frame()
	}
	click("歌词字号增大")
	wheel("恢复歌词字号", -3)
	if got := a.lyricScale(); math.Abs(float64(got)-1.07) > 0.00001 {
		t.Errorf("the wheel set scale to %v, want 107%%", got)
	}
	click("恢复歌词字号")
	if got := a.lyricScale(); got != 1 {
		t.Errorf("click reset scale to %v, want 100%%", got)
	}
	click("歌词提前 0.1 秒")
	wheel("恢复歌词时间", 3)
	if got := a.lyricShift(id); got != 130*time.Millisecond {
		t.Errorf("the wheel shifted lyrics by %v, want 130 ms", got)
	}
	if got := loadSettingsFrom(a.settings.path).LyricShift[id]; got != 130 {
		t.Errorf("saved shift is %d ms, want 130 ms", got)
	}
	click("恢复歌词时间")
	if got := a.lyricShift(id); got != 0 {
		t.Errorf("click reset shift to %v, want 0", got)
	}
}
