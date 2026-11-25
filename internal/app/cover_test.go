package app

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// coverApp is Now Playing showing the second of three songs.
func coverApp(t *testing.T) (*app, *ui.Tester, ui.Rect) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.pl.Cue([]netease.Track{{ID: 1, Name: "一"}, {ID: 2, Name: "二"}, {ID: 3, Name: "三"}})
	a.pl.Next()
	a.pl.SetLyrics(nil)
	a.nowPlaying = true
	tt := ui.NewTester(a.view, 860, 560)
	// Without the tilt and the swing, which the CPU draws slowly in depth:
	// the gestures do as they would.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	tt.Advance(500 * time.Millisecond)
	r, ok := tt.Find("专辑封面")
	if !ok {
		t.Fatal("no cover")
	}
	return a, tt, r
}

// drag drags from the middle of r by dx, over a few frames, and lets go
// after hold, still.
func drag(tt *ui.Tester, r ui.Rect, dx float32, hold time.Duration) {
	x, y := r.X+r.W/2, r.Y+r.H/2
	tt.Press(x, y)
	for i := 1; i <= 6; i++ {
		tt.Advance(16 * time.Millisecond)
		tt.Move(x+dx*float32(i)/6, y)
	}
	if hold > 0 {
		tt.Advance(hold)
	}
	tt.Release(x+dx, y)
	tt.Advance(time.Second)
	tt.Frame()
}

// Flung far to the left, the cover flies off for the next song; to the
// right, for the one before, however far this one has played.
func TestFlingCover(t *testing.T) {
	a, tt, r := coverApp(t)
	if a.pl.Current().ID != 2 {
		t.Fatalf("playing %d", a.pl.Current().ID)
	}
	drag(tt, r, -r.W*0.8, 0)
	if id := a.pl.Current().ID; id != 3 {
		t.Fatalf("flung left: playing %d", id)
	}
	if r2, ok := tt.Find("专辑封面"); !ok || r2.X < r.X-2 || r2.X > r.X+2 {
		t.Errorf("the next cover rests at %v, not %v", r2, r)
	}
	drag(tt, r, r.W*0.8, 0)
	if id := a.pl.Current().ID; id != 2 {
		t.Errorf("flung right: playing %d", id)
	}
}

// A drag too short, let go still, springs back, the song playing on.
func TestCoverSpringsBack(t *testing.T) {
	a, tt, r := coverApp(t)
	drag(tt, r, -r.W*0.3, 300*time.Millisecond)
	if id := a.pl.Current().ID; id != 2 {
		t.Fatalf("a short drag changed the song to %d", id)
	}
	if r2, _ := tt.Find("专辑封面"); r2.X < r.X-2 || r2.X > r.X+2 {
		t.Errorf("the cover rests at %v, not %v", r2, r)
	}
}

// Two fingers swiping across the trackpad fling it too.
func TestSwipeCover(t *testing.T) {
	a, tt, r := coverApp(t)
	x, y := r.X+r.W/2, r.Y+r.H/2
	for range 10 {
		tt.Scroll(x, y, r.W*0.06, 0)
		tt.Advance(16 * time.Millisecond)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if id := a.pl.Current().ID; id != 3 {
		t.Errorf("swiped left: playing %d", id)
	}
}

// Played from the menu or the keys, the next song's cover slides in from
// the right and the one before's from the left, each coming to rest in its
// place.
func TestCoverSlidesWithTheSong(t *testing.T) {
	a, tt, r := coverApp(t)
	tt.SetPreferences(ui.Preferences{})
	for _, step := range []struct {
		name string
		play func()
		id   int64
		dir  float32
	}{{"next", a.pl.Next, 3, 1}, {"prev", a.pl.Prev, 2, -1}} {
		step.play()
		tt.Frame()
		tt.Advance(60 * time.Millisecond)
		tt.Frame()
		if id := a.pl.Current().ID; id != step.id {
			t.Fatalf("%s: playing %d", step.name, id)
		}
		if r2, _ := tt.Find("专辑封面"); (r2.X-r.X)*step.dir < r.W*0.1 {
			t.Errorf("%s: the cover comes in at %v, from %v", step.name, r2.X, r.X)
		}
		tt.Advance(time.Second)
		tt.Frame()
		if r2, _ := tt.Find("专辑封面"); r2.X < r.X-2 || r2.X > r.X+2 {
			t.Errorf("%s: the cover rests at %v, not %v", step.name, r2, r)
		}
	}
}

// coverRegionDiff returns the mean per-pixel channel difference inside rect r
// (in DIPs) between two frames.
func coverRegionDiff(p, q *image.RGBA, r ui.Rect, scale float32) float64 {
	b := p.Bounds()
	x0 := max(b.Min.X, int(r.X*scale))
	y0 := max(b.Min.Y, int(r.Y*scale))
	x1 := min(b.Max.X, int((r.X+r.W)*scale))
	y1 := min(b.Max.Y, int((r.Y+r.H)*scale))
	var sum, n int64
	for y := y0; y < y1; y += 2 {
		for x := x0; x < x1; x += 2 {
			u, v := p.RGBAAt(x, y), q.RGBAAt(x, y)
			sum += int64(absInt(int(u.R)-int(v.R)) + absInt(int(u.G)-int(v.G)) + absInt(int(u.B)-int(v.B)))
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return float64(sum) / float64(n)
}

// Re-entering Now Playing with Ctrl+L while the pointer rests on the cover used
// to snap between the flat flying cover and the tilted resting cover on both
// exit and landing, without a transition animation.
func TestCoverTiltTransitionsOnNowPlayingToggle(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.BarSpin = false
	a.settings.Smoke = false
	a.settings.NpCalm = false
	url := "https://example.com/tilt-cover.jpg"
	markArt(a, url, 120)
	markArt(a, url, 600)
	a.art.add(netease.Sized(url, 600)+"#backdrop", a.art.peek(netease.Sized(url, 120)))
	tr := netease.Track{ID: 1, Name: "NIB", Duration: 180000}
	tr.Album.PicURL = url
	a.pl.Cue([]netease.Track{tr})
	a.nowPlaying = true

	tt := ui.NewTester(a.view, 1100, 760)
	settle := func() {
		for range 4 {
			tt.Frame()
			tt.Advance(time.Second)
		}
		tt.Frame()
	}
	settle()

	r, ok := tt.Find("专辑封面")
	if !ok {
		t.Fatal("no cover")
	}
	// Rest the pointer near a corner so the cover tilts in 3D perspective.
	mx, my := r.X+r.W*0.15, r.Y+r.H*0.15
	tt.Move(mx, my)
	settle()

	settledTilt := a.npSpot.tilt
	if math.Abs(float64(settledTilt.X)) < 2 || math.Abs(float64(settledTilt.Y)) < 2 {
		t.Fatalf("expected settled off-center tilt, got %+v", settledTilt)
	}
	pad := ui.Rect{X: r.X - 24, Y: r.Y - 24, W: r.W + 48, H: r.H + 48}
	scale := float32(tt.Image().Bounds().Dx()) / 1100

	// Ctrl+L #1: leave Now Playing with the pointer still on the cover.
	// The handover from the tilted cover to the flying cover must not snap flat.
	prev := grab(tt)
	var prevFlying bool
	var closeHandoverDiff float64
	a.nowPlaying = false
	for i := 0; i < 6; i++ {
		tt.Advance(motionFrame)
		tt.Frame()
		cur := grab(tt)
		flying := a.npFlying()
		if !prevFlying && flying {
			closeHandoverDiff = coverRegionDiff(prev, cur, pad, scale)
		}
		prevFlying = flying
		prev = cur
	}
	if closeHandoverDiff == 0 {
		t.Fatal("closing Now Playing never entered flight")
	}
	if closeHandoverDiff > 4 {
		t.Errorf("close handover snapped (cover diff %.2f on first closing flight frame)", closeHandoverDiff)
	}
	settle()

	// Ctrl+L #2: re-enter Now Playing with the pointer still resting on the cover.
	a.nowPlaying = true
	tt.Frame()
	prev = grab(tt)
	prevFlying = false
	var landDiff float64
	var tiltBeforeLand ui.Point
	var tiltSteps int
	var lastTilt ui.Point
	for i := 0; i < 80; i++ {
		tt.Advance(motionFrame)
		tt.Frame()
		cur := grab(tt)
		flying := a.npFlying()
		d := coverRegionDiff(prev, cur, pad, scale)
		if flying {
			tiltBeforeLand = a.npSpot.tilt
		}
		if prevFlying && !flying {
			landDiff = d
		}
		if !flying && i > 0 {
			if math.Abs(float64(a.npSpot.tilt.X-lastTilt.X))+math.Abs(float64(a.npSpot.tilt.Y-lastTilt.Y)) > 0.15 {
				tiltSteps++
			}
		}
		lastTilt = a.npSpot.tilt
		prevFlying = flying
		prev = cur
	}
	if math.Abs(float64(tiltBeforeLand.X))+math.Abs(float64(tiltBeforeLand.Y)) > 0.2 {
		t.Errorf("cover pre-tilted while hidden during opening flight: %+v", tiltBeforeLand)
	}
	if landDiff > 4 {
		t.Errorf("re-enter landing snapped (cover diff %.2f on landing frame)", landDiff)
	}
	if tiltSteps < 5 {
		t.Errorf("cover did not animate into perspective tilt after landing (only %d moving frames, final tilt %+v)", tiltSteps, a.npSpot.tilt)
	}
	if math.Abs(float64(a.npSpot.tilt.X-settledTilt.X)) > 0.3 || math.Abs(float64(a.npSpot.tilt.Y-settledTilt.Y)) > 0.3 {
		t.Errorf("cover did not reach settled tilt %+v, got %+v", settledTilt, a.npSpot.tilt)
	}
}
