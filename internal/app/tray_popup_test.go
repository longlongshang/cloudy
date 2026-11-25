package app

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/audio"
	"github.com/egoist/mygo/ui"
	"netease-music-mygo/internal/netease"
)

type testTrayPopup struct {
	visible, destroyed bool
	invalidations      int
	pending            []func()
	close              func(*mygo.CloseEvent)
}

func (w *testTrayPopup) Show()            { w.visible = true }
func (w *testTrayPopup) Hide()            { w.visible = false }
func (w *testTrayPopup) Destroy()         { w.destroyed = true }
func (w *testTrayPopup) Invalidate()      { w.invalidations++ }
func (w *testTrayPopup) Update(fn func()) { w.pending = append(w.pending, fn) }
func (w *testTrayPopup) flush() {
	for _, fn := range w.pending {
		fn()
	}
	w.pending = nil
}

func trayPopupFixture(t *testing.T) (*app, *ui.Tester, *[]*testTrayPopup) {
	t.Helper()
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.path = filepath.Join(t.TempDir(), "settings.json")
	a.settings.DeskX, a.settings.DeskY = 1, 1
	a.tray = &mygo.Tray{}
	all := []*testTrayPopup{}
	realFactory, realGlass, realHost := newTrayPopupWindow, trayGlass, trayHost
	trayGlass = func(*app) bool { return false }
	trayHost = func() (uintptr, bool) { return 0, false }
	newTrayPopupWindow = func(opts mygo.FlyoutOptions, close func(*mygo.CloseEvent)) trayPopupWindow {
		if opts.Tray != a.tray || opts.Parent != nil || !opts.Focusable || !opts.Hidden || opts.Shadow || opts.Content == nil {
			t.Fatal("menu must be an independent focusable tray flyout with its own content")
		}
		w := &testTrayPopup{close: close}
		all = append(all, w)
		return w
	}
	t.Cleanup(func() { newTrayPopupWindow, trayGlass, trayHost = realFactory, realGlass, realHost })
	tt := ui.NewTester(func(c *ui.Context) {
		a.pal = paletteFor(c.Theme())
		if a.trayPopup != nil {
			a.trayPopupView(c)
		}
	}, trayPopupWidth, trayPopupHeight)
	return a, tt, &all
}

// Tray clicks must close before dispatching playback, queue, like and lyrics actions.
func TestTrayPopupCommands(t *testing.T) {
	a, tt, windows := trayPopupFixture(t)
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船"}})
	a.profile = &netease.Profile{UserID: 1}
	out := audio.NewPlayer(&silence{})
	out.SetVolume(0)
	out.Play()
	a.pl.Attach(out)
	t.Cleanup(func() { out.Close() })
	click := func(label string) {
		t.Helper()
		a.toggleTrayPopup()
		tt.Frame()
		w := (*windows)[len(*windows)-1]
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
		tt.Frame()
		if a.trayPopup != nil || w.visible || w.destroyed || len(w.pending) != 1 {
			t.Fatal("click must hide now and defer destruction/action until after the frame")
		}
		w.flush()
		if !w.destroyed {
			t.Fatal("closed menu retained its renderer")
		}
	}
	click("暂停")
	if a.pl.Playing() {
		t.Fatal("tray did not pause")
	}
	a.toggleTrayPopup()
	tt.Frame()
	if !tt.Focused("打开Cloudy") {
		t.Fatal("menu has no initial keyboard focus")
	}
	tt.Key(0, ui.KeyDown)
	if !tt.Focused("播放") {
		t.Fatal("Down did not reach play")
	}
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	(*windows)[len(*windows)-1].flush()
	if !a.pl.Playing() || a.trayPopup != nil {
		t.Fatal("Space did not resume and close the menu")
	}

	realLike := likeSong
	requests := make(chan bool, 2)
	likeSong = func(id int64, like bool) error {
		if id != 7 {
			t.Errorf("wrong liked song %d", id)
		}
		requests <- like
		return nil
	}
	defer func() { likeSong = realLike }()
	click("喜欢")
	flushTestLike(a, 7)
	if !<-requests || !a.isLiked(a.pl.Current()) {
		t.Fatal("tray did not like the song")
	}
	a.likeAnswered(a.accountGen, 7, true, nil)
	click("取消喜欢")
	flushTestLike(a, 7)
	if <-requests || a.isLiked(a.pl.Current()) {
		t.Fatal("tray did not unlike the song")
	}
	a.likeAnswered(a.accountGen, 7, false, nil)
	before := a.settings.DeskLyrics
	click("桌面歌词")
	if a.settings.DeskLyrics == before {
		t.Fatal("tray did not toggle desktop lyrics")
	}
	click("锁定桌面歌词")
	if !a.settings.DeskLock {
		t.Fatal("tray did not lock desktop lyrics")
	}
	click("解锁桌面歌词")
	if a.settings.DeskLock {
		t.Fatal("tray did not unlock desktop lyrics")
	}

	// Missing local fixtures keep queue dispatch offline; the current track
	// changes synchronously before any decoder result is posted.
	missing := filepath.Join(t.TempDir(), "absent.mp3")
	a.pl.Cue([]netease.Track{{ID: 1, Name: "first", LocalPath: missing}, {ID: 2, Name: "second", LocalPath: missing}})
	click("下一首")
	if a.pl.Current().ID != 2 {
		t.Fatal("tray did not advance the queue")
	}
	click("上一首")
	if a.pl.Current().ID != 1 {
		t.Fatal("tray did not go back")
	}

	if dir := os.Getenv("TRAY_FRAMES"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		a.toggleTrayPopup()
		for _, dark := range []bool{false, true} {
			tt.SetDark(dark)
			name := "tray-light-100.png"
			if dark {
				tt.SetScale(1.25)
				name = "tray-dark-125.png"
			}
			tt.Frame()
			f, err := os.Create(filepath.Join(dir, name))
			if err != nil {
				t.Fatal(err)
			}
			err = png.Encode(f, tt.Image())
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
		}
		a.closeTrayPopup()
		(*windows)[len(*windows)-1].flush()
	}
}

// Repeated right-click, outside dismissal and Escape must release only their own popup.
func TestTrayPopupLifecycle(t *testing.T) {
	a, tt, windows := trayPopupFixture(t)
	if a.win != nil {
		t.Fatal("fixture must exercise a menu without a main window")
	}
	a.toggleTrayPopup()
	first := (*windows)[0]
	if !first.visible {
		t.Fatal("right click did not show the menu independently")
	}
	a.redraw()
	if first.invalidations == 0 {
		t.Fatal("open menu did not follow state redraws")
	}
	a.toggleTrayPopup()
	if a.trayPopup != nil || first.visible {
		t.Fatal("second right click left the menu open")
	}
	first.flush()
	a.toggleTrayPopup()
	second := (*windows)[1]
	first.close(&mygo.CloseEvent{})
	if a.trayPopup != second {
		t.Fatal("stale dismissal closed a newer menu")
	}
	second.close(&mygo.CloseEvent{}) // The native focusable-flyout dismissal callback.
	if a.trayPopup != nil || second.visible {
		t.Fatal("outside dismissal left a zombie menu")
	}
	second.flush()
	a.toggleTrayPopup()
	tt.Frame()
	third := (*windows)[2]
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.trayPopup != nil || third.visible {
		t.Fatal("Escape did not hide the menu")
	}
	third.flush()
	if !first.destroyed || !second.destroyed || !third.destroyed {
		t.Fatal("popup windows leaked")
	}
	a.toggleTrayPopup()
	tt.Frame()
	tt.Key(0, ui.KeyEnter)
	tt.Frame() // Default "open Cloudy" is safe with no main window.
	fourth := (*windows)[3]
	fourth.flush()
	if a.trayPopup != nil || !fourth.destroyed {
		t.Fatal("Enter did not dispatch and close")
	}
	a.quitting = true
	a.toggleTrayPopup()
	if a.trayPopup != nil || len(*windows) != 4 {
		t.Fatal("quitting opened a new popup")
	}
}

// With 界面玻璃 the menu is the app's glass over a copy of the desktop under
// it; from the overflow of hidden icons it does not take the keyboard, which
// would fold the overflow away from it.
func TestTrayPopupGlass(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.tray = &mygo.Tray{}
	var got mygo.FlyoutOptions
	realFactory, realGlass, realHost, realCapture := newTrayPopupWindow, trayGlass, trayHost, trayCaptureBehind
	t.Cleanup(func() {
		newTrayPopupWindow, trayGlass, trayHost, trayCaptureBehind = realFactory, realGlass, realHost, realCapture
	})
	trayGlass = func(*app) bool { return true }
	trayHost = func() (uintptr, bool) { return 1, true }
	trayCaptureBehind = func() func(uintptr) image.Image {
		return func(uintptr) image.Image { return image.NewRGBA(image.Rect(0, 0, trayPopupWidth, trayPopupHeight)) }
	}
	newTrayPopupWindow = func(opts mygo.FlyoutOptions, close func(*mygo.CloseEvent)) trayPopupWindow {
		got = opts
		return &testTrayPopup{close: close}
	}
	a.toggleTrayPopup()
	if !a.trayPopupGlass || got.Focusable || got.Shadow || got.Vibrancy != "" || a.trayBackdrop == nil {
		t.Fatalf("overflow glass tray menu options: %+v", got)
	}
	if got.Gap != trayPopupGap-trayPopupMargin {
		t.Fatalf("card is %d DIPs from the icon", got.Gap+trayPopupMargin)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		a.pal = paletteFor(c.Theme())
		if a.trayPopup != nil {
			a.trayPopupView(c)
		}
	}, trayPopupWidth, trayPopupHeight)
	tt.Frame()
	if _, ok := tt.Find("下一首"); !ok {
		t.Fatal("glass menu lost its commands")
	}
	a.closeTrayPopup()
	if a.trayBackdrop != nil {
		t.Fatal("closed menu kept its copy of the desktop")
	}
	a.toggleTrayPopup()
	if a.trayPopup == nil {
		t.Fatal("a right click after closing did not reopen the menu")
	}
	a.closeTrayPopup()
	a.trayPopupClosed = time.Now() // as a press outside, on the icon, closes it
	a.toggleTrayPopup()
	if a.trayPopup != nil {
		t.Fatal("a right click right after a dismissal reopened the menu")
	}
}

// The card's shadow stays inside the window's transparent margin: past it,
// the window's edge cut it off square (#22).
func TestTrayShadowWithinMargin(t *testing.T) {
	if reach := trayShadowY + trayShadowBlur + trayShadowSpread; reach > trayPopupMargin {
		t.Fatalf("the shadow reaches %d DIPs below the card, past the %d of margin", reach, trayPopupMargin)
	}
	if side := trayShadowBlur + trayShadowSpread; side > trayPopupMargin {
		t.Fatalf("the shadow reaches %d DIPs beside the card, past the %d of margin", side, trayPopupMargin)
	}
}

// The tray menu opens beside the right-click point, flipping horizontally or
// vertically near monitor edges while keeping the full 24 DIP shadow inside mon.
func TestTrayPopupPos(t *testing.T) {
	mon := mygo.Rectangle{X: 0, Y: 0, Width: 2560, Height: 1440}
	win := mygo.Rectangle{Width: trayPopupWidth, Height: trayPopupHeight}
	cardW, cardH := trayPopupWidth-2*trayPopupMargin, trayPopupHeight-2*trayPopupMargin

	// Top taskbar click away from right edge: card left aligns to click.X,
	// outer window stays inside mon.Y so the top shadow is not clipped.
	pos := trayPopupPos(mygo.Point{X: 2208, Y: 15}, win, mon)
	if pos.X+trayPopupMargin != 2208 || pos.Y != mon.Y {
		t.Fatalf("top taskbar pos = %+v, want cardLeft=2208 winY=%d", pos, mon.Y)
	}

	// Overflow icon click below top bar: opens right below click.Y without
	// being pushed down by WorkArea or icon-cell margins.
	pos = trayPopupPos(mygo.Point{X: 2100, Y: 62}, win, mon)
	if pos.X+trayPopupMargin != 2100 || pos.Y+trayPopupMargin != 62+trayClickGap {
		t.Fatalf("overflow click pos = %+v, want card=(2100,%d)", pos, 62+trayClickGap)
	}

	// Click near right edge: flips leftward so card right aligns to click.X.
	pos = trayPopupPos(mygo.Point{X: 2420, Y: 62}, win, mon)
	if cardRight := pos.X + trayPopupMargin + cardW; cardRight != 2420 || pos.X+trayPopupWidth > mon.Width {
		t.Fatalf("right-edge pos = %+v (cardRight=%d), want cardRight=2420 inside mon", pos, cardRight)
	}

	// Bottom-right taskbar click: flips upward and leftward.
	mon1080 := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}
	pos = trayPopupPos(mygo.Point{X: 1780, Y: 1056}, win, mon1080)
	if cardRight, cardBottom := pos.X+trayPopupMargin+cardW, pos.Y+trayPopupMargin+cardH; cardRight != 1780 || cardBottom != 1056-trayClickGap {
		t.Fatalf("bottom-right pos = %+v (card max %d,%d), want (1780,%d)", pos, cardRight, cardBottom, 1056-trayClickGap)
	}

	// Negative-coordinate secondary display: flips up and left inside negMon.
	negMon := mygo.Rectangle{X: -1920, Y: -1080, Width: 1920, Height: 1080}
	pos = trayPopupPos(mygo.Point{X: -100, Y: -24}, win, negMon)
	if pos.X < negMon.X || pos.X+trayPopupWidth > negMon.X+negMon.Width || pos.Y < negMon.Y || pos.Y+trayPopupHeight > negMon.Y+negMon.Height {
		t.Fatalf("negative monitor pos = %+v escaped %+v", pos, negMon)
	}
	if cardRight, cardBottom := pos.X+trayPopupMargin+cardW, pos.Y+trayPopupMargin+cardH; cardRight != -100 || cardBottom != -24-trayClickGap {
		t.Fatalf("negative monitor card max = (%d,%d), want (-100,%d)", cardRight, cardBottom, -24-trayClickGap)
	}

	// Extreme corners: outer 24 DIP shadow window never leaves monitor bounds.
	tl := trayPopupPos(mygo.Point{X: 5, Y: 5}, win, mon)
	br := trayPopupPos(mygo.Point{X: 2555, Y: 1435}, win, mon)
	if tl.X != 0 || tl.Y != 0 || br.X+trayPopupWidth != mon.Width || br.Y+trayPopupHeight != mon.Height {
		t.Fatalf("corner clamp tl=%+v br=%+v outside %+v", tl, br, mon)
	}
}

// The tray popup animates its enter on both first open and reopen, settles to
// zero pending frames after motionPane, and skips animation when motion is off.
func TestTrayPopupEnterMotion(t *testing.T) {
	a, tt, windows := trayPopupFixture(t)
	openAndSettle := func(name string) {
		t.Helper()
		a.toggleTrayPopup()
		tt.Frame()
		if !tt.FramePending() {
			t.Fatalf("%s: enter animation did not start on frame 1", name)
		}
		tt.Advance(motionPane.Duration + 20*time.Millisecond)
		tt.Frame()
		if tt.FramePending() {
			t.Fatalf("%s: enter animation kept requesting frames after %v", name, motionPane.Duration)
		}
	}
	openAndSettle("first full open")
	a.closeTrayPopup()
	(*windows)[len(*windows)-1].flush()

	openAndSettle("reopen full")
	a.closeTrayPopup()
	(*windows)[len(*windows)-1].flush()

	a.settings.Motion = "lite"
	openAndSettle("lite open")
	a.closeTrayPopup()
	(*windows)[len(*windows)-1].flush()

	a.settings.Motion = "off"
	a.toggleTrayPopup()
	tt.Frame()
	if tt.FramePending() {
		t.Fatal("motion off must not schedule enter frames")
	}
	a.closeTrayPopup()
	(*windows)[len(*windows)-1].flush()

	// Transform origin follows the click point relative to the placed card.
	mon := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}
	win := mygo.Rectangle{Width: trayPopupWidth, Height: trayPopupHeight}
	cases := []struct {
		click mygo.Point
		want  ui.Point
	}{
		{mygo.Point{X: 400, Y: 60}, ui.Point{X: 0, Y: 0}},    // opens down-right
		{mygo.Point{X: 1850, Y: 60}, ui.Point{X: 1, Y: 0}},   // flips down-left
		{mygo.Point{X: 1850, Y: 1050}, ui.Point{X: 1, Y: 1}}, // flips up-left
		{mygo.Point{X: 100, Y: 1050}, ui.Point{X: 0, Y: 1}},  // flips up-right
	}
	for _, tc := range cases {
		pos := trayPopupPos(tc.click, win, mon)
		if got := trayPopupOrigin(tc.click, pos); got != tc.want {
			t.Fatalf("click %+v (pos %+v): origin = %+v, want %+v", tc.click, pos, got, tc.want)
		}
	}
}
