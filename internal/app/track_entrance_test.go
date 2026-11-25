package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"netease-music-mygo/internal/netease"
)

func entranceTracks(n int, grouped bool) []netease.Track {
	tracks := make([]netease.Track, n)
	for i := range tracks {
		albumID := int64(1)
		albumName := "Album 01"
		if grouped {
			g := i/3 + 1
			albumID = int64(g)
			albumName = fmt.Sprintf("Album %02d", g)
		}
		tracks[i] = netease.Track{
			ID:       int64(i + 1),
			Name:     fmt.Sprintf("Track %03d", i+1),
			Duration: 200000,
			Album:    netease.Album{ID: albumID, Name: albumName},
			Artists:  []netease.Artist{{ID: 1, Name: "Artist"}},
		}
	}
	return tracks
}

// A tall track list waits for its first layout without flashing, animates every
// initially visible song beyond 14 (including after an empty async state), and
// never replays on scroll.
func TestTrackVisibleEntranceLifecycle(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.Smoke = false
	a.settings.BarSpin = false
	tracks := entranceTracks(40, false)
	r := route{page: pagePlaylist, id: 7}
	a.playlists[7] = &netease.Playlist{ID: 7, Name: "Tall Playlist", Tracks: tracks, TrackCount: len(tracks)}
	a.route = r
	a.pageDir = 1

	type frameLog struct {
		pending bool
		count   int
		shown   float32
	}
	var probeIdx int
	var probeShown float32
	var logs []frameLog
	record := true
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		if probeIdx >= 0 {
			probeShown = a.rowIn(c, probeIdx)
		}
		if record {
			logs = append(logs, frameLog{pending: a.rowsPending, count: a.rowsCount, shown: probeShown})
		}
	}, 1200, 1600)
	record = false

	// Frame 0 inside NewTester waited for layout without flashing (shown == 0),
	// and Frame 1 locked the committed Visible() song range (> 14), excluding
	// overscan rows built beyond the viewport.
	if len(logs) < 2 {
		t.Fatalf("expected at least 2 frames for layout + entrance lock, got %d", len(logs))
	}
	if !logs[0].pending || logs[0].count != 0 || logs[0].shown != 0 {
		t.Fatalf("frame 0 should wait for layout without flashing: %+v", logs[0])
	}
	if logs[1].pending || logs[1].count <= 14 || logs[1].shown != 0 {
		t.Fatalf("frame 1 should lock > 14 visible songs starting at 0: %+v", logs[1])
	}
	if !tt.FramePending() {
		t.Fatal("active entrance should keep requesting frames")
	}

	first, last := a.trackList.Visible()
	wantCount := last // row 0 is header; rows 1..last are songs 0..last-1
	if a.rowsStart != 0 || a.rowsCount != wantCount {
		t.Fatalf("locked range = [%d, +%d), want [0, +%d) from Visible=(%d,%d)", a.rowsStart, a.rowsCount, wantCount, first, last)
	}
	if a.rowsCount >= len(tracks) {
		t.Fatalf("expected viewport subset of %d tracks, got %d", len(tracks), a.rowsCount)
	}

	// Near the end of the stagger spread, the last visible song (> 14) is actively
	// animating while the first off-screen song past it is untouched (1).
	lastVisibleSong := a.rowsStart + a.rowsCount - 1
	probeIdx = lastVisibleSong
	tt.Advance(380 * time.Millisecond)
	tt.Frame()
	if probeShown <= 0 || probeShown >= 1 {
		t.Fatalf("last visible song %d shown=%v at 380ms, want in (0,1)", lastVisibleSong, probeShown)
	}
	probeIdx = lastVisibleSong + 1
	tt.Frame()
	if probeShown != 1 {
		t.Fatalf("off-screen song %d shown=%v, want 1", lastVisibleSong+1, probeShown)
	}

	// Once settled, scrolling away and back must not replay the entrance.
	probeIdx = -1
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	if a.rowsPending || a.rowsCount != 0 {
		t.Fatalf("entrance did not settle: pending=%v count=%d", a.rowsPending, a.rowsCount)
	}
	if tt.FramePending() {
		t.Fatal("settled list kept requesting frames")
	}
	a.trackList.ScrollTo(30, ui.Start)
	tt.Frame()
	a.trackList.ScrollTo(0, ui.Start)
	tt.Frame()
	if a.rowsPending || a.rowsCount != 0 || tt.FramePending() {
		t.Fatalf("scrolling replayed entrance: pending=%v count=%d framePending=%v", a.rowsPending, a.rowsCount, tt.FramePending())
	}

	// Empty-list layout on a new route must not consume the entrance before
	// async tracks arrive.
	a.open(route{page: pagePlaylist, id: 8})
	a.playlists[8] = &netease.Playlist{ID: 8, Name: "Async Playlist"}
	tt.Frame()
	if a.rowsRoute == a.route {
		t.Fatal("empty track list consumed route entrance before tracks loaded")
	}
	a.playlists[8].Tracks = tracks
	a.playlists[8].TrackCount = len(tracks)
	probeIdx = 0
	logs = logs[:0]
	record = true
	tt.Frame()
	record = false
	if len(logs) < 2 || !logs[0].pending || logs[0].shown != 0 {
		t.Fatalf("first frame after async tracks should wait for layout without flashing: %+v", logs)
	}
	if logs[1].pending || logs[1].count <= 14 || a.rowsCount <= 14 {
		t.Fatalf("async tracks failed to lock > 14 visible songs: frame1=%+v finalCount=%d", logs[1], a.rowsCount)
	}
}

// Grouped lists map visible list rows to song indices without letting headers,
// group titles, or footers consume slots; ReduceMotion and back navigation skip
// entrance immediately.
func TestTrackVisibleEntranceGroupsAndOff(t *testing.T) {
	// Unit check of list-row to song-index mapping across header, group titles,
	// footers, and scrolled viewports.
	items := []int{-1, 0, 1, -2, 2, 3, -3, 4}
	if s, n := visibleTrackRange(items, 0, 7); s != 0 || n != 4 {
		t.Fatalf("visibleTrackRange(0,7) = (%d,%d), want (0,4)", s, n)
	}
	if s, n := visibleTrackRange(items, 0, 9); s != 0 || n != 5 {
		t.Fatalf("visibleTrackRange(0,9 with footer) = (%d,%d), want (0,5)", s, n)
	}
	if s, n := visibleTrackRange(items, 4, 6); s != 2 || n != 2 {
		t.Fatalf("visibleTrackRange(4,6 scrolled) = (%d,%d), want (2,2)", s, n)
	}
	if s, n := visibleTrackRange(items, 0, 1); s != 0 || n != 0 {
		t.Fatalf("visibleTrackRange(0,1 header+group only) = (%d,%d), want (0,0)", s, n)
	}

	// Grouped playlist in a tall viewport: group headers must not consume
	// song animation slots, and the last visible song (> 14) must animate.
	a := newTestApp()
	a.settings = defaultSettings()
	a.settings.Smoke = false
	a.settings.BarSpin = false
	a.settings.AlbumGrid = false
	tracks := entranceTracks(45, true)
	r := route{page: pagePlaylist, id: 11}
	a.playlists[11] = &netease.Playlist{ID: 11, Name: "Grouped Playlist", Tracks: tracks, TrackCount: len(tracks)}
	a.route = r
	a.pageDir = 1
	a.plView = plView{route: r, sort: sortAlbum}

	var probeIdx int
	var probeShown float32
	tt := ui.NewTester(func(c *ui.Context) {
		a.view(c)
		if probeIdx >= 0 {
			probeShown = a.rowIn(c, probeIdx)
		}
	}, 1200, 1800)

	probeIdx = 0
	tt.Frame()
	tt.Frame()
	first, last := a.trackList.Visible()
	if a.rowsPending {
		t.Fatal("grouped playlist did not lock entrance after first layout")
	}
	if a.rowsStart != 0 || a.rowsCount <= 14 {
		t.Fatalf("grouped entrance range = [%d, +%d), want start=0 count>14", a.rowsStart, a.rowsCount)
	}
	// Because album titles occupy rows between row 1 and last, visible song count
	// must be strictly smaller than the raw list row span.
	if a.rowsCount >= last-first {
		t.Fatalf("group titles were not excluded: rowsCount=%d visible=(%d,%d)", a.rowsCount, first, last)
	}
	lastVisibleSong := a.rowsStart + a.rowsCount - 1
	if row := a.trackRow[lastVisibleSong]; row < first || row > last {
		t.Fatalf("last animated song %d maps to row %d outside Visible=(%d,%d)", lastVisibleSong, row, first, last)
	}
	if next := lastVisibleSong + 1; next < len(a.trackRow) && a.trackRow[next] <= last {
		t.Fatalf("visible song %d at row %d was missed (Visible=(%d,%d))", next, a.trackRow[next], first, last)
	}
	probeIdx = lastVisibleSong
	tt.Advance(380 * time.Millisecond)
	tt.Frame()
	if probeShown <= 0 || probeShown >= 1 {
		t.Fatalf("grouped last visible song %d shown=%v at 380ms, want in (0,1)", lastVisibleSong, probeShown)
	}

	// ReduceMotion / motionOff and motionLite skip entrance immediately on frame 0.
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	a.playlists[12] = &netease.Playlist{ID: 12, Name: "Still Playlist", Tracks: tracks, TrackCount: len(tracks)}
	a.open(route{page: pagePlaylist, id: 12})
	probeIdx = 0
	tt.Frame()
	if a.rowsPending || a.rowsCount != 0 || probeShown != 1 {
		t.Fatalf("ReduceMotion should skip entrance: pending=%v count=%d shown=%v", a.rowsPending, a.rowsCount, probeShown)
	}
	tt.SetPreferences(ui.Preferences{ReduceMotion: false})
	a.settings.Motion = "lite"
	a.playlists[13] = &netease.Playlist{ID: 13, Name: "Lite Playlist", Tracks: tracks, TrackCount: len(tracks)}
	a.open(route{page: pagePlaylist, id: 13})
	tt.Frame()
	if a.rowsPending || a.rowsCount != 0 || probeShown != 1 {
		t.Fatalf("motionLite should skip entrance: pending=%v count=%d shown=%v", a.rowsPending, a.rowsCount, probeShown)
	}

	// Back navigation skips entrance even in motionFull.
	a.settings.Motion = "full"
	a.back()
	probeIdx = 0
	tt.Frame()
	if a.rowsPending || a.rowsCount != 0 || probeShown != 1 {
		t.Fatalf("back navigation should skip entrance: pending=%v count=%d shown=%v", a.rowsPending, a.rowsCount, probeShown)
	}
}
