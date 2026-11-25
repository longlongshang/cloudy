package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The issue is written with what was typed: its first line the title, all
// of it and the versions the body. Nothing typed, the templates open; too
// long for an address, the body is left to paste.
func TestFeedbackIssueURL(t *testing.T) {
	link, long := feedbackIssueURL("  桌面歌词偶尔会闪\n切歌的时候  ")
	if long {
		t.Fatal("a short message is long")
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	if u.Host != "github.com" || u.Path != "/nya-owo/cloudy/issues/new" {
		t.Errorf("link %q", link)
	}
	if got := u.Query().Get("title"); got != "桌面歌词偶尔会闪" {
		t.Errorf("title %q", got)
	}
	if body := u.Query().Get("body"); !strings.HasPrefix(body, "桌面歌词偶尔会闪\n切歌的时候\n\n——\nCloudy ") {
		t.Errorf("body %q", body)
	}
	if link, _ := feedbackIssueURL("   "); link != feedbackIssues+"/choose" {
		t.Errorf("empty: %q", link)
	}
	link, long = feedbackIssueURL(strings.Repeat("很长的反馈", 600))
	if !long || len(link) > feedbackIssueMost || strings.Contains(link, "body=") {
		t.Errorf("long: %v, %d bytes", long, len(link))
	}
}

func TestFeedbackToIssue(t *testing.T) {
	oldCopy, oldOpen := copyText, openURL
	defer func() { copyText, openURL = oldCopy, oldOpen }()
	var copied, opened string
	copyText = func(s string) error { copied = s; return nil }
	openURL = func(u string) error { opened = u; return nil }
	a := newTestApp()
	a.feedback.text = "想要一个睡眠定时"
	a.feedbackToIssue()
	if !strings.HasPrefix(opened, feedbackIssues+"?") || copied != "" || a.feedback.failed {
		t.Errorf("opened %q copied %q note %q", opened, copied, a.feedback.note)
	}
	a.feedback.text = strings.Repeat("很长的反馈", 600)
	a.feedbackToIssue()
	if !strings.HasPrefix(copied, "很长的反馈") || a.feedback.failed {
		t.Errorf("long: copied %d bytes, note %q", len(copied), a.feedback.note)
	}
}

// The wall's file reads, empty names dropped; the one built in parses.
func TestParseSupport(t *testing.T) {
	d, err := parseSupport([]byte(`{"updates":37,"feedback":5,"supporters":[{"name":" 柚子 ","note":"加油"},{"name":"  "}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if d.Updates != 37 || d.Feedback != 5 || len(d.Supporters) != 1 || d.Supporters[0].Name != "柚子" {
		t.Errorf("%+v", d)
	}
	if _, err := supportFiles.ReadFile("support/supporters.json"); err != nil {
		t.Fatal(err)
	}
	if e := embeddedSupport(); e.Updates == 0 {
		t.Errorf("embedded %+v", e)
	}
	if codes := supportCodes(); codes[0] == nil || codes[1] == nil {
		t.Error("the pay codes built in do not decode")
	}
	if supportInitial("midnight") != "M" || supportInitial("") != "?" || supportInitial("云边") != "云" || supportInitial("꧁꫞「Six」☬「Seven」꫞꧂") != "S" || supportInitial("꧁꧂") != "꧁" {
		t.Error("initials")
	}
	// Every name built in keeps a seat of its own.
	seen := map[int]bool{}
	for _, s := range embeddedSupport().Supporters {
		if s.No <= 0 || seen[s.No] {
			t.Errorf("%q has seat %d", s.Name, s.No)
		}
		seen[s.No] = true
	}
}

// Seats: kept where the file gives them, the rest, earliest first, to the
// first seats free; the hall grows by a hundred once full.
func TestSupportSeats(t *testing.T) {
	d := supportData{Supporters: []supporter{{Name: "newest"}, {Name: "kept", No: 1}, {Name: "twin", No: 1}, {Name: "third", No: 3}, {Name: "oldest"}}}
	got := d.seated()
	want := []string{"kept", "oldest", "third", "twin", "newest"}
	for i, s := range got {
		if s.Name != want[i] || s.No != i+1 {
			t.Fatalf("seated %+v", got)
		}
	}
	if seats, next := supportSeats(got); seats != 100 || next != 6 {
		t.Errorf("seats %d next %d", seats, next)
	}
	if seats, next := supportSeats(nil); seats != 100 || next != 1 {
		t.Errorf("empty: seats %d next %d", seats, next)
	}
	var full []supporter
	for i := range 100 {
		full = append(full, supporter{No: i + 1, Name: fmt.Sprint(i)})
	}
	if seats, next := supportSeats(full); seats != 200 || next != 101 {
		t.Errorf("full: seats %d next %d", seats, next)
	}
	if r, s := seatPlace(26); r != 2 || s != 1 {
		t.Errorf("seat 26 at %d %d", r, s)
	}
}

// A name cut short in its row shows whole when the row is clicked; past
// twelve names, 查看全部 shows them all.
func TestSupportWallNames(t *testing.T) {
	a := newTestApp()
	long := "꧁꫞「Six」☬「Seven」꫞꧂ 一个很长很长很长很长很长很长的名字"
	sup := []supporter{{Name: long, No: 1}}
	for i := 2; i <= 14; i++ {
		sup = append(sup, supporter{Name: fmt.Sprint("友", i), No: i})
	}
	a.support = supportState{read: true, fetched: true, page: true, data: supportData{Supporters: sup}}
	a.settings = defaultSettings()
	a.settingsView.open, a.settingsView.tab = true, settingsAbout
	tt := ui.NewTester(a.view, 1180, 820)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	// Down to the names.
	a.settingsView.scroll[settingsAbout].Y = 1000
	tt.Frame()
	tt.Frame()
	if !tt.HasText("前") || !tt.HasText("位支持者") || !tt.HasText("012") || tt.HasText("013") || !tt.HasText("下一个座位") || !tt.HasText("015") {
		t.Fatal("no hall, or not the first twelve and the next seat")
	}
	if err := tt.Click("No.002 友2"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.support.name != 0 {
		t.Error("a name shown whole opened")
	}
	if err := tt.Click("No.001 " + long); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if a.support.name != 1 || !tt.HasText("第 1 排 1 座") {
		t.Error("the name cut short does not show whole")
	}
	a.support.name = 0
	tt.Frame()
	if err := tt.Click("查看全部 14 位"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !a.support.all || !tt.HasText("013") || !tt.HasText("014") {
		t.Error("查看全部 does not show every name")
	}
}

// The card under 关于 opens the page in its place; the page shows the
// wall GitHub holds, and goes back; another category leaves it.
func TestSupportPage(t *testing.T) {
	a := newTestApp()
	// As read from GitHub: posts are dropped in tests.
	a.support = supportState{read: true, fetched: true, data: supportData{Updates: 38, Supporters: []supporter{{Name: "云边的猫", Note: "丝绸背景太好看了"}}}}
	a.settings = defaultSettings()
	a.settingsView.open, a.settingsView.tab = true, settingsAbout
	tt := ui.NewTester(a.view, 1180, 820)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if !tt.HasText("支持 Cloudy") || !tt.HasText("去支持") {
		t.Fatal("关于 has no support card")
	}
	if err := tt.Click("去支持"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !a.support.page || !tt.HasText("让 Cloudy 一直唱下去") || !tt.HasText("云边的猫") || !tt.HasText("001") || !tt.HasText("38") {
		t.Fatal("the page does not show, or not the wall read")
	}
	if !tt.HasText("微信收款码\n还没有放") && !tt.HasText("扫码请 nya 喝杯奶茶") {
		t.Fatal("no codes")
	}
	// The code is covered: a first click uncovers it, and only then does
	// a click show it large.
	if a.support.shown {
		t.Fatal("the code shows before it was asked for")
	}
	if err := tt.Click("点一下显示微信收款码"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !a.support.shown || a.support.large {
		t.Fatalf("a first click: shown %v large %v, want shown only", a.support.shown, a.support.large)
	}
	if err := tt.Click("放大微信收款码"); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	tt.Frame()
	if !a.support.large {
		t.Error("the code does not open large")
	}
	a.support.large = false
	tt.Advance(time.Second)
	tt.Frame()
	if err := tt.Click("返回关于"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.support.page {
		t.Error("back did not go back")
	}
	a.support.page = true
	tt.Frame()
	_ = tt.Click("播放")
	tt.Frame()
	if a.support.page {
		t.Error("another category kept the page")
	}
}

// The wall's fetch failing leaves the one built in.
func TestSupportWallKeepsEmbedded(t *testing.T) {
	old := fetchSupport
	defer func() { fetchSupport = old }()
	fetchSupport = func(context.Context) (supportData, error) { return supportData{}, errors.New("offline") }
	a := newTestApp()
	d := a.supportWall()
	time.Sleep(20 * time.Millisecond) // the fetch fails; without a window its post is dropped anyway
	if d.Updates != embeddedSupport().Updates {
		t.Errorf("%+v", *d)
	}
}

// 更新内容 offers the page, once, at its foot.
func TestWhatsNewOffersSupport(t *testing.T) {
	a := newTestApp()
	a.settings = defaultSettings()
	a.whatsNew = whatsNewState{open: true, version: "1.3.7", notes: "- 新增丝绸背景"}
	tt := ui.NewTester(a.view, 1180, 820)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	tt.Frame()
	if err := tt.Click("支持一下 nya"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.whatsNew.open || !a.support.page || !a.settingsView.open || a.settingsView.tab != settingsAbout {
		t.Error("the line does not open the support page")
	}
}
