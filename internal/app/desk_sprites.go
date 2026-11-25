package app

import (
	"math"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The characters of the desktop lyrics flying one by one, each drawn on its
// own: standing lyrics out of their button and back into it, and the line
// turning between lying and standing, each character on a curve of its own
// from where it lay to where it stands, or back.

// deskTurnFly is how long a character takes from lying to standing.
const deskTurnFly = 620 * time.Millisecond

// deskStood is a piece of the standing line sung, where it was laid out.
type deskStood struct {
	piece deskPiece
	size  float32
	at    ui.Rect // in the window
}

// A deskSprite is a character of the line sung, on its own: what it is,
// where its middle is on the screen, how large, turned how far, and in
// what color.
type deskSprite struct {
	text  string
	at    int // its first character's, in the line
	x, y  float32
	size  float32
	turn  float32 // degrees: 90 for one lying in a standing line
	color ui.Color
	// lh is the line height it is laid out at, as its window lays it
	// out: deskStep standing, the font's own (0) lying.
	lh float32
}

// deskNowSprites returns the characters of the line sung as the lyrics'
// window laid them out last, on the screen; none before it has.
func (a *app) deskNowSprites() []deskSprite {
	if a.deskWin == nil {
		return nil
	}
	b := a.deskWin.Bounds()
	main, _ := a.deskLines()
	if a.settings.DeskVertical {
		return a.deskStandingSprites(main, b)
	}
	if a.deskLineAt.W == 0 || main.scroll {
		return nil // sliding, it turns at once
	}
	return a.deskLyingSprites(main, a.deskLineAt, b)
}

// deskLyingSprites returns the characters of m lying at line in the window
// at win, as deskLine lays them out.
func (a *app) deskLyingSprites(m deskMain, line ui.Rect, win mygo.Rectangle) []deskSprite {
	look := a.deskLookOf(a.settings.DeskStyle)
	pos := a.lyricPos()
	_, colors := a.deskChars(m, look, pos)
	ink := ui.ShapeText(m.shaped(), ui.Font{Family: a.deskFamily(), Size: m.size, Weight: 800}, 0)
	units := utf16.Encode([]rune(m.shaped()))
	var out []deskSprite
	r := 0 // the rune at units[at]
	for at := 0; at < len(units); {
		next := ink.NextBoundary(at)
		if next <= at {
			break
		}
		text := string(utf16.Decode(units[at:next]))
		c0, c1 := ink.Caret(at), ink.Caret(next)
		x1 := c1.X
		if c1.Y != c0.Y || x1 <= c0.X {
			x1 = c0.X + m.size*0.6 // the last of a line wrapped
		}
		if strings.TrimSpace(text) != "" && c0.X < deskRoom {
			color := ui.Color{}
			if r < len(colors) {
				color = colors[r]
			}
			out = append(out, deskSprite{text: text, at: r, size: m.size, color: color,
				x: float32(win.X) + line.X + (c0.X+x1)/2, y: float32(win.Y) + line.Y + c0.Y + c0.H/2})
		}
		r += len([]rune(text))
		at = next
	}
	return out
}

// deskStandingSprites returns the characters of m standing in the window at
// win, as deskColumn laid them out last (deskStood).
func (a *app) deskStandingSprites(m deskMain, win mygo.Rectangle) []deskSprite {
	look := a.deskLookOf(a.settings.DeskStyle)
	_, colors := a.deskChars(m, look, a.lyricPos())
	colorAt := func(i int) ui.Color {
		if i >= 0 && i < len(colors) {
			return colors[i]
		}
		return look.sung
	}
	var out []deskSprite
	for _, st := range a.deskStood {
		cx := float32(win.X) + st.at.X + st.at.W/2
		cy := float32(win.Y) + st.at.Y + st.at.H/2
		p := st.piece
		if !p.sideways {
			out = append(out, deskSprite{text: p.text, at: p.at, x: cx, y: cy, size: st.size, color: colorAt(p.at), lh: deskStep})
			continue
		}
		// A run lying on its side: each character of it down the column,
		// turned a quarter right, as the run is turned about its middle.
		ink := ui.ShapeText(p.text, ui.Font{Family: a.deskFamily(), Size: st.size, Weight: 800}, 0)
		w, _ := ink.Size()
		units := utf16.Encode([]rune(p.text))
		r := 0
		for at := 0; at < len(units); {
			next := ink.NextBoundary(at)
			if next <= at {
				break
			}
			text := string(utf16.Decode(units[at:next]))
			x0, x1 := ink.Caret(at).X, ink.Caret(next).X
			if x1 <= x0 {
				x1 = x0 + st.size*0.5
			}
			if strings.TrimSpace(text) != "" {
				out = append(out, deskSprite{text: text, at: p.at + r, x: cx, y: cy - w/2 + (x0+x1)/2,
					size: st.size, turn: 90, color: colorAt(p.at + r), lh: deskStep})
			}
			r += len([]rune(text))
			at = next
		}
	}
	return out
}

// paintDeskSprite paints sp with its middle at (x, y), scaled and turned so,
// in color, edged as its style has it.
func (a *app) paintDeskSprite(p *ui.Painter, sp deskSprite, x, y, scale, turn float32, color ui.Color, look deskLook) {
	if color.A == 0 {
		return
	}
	ink := ui.ShapeText(sp.text, ui.Font{Family: a.deskFamily(), Size: sp.size, Weight: 800, LineHeight: sp.lh}, 0)
	w, h := ink.Size()
	lx, ly := x-w/2, y-h/2
	p.Transform(ui.Scaling(scale, scale).Then(ui.Rotation(turn)), x, y, func() {
		if look.edge.A > 0 {
			width := max(1.1, sp.size*0.055)
			edge := look.edge.Alpha(float32(color.A) / 255 * float32(look.edge.A) / 255)
			for i := range 8 {
				t := float64(i) * math.Pi / 4
				p.TextLayout(ink, lx+width*float32(math.Cos(t)), ly+width*float32(math.Sin(t)), edge)
			}
		}
		p.TextLayout(ink, lx, ly, color)
	})
}

// deskFlySprites returns where the characters of a flight of sprites go
// from and to, on the screen: from the button or to it if either is none.
// ok is false while the lyrics' window, just made, has yet to lay them out.
func (a *app) deskFlySprites(f *deskFlight) (from, to []deskSprite, ok bool) {
	switch {
	case f.turning && f.turnPending:
		return f.src, nil, false
	case f.turning:
		to = a.deskNowSprites()
		return f.src, to, len(to) > 0
	case f.opening:
		to = a.deskNowSprites()
		return nil, to, len(to) > 0
	}
	return f.src, nil, true
}

// deskSpriteTiming returns how long after the one before each character of
// a flight of sprites sets off, how long one takes, and all of them.
func deskSpriteTiming(f *deskFlight, n int) (lag, fly, total time.Duration) {
	fly = deskGlyphFly
	if f.turning {
		fly = deskTurnFly
	}
	if n > 1 {
		lag = min(deskGlyphLag, deskGlyphLags/time.Duration(n-1))
	}
	return lag, fly, lag*time.Duration(max(n-1, 0)) + fly
}

// paintDeskSprites paints the characters of a flight of sprites gone into
// their way.
func (a *app) paintDeskSprites(p *ui.Painter, f *deskFlight, gone time.Duration) {
	look := a.deskLookOf(a.settings.DeskStyle)
	from, to, ok := a.deskFlySprites(f)
	ox, oy := f.origin.X, f.origin.Y
	if !ok {
		// Waiting for where they go: still where they were.
		for _, s := range from {
			a.paintDeskSprite(p, s, s.x-ox, s.y-oy, 1, s.turn, s.color, look)
		}
		return
	}
	bx, by := f.from.X+f.from.W/2, f.from.Y+f.from.H/2
	wobble := func(i int) float32 {
		t := float32(6 + (i*7)%9)
		if i%2 == 1 {
			t = -t
		}
		return t
	}
	switch {
	case f.turning:
		lag, fly, _ := deskSpriteTiming(f, len(from))
		byAt := make(map[int]deskSprite, len(to))
		for _, s := range to {
			byAt[s.at] = s
		}
		for i, s0 := range from {
			u := clamp01(float32(gone-time.Duration(i)*lag) / float32(fly))
			s1, found := byAt[s0.at]
			if !found {
				// Nowhere to go in the other: it fades where it was.
				a.paintDeskSprite(p, s0, s0.x-ox, s0.y-oy, 1-0.3*u, s0.turn, s0.color.Alpha(1-u), look)
				continue
			}
			e := easeRange(u, 0, 1)
			// Its own curve: bowed to one side of its way or the other,
			// by more or less of how far it goes.
			x0, y0, x1, y1 := s0.x-ox, s0.y-oy, s1.x-ox, s1.y-oy
			bend := 0.16 + 0.2*float32((i*5)%7)/6
			if i%2 == 1 {
				bend = -bend
			}
			cx, cy := (x0+x1)/2-(y1-y0)*bend, (y0+y1)/2+(x1-x0)*bend
			x := (1-e)*(1-e)*x0 + 2*(1-e)*e*cx + e*e*x1
			y := (1-e)*(1-e)*y0 + 2*(1-e)*e*cy + e*e*y1
			swell := 1 + 0.12*float32(math.Sin(math.Pi*float64(u)))
			scale := lerp(1, s1.size/s0.size, e) * swell
			turn := lerp(s0.turn, s1.turn, easeRange(u, 0.1, 0.9)) + wobble(i)*float32(math.Sin(math.Pi*float64(u)))
			// Laid out as where it lands once past the middle of its way,
			// where it goes too fast for the change to show.
			sp := s0
			if e > 0.5 {
				sp.lh = s1.lh
			}
			a.paintDeskSprite(p, sp, x, y, scale, turn, s0.color.Mix(s1.color, e), look)
		}
	case f.opening:
		lag, fly, _ := deskSpriteTiming(f, len(to))
		for i, s := range to {
			u := clamp01(float32(gone-time.Duration(i)*lag) / float32(fly))
			a.paintDeskSpriteOut(p, s, u, wobble(i), bx, by, ox, oy, look)
		}
	default:
		lag, fly, _ := deskSpriteTiming(f, len(from))
		for i, s := range from {
			// Back, the last one out goes first.
			order := len(from) - 1 - i
			u := 1 - clamp01(float32(gone-time.Duration(order)*lag)/float32(fly))
			a.paintDeskSpriteOut(p, s, u, wobble(i), bx, by, ox, oy, look)
		}
	}
}

// paintDeskSpriteOut paints s k of its way out of the button at (bx, by)
// to its place, as paintDeskGlyphs has a character of a lying line go.
func (a *app) paintDeskSpriteOut(p *ui.Painter, s deskSprite, k, wobble, bx, by, ox, oy float32, look deskLook) {
	cx, cy := s.x-ox, s.y-oy
	if k >= 1 {
		a.paintDeskSprite(p, s, cx, cy, 1, s.turn, s.color, look)
		return
	}
	travel := easeRange(k, 0, 1)
	nx, ny := lerp(bx, cx, travel), lerp(by, cy, 1-(1-travel)*(1-travel))
	scale := lerp(0.32, 1, easeRange(k, 0, 0.78)) + 0.08*float32(math.Sin(math.Pi*float64(clamp01((k-0.78)/0.22))))
	turn := s.turn + wobble*(1-easeRange(k, 0.3, 1))
	a.paintDeskSprite(p, s, nx, ny, scale, turn, s.color, look)
}

// startDeskTurn turns the line from lying to standing, as v says, or back:
// src is where its characters are in the window at old, made again at nb
// once the flight shows (finishDeskTurn). It reports whether it could.
func (a *app) startDeskTurn(src []deskSprite, old, nb mygo.Rectangle, v bool) bool {
	if a.motion != motionFull || a.tray == nil || a.deskWin == nil || a.deskFly.win != nil || len(src) == 0 {
		return false
	}
	const room = 60
	left := min(old.X, nb.X) - room
	top := min(old.Y, nb.Y) - room
	right := max(old.X+old.Width, nb.X+nb.Width) + room
	bottom := max(old.Y+old.Height, nb.Y+nb.Height) + room
	f := &a.deskFly
	*f = deskFlight{opening: true, sprites: true, turning: true, src: src,
		origin:      ui.Point{X: float32(left), Y: float32(top)},
		turnPending: true, turnTo: v, turnAt: nb}
	f.win = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Hidden: true, Width: right - left, Height: bottom - top, Content: ui.View(a.deskFlyView)})
	showAtOnce(f.win.NativeHandle())
	f.win.SetPosition(left, top)
	f.win.SetAlwaysOnTop(true)
	f.win.Show()
	f.win.SetIgnoreMouseEvents(true)
	return true
}

// startDeskStandFly sends standing lyrics out of their button at btn, on the
// screen, to their window at desk, or back, character by character.
func (a *app) startDeskStandFly(opening bool, desk mygo.Rectangle, btn ui.Rect, size float32) bool {
	var src []deskSprite
	if !opening {
		if src = a.deskNowSprites(); len(src) == 0 {
			return false
		}
	}
	left := int(min(btn.X, float32(desk.X)) - size)
	top := int(min(btn.Y, float32(desk.Y)) - size)
	right := int(max(btn.X+btn.W, float32(desk.X+desk.Width)) + size)
	bottom := int(max(btn.Y+btn.H, float32(desk.Y+desk.Height)) + size)
	f := &a.deskFly
	*f = deskFlight{opening: opening, sprites: true, src: src,
		origin: ui.Point{X: float32(left), Y: float32(top)},
		from:   ui.Rect{X: btn.X - float32(left), Y: btn.Y - float32(top), W: btn.W, H: btn.H},
		desk:   ui.Point{X: float32(desk.X - left), Y: float32(desk.Y - top)}}
	f.win = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Hidden: true, Width: right - left, Height: bottom - top, Content: ui.View(a.deskFlyView)})
	showAtOnce(f.win.NativeHandle())
	f.win.SetPosition(left, top)
	f.win.SetAlwaysOnTop(true)
	f.win.Show()
	f.win.SetIgnoreMouseEvents(true)
	return true
}

// finishDeskTurn makes the lyrics' window again, standing or lying, once
// the characters turning show over it: it comes in as they land.
func (a *app) finishDeskTurn() {
	f := &a.deskFly
	if f.win == nil {
		return
	}
	f.coming = true
	a.turnDesk(f.turnTo, f.turnAt)
	f.coming = false
	if a.deskWin != nil {
		a.deskWin.SetOpacity(0)
	}
}
