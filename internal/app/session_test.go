package app

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/audio"

	"netease-music-mygo/internal/netease"
)

func isolateSessionPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.json")
	old := sessionPath
	sessionPath = func() (string, error) { return path, nil }
	t.Cleanup(func() { sessionPath = old })
	return path
}

// The session is kept a while after playback changes and every while as
// it plays, and paused nothing more is due. It was kept by a ticker that
// woke the app, and redrew its window, every half minute even at rest.
func TestSessionRestsWhilePaused(t *testing.T) {
	isolateSessionPath(t)
	var due []func()
	defer func(after func(time.Duration, func())) { sessionAfter = after }(sessionAfter)
	sessionAfter = func(_ time.Duration, fn func()) { due = append(due, fn) }

	a := newTestApp()
	a.pl.Changed = a.keepSession
	a.pl.Cue([]netease.Track{{ID: 7, Name: "宝船", Duration: 200000}})
	out := audio.NewPlayer(&silence{})
	out.SetVolume(0)
	out.Play()
	a.pl.Attach(out)
	defer out.Close()

	a.pl.TogglePlay()
	a.pl.TogglePlay()
	if len(due) != 1 {
		t.Fatalf("%d saves due after pausing and playing", len(due))
	}
	due[0]()
	if len(due) != 2 {
		t.Fatalf("playing, %d saves due after one, want another", len(due)-1)
	}

	a.pl.TogglePlay()
	due[1]()
	if len(due) != 2 {
		t.Errorf("paused, %d more saves due", len(due)-2)
	}
}

// Clearing the queue immediately removes the saved session on disk so a
// restart stays empty, while normal save/restore and re-queueing still work.
// Previously saveSession returned early when the player had no current song,
// leaving the old session.json behind to be restored on the next launch (#45).
func TestSessionClearInvalidatesSavedSession(t *testing.T) {
	path := isolateSessionPath(t)
	var due []func()
	oldAfter := sessionAfter
	sessionAfter = func(_ time.Duration, fn func()) { due = append(due, fn) }
	t.Cleanup(func() { sessionAfter = oldAfter })

	dir := t.TempDir()
	tracks := []netease.Track{
		{ID: -1, Name: "第一首", Duration: 200000, LocalPath: filepath.Join(dir, "a.mp3")},
		{ID: -2, Name: "第二首", Duration: 180000, LocalPath: filepath.Join(dir, "b.mp3")},
	}

	// Normal save and restore keep the queued tracks across launches.
	a := newTestApp()
	a.pl.Changed = a.keepSession
	a.pl.Cue(tracks)
	a.saveSession()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file not written: %v", err)
	}

	b := newTestApp()
	b.pl.Changed = b.keepSession
	b.restoreSession()
	if len(b.pl.Queue()) != 2 || b.pl.Current() == nil || b.pl.Current().ID != tracks[0].ID {
		t.Fatalf("restored queue len=%d cur=%v, want 2 tracks starting at %d", len(b.pl.Queue()), b.pl.Current(), tracks[0].ID)
	}

	// Clearing only upcoming tracks keeps the current song and its session file.
	b.pl.ClearUpcoming()
	if !b.sessionDue || len(due) != 1 {
		t.Fatalf("ClearUpcoming did not schedule save: due=%v len=%d", b.sessionDue, len(due))
	}
	b.saveSession()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("ClearUpcoming removed session file: %v", err)
	}

	// Clearing the whole queue invalidates session.json immediately even while
	// a periodic save is already pending, without scheduling another timer.
	dueBefore := len(due)
	b.pl.Clear()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file still exists after Clear: err=%v", err)
	}
	if b.savedSession != "" {
		t.Fatalf("savedSession not cleared after remove: %q", b.savedSession)
	}
	if len(due) != dueBefore {
		t.Fatalf("Clear scheduled %d extra timer(s)", len(due)-dueBefore)
	}
	// Pending timer firing and quitting afterwards keep the session removed.
	for _, fn := range due {
		fn()
	}
	b.saveSession()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file reappeared after pending save/quit: err=%v", err)
	}

	// Simulated restart sees an empty queue.
	c := newTestApp()
	c.pl.Changed = c.keepSession
	c.restoreSession()
	if len(c.pl.Queue()) != 0 || c.pl.Current() != nil {
		t.Fatalf("restart restored tracks after Clear: len=%d cur=%v", len(c.pl.Queue()), c.pl.Current())
	}

	// Queueing again after Clear saves and restores normally, even for the same track.
	c.pl.Cue(tracks[:1])
	c.saveSession()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file not written after re-queue: %v", err)
	}
	// Clear again while no save timer is pending: removes immediately and stays idle.
	dueBefore = len(due)
	c.pl.Clear()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("session file not removed on idle Clear: err=%v", err)
	}
	if len(due) != dueBefore {
		t.Fatalf("idle Clear scheduled %d timer(s)", len(due)-dueBefore)
	}

	c.pl.Cue(tracks[:1])
	c.saveSession()
	d := newTestApp()
	t.Cleanup(func() { d.pl.Clear() })
	d.restoreSession()
	if len(d.pl.Queue()) != 1 || d.pl.Current() == nil || d.pl.Current().ID != tracks[0].ID {
		t.Fatalf("re-queued session did not restore: len=%d cur=%v", len(d.pl.Queue()), d.pl.Current())
	}
}

// Unrestored startup state and failed removals must not drop or mask a kept
// session: early saves before restoreSession keep disk state intact, and path
// or remove errors preserve savedSession so a later save retries.
func TestSessionClearGuardsAndFailureRetry(t *testing.T) {
	path := isolateSessionPath(t)
	oldAfter := sessionAfter
	sessionAfter = func(_ time.Duration, fn func()) {}
	t.Cleanup(func() { sessionAfter = oldAfter })

	tr := netease.Track{ID: -3, Name: "旧歌", Duration: 150000, LocalPath: filepath.Join(t.TempDir(), "old.mp3")}
	seed := newTestApp()
	seed.pl.Cue([]netease.Track{tr})
	seed.saveSession()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("seed session missing: %v", err)
	}

	// Before restoreSession runs on boot, an empty queue must not delete disk state.
	booting := newTestApp()
	booting.keepSession()
	booting.saveSession()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("startup before restoreSession deleted session file: %v", err)
	}

	booting.restoreSession()
	if booting.pl.Current() == nil || booting.pl.Current().ID != tr.ID || booting.savedSession == "" {
		t.Fatalf("restoreSession failed after early save: cur=%v saved=%q", booting.pl.Current(), booting.savedSession)
	}

	// Path lookup failure during Clear must not clear savedSession or delete the file.
	booting.pl.Changed = booting.keepSession
	sessionPath = func() (string, error) { return "", errors.New("user data unavailable") }
	booting.pl.Clear()
	if booting.savedSession == "" {
		t.Fatal("savedSession cleared despite sessionPath error")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("session file vanished despite sessionPath error: %v", err)
	}

	// os.Remove failure (non-empty directory at path) must keep savedSession for retry.
	sessionPath = func() (string, error) { return path, nil }
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove seed file: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(path, "keep"), 0o700); err != nil {
		t.Fatalf("create blocking dir: %v", err)
	}
	booting.saveSession()
	if booting.savedSession == "" {
		t.Fatal("savedSession cleared despite os.Remove failure")
	}

	// Once the failure clears, a subsequent saveSession removes the file and resets savedSession.
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("remove blocking dir: %v", err)
	}
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatalf("write stale session: %v", err)
	}
	booting.saveSession()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("retry did not remove session file: err=%v", err)
	}
	if booting.savedSession != "" {
		t.Fatalf("savedSession still set after successful retry: %q", booting.savedSession)
	}

	// An already-missing file (os.IsNotExist) also clears savedSession.
	booting.savedSession = "already-gone"
	booting.saveSession()
	if booting.savedSession != "" {
		t.Fatalf("savedSession not cleared for missing file: %q", booting.savedSession)
	}
}
