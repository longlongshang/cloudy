package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// The more panel's choices: the style, the color, a color of one's own,
// and the font.

// deskChoice is a capsule among choices, filled as fill has it, its label
// in ink; chosen, ringed and checked.
func (a *app) deskChoice(c *ui.Context, key, label string, chosen bool, fill func(ui.Element), ink ui.Color, do func()) {
	fade := ui.Animation{Duration: 140 * time.Millisecond}
	b := ui.ButtonBase(c.Key(key)).Height(32).Grow(1).Basis(0).Radius(999).Center().Gap(5).FocusRing(false).Label(label)
	fill(b)
	ring, width := white.Alpha(0.1), float32(1)
	switch {
	case chosen:
		ring, width = white, 2
	case b.Hovered():
		ring = white.Alpha(0.45)
	}
	press := ui.Pose{}
	if b.Pressed() {
		press.Scale = 0.95
	}
	b.Border(width, b.AnimateColor("ring", ring, fade)).AnimatePose(press, ui.Snappy).Children(func() {
		if chosen {
			ui.Icon(c, iconCheck).Size(11, 11).TextColor(ink)
		}
		ui.Text(c, label).FontSize(11.5).FontWeight(700).TextColor(ink).SingleLine()
	})
	if b.Clicked() {
		do()
	}
}

// deskChoiceRows lays n choices out three to a row, each as choice has it.
func deskChoiceRows(c *ui.Context, key string, n int, choice func(i int)) {
	ui.Column(c.Key(key)).Gap(7).Padding(8).Children(func() {
		for row := 0; row < n; row += 3 {
			ui.Row(c.Key(row)).Gap(7).Children(func() {
				for i := row; i < row+3; i++ {
					if i < n {
						choice(i)
					} else {
						ui.Box(c.Key(i)).Grow(1).Basis(0)
					}
				}
			})
		}
	})
}

// deskStylePicks are the lyrics' styles, a capsule each.
func (a *app) deskStylePicks(c *ui.Context, lamp ui.Color) {
	s := &a.settings
	deskChoiceRows(c, "styles", len(deskStyles), func(i int) {
		d := deskStyles[i]
		chosen := s.DeskStyle == d.id || (s.DeskStyle == "" && d.id == "glass")
		a.deskChoice(c, "style-"+d.id, d.label, chosen, func(e ui.Element) {
			if chosen {
				e.Background(lamp.Alpha(0.2))
			} else {
				e.Background(white.Alpha(0.06))
			}
		}, white.Alpha(0.92), func() {
			s.DeskStyle = d.id
			a.deskMoreSet(false)
		})
	})
}

// inkOn returns the color words read in on c.
func inkOn(c ui.Color) ui.Color {
	if luminance(c) > 0.5 {
		return ui.RGBA(18, 18, 22, 0.88)
	}
	return white
}

// deskSwatches are the colors the lyrics can light in, each a capsule of
// itself with its name in it, three to a row, and the picker of one's own
// under them while it is open.
func (a *app) deskSwatches(c *ui.Context, lamp ui.Color) {
	s, m := &a.settings, &a.desk.more
	cover := a.pal.accent
	if accent, ok := a.coverAccent(); ok {
		cover = accent
	}
	type swatch struct {
		id, label string
		color     ui.Color
	}
	all := []swatch{{"", "跟随封面", cover}, {"accent", "主题色", a.pal.accent}}
	for _, d := range deskColors {
		all = append(all, swatch{d.id, d.label, d.color})
	}
	own := deskHSV(m.h, m.sat, m.val)
	all = append(all, swatch{"custom", "自定义", own})
	ui.Column(c.Key("colors")).Children(func() {
		deskChoiceRows(c, "swatches", len(all), func(i int) {
			sw := all[i]
			if sw.id == "custom" {
				a.deskChoice(c, "sw-custom", "自定义", strings.HasPrefix(s.DeskColor, "#"), func(e ui.Element) {
					e.LinearGradient(ui.LinearGradient{From: own, To: own.Mix(white, 0.25), Angle: 90})
				}, inkOn(own), func() {
					m.pick = !m.pick
					s.DeskColor = hexOf(own)
					a.deskMoreSet(false)
				})
				return
			}
			fill := func(e ui.Element) { e.Background(sw.color) }
			if sw.id == "" {
				// The cover's, as it changes from song to song.
				fill = func(e ui.Element) {
					e.LinearGradient(ui.LinearGradient{From: cover, To: cover.Mix(white, 0.35), Angle: 90})
				}
			}
			a.deskChoice(c, "sw-"+sw.id, sw.label, s.DeskColor == sw.id, fill, inkOn(sw.color), func() {
				s.DeskColor = sw.id
				m.pick = false
				a.deskMoreSet(false)
			})
		})
		if m.pick {
			a.deskColorPicker(c)
		}
	})
}

// deskColorPicker picks a color of one's own, as the pickers everyone knows
// do: its saturation across and brightness up a square of its hue, its hue
// on a rainbow under it, and the color with its hex and its red, green and
// blue, each to type.
func (a *app) deskColorPicker(c *ui.Context) {
	s, m := &a.settings, &a.desk.more
	color := deskHSV(m.h, m.sat, m.val)
	pressed := false
	set := func(save bool) {
		s.DeskColor = hexOf(deskHSV(m.h, m.sat, m.val))
		if save {
			a.changed()
		}
		if a.deskWin != nil {
			a.deskWin.Invalidate()
		}
	}
	knob := func(p *ui.Painter, x, y, r float32, fill ui.Color) {
		p.Fill(ui.Rect{X: x - r - 1, Y: y - r + 0.5, W: 2*r + 2, H: 2*r + 2}, ui.RGBA(0, 0, 0, 0.28), r+1)
		k := ui.Rect{X: x - r, Y: y - r, W: 2 * r, H: 2 * r}
		p.Fill(k, fill, r)
		p.Stroke(k, white, r, 2.5)
	}
	ui.Column(c.Key("picker")).Gap(12).Margin(0, 8, 8, 8).Padding(12).Radius(14).Background(ui.RGBA(0, 0, 0, 0.2)).
		Border(1, white.Alpha(0.06)).Children(func() {
		// The square: white into the hue across, all into black down.
		sq := ui.Box(c.Key("sv")).Height(150).Cursor(ui.CursorCrosshair)
		if sq.Pressed() {
			pressed = true
			x, y, _ := sq.PointerPosition()
			if r := sq.LayoutBounds(); r.W > 0 && r.H > 0 {
				m.sat, m.val = clamp01(x/r.W), 1-clamp01(y/r.H)
				set(false)
			}
		}
		sq.Draw(func(p *ui.Painter, r ui.Rect) {
			p.Fill(r, deskHSV(m.h, 1, 1), 10)
			p.FillGradient(r, ui.LinearGradient{From: white, To: white.Alpha(0), Angle: 90}, 10)
			p.FillGradient(r, ui.LinearGradient{From: ui.RGBA(0, 0, 0, 0), To: ui.RGB(0, 0, 0), Angle: 180}, 10)
			knob(p, r.X+m.sat*r.W, r.Y+(1-m.val)*r.H, 8, color)
		})
		// The hue, a rainbow around the wheel.
		bar := ui.Box(c.Key("hue")).Height(18).Cursor(ui.CursorPointer)
		if bar.Pressed() {
			pressed = true
			x, _, _ := bar.PointerPosition()
			if w := bar.LayoutBounds().W; w > 0 {
				m.h = min(clamp01(x/w), 0.999)
				set(false)
			}
		}
		bar.Draw(func(p *ui.Painter, r ui.Rect) {
			strip := ui.Rect{X: r.X, Y: r.Y + 3, W: r.W, H: r.H - 6}
			p.Clip(strip, strip.H/2, func() {
				w := strip.W / 6
				for i := range 6 {
					seg := ui.Rect{X: strip.X + float32(i)*w - 0.5, Y: strip.Y, W: w + 1, H: strip.H}
					p.FillGradient(seg, ui.LinearGradient{From: deskHSV(float32(i)/6, 1, 1), To: deskHSV(float32(i+1)/6, 1, 1), Angle: 90}, 0)
				}
			})
			p.Stroke(strip, white.Alpha(0.12), strip.H/2, 1)
			knob(p, r.X+m.h*r.W, r.Y+r.H/2, 9, deskHSV(m.h, 1, 1))
		})
		// The color, and its numbers to type.
		ui.Row(c.Key("fields")).Gap(8).AlignItems(ui.End).Children(func() {
			ui.Box(c.Key("chip")).Size(30, 30).Radius(8).Background(color).Border(1, white.Alpha(0.25)).Shrink(0)
			typing := false
			a.deskField(c, "hex", "HEX", &m.hex, 2.2, &typing, func(v string) {
				if col, ok := deskColorOf("#" + strings.TrimPrefix(strings.TrimSpace(v), "#")); ok {
					m.h, m.sat, m.val = toHSVKeep(col, m.h)
					set(true)
				}
			})
			for i, name := range []string{"R", "G", "B"} {
				a.deskField(c, name, name, &m.rgb[i], 1, &typing, func(v string) {
					n, err := strconv.Atoi(strings.TrimSpace(v))
					if err != nil || n < 0 || n > 255 {
						return
					}
					col := deskHSV(m.h, m.sat, m.val)
					switch i {
					case 0:
						col.R = uint8(n)
					case 1:
						col.G = uint8(n)
					default:
						col.B = uint8(n)
					}
					m.h, m.sat, m.val = toHSVKeep(col, m.h)
					set(true)
				})
			}
			if !typing {
				// Not typed in: the fields show the color, for the next frame.
				m.syncFields(deskHSV(m.h, m.sat, m.val))
			}
		})
	})
	// Saved once it is let go, not at every step of a drag.
	if pressed {
		m.dragging = true
	} else if m.dragging {
		m.dragging = false
		a.changed()
	}
}

// syncFields shows color in the picker's fields.
func (m *deskMore) syncFields(color ui.Color) {
	m.hex = strings.TrimPrefix(hexOf(color), "#")
	m.rgb = [3]string{strconv.Itoa(int(color.R)), strconv.Itoa(int(color.G)), strconv.Itoa(int(color.B))}
}

// deskField is a field of the color picker, its name over it, grow wide
// among the others. While it has the keyboard it sets *typing, and what is
// typed in it goes to typed.
func (a *app) deskField(c *ui.Context, key, name string, value *string, grow float32, typing *bool, typed func(string)) {
	ui.Column(c.Key("field-" + key)).Grow(grow).Basis(0).MinWidth(0).Gap(4).Children(func() {
		ui.Text(c, name).FontSize(10).FontWeight(700).TextColor(white.Alpha(0.45)).TextAlign(ui.Center)
		box := ui.Row(c.Key("box")).Height(30).Padding(0, 6).Radius(8).AlignItems(ui.Center).Background(white.Alpha(0.07))
		box.Children(func() {
			before := *value
			in := ui.TextInputBase(c.Key("in"), value).Grow(1).MinWidth(0).FontSize(12.5).TextColor(white).
				TextAlign(ui.Center).Label(name)
			if in.Focused() {
				*typing = true
				box.Border(1, white.Alpha(0.5))
				if *value != before {
					typed(*value)
				}
			} else {
				box.Border(1, white.Alpha(0.08))
			}
		})
	})
}

// deskFontSelect is the desktop lyrics' font: a select showing the font
// chosen in itself, that opens a search and a list of the fonts on offer
// and the system's, each in itself.
func (a *app) deskFontSelect(c *ui.Context, lamp ui.Color) {
	s, m := &a.settings, &a.desk.more
	fade := ui.Animation{Duration: 140 * time.Millisecond}
	label, family := "跟随歌词字体", a.deskFamily()
	switch {
	case s.DeskFont == deskDefaultFont:
		label = "默认"
	case s.DeskFont != "":
		label = fontOf(s.DeskFont).label
	}
	ui.Column(c.Key("font")).Padding(6).Gap(6).Children(func() {
		b := ui.ButtonBase(c.Key("select")).Height(44).Padding(0, 14).Gap(10).Radius(14).AlignItems(ui.Center).FocusRing(false).Label("桌面歌词字体")
		bg := white.Alpha(0.06)
		if b.Hovered() || m.fonts {
			bg = white.Alpha(0.1)
		}
		b.Background(b.AnimateColor("bg", bg, fade)).Border(1, white.Alpha(0.08)).Children(func() {
			ui.Text(c, "Aa").Font(family).FontSize(15).FontWeight(700).TextColor(lamp).Width(26)
			ui.Text(c, label).Font(family).FontSize(13.5).FontWeight(600).TextColor(white).Grow(1).Shrink(1).MinWidth(0).SingleLine()
			turn := float32(0)
			if m.fonts {
				turn = 180
			}
			ui.Icon(c, iconDown).Size(12, 12).TextColor(white.Alpha(0.6)).
				AnimatePose(ui.Pose{Rotate: turn}, ui.Animation{Duration: 200 * time.Millisecond})
		})
		if b.Clicked() {
			m.fonts = !m.fonts
			m.query = ""
			if m.fonts && m.names == nil {
				m.names = systemFontNames()
			}
		}
		if !m.fonts {
			ui.Text(c, "只用于桌面歌词，不影响播放页").FontSize(11).TextColor(white.Alpha(0.42)).Margin(0, 4)
			return
		}
		// The search, over the list.
		field := ui.Row(c.Key("search")).Height(34).Padding(0, 10).Gap(7).Radius(8).AlignItems(ui.Center).Background(white.Alpha(0.07))
		field.Children(func() {
			ui.Icon(c, iconSearch).Size(13, 13).TextColor(white.Alpha(0.5))
			in := ui.TextInputBase(c.Key("query"), &m.query).Grow(1).MinWidth(0).FontSize(12.5).TextColor(white).
				Placeholder("搜索字体").Label("搜索字体").AutoFocus()
			if in.Focused() {
				field.Border(1, lamp.Alpha(0.7))
			} else {
				field.Border(1, white.Alpha(0.08))
			}
			if m.query != "" && iconButton(c.Key("clear"), iconX, 10, white.Alpha(0.6), "清除搜索").Size(20, 20).Clicked() {
				m.query = ""
			}
		})
		a.deskFontList(c, lamp)
	})
}

// A deskFontItem is a font of the list: a heading, or a font to choose.
type deskFontItem struct {
	id, label, family, heading string
}

// deskFontList is the list of fonts under the search: those on offer, then
// the system's, those the search finds, only those in view laid out, each
// drawn in its own font, which the system loads the first time.
func (a *app) deskFontList(c *ui.Context, lamp ui.Color) {
	s, m := &a.settings, &a.desk.more
	query := strings.ToLower(strings.TrimSpace(m.query))
	if query != m.seen {
		m.seen, m.list.Y = query, 0
	}
	finds := func(label string) bool { return query == "" || strings.Contains(strings.ToLower(label), query) }
	var offered []deskFontItem
	if finds("跟随歌词字体") {
		offered = append(offered, deskFontItem{id: "", label: "跟随歌词字体", family: a.lyricFamily()})
	}
	for _, f := range a.fontChoices() {
		id := f.id
		if id == "" {
			id = deskDefaultFont
		}
		if !finds(f.label) {
			continue
		}
		family := ""
		if f.id == "" || a.fontsLoaded[f.id] || (len(f.files) == 0 && f.imported == "") {
			family = familyOf(f.id)
		}
		offered = append(offered, deskFontItem{id: id, label: f.label, family: family})
	}
	var system []deskFontItem
	for _, name := range m.names {
		if finds(name) {
			system = append(system, deskFontItem{id: systemFontPrefix + name, label: name, family: name})
		}
	}
	var items []deskFontItem
	if len(offered) > 0 {
		items = append(append(items, deskFontItem{heading: "常用"}), offered...)
	}
	if len(system) > 0 {
		items = append(append(items, deskFontItem{heading: fmt.Sprintf("本机字体（%d）", len(system))}), system...)
	}
	const listH = 240
	first := max(0, int(m.list.Y/deskFontRow)-4)
	last := min(len(items), int((m.list.Y+listH)/deskFontRow)+5)
	first = min(first, last)
	fade := ui.Animation{Duration: 120 * time.Millisecond}
	ui.Scroll(c.Key("fonts")).Height(listH).TrackScroll(&m.list).Radius(14).Background(ui.RGBA(0, 0, 0, 0.16)).Padding(4).Children(func() {
		if len(items) == 0 {
			ui.Text(c, "没有找到匹配的字体").FontSize(12).TextColor(white.Alpha(0.45)).Margin(16, 12)
			return
		}
		ui.Box(c.Key("before")).Height(float32(first * deskFontRow))
		for i := first; i < last; i++ {
			it := items[i]
			if it.heading != "" {
				ui.Row(c.Key(i)).Height(deskFontRow).Padding(0, 10).AlignItems(ui.End).Children(func() {
					ui.Text(c, it.heading).FontSize(10.5).FontWeight(700).TextColor(white.Alpha(0.42)).Margin(0, 0, 6, 0)
				})
				continue
			}
			chosen := s.DeskFont == it.id
			b := ui.ButtonBase(c.Key(i)).Height(deskFontRow).Padding(0, 10).Gap(8).Radius(8).AlignItems(ui.Center).FocusRing(false).Label(it.label)
			bg := ui.Transparent
			switch {
			case chosen:
				bg = lamp.Alpha(0.18)
			case b.Hovered():
				bg = white.Alpha(0.07)
			}
			b.Background(b.AnimateColor("bg", bg, fade)).Children(func() {
				t := ui.Text(c, it.label).FontSize(13.5).TextColor(white.Alpha(0.92)).Grow(1).Shrink(1).MinWidth(0).SingleLine()
				if it.family != "" {
					t.Font(it.family)
				}
				if chosen {
					ui.Icon(c, iconCheck).Size(12, 12).TextColor(lamp)
				}
			})
			if b.Clicked() {
				if f := fontOf(it.id); f.id != "" {
					a.loadFontOf(f.id)
				}
				s.DeskFont = it.id
				a.deskMoreSet(false)
			}
		}
		ui.Box(c.Key("after")).Height(float32((len(items) - last) * deskFontRow))
	})
}

// toHSVKeep is toHSV, keeping hue h where c is grey and has none.
func toHSVKeep(c ui.Color, h float32) (float32, float32, float32) {
	nh, s, v := toHSV(c)
	if s == 0 {
		nh = h
	}
	return nh, s, v
}
