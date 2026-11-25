package app

import "github.com/egoist/mygo"

// The floating windows' spots, kept on a screen that is there (#29): a
// spot left on a monitor since unplugged, or one the framework scaled off
// the screens with mixed scales before 1.3.4, came back where no one could
// reach the window again.

// fitIn returns (x, y) moved so that a w×h window lies within one of
// areas: the one holding its middle, or else the one it overlaps the most.
// ok is false when it is on none of them.
func fitIn(areas []mygo.Rectangle, x, y, w, h int) (int, int, bool) {
	best, most := -1, 0
	cx, cy := x+w/2, y+h/2
	for i, r := range areas {
		if cx >= r.X && cy >= r.Y && cx < r.X+r.Width && cy < r.Y+r.Height {
			best = i
			break
		}
		ow := min(x+w, r.X+r.Width) - max(x, r.X)
		oh := min(y+h, r.Y+r.Height) - max(y, r.Y)
		if ow > 0 && oh > 0 && ow*oh > most {
			best, most = i, ow*oh
		}
	}
	if best < 0 {
		return x, y, false
	}
	r := areas[best]
	return clampInt(x, r.X, r.X+r.Width-w), clampInt(y, r.Y, r.Y+r.Height-h), true
}

// clampInt keeps v within lo and hi, at lo if the range is empty.
func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

// workAreas returns the work areas of the displays, the primary first, or
// none before the app runs (tests), which asks the main thread for them.
func workAreas() []mygo.Rectangle {
	if !mygo.App.IsReady() {
		return nil
	}
	var out []mygo.Rectangle
	for _, d := range mygo.Screen.Displays() {
		out = append(out, d.WorkArea)
	}
	return out
}

// screenAreas returns the displays whole, the primary first, taskbars and
// all: the desktop lyrics stay where they were left, over the taskbar or
// just above it, rather than pushed up off it.
func screenAreas() []mygo.Rectangle {
	if !mygo.App.IsReady() {
		return nil
	}
	var out []mygo.Rectangle
	for _, d := range mygo.Screen.Displays() {
		out = append(out, d.Bounds)
	}
	return out
}

// mainWorkArea returns the work area of the display the main window is
// most on, or the primary's.
func (a *app) mainWorkArea() mygo.Rectangle {
	if a.win != nil && a.win.IsVisible() {
		return mygo.Screen.DisplayMatching(a.win.Bounds()).WorkArea
	}
	return mygo.Screen.PrimaryDisplay().WorkArea
}

// refitFloats brings the desktop lyrics and the mini window back on a
// screen, as displays come and go or change scale.
func (a *app) refitFloats() {
	s := &a.settings
	areas := workAreas()
	if len(areas) == 0 {
		return
	}
	refit := func(win *mygo.Window, keepX, keepY *int) {
		if win == nil {
			return
		}
		b := win.Bounds()
		x, y, ok := fitIn(areas, b.X, b.Y, b.Width, b.Height)
		if !ok {
			wa := areas[0]
			x, y = wa.X+(wa.Width-b.Width)/2, wa.Y+wa.Height-b.Height-40
		}
		if x != b.X || y != b.Y {
			win.SetPosition(x, y)
			*keepX, *keepY = x, y
		}
	}
	// The lyrics go back to where they were left, brought onto a screen:
	// a game that changes the screen's resolution as it starts moves
	// them, and they come back as it ends, where they were.
	if a.deskWin != nil {
		a.placeDesk(a.deskSpot(s.DeskX, s.DeskY))
	}
	refit(a.miniWin, &s.MiniX, &s.MiniY)
}

// resetFloatSpots forgets where the desktop lyrics and the mini window were
// left, and puts those showing back in their first places.
func (a *app) resetFloatSpots() {
	s := &a.settings
	s.DeskX, s.DeskY, s.MiniX, s.MiniY = 0, 0, 0, 0
	if a.deskWin != nil {
		a.placeDesk(a.deskSpot(0, 0))
	}
	if a.miniWin != nil {
		a.miniWin.SetPosition(a.miniSpot(0, 0))
	}
	a.changed()
}
