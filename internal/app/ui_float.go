package app

import (
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/lyrics"
	"netease-music-mygo/internal/netease"
)

// Windows of their own, over the others: the lyrics on the desktop, and the
// player in small, for when the main window is out of the way.

const (
	deskW, deskH   = 920, 150
	deskVW, deskVH = 280, 720 // standing up
	deskToolsRoom  = 40       // over the lyrics, for their tools
	miniW, miniH   = 420, 116
	miniDisc       = 72 // the cover in the mini window
)

// spot returns where the lyrics go: where they were left, if that is on a
// screen still (brought within it, taskbar and all), or at the foot of the
// screen, centered.
func spot(x, y, w, h, foot int) (int, int) {
	if !mygo.App.IsReady() {
		return x, y // no screens to ask about (tests)
	}
	if x != 0 || y != 0 {
		if fx, fy, ok := fitIn(screenAreas(), x, y, w, h); ok {
			return fx, fy
		}
	}
	wa := mygo.Screen.PrimaryDisplay().WorkArea
	return wa.X + (wa.Width-w)/2, wa.Y + wa.Height - h - foot
}

// miniSpot returns where the mini window goes: where it was left, on the
// screen the main window is on, or else in that screen's corner, clear of
// the lyrics.
func (a *app) miniSpot(x, y int) (int, int) {
	if !mygo.App.IsReady() {
		return x, y // no screens to ask about (tests)
	}
	w, h := a.miniSize()
	wa := a.mainWorkArea()
	areas := []mygo.Rectangle{wa}
	if a.win == nil || !a.win.IsVisible() {
		areas = workAreas() // no main window to follow
	}
	if x != 0 || y != 0 {
		if fx, fy, ok := fitIn(areas, x, y, w, h); ok {
			return fx, fy
		}
	}
	return wa.X + wa.Width - w - 24, wa.Y + wa.Height - h - 24
}

// floatWindow opens a window of the app's that floats over the others, on
// nothing, its corners its content's. On the tray's icon if it has one, as a
// flyout: on Windows only those show what is behind what native UI leaves
// clear, and they never take the keyboard from the app in use. Closing it
// goes as off says; destruction waits until its current frame has ended.
func (a *app) floatWindow(title string, x, y, w, h int, view func(*ui.Context), off func()) *mygo.Window {
	var win *mygo.Window
	if a.tray != nil {
		win = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Hidden: true, Width: w, Height: h, Content: ui.View(view)})
	} else {
		win = mygo.NewWindow(mygo.WindowOptions{
			Title: mygo.App.Name() + title, Panel: true, Hidden: true, Frameless: true, Transparent: true,
			DisableShadow: true, DisableResize: true, X: x, Y: y, Width: w, Height: h, Content: ui.View(view),
		})
	}
	win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			off()
			a.redraw()
		}
	})
	return win
}

// applyDeskLyrics shows the lyrics on the desktop or hides them, as the
// settings say. Locked, the pointer goes through them to what is under them.
func (a *app) applyDeskLyrics() {
	s := &a.settings
	if a.win == nil {
		return
	}
	if !s.DeskLyrics || s.DeskLock {
		a.closeDeskMore()
	}
	if s.DeskLyrics && a.deskWin == nil {
		x, y := a.deskSpot(s.DeskX, s.DeskY)
		w, h := a.deskSize()
		a.desk.vertical = s.DeskVertical
		// What the window lays out, the characters fly to: none yet.
		a.deskStood, a.deskLineAt = nil, ui.Rect{}
		a.deskWin = a.floatWindow(" 桌面歌词", x, y, w, h, a.deskView, func() {
			s.DeskLyrics = false
			a.applyDeskLyrics()
		})
		// It fades in as the lyrics land, not as the system shows it.
		showAtOnce(a.deskWin.NativeHandle())
		a.deskWin.OnMove(func() {
			if a.deskWin != nil && a.deskWin.IsVisible() {
				b := a.deskWin.Bounds()
				// Where the app put them is not where they were left: a
				// screen that came back smaller for a while doesn't move
				// them for good.
				if d := &a.desk; d.placedSet && d.placed == (mygo.Point{X: b.X, Y: b.Y}) {
					return
				}
				a.desk.placedSet = false
				s.DeskX, s.DeskY = b.X, b.Y
				a.placeDeskMore()
			}
		})
		a.watchDesk()
	}
	if a.deskWin == nil {
		return
	}
	a.desk.hot = false
	a.deskWin.SetIgnoreMouseEvents(s.DeskLyrics && s.DeskLock)
	if s.DeskLyrics && (a.desk.covered || a.desk.idle) && a.deskFly.win == nil {
		// A game or a video is full screen in front, or nothing plays and
		// they are to hide then: out of the way until that ends.
		a.deskWin.Hide()
		return
	}
	if s.DeskLyrics {
		if a.deskFly.coming || (a.deskFly.win != nil && a.deskFly.opening) {
			// Out of the button: the window comes in as the lyrics land.
			a.deskWin.SetOpacity(0)
		}
		a.deskWin.Show()
		// A flyout shows by its anchor: from there, where it was left, and
		// above all, which it isn't by itself once another app comes up.
		a.placeDesk(a.deskSpot(s.DeskX, s.DeskY))
		a.deskWin.SetAlwaysOnTop(true)
	} else {
		a.stopDeskWatch()
		a.dropFloat(&a.deskWin)
	}
}

// deskSize is how large the lyrics' window is, lying or standing.
func (a *app) deskSize() (int, int) {
	if a.settings.DeskVertical {
		return deskVW, deskVH
	}
	return deskW, deskH + a.deskExtraH()
}

// deskExtraH is how much taller than deskH the lying lyrics' window is: room
// for the reading over the line (音译) at the largest size, whether it shows
// or not. The window never changes size for it: a flyout made another size
// goes back by its anchor, and the more panel with it (mygo places them
// all again), which jumped as 音译 was turned on or off.
func (a *app) deskExtraH() int {
	return int(math.Ceil((deskReadingSize*1.4 + 3) * 1.3))
}

// fitDeskSize makes the lyrics' window as large as the settings now have it.
func (a *app) fitDeskSize() {
	if a.deskWin == nil || a.desk.vertical {
		return
	}
	w, h := a.deskSize()
	if b := a.deskWin.Bounds(); b.Width != w || b.Height != h {
		// A flyout made another size goes back by its anchor, the tray's
		// icon: put where it was, kept on its screen.
		a.deskWin.SetSize(w, h)
		a.placeDesk(spot(b.X, b.Y, w, h, 40))
		a.placeDeskMore()
	}
}

// deskSpot returns where the lyrics' window goes, left at x, y.
func (a *app) deskSpot(x, y int) (int, int) {
	w, h := a.deskSize()
	if a.settings.DeskVertical && x == 0 && y == 0 && mygo.App.IsReady() {
		// Standing, they start at the right of the screen, in the middle.
		wa := mygo.Screen.PrimaryDisplay().WorkArea
		return wa.X + wa.Width - w - 40, wa.Y + (wa.Height-h)/2
	}
	return spot(x, y, w, h, 40)
}

// setDeskVertical stands the lyrics up or lays them down: their window is
// made again, at its new size.
func (a *app) setDeskVertical(v bool) {
	s := &a.settings
	if a.deskWin == nil || a.desk.vertical == v || a.deskFly.win != nil {
		s.DeskVertical = v
		if a.deskWin != nil && a.desk.vertical != v {
			a.stopDeskWatch()
			a.dropFloat(&a.deskWin)
			a.applyDeskLyrics()
		}
		a.changed()
		return
	}
	// Each character of the line goes on its own way from where it lies
	// to where it stands, or back. The window turns where it is: the
	// other one is made around the middle of this one.
	var src []deskSprite
	if a.deskWin.IsVisible() && !s.DeskLock {
		src = a.deskNowSprites()
	}
	old := a.deskWin.Bounds()
	w, h := deskVW, deskVH
	if !v {
		w, h = deskW, deskH+a.deskExtraH()
	}
	to := mygo.Rectangle{Width: w, Height: h}
	to.X, to.Y = spot(old.X+old.Width/2-w/2, old.Y+old.Height/2-h/2, w, h, 40)
	// The characters take over from the window before it goes: they show
	// where it showed them until their own window is up, and only then is
	// the window made again (finishDeskTurn), with no frame of nothing.
	if a.startDeskTurn(src, old, to, v) {
		return
	}
	a.turnDesk(v, to)
}

// turnDesk stands the lyrics up or lays them down, their window made
// again at to.
func (a *app) turnDesk(v bool, to mygo.Rectangle) {
	s := &a.settings
	s.DeskVertical = v
	s.DeskX, s.DeskY = to.X, to.Y
	a.stopDeskWatch()
	a.dropFloat(&a.deskWin)
	a.applyDeskLyrics()
	a.changed()
}

// placeDesk moves the lyrics to x, y, for the app: where they were left is
// still where the listener left them.
func (a *app) placeDesk(x, y int) {
	if a.deskWin == nil {
		return
	}
	a.desk.placed, a.desk.placedSet = mygo.Point{X: x, Y: y}, true
	a.deskWin.SetPosition(x, y)
}

// dropFloat releases a closed player's renderer after any frame building
// its close button. A later opening gets a fresh window at the saved spot.
func (a *app) dropFloat(slot **mygo.Window) {
	win := *slot
	if win == nil {
		return
	}
	*slot = nil
	win.Hide()
	a.post(win.Destroy)
	a.releaseSoon()
}

// toggleDeskLyrics turns them on or off, from wherever that is offered.
func (a *app) toggleDeskLyrics() {
	s := &a.settings
	s.DeskLyrics = !s.DeskLyrics
	switch {
	case s.DeskLyrics:
		// Out of the button, to where the window goes: the flight after
		// the window, to be over it.
		x, y := a.deskSpot(s.DeskX, s.DeskY)
		w, h := a.deskSize()
		a.deskFly.coming = true
		a.applyDeskLyrics()
		a.deskFly.coming = false
		if a.deskWin != nil && !a.startDeskFly(true, mygo.Rectangle{X: x, Y: y, Width: w, Height: h}) {
			a.deskWin.SetOpacity(1)
		}
		if s.DeskHideStopped && !a.pl.Playing() {
			a.toast = "桌面歌词会在开始播放后显示"
		}
	case a.deskWin != nil && a.startDeskFly(false, a.deskWin.Bounds()):
		// It goes back into the button first, and its window after.
		a.deskWin.SetIgnoreMouseEvents(true)
	default:
		a.applyDeskLyrics()
	}
	a.changed()
}

// toggleDeskLock locks them where they are, or lets them be moved again.
func (a *app) toggleDeskLock() {
	a.settings.DeskLock = !a.settings.DeskLock
	a.applyDeskLyrics()
	a.changed()
}

// floatTheme gives a floating window the app's font, on nothing.
func (a *app) floatTheme(c *ui.Context) {
	t := *c.Theme()
	t.Font = a.fontFamily()
	c.SetTheme(&t)
	c.Root().Background(ui.Transparent)
}

// How lyrics can look on the desktop. Whatever is behind them, they have to
// read: the first two on a surface of their own, and the two outlined by an
// edge as sharp as their letters, for a soft shadow is a smear on white.
var deskStyles = []struct{ id, label string }{
	{"glass", "深色胶囊"},
	{"light", "浅色胶囊"},
	{"outline", "白字描边"},
	{"color", "彩色描边"},
	{"plain", "无背景"},
}

// A deskLook is the colors of a style: what they lie on, if anything, the
// words sung and to come, the line under them, and their edge, if any.
type deskLook struct {
	pill, sung, unsung, second, edge ui.Color
}

// deskLookOf returns the colors of a style, in the color of what plays.
func (a *app) deskLookOf(id string) deskLook {
	lamp := a.pal.accent
	if c, ok := deskColorOf(a.settings.DeskColor); ok {
		lamp = c
	} else if accent, ok := a.coverAccent(); ok && a.settings.DeskColor == "" {
		lamp = accent
	}
	// The color is the app's, which may be one the listener picked, white
	// or black: brought as dark as words on white need, on the light
	// capsule and inside a white edge, or as light as on the dark ones.
	// Else the translation, a shade of it, went into its edge or capsule.
	switch id {
	case "light", "color":
		lamp = lumWithin(lamp, 0, 0.45)
	default:
		lamp = lumWithin(lamp, 0.3, 1)
	}
	switch id {
	case "light":
		return deskLook{pill: ui.RGBA(250, 250, 252, 0.94), sung: lamp.Mix(ui.RGB(0, 0, 0), 0.25),
			unsung: ui.RGBA(20, 20, 24, 0.48), second: ui.RGBA(20, 20, 24, 0.6)}
	case "outline":
		return deskLook{sung: lamp.Mix(white, 0.45), unsung: white, second: white, edge: ui.RGBA(0, 0, 0, 0.82)}
	case "color":
		return deskLook{sung: lamp.Mix(ui.RGB(0, 0, 0), 0.1), unsung: lamp.Mix(white, 0.55), second: lamp.Mix(ui.RGB(0, 0, 0), 0.2), edge: white}
	case "plain":
		return deskLook{sung: lamp.Mix(white, 0.45), unsung: white.Alpha(0.62), second: white.Alpha(0.7)}
	}
	return deskLook{pill: ui.RGBA(16, 16, 20, 0.8), sung: lamp.Mix(white, 0.55), unsung: white.Alpha(0.5), second: white.Alpha(0.62)}
}

// lumWithin brings c toward black or white until its luminance is within
// lo and hi, opaque.
func lumWithin(c ui.Color, lo, hi float64) ui.Color {
	c.A = 255
	switch l := luminance(c); {
	case l > hi:
		return c.Mix(ui.RGB(0, 0, 0), float32(1-hi/l))
	case l < lo:
		return c.Mix(white, float32((lo-l)/(1-l)))
	}
	return c
}

// deskView is the line playing, and what follows it or says it in Chinese.
// Over the pointer it shows it can be moved, and its tools.
func (a *app) deskView(c *ui.Context) {
	a.floatTheme(c)
	c.Root().Font(a.deskFamily())
	a.followCover()
	s, pl := &a.settings, a.pl
	lines, _ := a.shownLyrics()
	pos := a.lyricPos()
	scale := float32(1)
	if s.DeskScale > 0 {
		scale = float32(s.DeskScale)
	}
	look := a.deskLookOf(s.DeskStyle)
	main, next := a.deskLines()
	nextSize := a.deskNextSize(next, 19*scale)
	// While the line flies out of its button, or back, the flight shows
	// it; and the line under it comes in once it is down.
	shown, nextShown := a.deskFlyShown(c.Now())
	// The lyrics hang from a line under the room for their tools, rather
	// than in the middle: the tools stay just over them however many lines
	// or how large, and the line sung stays put as the one under it comes
	// and goes.
	root := ui.Column(c.Key("desk")).Fill().AlignItems(ui.Center).Padding(deskToolsRoom, 24, 0, 24)
	over := root.Hovered()
	if over && !s.DeskLock {
		root.Cursor(ui.CursorGrab)
	}
	root.DragWindow().Children(func() {
		if s.DeskVertical {
			a.desk.bodyAt = a.deskStanding(c, main, next, pos, look, 19*scale, shown, nextShown)
			a.deskControls(c, look, over)
			return
		}
		body := ui.Column(c.Key("body")).Gap(3).AlignItems(ui.Center).DragWindow()
		a.desk.bodyAt = body.LayoutBounds()
		if look.pill.A > 0 {
			body.Padding(10, 28).Radius(26).Background(look.pill)
		}
		body.Children(func() {
			if main.reading != "" {
				reading := a.deskLine(c, "reading", main.reading, nil, pos, a.deskNextSize(main.reading, deskReadingSize*scale), look, false, false)
				if nextShown < 1 {
					reading.Opacity(nextShown)
				}
			}
			var line ui.Element
			if main.scroll {
				line = a.deskScrollLine(c, main, pos, look)
			} else {
				line = a.deskLine(c, "main", main.text, main.words, pos, main.size, look, true, false)
			}
			// Where it lies, for the characters flying to it, or from it.
			a.deskLineAt = line.LayoutBounds()
			if a.deskFly.win != nil {
				a.deskFly.at = a.deskLineAt
			}
			if shown < 1 {
				line.Opacity(shown)
			}
			if next != "" {
				under := a.deskLine(c, "next", next, nil, pos, nextSize, look, false, false)
				if nextShown < 1 {
					under.Opacity(nextShown)
					c.AnimationFrame()
				}
			}
		})
		a.deskControls(c, look, over)
	})
	// Drawn again when what it shows changes, not on a clock: a window
	// over a game that draws all the time takes frames from it.
	if pl.Playing() && len(lines) > 0 {
		c.After(a.deskNextChange(lines, pos))
	}
}

// deskControls are the tools over unlocked lyrics, or the button that
// unlocks them over locked ones, the pointer over them.
func (a *app) deskControls(c *ui.Context, look deskLook, over bool) {
	switch {
	case !a.settings.DeskLock:
		a.deskTools(c, look, over)
	case a.desk.near:
		a.deskUnlock(c)
	default:
		a.desk.unlockAt = ui.Rect{}
	}
}

// deskStanding is the lyrics standing up: character under character, the
// line sung on the right, in columns from the right if it is long, and the
// one under it, or its translation, to its left.
func (a *app) deskStanding(c *ui.Context, main deskMain, next string, pos time.Duration, look deskLook, nextSize, shown, nextShown float32) ui.Rect {
	body := ui.Row(c.Key("body")).Gap(6).AlignItems(ui.Start).DragWindow()
	pad := float32(0)
	if look.pill.A > 0 {
		pad = 16
		body.Padding(pad, 10).Radius(20).Background(look.pill)
	}
	room := float32(deskVH-deskToolsRoom-8) - 2*pad
	// Where each character of the line sung stands, for those flying to
	// it or from it: laid out anew each frame.
	a.deskStood = a.deskStood[:0]
	body.Children(func() {
		under := func(e ui.Element) {
			if nextShown < 1 {
				e.Opacity(nextShown)
				c.AnimationFrame()
			}
		}
		if next != "" {
			under(a.deskColumns(c, "next", []rune(next), func(int) ui.Color { return look.second }, nextSize, 700, room, look, false))
		}
		chars, colors := a.deskChars(main, look, pos)
		line := a.deskColumns(c, "main", chars, func(i int) ui.Color { return colors[i] }, main.size, 800, room, look, true)
		if shown < 1 {
			line.Opacity(shown)
		}
		if main.reading != "" {
			under(a.deskColumns(c, "reading", []rune(main.reading), func(int) ui.Color { return look.second }, nextSize*deskReadingSize/19, 700, room, look, false))
		}
	})
	return body.LayoutBounds()
}

// deskChars returns the characters of the line sung, and the color of each:
// lit with its word, as the line lying down lights them.
func (a *app) deskChars(m deskMain, look deskLook, pos time.Duration) ([]rune, []ui.Color) {
	var chars []rune
	var colors []ui.Color
	if len(m.words) == 0 {
		for _, r := range m.text {
			chars, colors = append(chars, r), append(colors, look.sung)
		}
		return chars, colors
	}
	for i := range m.words {
		ink := a.deskInk(look, &m.words[i], pos)
		for _, r := range m.words[i].Text {
			chars, colors = append(chars, r), append(colors, ink)
		}
	}
	return chars, colors
}

// deskStep is how tall a character standing is, of its size.
const deskStep = 1.12

// A deskPiece is what stands as one in a column: a character of Chinese or
// Japanese, upright, or a run of others, a word of English, lying on its
// side, as vertical text has them.
type deskPiece struct {
	text     string
	at       int  // its first character's, in the line
	sideways bool // lying, turned a quarter right
	h        float32
}

// upright reports whether r stands upright in vertical text.
func upright(r rune) bool {
	return r >= 0x2E80 || unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
}

// deskPieces cuts chars into what stands as one, at size.
func deskPieces(c *ui.Context, chars []rune, size float32, weight int) []deskPiece {
	var out []deskPiece
	for i := 0; i < len(chars); {
		if upright(chars[i]) {
			out = append(out, deskPiece{text: string(chars[i]), at: i, h: size * deskStep})
			i++
			continue
		}
		j := i
		for j < len(chars) && !upright(chars[j]) {
			j++
		}
		run := strings.TrimSpace(string(chars[i:j]))
		if run == "" {
			out = append(out, deskPiece{at: i, h: size * 0.4}) // a space between
		} else {
			w, _ := c.MeasureText(0, ui.Span{Text: run, Size: size, Weight: weight})
			out = append(out, deskPiece{text: run, at: i, sideways: true, h: w + size*0.3})
		}
		i = j
	}
	return out
}

// deskColumns stands chars up in room: smaller, down to 18, to fit in one
// column, and else in as many as they take, the first on the right.
func (a *app) deskColumns(c *ui.Context, key string, chars []rune, color func(int) ui.Color, size float32, weight int, room float32, look deskLook, record bool) ui.Element {
	tall := func(ps []deskPiece) (h float32) {
		for _, p := range ps {
			h += p.h
		}
		return h
	}
	pieces := deskPieces(c, chars, size, weight)
	if h := tall(pieces); h > room {
		size = max(size*room/h, min(size, 18))
		pieces = deskPieces(c, chars, size, weight)
	}
	var cols [][]deskPiece
	var h float32
	for _, p := range pieces {
		if len(cols) == 0 || h+p.h > room {
			cols, h = append(cols, nil), 0
		}
		cols[len(cols)-1] = append(cols[len(cols)-1], p)
		h += p.h
	}
	return ui.Row(c.Key(key)).Gap(4).AlignItems(ui.Start).PassThrough().Children(func() {
		for i := len(cols) - 1; i >= 0; i-- {
			a.deskColumn(c, i, cols[i], color, size, weight, look, record)
		}
	})
}

// deskColumn is a column of what stands, edged as the lines lying down are,
// with eight copies of it around it.
func (a *app) deskColumn(c *ui.Context, key int, pieces []deskPiece, color func(int) ui.Color, size float32, weight int, look deskLook, record bool) {
	wide := size * deskStep
	col := func(k any, ink func(int) ui.Color) ui.Element {
		return ui.Column(c.Key(k)).Width(wide).PassThrough().Children(func() {
			for i, p := range pieces {
				box := ui.Box(c.Key(i)).Size(wide, p.h).PassThrough()
				if p.text == "" {
					continue
				}
				if record && k == "ink" {
					a.deskStood = append(a.deskStood, deskStood{piece: p, size: size, at: box.LayoutBounds()})
				}
				box.Children(func() {
					t := ui.Text(c, p.text).FontSize(size).FontWeight(weight).SingleLine().TextColor(ink(p.at))
					if !p.sideways {
						t.Absolute().Left(0).Right(0).Top(0).Bottom(0).TextAlign(ui.Center).LineHeight(deskStep)
						return
					}
					// Lying, turned about its middle to the middle of
					// its room.
					w := p.h - size*0.3
					t.Absolute().Width(w).Height(wide).LineHeight(deskStep).TextAlign(ui.Center).
						Left((wide - w) / 2).Top((p.h - wide) / 2).Rotate(90)
				})
			}
		})
	}
	ui.Box(c.Key(key)).Shrink(0).PassThrough().Children(func() {
		col("room", func(int) ui.Color { return ui.Transparent }).Opacity(0)
		if look.edge.A > 0 {
			width := max(1.1, size*0.055)
			for i := range 8 {
				turn := float64(i) * math.Pi / 4
				col(i, func(int) ui.Color { return look.edge }).Absolute().
					Left(width * float32(math.Cos(turn))).Top(width * float32(math.Sin(turn)))
			}
		}
		col("ink", color).Absolute().Left(0).Top(0)
	})
}

// deskNextChange returns how long until the desktop lyrics show something
// else: the next line, the next word to light, or a frame while one lights.
func (a *app) deskNextChange(lines []lyrics.Line, pos time.Duration) time.Duration {
	const frame = 40 * time.Millisecond
	wait := time.Second
	cur := lyrics.Index(lines, pos)
	if cur+1 < len(lines) {
		wait = min(wait, lines[cur+1].At-pos)
	}
	if cur >= 0 && a.settings.DeskWords {
		for _, wd := range lines[cur].Words {
			if pos < wd.Start {
				wait = min(wait, wd.Start-pos)
				break
			}
			if pos < max(wd.End, wd.Start+50*time.Millisecond) { // as wordLit lights it
				return frame
			}
		}
	}
	if sp := a.pl.Speed(); sp > 0 {
		wait = time.Duration(float64(wait) / sp)
	}
	// Just after the change, not just before it.
	return max(wait+10*time.Millisecond, frame)
}

// deskUnlock is the button over locked lyrics that unlocks them, while the
// pointer is over them. The window lets the pointer through but there, as
// watchDesk has it.
func (a *app) deskUnlock(c *ui.Context) {
	ui.Row(c.Key("unlock-row")).Absolute().Top(deskToolsRoom - 35).Left(0).Right(0).Justify(ui.Center).PassThrough().Children(func() {
		b := ui.ButtonBase(c.Key("unlock")).Size(30, 30).Radius(999).Center().FocusRing(false).Label("解锁桌面歌词")
		fade := ui.Animation{Duration: 120 * time.Millisecond}
		bg, fg := a.miniSurface().Alpha(0.78), white.Alpha(0.8)
		if b.Hovered() {
			bg, fg = a.miniSurface().Alpha(0.96), white
		}
		b.Background(b.AnimateColor("bg", bg, fade)).TextColor(b.AnimateColor("fg", fg, fade)).
			Border(1, white.Alpha(0.14)).Shadow(0, 6, 16, -6, ui.RGBA(0, 0, 0, 0.5))
		b.Children(func() { ui.Icon(c, iconUnlock).Size(14, 14) })
		a.desk.unlockAt = b.LayoutBounds()
		if b.Clicked() {
			a.toggleDeskLock()
			a.say("desk-lock", "桌面歌词已解锁")
		}
	})
}

// deskLine is a line of the lyrics, lit word by word as it is sung if it is
// the main one, with an edge of eight copies of it around it if the style has
// one: there is no stroke to give text. The copies are laid out as the line
// is, word by word too: a line of runs and the same text in one run are not
// always as wide, kana and hanzi falling back to fonts of their own, and an
// edge laid out the other way drifted off its words along the line.
func (a *app) deskLine(c *ui.Context, key, text string, words []lyrics.Word, pos time.Duration, size float32, look deskLook, main, wrap bool) ui.Element {
	style := func(e ui.Element) ui.Element {
		weight := 700
		if main {
			weight = 800
		}
		e = e.FontSize(size).FontWeight(weight).MaxWidth(deskRoom)
		if wrap {
			return e.MaxLines(2).TextAlign(ui.Center)
		}
		return e.SingleLine()
	}
	byWord := main && len(words) > 0
	// lay is the line in color, or each word in a color of its own.
	lay := func(key any, color func(wd lyrics.Word) ui.Color) ui.Element {
		if !byWord {
			return style(ui.Text(c.Key(key), text)).TextColor(color(lyrics.Word{}))
		}
		return style(ui.RichText(c.Key(key))).Children(func() {
			for i, wd := range words {
				ui.Text(c.Key(i), wd.Text).TextColor(color(wd))
			}
		})
	}
	flat := func(color ui.Color) func(lyrics.Word) ui.Color {
		return func(lyrics.Word) ui.Color { return color }
	}
	return ui.Box(c.Key(key)).Shrink(0).PassThrough().Children(func() {
		// Positioned elements paint over those in the flow, in order: the
		// line itself is positioned too, last, and this holds its place.
		lay("room", flat(ui.Transparent)).Opacity(0)
		if look.edge.A > 0 {
			width := max(1.1, size*0.055)
			for i := range 8 {
				turn := float64(i) * math.Pi / 4
				lay(i, flat(look.edge)).Absolute().
					Left(width * float32(math.Cos(turn))).Top(width * float32(math.Sin(turn)))
			}
		}
		var line ui.Element
		switch {
		case byWord && a.settings.DeskWords:
			line = lay("words", func(wd lyrics.Word) ui.Color { return look.unsung.Mix(look.sung, wordLit(wd, pos)) })
		case main:
			line = lay("words", flat(look.sung))
		default:
			line = lay("line", flat(look.second))
		}
		line.Absolute().Left(0).Top(0)
	})
}

// deskTools change the lyrics' look and size, lock them where they are, and
// close them: a capsule of the mini window's stuff, in the color of what
// plays, to read over any style, centered just over the lyrics. It rises in
// as the pointer comes over them, shown, and sinks away as it leaves.
func (a *app) deskTools(c *ui.Context, look deskLook, shown bool) {
	s := &a.settings
	size := func(by float64) {
		if s.DeskScale == 0 {
			s.DeskScale = 1
		}
		s.DeskScale = math.Max(0.7, math.Min(s.DeskScale+by, 1.3))
		a.changed()
		a.fitDeskSize()
	}
	style := 0
	for i, d := range deskStyles {
		if d.id == s.DeskStyle {
			style = i
		}
	}
	pose := ui.Pose{Y: 6, Scale: 0.96, Fade: 1}
	if shown {
		pose = ui.Pose{}
	}
	fade := ui.Animation{Duration: 120 * time.Millisecond}
	// As wide as the window, as the lyrics are centered in it: the capsule
	// centers over them, and the pointer goes through the rest to drag them.
	ui.Row(c.Key("tools-row")).Absolute().Top(deskToolsRoom - 35).Left(0).Right(0).Justify(ui.Center).PassThrough().Children(func() {
		bar := ui.Row(c.Key("tools")).Height(30).Padding(2).Gap(1).Radius(999).AlignItems(ui.Center).
			Background(a.miniSurface().Alpha(0.94)).Border(1, white.Alpha(0.14)).Shadow(0, 6, 16, -6, ui.RGBA(0, 0, 0, 0.5))
		bar.AnimatePose(pose, ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.18}})
		bar.Children(func() {
			// Every button keeps its width, whatever it says: one that changed
			// would move those beside it from under the pointer, between two
			// clicks.
			chip := func(key, says string, wide float32, hot ui.Color, do func(), inside func()) ui.Element {
				b := ui.ButtonBase(c.Key(key)).Size(wide, 26).Radius(13).Gap(6).Center().FocusRing(false).Label(says)
				bg, fg := ui.Transparent, white.Alpha(0.8)
				if b.Hovered() {
					bg, fg = hot, white
				}
				b.Background(b.AnimateColor("bg", bg, fade)).TextColor(b.AnimateColor("fg", fg, fade))
				press := ui.Pose{}
				if b.Pressed() {
					press.Scale = 0.9
				}
				b.AnimatePose(press, ui.Snappy).Children(inside)
				if b.Clicked() {
					do()
				}
				return b
			}
			rule := func() { ui.Box(c).Size(1, 14).Margin(0, 4).Background(white.Alpha(0.14)) }
			hover := white.Alpha(0.12)

			// The style: a dot of its colors, the words' on what they lie on
			// or are edged with, and its name. A click goes on to the next.
			// Standing, the window is too narrow for the style's name: its dot
			// alone, the name in the more panel.
			lookW := float32(88)
			if s.DeskVertical {
				lookW = 28
			}
			chip("look", "换一种样式", lookW, hover, func() {
				next := (style + 1) % len(deskStyles)
				s.DeskStyle = deskStyles[next].id
				a.changed()
				a.say("desk-style", "桌面歌词："+deskStyles[next].label)
			}, func() {
				ring := white.Alpha(0.35)
				switch {
				case look.pill.A > 0:
					ring = look.pill.Alpha(1)
				case look.edge.A > 0:
					ring = look.edge
				}
				ui.Box(c).Size(12, 12).Radius(999).Background(look.sung).Border(2.5, ring).Shadow(0, 0, 0, 1, white.Alpha(0.25))
				if !s.DeskVertical {
					ui.Text(c, deskStyles[style].label).FontSize(12).FontWeight(600).SingleLine()
				}
			})
			rule()
			// The size, between a small A and a large one.
			chip("smaller", "歌词字号减小", 28, hover, func() { size(-0.1) }, func() {
				ui.Text(c, "A").FontSize(11).FontWeight(700)
			})
			chip("larger", "歌词字号增大", 28, hover, func() { size(0.1) }, func() {
				ui.Text(c, "A").FontSize(16).FontWeight(700)
			})
			stand, standLabel := "竖", "竖排显示"
			if s.DeskVertical {
				stand, standLabel = "横", "横排显示"
			}
			chip("stand", standLabel, 28, hover, func() { a.setDeskVertical(!s.DeskVertical) }, func() {
				ui.Text(c, stand).FontSize(12).FontWeight(700)
			})
			a.desk.moreAt = chip("more", "更多选项", 28, hover, a.toggleDeskMore, func() {
				ui.Icon(c, iconMore).Size(14, 14)
			}).LayoutBounds()
			rule()
			chip("lock", "锁定桌面歌词", 28, hover, func() {
				s.DeskLock = true
				a.applyDeskLyrics()
				a.changed()
				a.toast = "桌面歌词已锁定，鼠标移到歌词上点锁形按钮即可解锁"
			}, func() { ui.Icon(c, iconLock).Size(13, 13) })
			chip("close", "关闭桌面歌词", 28, ui.RGBA(232, 66, 63, 0.9), a.toggleDeskLyrics, func() {
				ui.Icon(c, iconX).Size(13, 13)
			})
		})
	})
}

// miniSurface is what the mini window is made of: dark, in the cover's color.
func (a *app) miniSurface() ui.Color {
	tint := ui.RGB(70, 70, 78)
	if art := a.npArt; art != nil && !art.failed {
		tint = art.tint
	}
	return ui.Hex("#141418").Mix(tint, 0.3)
}

// How the player goes between the bar of the main window and the mini window.
const morphTime = 460 * time.Millisecond

// A floatMorph is the player on its way: the bar's capsule and its cover, going
// over the screen, which is what shows the small window is the bar of the large
// one. It is drawn on a clear window as large as its way.
type floatMorph struct {
	win       *mygo.Window
	toMini    bool
	box, disc [2]ui.Rect // from and to, in that window
	corner    [2]float32 // the capsule's, from and to
	round     [2]float32 // the cover's corners, from and to
	square    bool       // the mini window is the square, the cover filling it
	artPx     int        // the size of the cover asked, the mini window's
	placed    bool       // the mini window is there, not yet shown
	at        time.Time
	handed    bool // the mini window has come in, or gone
	ending    int  // frames since it arrived, to leave the mini window showing
}

// barOnScreen returns where the bar of the main window and its cover are, on
// the screen.
func (a *app) barOnScreen() (bar, cover ui.Rect) {
	cb := a.win.ContentBounds()
	at := func(r ui.Rect) ui.Rect {
		return ui.Rect{X: float32(cb.X) + r.X, Y: float32(cb.Y) + r.Y, W: r.W, H: r.H}
	}
	return at(a.barBox), at(a.barSpot.cover)
}

// startMorph sends the player between the bar and the mini window at x and y,
// and reports whether it could: otherwise it is there at once.
func (a *app) startMorph(x, y int, toMini bool) bool {
	bar, cover := a.barOnScreen()
	if a.motion != motionFull || a.tray == nil || a.morph.win != nil || bar.W == 0 {
		return false
	}
	mw, mh := a.miniSize()
	mini := ui.Rect{X: float32(x), Y: float32(y), W: float32(mw), H: float32(mh)}
	disc := ui.Rect{X: mini.X + 14, Y: mini.Y + 12, W: miniDisc, H: miniDisc}
	if at := a.more.miniCoverAt; at.W > 0 && at.X+at.W <= mini.W && at.Y+at.H <= mini.H {
		disc = ui.Rect{X: mini.X + at.X, Y: mini.Y + at.Y, W: at.W, H: at.H}
	}
	left := int(min(bar.X, mini.X)) - 8
	top := int(min(bar.Y, mini.Y)) - 8
	w := int(max(bar.X+bar.W, mini.X+mini.W)) + 8 - left
	h := int(max(bar.Y+bar.H, mini.Y+mini.H)) + 8 - top
	in := func(r ui.Rect) ui.Rect {
		return ui.Rect{X: r.X - float32(left), Y: r.Y - float32(top), W: r.W, H: r.H}
	}
	m := &a.morph
	*m = floatMorph{toMini: toMini}
	// The cover keeps its corners at either end: round on a turning disc,
	// square on a card, as large as the window's on the square.
	barRound := min(a.barSpot.radius, cover.W/2)
	miniRound := disc.W / 2
	if r := a.more.miniCoverRound; r > 0 && a.more.miniCoverAt.W > 0 {
		miniRound = min(r, disc.W/2)
	}
	m.box, m.disc, m.corner = [2]ui.Rect{in(bar), in(mini)}, [2]ui.Rect{in(cover), in(disc)}, [2]float32{bar.H / 2, 26}
	m.round = [2]float32{barRound, miniRound}
	m.square = miniRound >= 26 && disc.W >= mini.W-0.5
	m.artPx = miniArtPx(disc.W, disc.H)
	if tr := a.pl.Current(); tr != nil {
		a.art.get(netease.Sized(tr.Album.PicURL, m.artPx), false) // asked now, there when it lands
	}
	if !toMini {
		m.box[0], m.box[1] = m.box[1], m.box[0]
		m.disc[0], m.disc[1] = m.disc[1], m.disc[0]
		m.corner[0], m.corner[1] = m.corner[1], m.corner[0]
		m.round[0], m.round[1] = m.round[1], m.round[0]
	}
	m.win = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Hidden: true, Width: w, Height: h, Content: ui.View(a.morphView)})
	// Placed before it shows: shown first, it shows a frame where a new
	// window goes before it moves.
	m.win.SetPosition(left, top)
	m.win.SetAlwaysOnTop(true)
	m.win.SetIgnoreMouseEvents(true)
	m.win.Show()
	return true
}

// morphView draws the player on its way, and fades the main window as it
// goes from it or comes back to it.
func (a *app) morphView(c *ui.Context) {
	a.floatTheme(c)
	m := &a.morph
	if m.win == nil {
		return
	}
	now := c.Now()
	if m.at.IsZero() {
		m.at = now
	}
	t := clamp01(float32(now.Sub(m.at)) / float32(morphTime))
	// The window comes in over the capsule as that arrives, or goes off it
	// before that leaves: what is on it doesn't pop in, or out.
	const handOver = 0.62
	var move float32
	if m.toMini {
		move = t
		a.win.SetOpacity(float64(1 - easeRange(t, 0, 0.75)))
		// The window is made at the start, but shown only as the capsule
		// arrives, as before: made as it should show, its making is a
		// stall the capsule waits on; shown unseen, it draws nothing until
		// it is seen, and pops in whole at the end of its fade.
		if !m.placed {
			m.placed = true
			a.makeMini()
		}
		if t >= handOver {
			if !m.handed {
				m.handed = true
				a.placeMini()
			}
			a.miniWin.SetOpacity(float64(easeRange(t, handOver, 1)))
		}
	} else {
		move = clamp01((t - 0.14) / 0.86)
		// The window is all there before the capsule lands, for the bar's
		// glass to be there to melt into.
		a.win.SetOpacity(float64(easeRange(t, 0.2, 0.72)))
		if !m.handed {
			a.miniWin.SetOpacity(float64(1 - easeRange(t, 0, 0.2)))
			if t >= 0.2 {
				m.handed = true
				a.miniWin.Hide()
				a.miniWin.SetOpacity(1)
			}
		}
	}
	k := 1 - (1-move)*(1-move)*(1-move)
	box, disc := lerpRect(m.box[0], m.box[1], k), lerpRect(m.disc[0], m.disc[1], k)
	if m.square {
		// The cover is to fill the capsule, clipped to it: going out it
		// grows a little ahead of it, and coming back stays filling it a
		// little longer, so the two meet gently rather than at the last
		// frame.
		kc := 1 - (1-move)*(1-move)*(1-move)*(1-move)*(1-move)
		if !m.toMini {
			kc = k * k
		}
		disc = lerpRect(m.disc[0], m.disc[1], kc)
	}
	// Back on the bar, the solid capsule clears into the bar's glass under
	// it: its fill thins first, its edge and shadow after, and its cover
	// last, onto the bar's own.
	fill, edge, cover := float32(1), float32(1), float32(1)
	if !m.toMini {
		fill = 1 - easeRange(move, 0.5, 0.92)
		edge = 1 - easeRange(move, 0.7, 1)
		cover = 1 - easeRange(move, 0.8, 1)
	}
	corner := lerp(m.corner[0], m.corner[1], k)
	capsule := ui.Box(c.Key("capsule")).Absolute().Left(box.X).Top(box.Y).Size(box.W, box.H).Radius(corner).
		Background(a.miniSurface().Alpha(fill)).Border(1, white.Alpha((0.14+0.12*(1-fill))*edge)).
		Shadow(0, 12, 30, -10, ui.RGBA(0, 0, 0, 0.45*fill))
	tr := a.pl.Current()
	discs := m.round[0] >= m.disc[0].W/2-0.5 && m.round[1] >= m.disc[1].W/2-0.5
	switch {
	case tr == nil || cover <= 0:
	case discs && a.spins():
		a.barDisc(c, tr, disc.W, discSmooth).Absolute().Left(disc.X).Top(disc.Y).Opacity(cover)
	default:
		// Its corners go from the one end's to the other's, rather than a
		// disc that turns square as the mini window takes over; inside the
		// capsule, which clips it to its own.
		r := lerp(m.round[0], m.round[1], k)
		if m.square {
			capsule.Clip()
		}
		capsule.Children(func() {
			ui.Box(c.Key("morph-cover")).Absolute().Left(disc.X-box.X).Top(disc.Y-box.Y).Size(disc.W, disc.H).Radius(r).Opacity(cover).
				Shadow(0, 6, 18, -6, ui.RGBA(0, 0, 0, 0.4*cover*(1-b2f(m.square)))).Children(func() {
				a.coverFill(c, tr.Album.PicURL, m.artPx, r)
				if m.square {
					// The square's shade, coming as the cover grows into it.
					near := easeRange(k, 0.45, 1)
					if !m.toMini {
						near = 1 - easeRange(k, 0, 0.55)
					}
					ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Opacity(near).PassThrough().
						Draw(func(p *ui.Painter, rr ui.Rect) { drawSquareShade(p, rr, 0) })
				}
			})
		})
	}
	c.AnimationFrame()
	if t < 1 {
		return
	}
	if m.ending++; m.ending < 3 {
		return
	}
	win, toMini := m.win, m.toMini
	m.win = nil
	// Not from within its own frame.
	a.post(func() {
		win.Hide()
		win.Destroy()
		if toMini {
			a.hideMain()
		} else {
			a.dropFloat(&a.miniWin)
		}
		a.win.SetOpacity(1)
	})
}

// placeMini shows the mini window where it goes.
func (a *app) placeMini() {
	s := &a.settings
	a.makeMini()
	x, y := a.miniSpot(s.MiniX, s.MiniY)
	w, h := a.miniSize()
	a.miniWin.SetOpacity(0)
	a.miniWin.SetBounds(mygo.Rectangle{X: x, Y: y, Width: w, Height: h})
	a.miniWin.Show()
	a.applyMiniLayer()
	if a.morph.win == nil {
		a.miniWin.SetOpacity(1)
	}
}

// makeMini makes the mini window, hidden, if it isn't made.
func (a *app) makeMini() {
	s := &a.settings
	if a.miniWin == nil {
		x, y := a.miniSpot(s.MiniX, s.MiniY)
		w, h := a.miniSize()
		a.miniWin = a.floatWindow(" 迷你窗口", x, y, w, h, a.miniView, a.closeMini)
		// Kept on the desktop, it goes back under the others once used.
		a.miniWin.OnFocus(func() { a.post(a.sinkMini) })
		a.miniWin.OnBlur(func() { a.post(a.sinkMini) })
		a.miniWin.OnMove(func() {
			if a.miniWin != nil && a.miniWin.IsVisible() {
				b := a.miniWin.Bounds()
				s.MiniX, s.MiniY = b.X, b.Y
			}
		})
	}
}

// onShowMini, when set, stands in for showMini, for the tests.
var onShowMini func(*app)

// showMini puts the player in its small window, in place of the main one: the
// bar goes there, and the window around it fades.
func (a *app) showMini() {
	if onShowMini != nil {
		onShowMini(a)
		return
	}
	if a.win == nil || a.morph.win != nil {
		return
	}
	s := &a.settings
	x, y := a.miniSpot(s.MiniX, s.MiniY)
	if !a.startMorph(x, y, true) {
		a.placeMini()
		a.hideMain()
	}
}

// closeMini goes back to the main window: the small one goes back into its bar.
func (a *app) closeMini() {
	if a.morph.win != nil {
		return
	}
	var x, y int
	if a.miniWin != nil {
		b := a.miniWin.Bounds()
		x, y = b.X, b.Y
	}
	if a.miniWin != nil && a.startMorph(x, y, false) {
		a.win.SetOpacity(0)
	} else if a.miniWin != nil {
		a.dropFloat(&a.miniWin)
	}
	a.showMain()
}
