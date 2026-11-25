package app

import (
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The desktop lyrics' more panel: what shows with the line (its
// translation, its reading, the next line), the color they light in, their
// style and their own font. A window of its own beside the lyrics, never
// over them, so that what changes shows as it changes, of the tools' stuff.

const (
	deskMoreW, deskMoreH = 340, 584
	deskMoreEdge         = 18 // around the panel, for its shadow
	deskMoreGap          = 14 // between it and the lyrics
	// deskDefaultFont is DeskFont for the app's default font, which ""
	// isn't: that follows the lyrics' font.
	deskDefaultFont = "default"
	// deskFontRow is how tall a font is in the font list.
	deskFontRow = 36
)

// deskMore is the panel's own state.
type deskMore struct {
	opened   time.Time
	frames   int
	fonts    bool // the font list is open
	pick     bool // the color picker is open
	h, sat   float32
	val      float32
	dragging bool // a color being dragged, saved as it is let go
	names    []string
	at       mygo.Point // where placeDeskMore put it
	list     ui.ScrollState
	// query is the font search; seen what the list was scrolled for.
	query, seen string
	// hex and rgb are the color's fields, as typed while one has the
	// keyboard, else as the color has them.
	hex string
	rgb [3]string
}

// deskColors are the colors the desktop lyrics can light in, besides the
// cover's, the app's and one of the listener's own.
var deskColors = []struct {
	id, label string
	color     ui.Color
}{
	{"sakura", "樱花粉", ui.RGB(255, 138, 176)},
	{"coral", "珊瑚橙", ui.RGB(255, 136, 92)},
	{"lemon", "柠檬黄", ui.RGB(246, 210, 72)},
	{"mint", "薄荷绿", ui.RGB(74, 214, 160)},
	{"cyan", "青碧", ui.RGB(56, 204, 214)},
	{"sky", "天空蓝", ui.RGB(84, 164, 255)},
	{"grape", "葡萄紫", ui.RGB(168, 122, 255)},
	{"rose", "玫瑰红", ui.RGB(240, 72, 104)},
	{"snow", "纯白", ui.RGB(255, 255, 255)},
	{"gold", "香槟金", ui.RGB(222, 186, 120)},
}

// deskColorOf returns the color of id: one of deskColors, or one of the
// listener's own as #rrggbb.
func deskColorOf(id string) (ui.Color, bool) {
	if hex, ok := strings.CutPrefix(id, "#"); ok && len(hex) == 6 {
		if n, err := strconv.ParseUint(hex, 16, 32); err == nil {
			return ui.RGB(uint8(n>>16), uint8(n>>8), uint8(n)), true
		}
	}
	for _, d := range deskColors {
		if d.id == id {
			return d.color, true
		}
	}
	return ui.Color{}, false
}

// hsv returns the color of hue h, saturation s and value v, each 0 to 1.
func deskHSV(h, s, v float32) ui.Color {
	h = float32(math.Mod(float64(h), 1)) * 6
	i := int(h)
	f := h - float32(i)
	p, q, t := v*(1-s), v*(1-s*f), v*(1-s*(1-f))
	var r, g, b float32
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	default:
		r, g, b = v, p, q
	}
	return ui.RGB(uint8(r*255+0.5), uint8(g*255+0.5), uint8(b*255+0.5))
}

// toHSV returns the hue, saturation and value of c, each 0 to 1.
func toHSV(c ui.Color) (h, s, v float32) {
	r, g, b := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255
	hi, lo := max(r, g, b), min(r, g, b)
	v, d := hi, hi-lo
	if hi > 0 {
		s = d / hi
	}
	switch {
	case d == 0:
		h = 0
	case hi == r:
		h = float32(math.Mod(float64((g-b)/d), 6)) / 6
	case hi == g:
		h = ((b-r)/d + 2) / 6
	default:
		h = ((r-g)/d + 4) / 6
	}
	if h < 0 {
		h++
	}
	return h, s, v
}

// toggleDeskMore opens the more panel, or closes it.
func (a *app) toggleDeskMore() {
	if a.desk.moreWin != nil {
		a.closeDeskMore()
		return
	}
	// The press on the button closed it, as one outside it: not opened
	// again by the click it ends in.
	if time.Since(a.desk.moreShut) < 400*time.Millisecond {
		return
	}
	a.openDeskMore()
}

// openDeskMore opens the more panel beside the lyrics. On nothing, as the
// lyrics' own window is: a flyout of the tray's icon where there is one,
// that takes the keyboard, for the font search and the color's numbers,
// and closes as a menu does, on a press outside it or Escape.
func (a *app) openDeskMore() {
	if a.deskWin == nil {
		return
	}
	w, h := deskMoreW+2*deskMoreEdge, deskMoreH+2*deskMoreEdge
	var win *mygo.Window
	if a.tray != nil {
		win = mygo.NewFlyout(mygo.FlyoutOptions{Tray: a.tray, Hidden: true, Focusable: true, Width: w, Height: h, Content: ui.View(a.deskMoreView)})
	} else {
		win = mygo.NewWindow(mygo.WindowOptions{
			Title: mygo.App.Name() + " 桌面歌词设置", Panel: true, Hidden: true, Frameless: true, Transparent: true,
			DisableShadow: true, DisableResize: true, AlwaysOnTop: true, SkipTaskbar: true,
			Width: w, Height: h, Content: ui.View(a.deskMoreView),
		})
	}
	win.OnClose(func(e *mygo.CloseEvent) {
		if !a.quitting {
			e.PreventDefault()
			a.closeDeskMore()
		}
	})
	m := &a.desk.more
	m.opened, m.frames, m.fonts, m.query = time.Now(), 0, false, ""
	c, _ := deskColorOf(a.settings.DeskColor)
	if strings.HasPrefix(a.settings.DeskColor, "#") {
		m.h, m.sat, m.val = toHSV(c)
	} else if m.val == 0 {
		m.h, m.sat, m.val = 0.92, 0.45, 1
	}
	a.desk.moreWin = win
	// A flyout goes back by its anchor, the tray's icon, as windows of the
	// app move or change size (the lyrics' own, as a line is added): put
	// back beside the lyrics then.
	win.OnMove(func() {
		if a.desk.moreWin == win {
			if b := win.Bounds(); (mygo.Point{X: b.X, Y: b.Y}) != a.desk.more.at {
				a.post(a.placeDeskMore)
			}
		}
	})
	win.Show()
	a.placeDeskMore()
	win.SetAlwaysOnTop(true)
}

// placeDeskMore puts the more panel beside the lyrics, where it is open: to
// their right, their left, over their window or under it, the first where
// it is on their screen whole, else where most of it is.
func (a *app) placeDeskMore() {
	win := a.desk.moreWin
	if win == nil || a.deskWin == nil {
		return
	}
	b := a.deskWin.Bounds()
	body := mygo.Rectangle{X: b.X + int(a.desk.bodyAt.X), Y: b.Y + int(a.desk.bodyAt.Y), Width: int(a.desk.bodyAt.W), Height: int(a.desk.bodyAt.H)}
	if body.Width == 0 {
		body = b
	}
	w, h := deskMoreW+2*deskMoreEdge, deskMoreH+2*deskMoreEdge
	wa := mygo.Screen.DisplayNearestPoint(mygo.Point{X: body.X + body.Width/2, Y: body.Y + body.Height/2}).WorkArea
	clampX := func(x int) int { return max(wa.X, min(x, wa.X+wa.Width-w)) }
	clampY := func(y int) int { return max(wa.Y, min(y, wa.Y+wa.Height-h)) }
	cy := clampY(body.Y + body.Height/2 - h/2)
	cx := clampX(body.X + body.Width/2 - w/2)
	type spot struct{ x, y int }
	spots := []spot{
		{body.X + body.Width + deskMoreGap - deskMoreEdge, cy}, // right
		{body.X - deskMoreGap - w + deskMoreEdge, cy},          // left
		{cx, b.Y - h + deskMoreEdge - 4},                       // over the window, tools and all
		{cx, b.Y + b.Height - deskMoreEdge + 4},                // under it
	}
	best, bestIn := spots[0], -1
	for _, s := range spots {
		// How much of it is on the screen.
		in := max(0, min(s.x+w, wa.X+wa.Width)-max(s.x, wa.X)) * max(0, min(s.y+h, wa.Y+wa.Height)-max(s.y, wa.Y))
		if in == w*h {
			best = s
			break
		}
		if in > bestIn {
			best, bestIn = s, in
		}
	}
	a.desk.more.at = mygo.Point{X: clampX(best.x), Y: clampY(best.y)}
	win.SetPosition(a.desk.more.at.X, a.desk.more.at.Y)
}

// closeDeskMore closes the more panel, if open.
func (a *app) closeDeskMore() {
	if a.desk.moreWin == nil {
		return
	}
	a.desk.moreShut = time.Now()
	if a.desk.more.dragging {
		a.desk.more.dragging = false
		a.changed()
	}
	a.dropFloat(&a.desk.moreWin)
	if a.deskWin != nil {
		a.deskWin.Invalidate()
	}
}

// deskMoreSet saves a change from the panel and shows it on the lyrics: the
// window grown or shrunk too if fit, for the lines it shows.
func (a *app) deskMoreSet(fit bool) {
	a.changed()
	if fit {
		a.fitDeskSize()
	}
	if a.deskWin != nil {
		a.deskWin.Invalidate()
	}
}

// deskMoreView is the more panel: a card of the tools' stuff, its head and
// its groups one under another, scrolling if they don't fit.
func (a *app) deskMoreView(c *ui.Context) {
	a.floatTheme(c)
	s, m := &a.settings, &a.desk.more
	lamp := a.deskLookOf("glass").sung
	fade := ui.Animation{Duration: 140 * time.Millisecond}
	lines, _ := a.shownLyrics()
	hasTrans, hasRoman := false, false
	for _, l := range lines {
		hasTrans = hasTrans || l.Translation != ""
		hasRoman = hasRoman || l.Roman != ""
	}
	// It comes in from a little lower and smaller, once.
	m.frames++
	pose := ui.Pose{}
	if m.frames <= 1 {
		pose = ui.Pose{Y: 10, Scale: 0.97, Fade: 1}
		c.AnimationFrame()
	}

	ui.Box(c.Key("edge")).Fill().Padding(deskMoreEdge).Children(func() {
		card := ui.Column(c.Key("card")).Fill().Radius(26).Background(a.miniSurface().Alpha(0.97)).
			Border(1, white.Alpha(0.12)).Shadow(0, 12, 32, -10, ui.RGBA(0, 0, 0, 0.6)).Clip()
		card.AnimatePose(pose, ui.Animation{Spring: ui.SpringOptions{Duration: 320 * time.Millisecond, Bounce: 0.14}})
		card.Children(func() {
			// A glow of the lyrics' color at the top, faint.
			ui.Box(c.Key("glow")).Absolute().Left(0).Right(0).Top(0).Height(120).
				Gradient(lamp.Alpha(0.16), lamp.Alpha(0), 180).PassThrough()
			a.deskMoreHead(c, lamp)
			ui.Scroll(c.Key("body")).Grow(1).Padding(2, 14, 16, 14).Gap(4).Children(func() {
				note := func(says string, has bool, what string) string {
					if has || len(lines) == 0 {
						return says
					}
					return "当前歌曲没有" + what
				}
				a.deskGroup(c, "外语歌词", func(rule func()) {
					a.deskToggle(c, "trans", "意译", note("中文翻译，显示在原文下方（竖排在左侧）", hasTrans, "翻译"), &s.DeskTrans, true, lamp)
					rule()
					a.deskToggle(c, "roman", "音译", note("按读音标在原文上方（竖排在右侧），可与意译同时显示", hasRoman, "音译"), &s.DeskRoman, true, lamp)
				})
				a.deskGroup(c, "显示", func(rule func()) {
					a.deskToggle(c, "next", "下一句", "没有翻译时在下方预告下一句", &s.DeskNext, false, lamp)
					rule()
					a.deskToggle(c, "words", "逐字高亮", "唱到哪个字亮到哪个字", &s.DeskWords, false, lamp)
					rule()
					a.deskToggle(c, "hide", "停止播放时隐藏", "继续播放时自动出现", &s.DeskHideStopped, false, lamp)
				})
				a.deskGroup(c, "样式", func(func()) { a.deskStylePicks(c, lamp) })
				a.deskGroup(c, "配色", func(func()) { a.deskSwatches(c, lamp) })
				a.deskGroup(c, "字体", func(func()) { a.deskFontSelect(c, lamp) })
				_ = fade
			})
		})
	})
}

// deskMoreHead is the panel's head: what it is, over what it does, and a
// button that closes it.
func (a *app) deskMoreHead(c *ui.Context, lamp ui.Color) {
	fade := ui.Animation{Duration: 140 * time.Millisecond}
	ui.Row(c.Key("head")).Padding(16, 14, 10, 16).Gap(11).AlignItems(ui.Center).Children(func() {
		ui.Box(c.Key("tile")).Size(34, 34).Radius(8).Center().Background(lamp.Alpha(0.18)).
			Border(1, lamp.Alpha(0.3)).TextColor(lamp).Children(func() {
			ui.Icon(c, iconLyrics).Size(17, 17)
		})
		ui.Column(c.Key("titles")).Grow(1).Gap(2).Children(func() {
			ui.Text(c, "桌面歌词").FontSize(15).FontWeight(700).TextColor(white)
			ui.Text(c, "改动会立刻显示在歌词上").FontSize(11).TextColor(white.Alpha(0.5))
		})
		b := ui.ButtonBase(c.Key("close")).Size(28, 28).Radius(14).Center().FocusRing(false).Label("关闭")
		bg, fg := white.Alpha(0.06), white.Alpha(0.7)
		if b.Hovered() {
			bg, fg = white.Alpha(0.14), white
		}
		b.Background(b.AnimateColor("bg", bg, fade)).TextColor(b.AnimateColor("fg", fg, fade)).Children(func() {
			ui.Icon(c, iconX).Size(11, 11)
		})
		if b.Clicked() {
			a.post(a.closeDeskMore)
		}
	})
}

// deskGroup is a group of the panel: its heading over a card of what it
// holds, rule drawing a hairline between two of its rows.
func (a *app) deskGroup(c *ui.Context, heading string, body func(rule func())) {
	ui.Column(c.Key("group-"+heading)).Gap(7).Margin(10, 0, 0, 0).Children(func() {
		ui.Text(c, heading).FontSize(11.5).FontWeight(700).TextColor(white.Alpha(0.5)).Margin(0, 4, 0, 4)
		card := ui.Column(c.Key("card")).Radius(14).Background(white.Alpha(0.045)).Border(1, white.Alpha(0.07)).Padding(4)
		card.Children(func() {
			n := 0
			body(func() {
				n++
				ui.Box(c.Key(n)).Height(1).Margin(0, 12).Background(white.Alpha(0.06))
			})
		})
	})
}

// deskToggle is a row of a group that turns a setting on or off.
func (a *app) deskToggle(c *ui.Context, key, label, note string, on *bool, fit bool, lamp ui.Color) {
	fade := ui.Animation{Duration: 140 * time.Millisecond}
	b := ui.ButtonBase(c.Key(key)).Padding(9, 10).Gap(12).Radius(14).AlignItems(ui.Center).FocusRing(false).Label(label)
	bg := ui.Transparent
	if b.Hovered() {
		bg = white.Alpha(0.05)
	}
	b.Background(b.AnimateColor("bg", bg, fade)).Children(func() {
		ui.Column(c.Key("words")).Grow(1).Shrink(1).MinWidth(0).Gap(3).PassThrough().Children(func() {
			ui.Text(c, label).FontSize(13).FontWeight(600).TextColor(white.Alpha(0.94))
			if note != "" {
				ui.Text(c, note).FontSize(11).TextColor(white.Alpha(0.48))
			}
		})
		a.deskSwitch(c, *on, lamp)
	})
	if b.Clicked() {
		*on = !*on
		a.deskMoreSet(fit)
	}
}

// deskSwitch is a switch, on or off, lit in lamp.
func (a *app) deskSwitch(c *ui.Context, on bool, lamp ui.Color) {
	fade := ui.Animation{Duration: 180 * time.Millisecond}
	track := ui.Box(c.Key("switch")).Size(38, 22).Radius(999).Shrink(0).PassThrough()
	bg, edge, x := white.Alpha(0.12), white.Alpha(0.1), float32(0)
	if on {
		bg, edge, x = lamp, lamp.Mix(white, 0.3), 16
	}
	track.Background(track.AnimateColor("bg", bg, fade)).Border(1, track.AnimateColor("edge", edge, fade)).Children(func() {
		ui.Box(c.Key("knob")).Size(18, 18).Radius(999).Absolute().Left(1).Top(1).Background(white).
			Shadow(0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.35)).PassThrough().
			AnimatePose(ui.Pose{X: x}, ui.Animation{Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: 0.22}})
	})
}
