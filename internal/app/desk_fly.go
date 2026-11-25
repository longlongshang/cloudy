package app

import (
	"math"
	"strings"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
)

// How the desktop lyrics fly out of their button, character by character,
// and back into it.
const (
	deskGlyphFly  = 380 * time.Millisecond // a character's way
	deskGlyphLag  = 22 * time.Millisecond  // between one setting off and the next
	deskGlyphLags = 240 * time.Millisecond // between the first and the last, at most
	deskNextFade  = 260 * time.Millisecond // the line under them, once they are down
	deskFlyStep   = 34 * time.Millisecond  // the most a frame moves them on
)

// A deskFlight is the desktop lyrics on their way out of the player bar's
// button, or back into it: each character of the line playing leaves the
// button in turn, arcs over to where it shows in the lyrics' window and
// lands there, the line made up again as that window shows it; or, back,
// each leaves its place in turn for the button. Down, they are the line
// the lyrics' window shows, to the pixel, which takes over from them; the
// line under it fades in after. They are drawn on a clear window as large
// as their way.
type deskFlight struct {
	win     *mygo.Window
	coming  bool // its window is being made, to show once they fly
	opening bool
	from    ui.Rect  // the button, in that window
	desk    ui.Point // the lyrics' window's corner, in that window
	line    deskMain // the line that flies, as it was when it set off
	at      ui.Rect  // where the line lies in the lyrics' window
	gone    time.Duration
	last    time.Time // the frame before
	frames  int       // shown so far
	// landed is when they came down, and the lyrics' window showed the
	// line again; zero before.
	landed time.Time
	ending int // frames since
	// sprites flies the characters each drawn on its own, as standing
	// lyrics are (desk_sprites.go): turning, between lying and standing,
	// from src, where they were; going back into the button, from src too.
	// origin is the flight's window's corner on the screen.
	sprites, turning bool
	src              []deskSprite
	origin           ui.Point
	waited           int // frames waiting for where they go
	// turnPending says the window is yet to be made again, standing as
	// turnTo says, at turnAt, once the flight shows.
	turnPending bool
	turnTo      bool
	turnAt      mygo.Rectangle
}

// deskMain is the line the desktop lyrics show large: its text, its words
// if it is sung word by word, and its size.
type deskMain struct {
	text  string
	words []lyrics.Word
	size  float32
	// scroll says it is too long for its room even smaller: it slides
	// along as it is sung, from at to end (deskScrollLine).
	scroll  bool
	at, end time.Duration
	// reading is how the line reads (音译), over it lying, to its right
	// standing.
	reading string
}

const (
	// deskRoom is how wide a line of the desktop lyrics may be: the window,
	// less its sides, a capsule's and an edge's.
	deskRoom = deskW - 48 - 56 - 8
	// deskShrink is how much smaller than its size the line under the one
	// sung goes, at most, to fit; longer, it ends in an ellipsis.
	deskShrink = 0.72
	// deskScrollShrink is how much smaller than its size the line sung
	// goes, at most, to fit; longer, it slides along as it is sung.
	deskScrollShrink = 0.85
)

// deskLines returns the line the desktop lyrics show large, and the one
// under it, if any: what follows it, or says it in Chinese.
func (a *app) deskLines() (main deskMain, next string) {
	scale := float32(1)
	if a.settings.DeskScale > 0 {
		scale = float32(a.settings.DeskScale)
	}
	tr := a.pl.Current()
	if tr == nil {
		return deskMain{text: mygo.App.Name(), size: 30 * scale}, ""
	}
	lines, _ := a.shownLyrics()
	cur := lyrics.Index(lines, a.lyricPos())
	if len(lines) == 0 || cur < 0 {
		return deskMain{text: tr.Name, size: 30 * scale}, ""
	}
	l := &lines[cur]
	s := &a.settings
	main = deskMain{text: l.Text, words: l.Words, size: 32 * scale, at: l.At, end: l.End}
	var subs []string
	if s.DeskTrans && l.Translation != "" {
		subs = append(subs, l.Translation)
	}
	if s.DeskRoman && l.Roman != "" {
		main.reading = l.Roman
	}
	if len(subs) == 0 && s.DeskNext && cur+1 < len(lines) {
		subs = append(subs, lines[cur+1].Text)
	}
	if len(subs) > 0 {
		next = subs[0]
	}
	if s.DeskVertical {
		return main, next // standing, deskColumns fits them
	}
	main.size, main.scroll = a.deskFit(main)
	return main, next
}

// deskReadingSize is the size of the line's reading, of its size as
// designed.
const deskReadingSize = 16

// deskFit returns the size the line m shows at, and whether it slides: as
// large as it is if it fits, smaller, down to deskScrollShrink of that, if
// that has it fit, and else that small, sliding along as it is sung. On two
// lines, a long line was hard to follow; cut short, only its start showed.
func (a *app) deskFit(m deskMain) (float32, bool) {
	w, _ := a.deskShape(m, deskLook{}, 0, false).Size()
	if w <= deskRoom {
		return m.size, false
	}
	if fit := m.size * deskRoom / w * 0.98; fit >= m.size*deskScrollShrink {
		return fit, false
	}
	return m.size * deskScrollShrink, true
}

// deskNextSize returns the size the line under the one sung shows at: as
// large as it is, or smaller, down to deskShrink of that, to fit. Longer
// still, it ends in an ellipsis.
func (a *app) deskNextSize(text string, size float32) float32 {
	if text == "" {
		return size
	}
	w, _ := ui.ShapeText(text, ui.Font{Family: a.deskFamily(), Size: size, Weight: 700}, 0).Size()
	if w <= deskRoom {
		return size
	}
	return max(size*deskRoom/w*0.98, size*deskShrink)
}

// deskFlyShown says how much of the lyrics' window's text shows while they
// fly: none of the line while it is on its way out or back, which the
// flight draws, and the line under it fading in once they are down.
func (a *app) deskFlyShown(now time.Time) (line, next float32) {
	f := &a.deskFly
	switch {
	case f.turnPending:
		return 1, 1 // the window still shows them, the flight not yet
	case f.win != nil && f.opening && f.landed.IsZero():
		return 0, 0
	case f.win != nil && !f.opening && f.frames > 1:
		return 0, 0
	case f.opening && !f.landed.IsZero():
		return 1, easeRange(float32(now.Sub(f.landed))/float32(deskNextFade), 0, 1)
	}
	return 1, 1
}

// deskFlyLags returns how long after the one before each of n characters
// sets off, and how long they all take.
func deskFlyLags(n int) (lag, total time.Duration) {
	if n > 1 {
		lag = min(deskGlyphLag, deskGlyphLags/time.Duration(n-1))
	}
	return lag, lag*time.Duration(max(n-1, 0)) + deskGlyphFly
}

// startDeskFly sends the lyrics out of their button to their window at its
// bounds, or back, and reports whether it could: otherwise they come or go
// at once.
func (a *app) startDeskFly(opening bool, desk mygo.Rectangle) bool {
	if a.motion != motionFull || a.tray == nil || a.win == nil || !a.win.IsVisible() || a.win.IsMinimized() ||
		a.deskBtn.W == 0 || a.deskFly.win != nil || a.morph.win != nil {
		return false
	}
	main, _ := a.deskLines()
	if strings.TrimSpace(main.text) == "" || main.scroll && !a.settings.DeskVertical {
		// Sliding, it shows at once, not character by character.
		return false
	}
	cb := a.win.ContentBounds()
	btn := ui.Rect{X: float32(cb.X) + a.deskBtn.X, Y: float32(cb.Y) + a.deskBtn.Y, W: a.deskBtn.W, H: a.deskBtn.H}
	if a.settings.DeskVertical {
		return a.startDeskStandFly(opening, desk, btn, main.size)
	}
	// Where the line lies: as the lyrics' window laid it out last, going
	// back; coming out, reckoned as it will, until it does.
	at := a.deskLineAt
	if opening || at.W == 0 {
		ink := a.deskShape(main, a.deskLookOf(a.settings.DeskStyle), 0, false)
		w, h := ink.Size()
		w = min(w, deskRoom)
		pad := float32(0)
		if a.deskLookOf(a.settings.DeskStyle).pill.A > 0 {
			pad = 10
		}
		at = ui.Rect{X: (deskW - w) / 2, Y: deskToolsRoom + pad, W: w, H: h}
	}
	line := ui.Rect{X: float32(desk.X) + at.X, Y: float32(desk.Y) + at.Y, W: at.W, H: at.H}
	// Room for a character turned or swelling where it lands.
	room := main.size
	left := int(min(btn.X, line.X) - room)
	top := int(min(btn.Y, line.Y) - room)
	right := int(max(btn.X+btn.W, line.X+line.W) + room)
	bottom := int(max(btn.Y+btn.H, line.Y+line.H) + room)
	f := &a.deskFly
	*f = deskFlight{opening: opening, line: main, at: at,
		from: ui.Rect{X: btn.X - float32(left), Y: btn.Y - float32(top), W: btn.W, H: btn.H},
		desk: ui.Point{X: float32(desk.X - left), Y: float32(desk.Y - top)}}
	f.win = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Hidden: true, Width: right - left, Height: bottom - top, Content: ui.View(a.deskFlyView)})
	showAtOnce(f.win.NativeHandle())
	f.win.SetPosition(left, top)
	f.win.SetAlwaysOnTop(true)
	f.win.Show()
	f.win.SetIgnoreMouseEvents(true)
	return true
}

// deskFlyView draws the characters on their way, handing over to the
// lyrics' window as they land, or taking over from it as they leave.
func (a *app) deskFlyView(c *ui.Context) {
	a.floatTheme(c)
	a.lyricFace(c.Root())
	f := &a.deskFly
	if f.win == nil {
		return
	}
	// Time goes on as frames come, a slow one moving them on no more than
	// a frame's worth: the first frames of a new window come late, and
	// counting all of that would skip the start of the way.
	now := c.Now()
	if !f.last.IsZero() {
		f.gone += min(now.Sub(f.last), deskFlyStep)
	}
	f.last = now
	f.frames++
	if f.turnPending && f.frames >= 2 {
		// The characters show now, over the window: it can be made again.
		f.turnPending = false
		a.post(a.finishDeskTurn)
	}
	if f.frames == 2 && !f.opening && a.deskWin != nil {
		// The flight shows the line now: its window can leave it to it.
		a.deskWin.Invalidate()
	}
	var total time.Duration
	if f.sprites {
		from, to, ok := a.deskFlySprites(f)
		if !ok && f.waited < 45 {
			// The lyrics' window has yet to lay them out: time waits.
			f.waited++
			f.gone = 0
		}
		_, _, total = deskSpriteTiming(f, max(len(from), len(to)))
		if !ok && f.waited >= 45 {
			f.gone = total
		}
	} else {
		n := len(deskGlyphs(a.deskShape(f.line, a.deskLookOf(a.settings.DeskStyle), 0, false), f.line))
		_, total = deskFlyLags(n)
	}
	k := float32(f.gone) / float32(total)
	// The lyrics' window, its capsule if it has one, comes in as the
	// characters near it, or goes as they leave.
	if a.deskWin != nil && !f.turnPending {
		if f.opening {
			a.deskWin.SetOpacity(float64(easeRange(k, 0.5, 0.92)))
		} else if !a.settings.DeskLyrics {
			a.deskWin.SetOpacity(float64(1 - easeRange(k, 0.05, 0.4)))
		}
	}
	gone := f.gone
	ui.Box(c.Key("fly")).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough().Draw(func(p *ui.Painter, _ ui.Rect) {
		a.paintDeskFly(p, f, gone)
	})
	c.AnimationFrame()
	if f.gone < total {
		return
	}
	if f.opening && f.landed.IsZero() {
		// Down: the lyrics' window shows the line where they are.
		f.landed = now
		if a.deskWin != nil {
			a.deskWin.SetOpacity(1)
			a.deskWin.Invalidate()
		}
	}
	// A few frames more, for the lyrics' window to show its own line
	// before these leave it.
	if f.ending++; f.ending < 4 {
		return
	}
	win, opening := f.win, f.opening
	f.win = nil
	// Not from within its own frame.
	a.post(func() {
		win.Hide()
		win.Destroy()
		switch {
		case a.deskWin == nil:
		case a.settings.DeskLyrics:
			a.deskWin.SetOpacity(1)
			a.placeDeskMore()
		case !opening:
			a.dropFloat(&a.deskWin)
		}
		a.redraw()
	})
}

// paintDeskFly paints the characters gone into their way, from the button
// to where they land, and a ring going out of the button as they leave it
// or come back.
func (a *app) paintDeskFly(p *ui.Painter, f *deskFlight, gone time.Duration) {
	bx, by := f.from.X+f.from.W/2, f.from.Y+f.from.H/2
	if f.sprites {
		a.paintDeskSprites(p, f, gone)
		if f.turning {
			return
		}
		from, to, _ := a.deskFlySprites(f)
		_, _, total := deskSpriteTiming(f, max(len(from), len(to)))
		a.paintDeskRipple(p, f, gone, total, bx, by)
		return
	}
	look := a.deskLookOf(a.settings.DeskStyle)
	pos := a.lyricPos()
	ink := a.deskShape(f.line, look, pos, true)
	glyphs := deskGlyphs(ink, f.line)
	lag, total := deskFlyLags(len(glyphs))
	ox, oy := f.desk.X+f.at.X, f.desk.Y+f.at.Y
	a.paintDeskGlyphs(p, f.line, look, pos, ox, oy, func(i int, g ui.Rect) (k float32, turn float32) {
		// Back, the way out run backwards: the last one out goes first,
		// from the end of the line, none crossing those still in it.
		order := i
		if !f.opening {
			order = len(glyphs) - 1 - i
		}
		u := clamp01(float32(gone-time.Duration(order)*lag) / float32(deskGlyphFly))
		if !f.opening {
			u = 1 - u
		}
		// Each its own way to turn, the same each time.
		turn = float32(6 + (i*7)%9)
		if i%2 == 1 {
			turn = -turn
		}
		return u, turn
	}, bx, by)
	a.paintDeskRipple(p, f, gone, total, bx, by)
}

// paintDeskRipple paints the ring around the button the characters fly out
// of, or back into, total being how long they take.
func (a *app) paintDeskRipple(p *ui.Painter, f *deskFlight, gone, total time.Duration, bx, by float32) {
	// A ring goes out from the button as they leave it, or as the last of
	// them comes back into it: a thin one, around the icon, never over it.
	accent := a.pal.accent
	if ca, ok := a.coverAccent(); ok && a.settings.AccentCover {
		accent = ca
	}
	const ripple = 420 * time.Millisecond
	at := gone
	if !f.opening {
		at = gone - (total - deskGlyphFly/3)
	}
	if u := float32(at) / float32(ripple); u >= 0 && u < 1 {
		e := easeRange(u, 0, 1)
		r := f.from.H/2 + 2 + 10*e
		p.Stroke(ui.Rect{X: bx - r, Y: by - r, W: 2 * r, H: 2 * r}, accent.Alpha(0.6*(1-u)), r, 1.5)
	}
}

// deskShape lays out the line as the lyrics' window does (deskLine): word
// by word if it is sung so, in the colors of where it is at pos if colored,
// else to be painted in one color.
func (a *app) deskShape(m deskMain, look deskLook, pos time.Duration, colored bool) *ui.TextLayout {
	font := ui.Font{Family: a.deskFamily(), Size: m.size, Weight: 800}
	width := float32(0)
	if len(m.words) == 0 {
		return ui.ShapeText(m.text, font, width)
	}
	spans := make([]ui.Span, len(m.words))
	for i, wd := range m.words {
		spans[i] = ui.Span{Text: wd.Text}
		if colored {
			spans[i].Color = a.deskInk(look, &wd, pos)
		}
	}
	return ui.ShapeRichText(spans, font, width)
}

// deskFamily is the font of the desktop lyrics.
func (a *app) deskFamily() string {
	if a.settings.DeskFont != "" {
		return familyOf(a.settings.DeskFont)
	}
	if a.settings.LyricFont != "" {
		return a.lyricFamily()
	}
	return a.fontFamily()
}

// deskInk is the color of a word of the line large on the desktop, or of
// all of it with no word.
func (a *app) deskInk(look deskLook, wd *lyrics.Word, pos time.Duration) ui.Color {
	if wd != nil && a.settings.DeskWords {
		return look.unsung.Mix(look.sung, wordLit(*wd, pos))
	}
	return look.sung
}

// deskGlyphs returns where each character of the line m, as laid out in
// ink, lies in it: from its left to the next one's, over and under the line
// for the edge. Spaces go with the character before them, and what the
// lyrics' window cuts off is left out.
func deskGlyphs(ink *ui.TextLayout, m deskMain) []ui.Rect {
	w, h := ink.Size()
	limit := min(w, deskRoom)
	units := utf16.Encode([]rune(m.shaped()))
	var out []ui.Rect
	for at := 0; at < len(units); {
		next := ink.NextBoundary(at)
		if next <= at {
			break
		}
		x0, x1 := ink.Caret(at).X, ink.Caret(next).X
		if x0 > x1 {
			x0, x1 = x1, x0
		}
		if x0 < limit && strings.TrimSpace(string(utf16.Decode(units[at:next]))) != "" {
			out = append(out, ui.Rect{X: x0, Y: -h * 0.6, W: min(x1, limit) - x0, H: h * 2.2})
		}
		at = next
	}
	// Each reaches the next, over the spaces between, which the edge and
	// some letters reach into: together they are the whole line.
	for i := 0; i+1 < len(out); i++ {
		out[i].W = out[i+1].X - out[i].X
	}
	return out
}

// shaped is the text of the line as deskShape lays it out: its words
// together, if it has them.
func (m deskMain) shaped() string {
	if len(m.words) == 0 {
		return m.text
	}
	var b strings.Builder
	for _, wd := range m.words {
		b.WriteString(wd.Text)
	}
	return b.String()
}

// paintDeskGlyphs paints the line, its edge first if it has one, as the
// lyrics' window does, at (ox, oy), each character where way says it is:
// k of the way from the button at (bx, by), where it is a third of its size,
// to its place, turned by turn degrees out of the way. All at their place,
// it is the line as the lyrics' window shows it.
func (a *app) paintDeskGlyphs(p *ui.Painter, m deskMain, look deskLook, pos time.Duration, ox, oy float32, way func(i int, g ui.Rect) (k, turn float32), bx, by float32) {
	ink := a.deskShape(m, look, pos, true)
	var edge *ui.TextLayout
	if look.edge.A > 0 {
		edge = a.deskShape(m, look, pos, false)
	}
	width := max(1.1, m.size*0.055)
	color := a.deskInk(look, nil, pos)
	paint := func() {
		if edge != nil {
			for i := range 8 {
				turn := float64(i) * math.Pi / 4
				p.TextLayout(edge, ox+width*float32(math.Cos(turn)), oy+width*float32(math.Sin(turn)), look.edge)
			}
		}
		p.TextLayout(ink, ox, oy, color)
	}
	glyphs := deskGlyphs(ink, m)
	_, h := ink.Size()
	for i, g := range glyphs {
		k, turn := way(i, g)
		cx, cy := ox+g.X+g.W/2, oy+h/2
		// The first and the last reach a little past the line, for ink
		// over its ends.
		clip := ui.Rect{X: ox + g.X, Y: oy + g.Y, W: g.W, H: g.H}
		if i == 0 {
			clip.X -= m.size * 0.4
			clip.W += m.size * 0.4
		}
		if i == len(glyphs)-1 {
			clip.W += m.size * 0.4
		}
		if k >= 1 {
			p.Clip(clip, 0, paint)
			continue
		}
		// Its center where it lands, and where it is now: on the way out of
		// the button it rises sooner than it crosses, and comes down last.
		travel := easeRange(k, 0, 1)
		nx, ny := lerp(bx, cx, travel), lerp(by, cy, 1-(1-travel)*(1-travel))
		// A third of its size out of the button, growing on its way, a
		// little more just before it lands, then itself.
		s := lerp(0.32, 1, easeRange(k, 0, 0.78)) + 0.08*float32(math.Sin(math.Pi*float64(clamp01((k-0.78)/0.22))))
		r := turn * (1 - easeRange(k, 0.3, 1))
		t := ui.Scaling(s, s).Then(ui.Rotation(r)).Then(ui.Translation(nx-cx, ny-cy))
		p.Transform(t, cx, cy, func() { p.Clip(clip, 0, paint) })
	}
}
