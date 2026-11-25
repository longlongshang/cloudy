package app

import (
	"fmt"
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

// Opening Now Playing from the light theme used to keep the flying title dark
// gray over the opaque dark backdrop, double-draw both titles at landing, and
// snap from grayscale AA to ClearType RGB fringes after the motion finished.
func TestNowPlayingTitleFlightNoColorOrHandoverFlash(t *testing.T) {
	for _, scale := range []float32{1, 1.25} {
		t.Run(fmt.Sprintf("scale=%.2f", scale), func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.settings.Smoke = false
			a.settings.Fog = false
			a.settings.Appearance = "light"
			a.settings.BarSpin = false
			url := "https://fixture.invalid/nib-title"
			markArt(a, url, 120)
			art := a.art.peek(netease.Sized(url, 120))
			a.art.add(netease.Sized(url, 600)+"#backdrop", art)
			a.art.add(netease.Sized(url, 600), art)
			tr := netease.Track{
				ID:       101,
				Name:     "NIB",
				Duration: 366000,
				Artists:  []netease.Artist{{ID: 1, Name: "Black Sabbath"}},
				Album:    netease.Album{ID: 1, Name: "Under Wheels of Confusion", PicURL: url},
			}
			a.pl.Cue([]netease.Track{tr})

			tt := ui.NewTester(a.view, 1100, 760)
			tt.SetScale(scale)
			tt.Move(550, 380)
			settle := func() {
				for range 3 {
					tt.Frame()
					tt.Advance(time.Second)
				}
				tt.Frame()
			}
			tt.Frame()
			// Open and close once so Now Playing's layout settles before measuring the flight.
			a.nowPlaying = true
			settle()
			a.nowPlaying = false
			settle()

			measureTitle := func(img *image.RGBA, rect ui.Rect) (maxLum, sumLum, maxChroma int) {
				x0, y0 := int(rect.X*scale), int(rect.Y*scale)
				x1, y1 := int((rect.X+min(rect.W, 60))*scale), int((rect.Y+rect.H)*scale)
				bg := img.RGBAAt(max(0, x0), max(0, y0-int(6*scale)))
				bgLum := (int(bg.R) + int(bg.G) + int(bg.B)) / 3
				for y := max(0, y0); y < min(img.Bounds().Dy(), y1); y++ {
					for x := max(0, x0); x < min(img.Bounds().Dx(), x1); x++ {
						c := img.RGBAAt(x, y)
						lum := (int(c.R) + int(c.G) + int(c.B)) / 3
						if lum > maxLum {
							maxLum = lum
						}
						if lum > bgLum+15 {
							sumLum += lum - bgLum
						}
						// Color difference relative to the purple backdrop: neutral
						// grayscale AA stays bounded by the backdrop's own tint,
						// whereas ClearType RGB fringes spike past 100.
						dr := int(c.R) - int(bg.R)
						dg := int(c.G) - int(bg.G)
						db := int(c.B) - int(bg.B)
						ch := max(int(math.Abs(float64(dr-dg))), int(math.Abs(float64(dg-db))), int(math.Abs(float64(dr-db))))
						if ch > maxChroma {
							maxChroma = ch
						}
					}
				}
				return maxLum, sumLum, maxChroma
			}

			type sample struct {
				k         float32
				maxLum    int
				sumLum    int
				maxChroma int
			}
			var samples []sample
			a.nowPlaying = true
			for i := 0; i < 45; i++ {
				if i > 0 {
					tt.Advance(16 * time.Millisecond)
				}
				tt.Frame()
				k := a.npK
				if k < 0.35 {
					continue
				}
				rect := a.npSpot.title
				if a.npTitleFlying() {
					rect = lerpRect(a.barSpot.title, a.npSpot.title, k)
				}
				maxLum, sumLum, maxChroma := measureTitle(tt.Image(), rect)
				samples = append(samples, sample{k: k, maxLum: maxLum, sumLum: sumLum, maxChroma: maxChroma})
			}
			if len(samples) < 10 {
				t.Fatalf("expected flight samples, got %d", len(samples))
			}
			settled := samples[len(samples)-1]
			if settled.maxLum < 250 || settled.sumLum == 0 {
				t.Fatalf("settled title not white: %+v", settled)
			}
			for _, s := range samples {
				// Once Now Playing's dark backdrop is opaque (k >= 0.3), the flying
				// title must already be white rather than brightening from gray.
				if s.k >= 0.35 && s.maxLum < 250 {
					t.Errorf("k=%.5f: title max luminance %d, want white (>=250)", s.k, s.maxLum)
				}
				// During the landing handover, the title must neither flash brighter
				// from double-drawing nor change subpixel color fringes.
				if s.k >= 0.99 {
					ratio := float64(s.sumLum) / float64(settled.sumLum)
					if math.Abs(ratio-1) > 0.05 {
						t.Errorf("k=%.5f: landing title luminance %d vs settled %d (ratio %.3f)", s.k, s.sumLum, settled.sumLum, ratio)
					}
					if d := int(math.Abs(float64(s.maxChroma - settled.maxChroma))); d > 8 {
						t.Errorf("k=%.5f: title chroma jumped from %d to settled %d", s.k, s.maxChroma, settled.maxChroma)
					}
				}
			}
		})
	}
}
