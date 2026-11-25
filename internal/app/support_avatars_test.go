package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// Only pictures in avatars/ are read, so the file can point nowhere else.
func TestSupportAvatarPath(t *testing.T) {
	for path, want := range map[string]bool{
		"avatars/3.png":             true,
		"avatars/12.jpg":            true,
		"avatars/a.JPEG":            true,
		"supporters.json":           false,
		"avatars/../nya.jpg":        false,
		"avatars/3.gif":             false,
		"avatars/x.png?raw=1":       false,
		`avatars\3.png`:             false,
		"https://example.com/a.png": false,
	} {
		if got := supportAvatarPath(path); got != want {
			t.Errorf("supportAvatarPath(%q) = %v, want %v", path, got, want)
		}
	}
}

// The pictures built in show at once, and a supporter without one, or with
// a path that is no picture's, keeps the initial.
func TestSupportAvatarsBuiltIn(t *testing.T) {
	a := newTestApp()
	for _, s := range embeddedSupport().Supporters {
		if s.Avatar != "" && a.supportAvatar(s) == nil {
			t.Errorf("%s: picture %s not read", s.Name, s.Avatar)
		}
	}
	if a.supportAvatar(supporter{Name: "无"}) != nil {
		t.Error("a picture for a supporter without one")
	}
	if a.supportAvatar(supporter{Name: "坏", Avatar: "../go.mod"}) != nil {
		t.Error("a picture read from outside avatars/")
	}
	if supportNya() == nil {
		t.Error("nya's picture not built in")
	}
	tt := ui.NewTester(a.view, 1100, 760)
	tt.Frame()
}

// The issues and pull requests show as GitHub counts them once read, and
// as the file wrote them down before, or where GitHub gave nothing.
func TestSupportWorkCounts(t *testing.T) {
	a := newTestApp()
	d := &supportData{Issues: 11, PRs: 17}
	if i, p := a.supportWorkCounts(d); i != 11 || p != 17 {
		t.Fatalf("before GitHub: %d %d, want the file's 11 17", i, p)
	}
	a.support.counts.issues, a.support.counts.prs = 13, 20
	if i, p := a.supportWorkCounts(d); i != 13 || p != 20 {
		t.Fatalf("after GitHub: %d %d, want 13 20", i, p)
	}
	if i, p := a.supportWorkCounts(&supportData{}); i != 13 || p != 20 {
		t.Fatalf("a file without them: %d %d, want GitHub's 13 20", i, p)
	}
}
