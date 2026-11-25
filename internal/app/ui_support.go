package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// supportEpoch is when the light of the page started drifting.
var supportEpoch = time.Now()

// The support page (支持 Cloudy): a card under 关于 opens it, in place of
// 关于's page, as does a line at the foot of 更新内容. It asks nothing of
// anyone elsewhere: no popup, no dot, nothing given in return.

var (
	supportIconStar = strokeIcon(`<path d="M12 3.2l2.7 5.6 6.1.8-4.5 4.2 1.1 6-5.4-2.9-5.4 2.9 1.1-6-4.5-4.2 6.1-.8z"/>`)
	supportIconTV   = strokeIcon(`<rect x="2.8" y="5.5" width="18.4" height="14" rx="3"/><path d="M7.5 2.8 10 5.5M16.5 2.8 14 5.5M9.5 11v3M14.5 11v3"/>`)
	supportIconNext = strokeIcon(`<path d="m9 6 6 6-6 6"/>`)
)

// supportMethods are the codes the page offers, and the tint of each.
var supportMethods = [2]struct {
	label, app string
	tint       ui.Color
}{
	{"微信", "微信", ui.Hex("#2fb36b")},
	{"支付宝", "支付宝", ui.Hex("#3a86f0")},
}

// supportAfdian is the author's page on 爱发电, where support can be
// monthly and come with a note.
const supportAfdian = "https://ifdian.net/a/nya12138"

// supportBilibili is the author's Bilibili space, where the videos are.
func supportBilibili() string {
	return fmt.Sprintf("https://space.bilibili.com/%d", feedbackBilibiliUID)
}

// supportGlow paints soft rounds of color drifting in r, clipped to its
// corners: the light of the card and of the page's head. t is in seconds.
func (a *app) supportGlow(p *ui.Painter, r ui.Rect, radius float32, strength float32, t float64) {
	accent := a.pal.accent
	warm, cool := ui.Hex("#e2965c"), ui.Hex("#965cb4")
	if !a.pal.dark {
		strength *= 0.7
	}
	blobs := []struct {
		x, y, size float32
		c          ui.Color
		a          float32
		period     float64
	}{
		{0.12, 0.05, 0.62, accent, 0.42, 11},
		{0.55, 0.85, 0.55, warm, 0.26, 14},
		{1.02, 0.0, 0.5, cool, 0.28, 17},
	}
	p.Clip(r, radius, func() {
		for i, b := range blobs {
			ph := t/b.period*2*math.Pi + float64(i)
			x := r.X + r.W*b.x + float32(math.Sin(ph))*18
			y := r.Y + r.H*b.y + float32(math.Cos(ph*0.8))*10
			size := max(r.W, r.H) * b.size * (1 + 0.06*float32(math.Sin(ph*1.3)))
			// A shadow of a dot is a round of light, soft all the way out.
			p.Shadow(ui.Rect{X: x, Y: y, W: 1, H: 1}, 0, 0, 0, size, size*0.12, b.c.Alpha(b.a*strength))
		}
	})
}

// supportMoves says whether the light drifts: in full motion, and not to
// save power.
func (a *app) supportMoves() bool {
	return a.motion == motionFull && !a.settings.PowerSaver
}

// supportCard is the card under 关于 that opens the page.
func (a *app) supportCard(k *settingsKit) {
	c, ink := k.c, k.ink
	wall := a.supportWall()
	row := ui.Row(c).Padding(18, 20).Gap(16).AlignItems(ui.Center)
	row.Draw(func(p *ui.Painter, r ui.Rect) {
		a.supportGlow(p, r, 14, 0.45, p.Now().Sub(supportEpoch).Seconds())
		if a.supportMoves() {
			p.AnimationFrameRate(20)
		}
	})
	row.Children(func() {
		heartTile(c, 52, 14, 24)
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(3).Children(func() {
			ui.Text(c, "支持 Cloudy").FontSize(14).FontWeight(700)
			ui.Text(c, "Cloudy 免费、开源、没有广告。如果它陪你听了很多歌，可以请 nya 喝杯奶茶，让它继续更新下去").
				FontSize(12).TextColor(ink.muted).LineHeight(1.5)
			// Who has, once someone has: nothing while the wall is empty.
			n := len(wall.Supporters)
			if n == 0 {
				return
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Margin(7, 0, 0, 0).Children(func() {
				ui.Row(c).Children(func() {
					for i, s := range wall.Supporters[:min(n, 3)] {
						m := float32(-7)
						if i == 0 {
							m = 0
						}
						a.supportRound(c, s, 22, 10).Margin(0, 0, 0, m)
					}
				})
				ui.Text(c, fmt.Sprintf("已有 %d 位朋友支持过", n)).FontSize(12).TextColor(ink.muted)
			})
		})
		if fillButton(c.Key("support-open"), "去支持", supportIconNext, a.pal.accent, a.pal.onAccent, false).Clicked() {
			a.support.page, a.support.all, a.support.shown = true, false, false
			a.settingsView.scroll[settingsAbout].Y = 0
		}
	})
}

// heartTile is the heart on its round square, in the reds of the app.
func heartTile(c *ui.Context, size, radius, icon float32) {
	ui.Box(c).Size(size, size).Radius(radius).Center().Shrink(0).
		Shadow(0, 6, 18, 0, ui.Hex("#e5484d").Alpha(0.32)).
		Draw(func(p *ui.Painter, r ui.Rect) {
			p.FillGradient(r, ui.LinearGradient{From: ui.Hex("#f07a7e"), To: ui.Hex("#a52a40"), Angle: 145}, radius)
			p.Fill(ui.Rect{X: r.X + radius, Y: r.Y, W: r.W - 2*radius, H: 1}, white.Alpha(0.35), 0)
		}).
		Children(func() {
			ui.Icon(c, iconHeart).Size(icon, icon).TextColor(white)
		})
}

// supportRound is a supporter's picture, or the first letter of the name
// on a round of its color while it has none or it is not read yet, ringed
// in the card's fill. The picture fades in over the letter.
func (a *app) supportRound(c *ui.Context, s supporter, size, font float32) ui.Element {
	bmp := a.supportAvatar(s)
	round := ui.Box(c).Size(size, size).Radius(999).Center().Shrink(0).Background(supportTint(s.Name)).
		Border(2, ui.RGBA(0, 0, 0, 0.12))
	in := artIn(round, bmp)
	return round.Children(func() {
		if in < 1 {
			ui.Text(c, supportInitial(s.Name)).FontSize(font).FontWeight(700).TextColor(white).Opacity(1 - in)
		}
		artImage(c, bmp, size/2, in)
	})
}

// supportCodeFace is a pay code on white, or where the app has none, a
// box that says so.
func supportCodeFace(c *ui.Context, bmp *ui.Bitmap, label string, size float32) {
	ui.Box(c).Size(size, size).Radius(8).Background(white).Center().Children(func() {
		if bmp != nil {
			// A wider white edge than the code's own, for phones to find it.
			ui.Image(c, bmp).Size(size*0.88, size*0.88).Fit(ui.Contain)
			return
		}
		dark := ui.Hex("#1d1d1f")
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			f := size * 0.22
			for _, at := range [][2]float32{{0, 0}, {1, 0}, {0, 1}} {
				x := r.X + 8 + at[0]*(r.W-16-f)
				y := r.Y + 8 + at[1]*(r.H-16-f)
				p.Fill(ui.Rect{X: x, Y: y, W: f, H: f}, dark, 6)
				p.Fill(ui.Rect{X: x + 5, Y: y + 5, W: f - 10, H: f - 10}, white, 3)
				p.Fill(ui.Rect{X: x + 9, Y: y + 9, W: f - 18, H: f - 18}, dark, 2)
			}
		})
		ui.Text(c, label+"收款码\n还没有放").FontSize(12).LineHeight(1.5).TextColor(ui.Hex("#55555a")).TextAlign(ui.Center).Absolute()
	})
}

// supportCodeDialog shows the code chosen large, for a phone held to it.
func (a *app) supportCodeDialog(c *ui.Context) {
	st := &a.support
	if !a.settingsView.open {
		st.large = false
	}
	a.dialog(c, "support-code-large", 0.35, &st.large, func(panel ui.Element) {
		ink := a.inkOn(false)
		m := supportMethods[st.method]
		panel.Width(400).TextColor(ink.text).Label(m.label + "收款码")
		a.dialogPane(panel, 26, false)
		panel.Children(func() {
			dialogHeader(c, ink, "用"+m.app+"扫一扫", "金额随意，谢谢你", func() { st.large = false })
			ui.Row(c).Justify(ui.Center).Padding(4, 28, 0, 28).Children(func() {
				ui.Box(c).Padding(10).Radius(20).Background(m.tint).Children(func() {
					supportCodeFace(c, supportCodes()[st.method], m.label, 300)
				})
			})
			dialogFooter(c, func() {
				other := supportMethods[1-st.method]
				if settingButton(c.Key("switch"), ink, "换成"+other.label, nil, false, false).Clicked() {
					st.method = 1 - st.method
				}
				if primaryButton(c.Key("ok"), "好的", nil, a.pal, false).Clicked() {
					st.large = false
				}
			})
		})
	})
}

// supportLine is the line at the foot of 更新内容 that opens the page.
func (a *app) supportLine(c *ui.Context, ink paneInk, then func()) {
	b := ui.ButtonBase(c.Key("support-line")).Height(32).Gap(6).AlignItems(ui.Center).Grow(1).Shrink(1).MinWidth(0).
		FocusRing(false).Label("支持一下 nya").Cursor(ui.CursorPointer)
	col := ink.muted
	if b.Hovered() {
		col = a.pal.accent
	}
	b.TextColor(b.AnimateColor("ink", col, motionHover)).Children(func() {
		ui.Icon(c, iconHeart).Size(13, 13).TextColor(a.pal.accent)
		ui.Text(c, "喜欢这次更新？支持一下 nya").FontSize(12.5).SingleLine().Shrink(1).MinWidth(0)
		ui.Icon(c, supportIconNext).Size(11, 11)
	})
	if b.Clicked() {
		then()
	}
}

// supportWords are what a search of the settings finds the card by.
var supportWords = strings.Join([]string{"支持", "赞助", "捐赠", "打赏", "感谢墙", "收款码", "奶茶", "Cloudy"}, " ")
