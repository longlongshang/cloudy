package app

import (
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// deskState is what the desktop lyrics follow outside their window: the
// pointer over them while they are locked, which lets nothing else through
// to them, and whether a game or a video is full screen in front.
type deskState struct {
	stop      chan struct{} // stops watchDesk
	placed    mygo.Point    // where placeDesk last put the window
	placedSet bool          // and the window hasn't been moved since
	near      bool          // the pointer is over the locked lyrics
	hot       bool          // over their unlock button, which takes it
	unlockAt  ui.Rect       // that button, in the window, while it shows
	covered   bool          // something full screen is in front
	checked   time.Time     // when that was last asked
	idle      bool          // nothing plays, and they hide then
	idleSince time.Time     // when nothing has played since, or zero
	vertical  bool          // the window is made standing up
	// moreAt is the tools' more button, in the window; moreWin the panel
	// it opens, and moreShut when that last closed.
	moreAt   ui.Rect
	bodyAt   ui.Rect // the lyrics, lying or standing, in the window
	moreWin  *mygo.Window
	moreShut time.Time
	more     deskMore
}

// watchDesk follows, while the desktop lyrics are open, the pointer over
// them and what is full screen, from a clock off the main thread: the
// window, locked, hears nothing of the pointer, and drawing it on a clock
// would take frames from a game.
func (a *app) watchDesk() {
	a.stopDeskWatch()
	stop := make(chan struct{})
	a.desk.stop = stop
	go func() {
		t := time.NewTicker(120 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				mygo.RunOnMain(a.deskTick)
			}
		}
	}()
}

// stopDeskWatch stops watchDesk, as the lyrics close.
func (a *app) stopDeskWatch() {
	if a.desk.stop != nil {
		close(a.desk.stop)
		a.desk.stop = nil
	}
	a.desk.near, a.desk.hot, a.desk.covered, a.desk.idle = false, false, false, false
	a.desk.idleSince = time.Time{}
}

// deskTick is a tick of watchDesk, on the main thread.
func (a *app) deskTick() {
	w := a.deskWin
	if w == nil || a.desk.stop == nil {
		return
	}
	s := &a.settings
	now := time.Now()
	// Nothing playing a moment, not between two songs, they go.
	switch {
	case !s.DeskHideStopped || a.pl.Playing():
		a.desk.idleSince = time.Time{}
	case a.desk.idleSince.IsZero():
		a.desk.idleSince = now
	}
	if idle := !a.desk.idleSince.IsZero() && now.Sub(a.desk.idleSince) >= 1500*time.Millisecond; idle != a.desk.idle {
		a.desk.idle = idle
		a.applyDeskLyrics()
	}
	if now.Sub(a.desk.checked) >= time.Second {
		a.desk.checked = now
		if covered := !s.DeskOverFullscreen && fullscreenInFront(); covered != a.desk.covered {
			a.desk.covered = covered
			a.applyDeskLyrics()
		}
	}
	if !s.DeskLock || !w.IsVisible() {
		a.desk.near = false
		return
	}
	p := mygo.Screen.CursorScreenPoint()
	b := w.Bounds()
	in := func(x, y, w, h float32) bool {
		px, py := float32(p.X), float32(p.Y)
		return px >= x && py >= y && px < x+w && py < y+h
	}
	near := in(float32(b.X), float32(b.Y), float32(b.Width), float32(b.Height))
	u := a.desk.unlockAt
	hot := near && u.W > 0 && in(float32(b.X)+u.X, float32(b.Y)+u.Y, u.W, u.H)
	if hot != a.desk.hot {
		// The window takes the pointer over the button alone, for it to
		// be clicked; everywhere else it goes through to what is under.
		a.desk.hot = hot
		w.SetIgnoreMouseEvents(!hot)
	}
	if near != a.desk.near {
		a.desk.near = near
		w.Invalidate()
	}
}
