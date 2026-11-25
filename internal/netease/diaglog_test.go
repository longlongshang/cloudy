package netease

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// useDiagLog opens the log in a directory of the test's, and closes it
// after.
func useDiagLog(t *testing.T) string {
	dir := t.TempDir()
	startDiagLogIn(dir)
	t.Cleanup(func() {
		diag.mu.Lock()
		diag.f.Close()
		diag.f, diag.path, diag.size = nil, "", 0
		diag.mu.Unlock()
	})
	return filepath.Join(dir, diagName)
}

// The log tells cookies apart by digest, and never holds their values.
func TestDiagLogKeepsCookieValuesOut(t *testing.T) {
	path := useDiagLog(t)
	res := &http.Response{StatusCode: 200, Header: http.Header{"Set-Cookie": {
		"MUSIC_U=very-secret-session; Max-Age=1296000; Domain=.music.163.com; Path=/",
	}}}
	diagReply("GET", "/api/x", time.Now(), res, &apiError{Code: 301, Message: "需要登录"}, nil)
	b, _ := os.ReadFile(path)
	log := string(b)
	if strings.Contains(log, "very-secret-session") {
		t.Fatalf("the log holds a cookie's value: %s", log)
	}
	for _, want := range []string{"code=301", "MUSIC_U=" + digest("very-secret-session"), "domain=.music.163.com", "max-age=1296000"} {
		if !strings.Contains(log, want) {
			t.Errorf("the log lacks %q: %s", want, log)
		}
	}
}

// Past its size, the log moves aside and starts again, keeping one before.
func TestDiagLogRotates(t *testing.T) {
	path := useDiagLog(t)
	line := strings.Repeat("x", 1000)
	for range diagMax/1000 + 10 {
		diagf("%s", line)
	}
	old, err := os.Stat(path + ".1")
	if err != nil {
		t.Fatal(err)
	}
	cur, _ := os.Stat(path)
	if old.Size() <= diagMax || cur.Size() >= diagMax {
		t.Errorf("sizes after rotating: %d and %d", old.Size(), cur.Size())
	}
}
