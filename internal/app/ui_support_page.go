package app

import (
	"bytes"
	"fmt"
	"image"
	"log"
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// The support page, as laid out in 1.3.8: a head that says it in a word
// and shows what came of it, the code kept covered until asked for, the
// wall, and what helps besides money, as tiles. Fewer boxes than before,
// each of its own weight.

var (
	supportIconEye = strokeIcon(`<path d="M2.1 12.3a1 1 0 0 1 0-.6C3.6 7.6 7.5 5 12 5s8.4 2.6 9.9 6.7a1 1 0 0 1 0 .6C20.4 16.4 16.5 19 12 19s-8.4-2.6-9.9-6.7"/><circle cx="12" cy="12" r="3"/>`)
	supportIconGo  = strokeIcon(`<path d="M7 17 17 7"/><path d="M8 7h9v9"/>`)
)

// supportReveal is how the code comes out from under its cover.
var supportReveal = ui.Animation{Duration: 420 * time.Millisecond, Ease: ui.CubicBezier(0.22, 1, 0.36, 1)}

// supportPage is the page: why, what came of it, the codes, the wall, and
// what helps besides money.
func (a *app) supportPage(k *settingsKit) {
	c, ink := k.c, k.ink
	st := &a.support
	wall := a.supportWall()

	back := ui.ButtonBase(c.Key("support-back")).Height(28).PaddingX(10).Gap(4).Radius(999).AlignItems(ui.Center).
		AlignSelf(ui.Start).FocusRing(false).Label("返回关于").Margin(0, 0, 10, -6)
	bg := ui.Color{}
	if back.Hovered() {
		bg = ink.hover
	}
	back.Background(back.AnimateColor("bg", bg, motionHover)).TextColor(ink.muted).Children(func() {
		ui.Icon(c, iconBack).Size(14, 14)
		ui.Text(c, "关于").FontSize(13)
	})
	if back.Clicked() {
		st.page = false
	}

	a.supportHead(c, ink, wall)
	a.supportPay(k, ink)

	// The wall.
	supportSection(c, ink, "感谢墙", "只列同意公开的朋友")
	fill, edge := a.cardColors()
	ui.Column(c).Padding(24, 26, 24, 26).Gap(18).Radius(26).Background(fill).Border(0.5, edge).Children(func() {
		seated := wall.seated()
		seats, next := supportSeats(seated)
		ui.Row(c).Gap(20).AlignItems(ui.Center).Children(func() {
			a.supportWallTitle(c, ink, len(seated), seats, 24)
			a.supportTicket(c, ink, next, 34)
		})
		a.supportSeatMap(c, ink, seated, seats, next, 13)
		ui.Column(c).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Baseline).Children(func() {
				supportNum(c, fmt.Sprintf("No.001 – %03d", supportBlock), 15).Grow(1)
				if len(seated) > 0 {
					ui.Text(c, "最早的在前").FontSize(12).TextColor(ink.muted)
				}
			})
			if len(seated) == 0 {
				ui.Text(c, "还没有人入座。第一个支持的朋友，会坐在 No.001。").FontSize(13).TextColor(ink.muted).Margin(2, 0, 4, 0)
				return
			}
			a.supportSeatRows(c, ink, seated[:min(len(seated), supportWallRows)], false)
			if len(seated) > supportWallRows {
				ui.Row(c).Justify(ui.Center).Margin(4, 0, 0, 0).Children(func() {
					b := k.button("support-all", fmt.Sprintf("查看全部 %d 位", len(seated)), nil, false, false)
					if b.Clicked() {
						a.pressedFrom(b, 999)
						st.all = true
					}
				})
			}
		})
	})

	// What helps besides money.
	supportSection(c, ink, "不付钱也能帮忙", "")
	ui.Grid(c).Columns(3).GapX(10).Children(func() {
		a.supportTile(c, ink, "support-star", "给仓库点个 Star", supportIconStar, ui.Hex("#e3a53a"), func() { openLink(appSource) })
		a.supportTile(c, ink, "support-bilibili", "在 B 站一键三连", supportIconTV, ui.Hex("#f0679a"), func() { openLink(supportBilibili()) })
		a.supportTile(c, ink, "support-feedback", "告诉我哪里不好用", iconComment, a.pal.accent, func() {
			st.page = false
			a.settingsView.scroll[settingsAbout].Y = math.MaxFloat32
		})
	})

	// The fine print, once, at the foot.
	ui.Row(c).Justify(ui.Center).AlignItems(ui.Center).Gap(6).Margin(30, 0, 6, 0).Children(func() {
		ui.Icon(c, iconHeart).Size(11, 11).TextColor(a.pal.accent.Alpha(0.8))
		ui.Text(c, "Cloudy 与网易云音乐无关 · 赞助不换取任何功能，以后也不会").FontSize(12).TextColor(ink.faint)
	})
}

// supportNya is nya's own picture, the one on GitHub and in the middle of
// the codes (support/nya.jpg), nil where the app has none.
var supportNya = sync.OnceValue(func() *ui.Bitmap {
	b, err := supportFiles.ReadFile("support/nya.jpg")
	if err != nil {
		return nil
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		log.Printf("support: nya.jpg: %v", err)
		return nil
	}
	return ui.NewBitmap(img)
})

// nyaPortrait is nya's picture beside what nya says, as it signs it: a
// rounded square, as the picture's own, lifted off the card. The heart
// stands in where the app has no picture.
func nyaPortrait(c *ui.Context, size float32) {
	bmp := supportNya()
	if bmp == nil {
		heartTile(c, size, size*0.3, size*0.45)
		return
	}
	radius := size * 0.26
	ui.Box(c).Size(size, size).Radius(radius).Shrink(0).
		Shadow(0, 10, 26, -4, ui.RGBA(0, 0, 0, 0.32)).
		Border(0.5, white.Alpha(0.14)).
		Children(func() { artImage(c, bmp, radius, 1) }).
		Label("nya")
}

// supportSection is the quiet title over a part of the page, and a note
// at its right.
func supportSection(c *ui.Context, ink paneInk, title, note string) {
	ui.Row(c).Margin(30, 6, 10, 6).AlignItems(ui.Baseline).Children(func() {
		ui.Text(c, title).FontSize(13).FontWeight(650).TextColor(ink.text.Alpha(0.72)).Grow(1)
		if note != "" {
			ui.Text(c, note).FontSize(12).TextColor(ink.faint)
		}
	})
}

// supportHead is the page's head: what it asks in a word, nya's own, and
// under a line, what came of it and who has helped.
func (a *app) supportHead(c *ui.Context, ink paneInk, wall *supportData) {
	accent := a.pal.accent
	fill, edge := a.cardColors()
	head := ui.Column(c.Key("support-head")).Padding(30, 32, 24, 32).Radius(26).Background(fill).Border(0.5, edge)
	head.Draw(func(p *ui.Painter, r ui.Rect) {
		a.supportGlow(p, r, 26, 1, p.Now().Sub(supportEpoch).Seconds())
		if a.supportMoves() {
			p.AnimationFrameRate(30)
		}
	})
	head.Children(func() {
		ui.Row(c).Gap(24).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, iconHeart).Size(13, 13).TextColor(accent)
					ui.Text(c, "支持 Cloudy").FontSize(12.5).FontWeight(650).LetterSpacing(0.6).TextColor(accent)
				})
				ui.Text(c, "让 Cloudy 一直唱下去").FontSize(30).FontWeight(800).LetterSpacing(-0.6).LineHeight(1.25).Margin(10, 0, 0, 0)
				ui.Text(c, "Cloudy 是我业余时间一点点做出来的，一直免费、开源、没有广告。你的支持会变成更多的更新、更快的修复，也让我知道，有人在用它听歌。").
					FontSize(13.5).LineHeight(1.8).TextColor(ink.text.Alpha(0.8)).MaxWidth(470).Margin(12, 0, 0, 0)
				ui.Text(c, "— nya").FontSize(12.5).FontWeight(500).TextColor(ink.muted).Margin(10, 0, 0, 0)
			})
			nyaPortrait(c, 100)
		})
		ui.Box(c).Height(0.5).Background(ink.line).Margin(24, 0, 18, 0)
		ui.Row(c).Gap(22).AlignItems(ui.Center).Children(func() {
			// What came of it: the updates, and from GitHub, the issues
			// solved and the pull requests merged, each a way there.
			type stat struct {
				n           int
				label, link string
			}
			issues, prs := a.supportWorkCounts(wall)
			var stats []stat
			for _, s := range []stat{
				{wall.Updates, "次更新", ""},
				{wall.Feedback, "条反馈", ""},
				{issues, "Issue 已解决", supportIssuesPage},
				{prs, "PR 已合并", supportPRsPage},
			} {
				if s.n > 0 {
					stats = append(stats, s)
				}
			}
			for i, s := range stats {
				if i > 0 {
					ui.Box(c).Width(0.5).Height(30).Background(ink.line)
				}
				if s.link == "" {
					ui.Column(c).Gap(1).Shrink(0).Children(func() {
						supportNum(c, fmt.Sprint(s.n), 30).LineHeight(1.05)
						ui.Text(c, s.label).FontSize(12).TextColor(ink.muted)
					})
					continue
				}
				b := ui.ButtonBase(c.Key(s.label)).Shrink(0).FocusRing(false).Cursor(ui.CursorPointer).
					Label(fmt.Sprintf("%d %s，在 GitHub 上看", s.n, s.label)).Tooltip("在 GitHub 上看")
				label := ink.muted
				if b.Hovered() {
					label = accent
				}
				label = b.AnimateColor("label", label, motionHover)
				b.Children(func() {
					ui.Column(c).Gap(1).Children(func() {
						supportNum(c, fmt.Sprint(s.n), 30).LineHeight(1.05)
						ui.Row(c).Gap(3).AlignItems(ui.Center).Children(func() {
							ui.Text(c, s.label).FontSize(12).TextColor(label)
							ui.Icon(c, supportIconGo).Size(10, 10).TextColor(label)
						})
					})
				})
				if b.Clicked() {
					openLink(s.link)
				}
			}
			ui.Spacer(c)
			// Who has, once someone has.
			n := len(wall.Supporters)
			if n == 0 {
				return
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Shrink(1).MinWidth(0).Children(func() {
				ui.Row(c).Shrink(0).Children(func() {
					for i, s := range wall.Supporters[:min(n, 4)] {
						m := float32(-9)
						if i == 0 {
							m = 0
						}
						a.supportRound(c, s, 30, 12.5).Margin(0, 0, 0, m)
					}
				})
				ui.Column(c).Shrink(1).MinWidth(0).Children(func() {
					supportNum(c, fmt.Sprint(n), 17).LineHeight(1.1)
					ui.Text(c, "位朋友支持过").FontSize(12).TextColor(ink.muted).SingleLine()
				})
			})
		})
	})
}

// supportPay is the card of the codes: the code, covered until a click
// asks for it, the way to pay by it in three steps, and 爱发电.
func (a *app) supportPay(k *settingsKit, ink paneInk) {
	c := k.c
	st := &a.support
	m := supportMethods[st.method]
	fill, edge := a.cardColors()
	accent := a.pal.accent
	ui.Row(c.Key("support-pay")).Margin(14, 0, 0, 0).Padding(26, 30, 26, 26).Gap(30).AlignItems(ui.Center).
		Radius(26).Background(fill).Border(0.5, edge).Children(func() {
		ui.Column(c).Gap(14).AlignItems(ui.Center).Shrink(0).Children(func() {
			a.supportCode(c, m, 156)
			a.segmentedControl(c.Key("support-method"), &st.method, []string{supportMethods[0].label, supportMethods[1].label}, true, nil)
		})
		ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Children(func() {
			ui.Text(c, "扫码请 nya 喝杯奶茶").FontSize(19).FontWeight(750).LetterSpacing(-0.2)
			ui.Column(c).Gap(13).Margin(18, 0, 22, 0).Children(func() {
				// Each step in phrases that keep together: a line too long
				// for the column breaks between them, never leaving a word
				// of its own on the next line.
				for i, phrases := range [][]string{
					{"打开" + m.app + "，扫左边的码"},
					{"金额随意，一块钱也是心意"},
					{"想上感谢墙？", "备注你的名字与网易云 UID，", "或者在 B 站私信 nya"},
				} {
					ui.Row(c).Gap(12).AlignItems(ui.Baseline).Children(func() {
						supportNum(c, fmt.Sprintf("%02d", i+1), 16).TextColor(accent).Width(20).Shrink(0)
						ui.Row(c).Wrap().Shrink(1).MinWidth(0).AlignItems(ui.Baseline).Children(func() {
							for _, p := range phrases {
								ui.Text(c, p).FontSize(13.5).LineHeight(1.6).TextColor(ink.text.Alpha(0.86)).NoWrap()
							}
						})
					})
				}
			})
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				if k.button("support-large", "放大收款码", iconExpand, false, false).Clicked() {
					// Asked for large, it is asked for: no cover there either.
					st.shown, st.large = true, true
				}
				if k.button("support-afdian", "去爱发电", iconExternal, false, false).
					Tooltip("在爱发电支持：可以按月赞助，留一句话就能上感谢墙").Clicked() {
					openLink(supportAfdian)
				}
			})
		})
	})
}

// supportCode is the code on its tint, covered at first: blurred past
// reading under a veil that says to click, and a click takes the cover
// off, as it fades away; shown, a click shows it large. The page covers
// it again each time it opens (st.shown).
func (a *app) supportCode(c *ui.Context, m struct {
	label, app string
	tint       ui.Color
}, size float32) {
	st := &a.support
	codes := supportCodes()
	label := "点一下显示" + m.label + "收款码"
	if st.shown {
		label = "放大" + m.label + "收款码"
	}
	code := ui.ButtonBase(c.Key("support-code")).Padding(9).Radius(20).FocusRing(false).Label(label).Cursor(ui.CursorPointer)
	k := float32(0)
	if st.shown {
		k = 1
	}
	if a.motion != motionOff {
		k = clamp01(code.AnimateTo("shown", k, supportReveal))
	}
	tint := code.AnimateColor("tint", m.tint, ui.Animation{Duration: 300 * time.Millisecond})
	lift := float32(1)
	if code.Hovered() {
		lift = 1.02
	}
	code.Background(tint).Shadow(0, 10, 28, 0, tint.Alpha(0.18+0.12*k)).
		AnimatePose(ui.Pose{Scale: lift}, ui.Snappy).
		Children(func() {
			face := ui.Box(c).Size(size, size).Radius(14).Clip()
			face.Children(func() {
				inner := ui.Box(c).Fill()
				if k < 1 {
					// Past reading while covered: a phone finds no code in it.
					inner.Blur(14 * (1 - k))
				}
				inner.Children(func() { supportCodeFace(c, codes[st.method], m.label, size) })
				if k >= 1 {
					return
				}
				// The veil and what it says, fading with the blur.
				ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().Gap(9).
					Background(ui.RGBA(18, 18, 22, 0.42)).Opacity(1 - k).PassThrough().Children(func() {
					ui.Box(c).Size(40, 40).Radius(999).Center().Background(white.Alpha(0.92)).
						Shadow(0, 4, 14, 0, ui.RGBA(0, 0, 0, 0.22)).Children(func() {
						ui.Icon(c, supportIconEye).Size(19, 19).TextColor(m.tint)
					})
					ui.Text(c, "点一下显示").FontSize(13).FontWeight(650).TextColor(white).
						Shadow(0, 1, 6, 0, ui.RGBA(0, 0, 0, 0.4))
				})
			})
		})
	if code.Clicked() {
		if !st.shown {
			st.shown = true
		} else {
			a.pressedFrom(code, 20)
			st.large = true
		}
	}
}

// supportTile is a tile of what helps besides money: its mark on a round
// square of its color with an arrow across from it, and under them what to
// do, whole at any width; the whole tile is the button.
func (a *app) supportTile(c *ui.Context, ink paneInk, key, title string, icon *ui.SVG, tint ui.Color, act func()) {
	fill, edge := a.cardColors()
	b := ui.ButtonBase(c.Key(key)).Padding(14, 14, 15, 16).Radius(20).MinWidth(0).
		FocusRing(false).Label(title).Cursor(ui.CursorPointer)
	bg, arrow := fill, ink.faint
	if b.Hovered() {
		bg, arrow = ink.hover.Over(fill), ink.text
	}
	b.Background(b.AnimateColor("bg", bg, motionHover)).Border(0.5, edge).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(14).Children(func() {
			ui.Row(c).AlignItems(ui.Start).Children(func() {
				ui.Box(c).Size(36, 36).Radius(8).Center().Shrink(0).Background(tint.Alpha(0.16)).Children(func() {
					ui.Icon(c, icon).Size(18, 18).TextColor(tint)
				})
				ui.Spacer(c)
				ui.Icon(c, supportIconGo).Size(15, 15).Shrink(0).TextColor(b.AnimateColor("arrow", arrow, motionHover))
			})
			ui.Text(c, title).FontSize(13.5).FontWeight(600).LineHeight(1.4).MinWidth(0)
		})
	})
	if b.Clicked() {
		act()
	}
}
