package app

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func solidImage(c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: c}, image.Point{}, draw.Src)
	return img
}

func writeSolidPNG(t *testing.T, dir, name string, c color.RGBA) string {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, solidImage(c)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

// Switching back to a playlist triggers a background refresh after 5s; if the
// refresh returns a new uncached cover URL for the same track ID, both the
// track row thumbnail and the header's stacked cover card must keep showing the
// old cover until the replacement image finishes loading.
func TestTrackCoverKeepsDisplayedImageAcrossPlaylistRefresh(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.Smoke = false
	a.settings.BarSpin = false

	artPosts := make(chan func(), 8)
	a.art.post = func(fn func()) { artPosts <- fn }

	green := color.RGBA{G: 220, A: 255}
	blue := color.RGBA{B: 220, A: 255}
	red := color.RGBA{R: 220, A: 255}
	oldURL := "https://p1.music.126.net/old-track-cover.jpg"
	otherURL1 := "https://p1.music.126.net/other-track-cover-1.jpg"
	otherURL2 := "https://p1.music.126.net/other-track-cover-2.jpg"
	newPath := writeSolidPNG(t, t.TempDir(), "new-track-cover.png", red)

	for _, px := range []int{96, 240} {
		a.art.add(netease.Sized(oldURL, px), &artwork{bmp: ui.NewBitmap(solidImage(green)), used: time.Now(), size: 1})
		a.art.add(netease.Sized(otherURL1, px), &artwork{bmp: ui.NewBitmap(solidImage(blue)), used: time.Now(), size: 1})
		a.art.add(netease.Sized(otherURL2, px), &artwork{bmp: ui.NewBitmap(solidImage(blue)), used: time.Now(), size: 1})
	}
	tracksWith := func(firstURL string) []netease.Track {
		return []netease.Track{
			{ID: 101, Name: "Song One", Duration: 200000, Album: netease.Album{ID: 10, Name: "Alb 1", PicURL: firstURL}},
			{ID: 102, Name: "Song Two", Duration: 200000, Album: netease.Album{ID: 11, Name: "Alb 2", PicURL: otherURL1}},
			{ID: 103, Name: "Song Three", Duration: 200000, Album: netease.Album{ID: 12, Name: "Alb 3", PicURL: otherURL2}},
		}
	}
	a.playlists[1] = &netease.Playlist{ID: 1, Name: "Playlist A", TrackCount: 3, Tracks: tracksWith(oldURL)}
	a.playlists[2] = &netease.Playlist{
		ID: 2, Name: "Playlist B", TrackCount: 1,
		Tracks: []netease.Track{{ID: 202, Name: "Song B", Duration: 200000}},
	}
	a.playlistAt = map[int64]time.Time{
		1: time.Now().Add(-personalRefreshGap - time.Second),
		2: time.Now(),
	}

	fetched := make(chan *netease.Playlist, 1)
	origFetch := fetchPlaylist
	defer func() { fetchPlaylist = origFetch }()
	fetchPlaylist = func(id int64) (*netease.Playlist, error) {
		pl := &netease.Playlist{ID: id, Name: "Playlist A", TrackCount: 3, Tracks: tracksWith(newPath)}
		fetched <- pl
		return pl, nil
	}

	a.route = route{page: pagePlaylist, id: 1}
	tt := ui.NewTester(a.view, 1200, 760)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()

	// Switch A -> B -> A; returning to A renders cached data and starts refresh.
	a.open(route{page: pagePlaylist, id: 2})
	tt.Frame()
	a.open(route{page: pagePlaylist, id: 1})
	tt.Frame()

	rowRect, ok := tt.Find("Song One")
	if !ok {
		t.Fatal("track row did not mount on playlist A")
	}
	stackRect, ok := tt.Find("页面中的封面")
	if !ok {
		t.Fatal("header cover stack did not mount on playlist A")
	}
	// Row layout: 12px paddingX + 26px index box + 14px gap = 52px; cover is 38x38 -> center at +71px.
	// Stack front card (i=0): Right(0).Top(16).Size(132, 132) inside the 204x164 stack box.
	points := []struct {
		name string
		x, y int
	}{
		{"track row cover", int(rowRect.X + 71), int(rowRect.Y + rowRect.H/2)},
		{"header stack cover", int(stackRect.X + stackRect.W - 66), int(stackRect.Y + 16 + 66)},
	}
	checkColor := func(stage string, want color.RGBA) {
		t.Helper()
		frame := tt.Image()
		for _, pt := range points {
			got := frame.RGBAAt(pt.x, pt.y)
			if abs32(float32(got.R)-float32(want.R)) > 35 || abs32(float32(got.G)-float32(want.G)) > 35 || got.B > 35 {
				t.Fatalf("%s (%s): pixel at (%d,%d) = %v, want ~%v", stage, pt.name, pt.x, pt.y, got, want)
			}
		}
	}
	checkColor("before refresh", green)

	// Apply background playlist refresh that swaps Song One's cover URL.
	var refreshed *netease.Playlist
	select {
	case refreshed = <-fetched:
	case <-time.After(5 * time.Second):
		t.Fatal("background playlist refresh did not run")
	}
	a.playlistAnswered(a.accountGen, 1, refreshed, nil, false)
	tt.Frame()
	checkColor("while replacement loads", green)

	// Finish loading both sizes (96px row + 240px stack) and confirm both update.
	for range 2 {
		select {
		case apply := <-artPosts:
			apply()
		case <-time.After(5 * time.Second):
			t.Fatal("replacement cover image did not load")
		}
	}
	tt.Frame()
	checkColor("after replacement loads", red)
}

// A different song ID, account switch, or empty cover URL must never inherit
// the previous song's displayed cover.
func TestTrackCoverKeepsImageOnlyForSameIdentity(t *testing.T) {
	green := color.RGBA{G: 220, A: 255}
	for _, tc := range []struct {
		name   string
		mutate func(a *app, tr *netease.Track)
	}{
		{"different track", func(a *app, tr *netease.Track) { tr.ID++; tr.Album.PicURL = "waiting-cover" }},
		{"different account", func(a *app, tr *netease.Track) { a.accountGen++; tr.Album.PicURL = "waiting-cover" }},
		{"empty url", func(a *app, tr *netease.Track) { tr.Album.PicURL = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newTestApp()
			a.settings = defaultSettings()
			a.art.add("loaded-cover", &artwork{bmp: ui.NewBitmap(solidImage(green)), used: time.Now(), size: 1})
			a.art.add("waiting-cover", &artwork{failed: true, retryAt: time.Now().Add(time.Hour)})
			tr := netease.Track{ID: 42, Album: netease.Album{PicURL: "loaded-cover"}}
			tt := ui.NewTester(func(c *ui.Context) {
				a.pal = paletteFor(c.Theme())
				ui.Row(c).Children(func() {
					a.trackCover(c, &tr, 96, 38, 8)
				})
			}, 100, 100)
			if got := tt.Image().RGBAAt(19, 19); got != green {
				t.Fatalf("initial cover pixel = %v, want %v", got, green)
			}
			tc.mutate(a, &tr)
			tt.Frame()
			if got := tt.Image().RGBAAt(19, 19); got == green {
				t.Fatalf("%s unexpectedly inherited previous song cover", tc.name)
			}
		})
	}
}

// Returning to a playlist whose 96px row thumbnails are cached while the 240px
// stack covers are not yet loaded must show the cached 96px cover immediately
// instead of flashing a grey placeholder before upgrading to 240px.
func TestCoverStackUsesCachedThumbnailWhileFullSizeLoads(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.Smoke = false
	a.settings.BarSpin = false

	artPosts := make(chan func(), 8)
	a.art.post = func(fn func()) { artPosts <- fn }

	green := color.RGBA{G: 220, A: 255}
	blue := color.RGBA{B: 220, A: 255}
	red := color.RGBA{R: 220, A: 255}
	dir := t.TempDir()
	cover1 := writeSolidPNG(t, dir, "cover1.png", red)
	cover2 := "https://p1.music.126.net/stack-other-2.jpg"
	cover3 := "https://p1.music.126.net/stack-other-3.jpg"

	// Only the 96px thumbnail for cover1 is cached; its 240px copy is not ready.
	a.art.add(netease.Sized(cover1, 96), &artwork{bmp: ui.NewBitmap(solidImage(green)), used: time.Now(), size: 1})
	for _, px := range []int{96, 240} {
		a.art.add(netease.Sized(cover2, px), &artwork{bmp: ui.NewBitmap(solidImage(blue)), used: time.Now(), size: 1})
		a.art.add(netease.Sized(cover3, px), &artwork{bmp: ui.NewBitmap(solidImage(blue)), used: time.Now(), size: 1})
	}
	a.playlists[1] = &netease.Playlist{
		ID: 1, Name: "Playlist A", TrackCount: 3,
		Tracks: []netease.Track{
			{ID: 101, Name: "Song One", Duration: 200000, Album: netease.Album{ID: 10, Name: "Alb 1", PicURL: cover1}},
			{ID: 102, Name: "Song Two", Duration: 200000, Album: netease.Album{ID: 11, Name: "Alb 2", PicURL: cover2}},
			{ID: 103, Name: "Song Three", Duration: 200000, Album: netease.Album{ID: 12, Name: "Alb 3", PicURL: cover3}},
		},
	}
	a.playlists[2] = &netease.Playlist{ID: 2, Name: "Playlist B"}
	a.playlistAt = map[int64]time.Time{1: time.Now(), 2: time.Now()}

	a.route = route{page: pagePlaylist, id: 2}
	tt := ui.NewTester(a.view, 1200, 760)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()

	// Switch B -> A: first frame must render the cached 96px fallback (green),
	// not a grey placeholder, while 240px loads asynchronously.
	a.open(route{page: pagePlaylist, id: 1})
	tt.Frame()
	stackRect, ok := tt.Find("页面中的封面")
	if !ok {
		t.Fatal("header cover stack did not mount")
	}
	x, y := int(stackRect.X+stackRect.W-66), int(stackRect.Y+16+66)
	check := func(stage string, want color.RGBA) {
		t.Helper()
		got := tt.Image().RGBAAt(x, y)
		if abs32(float32(got.R)-float32(want.R)) > 35 || abs32(float32(got.G)-float32(want.G)) > 35 || got.B > 35 {
			t.Fatalf("%s: stack pixel at (%d,%d) = %v, want ~%v", stage, x, y, got, want)
		}
	}
	check("first frame with only 96px cached", green)

	// Once the 240px image finishes loading, the stack card upgrades to red.
	select {
	case apply := <-artPosts:
		apply()
	case <-time.After(5 * time.Second):
		t.Fatal("240px cover image did not load")
	}
	tt.Frame()
	check("after 240px loads", red)
}
