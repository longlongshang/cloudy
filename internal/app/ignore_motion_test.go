package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// With Windows's animation effects off the app keeps still, unless the
// listener asked it to move anyway (PR #28); its own 关闭 still stills it,
// and the row offering it shows only where it changes something.
func TestIgnoreSystemMotion(t *testing.T) {
	defer ui.SetIgnoreReduceMotion(false)
	a := newTestApp()
	a.settings = defaultSettings()
	tt := ui.NewTester(a.view, 1100, 760)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if a.motion != motionOff {
		t.Fatalf("the desktop asks for less motion, motion is %d, want off", a.motion)
	}
	a.settings.IgnoreSystemMotion = true
	a.changed()
	tt.Frame()
	if a.motion != motionFull {
		t.Fatalf("ignoring the desktop, motion is %d, want full", a.motion)
	}
	a.settings.Motion = "off"
	tt.Frame()
	if a.motion != motionOff {
		t.Fatalf("the app's own 关闭, motion is %d, want off", a.motion)
	}
	a.settings.Motion = ""
	a.settings.IgnoreSystemMotion = false
	a.changed()
	tt.Frame()
	if a.motion != motionOff {
		t.Fatalf("no longer ignoring the desktop, motion is %d, want off", a.motion)
	}

	// The row: there while the desktop asks for less motion, not otherwise.
	a.settingsView.open, a.settingsView.tab = true, settingsAppearance
	tt.Frame()
	if _, ok := tt.Find("无视系统动效设置"); !ok {
		t.Error("no 无视系统动效设置 while the desktop asks for less motion")
	}
	tt.SetPreferences(ui.Preferences{})
	tt.Frame()
	if _, ok := tt.Find("无视系统动效设置"); ok {
		t.Error("无视系统动效设置 shown where it changes nothing")
	}
}
