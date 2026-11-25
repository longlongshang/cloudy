package app

import (
	"math"
	"time"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

// A line too long for the desktop lyrics' room, even smaller, slides along
// as it is sung: the word being sung kept a little left of the middle, the
// line's start at rest until the singing gets there and its end at rest
// once it has. Its characters fade out toward either edge of the room it
// slides past, where there is more of it.

const (
	deskScrollFade = 40   // DIPs over which characters fade at an edge
	deskScrollLead = 0.42 // where in the room the word sung stays
)

// deskScrollLine is the line m sliding past the room as it is sung at pos.
func (a *app) deskScrollLine(c *ui.Context, m deskMain, pos time.Duration, look deskLook) ui.Element {
	ink := a.deskShape(m, look, 0, false)
	w, h := ink.Size()
	run := max(0, w-deskRoom)
	chars, colors := a.deskChars(m, look, pos)

	// Where each character lies along the line, from its left.
	units := utf16.Encode(chars)
	type glyph struct {
		text     string
		x0, x1   float32
		from, to int // its runes
	}
	var glyphs []glyph
	r := 0
	for at := 0; at < len(units); {
		next := ink.NextBoundary(at)
		if next <= at {
			break
		}
		text := string(utf16.Decode(units[at:next]))
		x0, x1 := ink.Caret(at).X, ink.Caret(next).X
		if x1 < x0 {
			x1 = x0
		}
		n := len([]rune(text))
		glyphs = append(glyphs, glyph{text: text, x0: x0, x1: x1, from: r, to: r + n})
		r += n
		at = next
	}

	// How far along the line the singing is: through the word sung, as it
	// lights, or through the line's time with no words.
	sung := float32(0)
	if len(m.words) > 0 {
		rune0 := 0
		for _, wd := range m.words {
			n := len([]rune(wd.Text))
			x0, x1 := deskRunesX(glyphs, rune0, rune0+n, func(g glyph) (int, int, float32, float32) { return g.from, g.to, g.x0, g.x1 })
			switch {
			case pos >= wd.End:
				sung = x1
			case pos >= wd.Start:
				sung = x0 + (x1-x0)*wordLit(wd, pos)
			}
			rune0 += n
		}
	} else if m.end > m.at {
		sung = w * clamp01(float32(pos-m.at)/float32(m.end-m.at))
	}
	off := min(max(0, sung-deskRoom*deskScrollLead), run)
	moving := off > 0 && off < run && a.pl.Playing()

	fade := func(x float32) float32 {
		v := x - off
		k := float32(1)
		if off > 0.5 {
			k = min(k, clamp01(v/deskScrollFade))
		}
		if off < run-0.5 {
			k = min(k, clamp01((deskRoom-v)/deskScrollFade))
		}
		return k
	}
	lay := func(key any, color func(g glyph) ui.Color) ui.Element {
		return ui.RichText(c.Key(key)).FontSize(m.size).FontWeight(800).SingleLine().Children(func() {
			for i, g := range glyphs {
				ui.Text(c.Key(i), g.text).TextColor(color(g))
			}
		})
	}
	box := ui.Box(c.Key("main")).Width(deskRoom).Height(h).Shrink(0).ClipX().PassThrough()
	box.Children(func() {
		ui.Box(c.Key("slide")).Absolute().Left(-off).Top(0).Width(w).Height(h).PassThrough().Children(func() {
			if look.edge.A > 0 {
				width := max(1.1, m.size*0.055)
				for i := range 8 {
					turn := float64(i) * math.Pi / 4
					lay(i, func(g glyph) ui.Color { return look.edge.Alpha(float32(look.edge.A) / 255 * fade((g.x0+g.x1)/2)) }).
						Absolute().Left(width * float32(math.Cos(turn))).Top(width * float32(math.Sin(turn)))
				}
			}
			lay("ink", func(g glyph) ui.Color {
				col := look.sung
				if g.from < len(colors) {
					col = colors[g.from]
				}
				return col.Alpha(float32(col.A) / 255 * fade((g.x0+g.x1)/2))
			}).Absolute().Left(0).Top(0)
		})
	})
	if moving {
		c.AnimationFrame()
	}
	return box
}

// deskRunesX returns where the runes from to to of glyphs lie, across.
func deskRunesX[G any](glyphs []G, from, to int, of func(G) (int, int, float32, float32)) (x0, x1 float32) {
	first := true
	for _, g := range glyphs {
		gf, gt, gx0, gx1 := of(g)
		if gt <= from || gf >= to {
			continue
		}
		if first {
			x0, first = gx0, false
		}
		x1 = gx1
	}
	return x0, x1
}
