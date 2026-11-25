package app

import (
	"fmt"
	"log"
	"math"
	"sync"

	"github.com/egoist/mygo/ui"
)

// The thanks wall: a hall of seats, one for each supporter in the order
// they came, and their names under it, each with its number.

// supportNumFamily is the family of the wall's numbers: Barlow Condensed
// SemiBold (SIL OFL 1.1, support/BarlowCondensed-OFL.txt), narrow figures
// as on a ticket.
const supportNumFamily = "Cloudy Numerals"

// supportNumerals registers the numbers' font, the first time the wall
// shows.
var supportNumerals = sync.OnceFunc(func() {
	b, err := supportFiles.ReadFile("support/BarlowCondensed-SemiBold.ttf")
	if err == nil {
		err = ui.RegisterFont(b, supportNumFamily)
	}
	if err != nil {
		log.Printf("support: numerals: %v", err)
	}
})

// supportNum is text in the numbers' font.
func supportNum(c *ui.Context, s string, size float32) ui.Element {
	supportNumerals()
	return ui.Text(c, s).Font(supportNumFamily).FontSize(size).FontWeight(600).FontFeatures("tnum")
}

// seatNo is a seat's number as the wall writes it: 007.
func seatNo(no int) string { return fmt.Sprintf("%03d", no) }

// supportWallTitle is 前 100 位支持者 and how many have sat.
func (a *app) supportWallTitle(c *ui.Context, ink paneInk, taken, seats int, size float32) {
	accent := a.pal.accent
	ui.Column(c).Grow(1).Shrink(1).MinWidth(0).Gap(8).Children(func() {
		ui.Row(c).Gap(size * 0.4).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "前").FontSize(size).FontWeight(700)
			ui.Box(c).Padding(3, 10, 2, 10).Radius(8).Background(accent.Alpha(0.13)).
				Draw(func(p *ui.Painter, r ui.Rect) {
					p.StrokeDashed(r, accent.Alpha(0.5), 8, 1)
				}).
				Children(func() {
					supportNum(c, fmt.Sprint(seats), size*1.08).TextColor(accent).LineHeight(1.1)
				})
			ui.Text(c, "位支持者").FontSize(size).FontWeight(700)
		})
		ui.Text(c, fmt.Sprintf("已入座 %d 位 · 还剩 %d 个座位 · 按支持的先后入座", taken, seats-taken)).
			FontSize(13).TextColor(ink.muted)
	})
}

// supportTicket is the card of the next seat: its number torn off from
// where it is.
func (a *app) supportTicket(c *ui.Context, ink paneInk, next int, size float32) {
	accent := a.pal.accent
	row, seat := seatPlace(next)
	ui.Row(c).Radius(14).Background(ink.hover).Border(0.5, ink.line).Shrink(0).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Justify(ui.Center).Padding(10, 16, 8, 16).Children(func() {
			ui.Row(c).Gap(5).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(6, 6).Radius(999).Background(accent)
				supportNum(c, "NO.", size*0.36).TextColor(accent)
			})
			supportNum(c, seatNo(next), size).LineHeight(1.05)
		})
		ui.Box(c).Width(2).Margin(8, 0, 8, 0).Draw(func(p *ui.Painter, r ui.Rect) {
			for y := r.Y; y < r.Y+r.H; y += 7 {
				p.Fill(ui.Rect{X: r.X, Y: y, W: 1.5, H: min(4, r.Y+r.H-y)}, ink.line.Alpha(1.6), 0)
			}
		})
		ui.Column(c).Justify(ui.Center).Gap(3).Padding(10, 18, 10, 16).Children(func() {
			ui.Text(c, "下一个座位").FontSize(13.5).FontWeight(700)
			ui.Text(c, fmt.Sprintf("第 %d 排 %d 座 · 留给你", row, seat)).FontSize(12).TextColor(ink.muted)
		})
	})
}

// supportSeatMap is the hall: a small square for each seat, lit where
// someone sat, the next one beating.
func (a *app) supportSeatMap(c *ui.Context, ink paneInk, seated []supporter, seats, next int, size float32) {
	accent := a.pal.accent
	who := make(map[int]supporter, len(seated))
	for _, s := range seated {
		who[s.No] = s
	}
	empty := ink.text.Alpha(0.11)
	if !a.pal.dark {
		empty = ink.text.Alpha(0.1)
	}
	well := ui.RGBA(0, 0, 0, 0.2)
	if !a.pal.dark {
		well = ink.text.Alpha(0.035)
	}
	ui.Column(c).Padding(14, 14, 12, 14).Gap(12).Radius(14).Background(well).Label(fmt.Sprintf("%d 个座位，已入座 %d 个", seats, len(seated))).Children(func() {
		ui.Grid(c).Columns(supportRowSeats).GapY(size * 0.7).Children(func() {
			for no := 1; no <= seats; no++ {
				ui.Row(c.Key(no)).Justify(ui.Center).Children(func() {
					seat := ui.Box(c).Size(size, size).Radius(4)
					s, sat := who[no]
					switch {
					case sat:
						seat.Background(accent).Shadow(0, 0, 8, 0, accent.Alpha(0.45)).
							Tooltip(fmt.Sprintf("No.%s · %s", seatNo(no), s.Name))
						// A picture sits in its seat, over the lit square.
						if bmp := a.supportAvatar(s); bmp != nil {
							in := artIn(seat, bmp)
							seat.Children(func() { artImage(c, bmp, 4, in) })
						}
					case no == next:
						seat.Tooltip(fmt.Sprintf("No.%s · 下一个座位，留给你", seatNo(no))).Draw(func(p *ui.Painter, r ui.Rect) {
							p.Stroke(r, accent, 4, 1.5)
							if !a.supportMoves() {
								return
							}
							// A ring that leaves it and fades, as a beat.
							k := float32(math.Mod(p.Now().Sub(supportEpoch).Seconds(), 1.8) / 1.8)
							g := 6 * k
							p.Stroke(ui.Rect{X: r.X - g, Y: r.Y - g, W: r.W + 2*g, H: r.H + 2*g}, accent.Alpha(0.55*(1-k)), 4+g, 1.5)
							p.AnimationFrameRate(30)
						})
					default:
						seat.Background(empty)
					}
				})
			}
		})
		ui.Row(c).Gap(16).Padding(0, 2).Children(func() {
			legend := func(label string, draw func(p *ui.Painter, r ui.Rect)) {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(9, 9).Draw(draw)
					ui.Text(c, label).FontSize(11.5).TextColor(ink.muted)
				})
			}
			legend("已入座", func(p *ui.Painter, r ui.Rect) { p.Fill(r, accent, 3) })
			legend("空座", func(p *ui.Painter, r ui.Rect) { p.Fill(r, empty, 3) })
			legend("下一个座位", func(p *ui.Painter, r ui.Rect) { p.Stroke(r, accent, 3, 1.5) })
		})
	})
}

// supportSeatRows are names in rows of three, each with its round and its
// number; a name cut short shows whole when its row is clicked.
func (a *app) supportSeatRows(c *ui.Context, ink paneInk, seated []supporter, large bool) {
	height, round, font, num := float32(48), float32(28), float32(13.5), float32(19)
	if large {
		height, round, font, num = 54, 32, 14.5, 22
	}
	ui.Grid(c).Columns(3).GapX(8).GapY(8).Children(func() {
		for _, s := range seated {
			a.supportSeatRow(c.Key(s.No), ink, s, height, round, font, num)
		}
	})
}

func (a *app) supportSeatRow(c *ui.Context, ink paneInk, s supporter, height, round, font, num float32) {
	st := &a.support
	b := ui.ButtonBase(c).Height(height).Padding(0, 14, 0, 10).Gap(10).Radius(14).
		AlignItems(ui.Center).MinWidth(0).FocusRing(false).Label(fmt.Sprintf("No.%s %s", seatNo(s.No), s.Name))
	// Cut short? What the row leaves the name, as laid out last frame,
	// against what the name takes.
	cut := false
	if bw := b.LayoutBounds().W; bw > 0 {
		nw, _ := c.MeasureText(0, ui.Span{Text: seatNo(s.No), Font: supportNumFamily, Size: num, Weight: 600})
		tw, _ := c.MeasureText(0, ui.Span{Text: s.Name, Size: font, Weight: 500})
		cut = tw > bw-24-round-20-nw+0.5
	}
	bg := ink.hover
	if cut {
		b.Cursor(ui.CursorPointer).Tooltip("点一下看完整的名字")
		if b.Hovered() || st.name == s.No {
			bg = ink.press
		}
	}
	b.Background(b.AnimateColor("bg", bg, motionHover)).Children(func() {
		a.supportRound(c, s, round, round*0.43)
		ui.Text(c, s.Name).FontSize(font).FontWeight(500).SingleLine().Grow(1).Shrink(1).MinWidth(0)
		supportNum(c, seatNo(s.No), num).TextColor(ink.muted)
	})
	if b.Clicked() && cut {
		if st.name == s.No {
			st.name = 0
		} else {
			st.name = s.No
		}
	}
	open := st.name == s.No
	if !open {
		return
	}
	ui.PopoverBase(c.Key(fmt.Sprint("whole-", s.No)), b, &open, func(panel ui.Element) {
		pink := a.inkOn(false)
		panel.AttachTo(b, ui.AnchorBottomStart, ui.AnchorTopStart).Margin(6, 0, 0, 0).MaxWidth(360).
			Padding(12, 16, 12, 16).TextColor(pink.text).Label("完整的名字")
		a.pane(panel, 14, false)
		panel.Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, s.Name).FontSize(15).FontWeight(600).LineHeight(1.6)
				row, seat := seatPlace(s.No)
				ui.Row(c).Gap(8).AlignItems(ui.Baseline).Children(func() {
					supportNum(c, "No."+seatNo(s.No), 15).TextColor(a.pal.accent)
					ui.Text(c, fmt.Sprintf("第 %d 排 %d 座", row, seat)).FontSize(12).TextColor(pink.muted)
				})
			})
		})
	})
	if !open {
		st.name = 0
	}
}

// supportWallDialog is the whole wall, 查看全部's: the hall, larger, and
// every name, a hundred to a block.
func (a *app) supportWallDialog(c *ui.Context) {
	st := &a.support
	if !a.settingsView.open || !st.page {
		st.all = false
	}
	a.dialog(c, "support-wall", 0.35, &st.all, func(panel ui.Element) {
		ink := a.inkOn(false)
		seated := a.supportWall().seated()
		seats, next := supportSeats(seated)
		panel.Width(940).TextColor(ink.text).Label("感谢墙")
		a.dialogPane(panel, 26, false)
		panel.Children(func() {
			ui.Row(c).Padding(30, 20, 20, 34).Gap(22).AlignItems(ui.Center).Children(func() {
				a.supportWallTitle(c, ink, len(seated), seats, 30)
				a.supportTicket(c, ink, next, 40)
				if iconButton(c.Key("close"), iconX, 16, ink.muted, "关闭").AlignSelf(ui.Start).Clicked() {
					st.all = false
				}
			})
			ui.Box(c).Padding(0, 34, 0, 34).Children(func() {
				a.supportSeatMap(c, ink, seated, seats, next, 15)
			})
			// The names, in blocks of a hundred.
			rows := (len(seated) + 2) / 3
			blocks := (seats + supportBlock - 1) / supportBlock
			tall := float32(rows)*62 + float32(blocks)*40 + 24
			// What the window leaves under the head and the hall, kept roomy.
			_, winH := c.Size()
			room := winH - 330 - float32(seats/supportRowSeats)*26
			ui.Scroll(c.Key("names")).Height(min(tall, max(room, 180))).Padding(20, 34, 26, 34).Children(func() {
				ui.Column(c).Gap(14).Children(func() {
					for from := 1; from <= seats; from += supportBlock {
						var block []supporter
						for _, s := range seated {
							if s.No >= from && s.No < from+supportBlock {
								block = append(block, s)
							}
						}
						if len(block) == 0 {
							continue
						}
						ui.Column(c.Key(from)).Gap(12).Children(func() {
							supportNum(c, fmt.Sprintf("No.%s – %s", seatNo(from), seatNo(from+supportBlock-1)), 16)
							a.supportSeatRows(c, ink, block, true)
						})
					}
				})
			})
		})
	})
}
