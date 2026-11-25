package netease

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// The diagnostic log, netease.log in the user data directory, records each
// request to NetEase, the cookies it sets and the steps of a login, for a
// listener to send in when the session stops working. Cookie values never
// go in it, only a short digest of each, enough to tell two apart.
// It starts with StartDiagLog, so tests write none.

const (
	diagName = "netease.log"
	diagMax  = 2 << 20 // bytes; past it the log moves to netease.log.1
)

var diag struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
}

// StartDiagLog opens the log, after what was kept of the last launch, and
// notes which version is running.
func StartDiagLog(version string) {
	dir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return
	}
	startDiagLogIn(dir)
	diagf("start: version %s, device %s", version, digest(device()))
}

func startDiagLogIn(dir string) {
	diag.mu.Lock()
	defer diag.mu.Unlock()
	if diag.f != nil {
		return
	}
	os.MkdirAll(dir, 0o700)
	diag.path = filepath.Join(dir, diagName)
	diagOpen()
}

// diagOpen opens the log to add to it. The caller holds diag.mu.
func diagOpen() {
	f, err := os.OpenFile(diag.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		diag.f = nil
		return
	}
	diag.f = f
	diag.size = 0
	if st, err := f.Stat(); err == nil {
		diag.size = st.Size()
	}
}

// DiagLogPath is where the log is, or "" before StartDiagLog.
func DiagLogPath() string {
	diag.mu.Lock()
	defer diag.mu.Unlock()
	if diag.f != nil {
		diag.f.Sync()
	}
	return diag.path
}

// diagf adds a line to the log, if it is open.
func diagf(format string, args ...any) {
	diag.mu.Lock()
	defer diag.mu.Unlock()
	if diag.f == nil {
		return
	}
	line := time.Now().Format("2006-01-02 15:04:05.000 ") + fmt.Sprintf(format, args...)
	line = strings.ReplaceAll(line, "\n", " ") + "\n"
	n, _ := diag.f.WriteString(line)
	diag.size += int64(n)
	if diag.size > diagMax {
		diag.f.Close()
		// Windows renames onto no file that exists.
		os.Remove(diag.path + ".1")
		os.Rename(diag.path, diag.path+".1")
		diagOpen()
	}
}

// digest stands for a secret in the log: the first bytes of its hash.
func digest(v string) string {
	if v == "" {
		return "(empty)"
	}
	h := sha256.Sum256([]byte(v))
	return hex.EncodeToString(h[:4])
}

// diagCookies describes cookies without their values: name, digest, and
// where and how long the site means them to be kept.
func diagCookies(cookies []*http.Cookie) string {
	parts := make([]string, 0, len(cookies))
	for _, c := range cookies {
		s := c.Name + "=" + digest(c.Value)
		if c.Domain != "" {
			s += " domain=" + c.Domain
		}
		if c.Path != "" && c.Path != "/" {
			s += " path=" + c.Path
		}
		switch {
		case c.MaxAge < 0:
			s += " delete"
		case c.MaxAge > 0:
			s += fmt.Sprintf(" max-age=%d", c.MaxAge)
		}
		if !c.Expires.IsZero() {
			s += " expires=" + c.Expires.UTC().Format(time.RFC3339)
		}
		parts = append(parts, s)
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

// diagSession describes the session cookies the jar holds now: more than
// one MUSIC_U means requests send them all.
func diagSession() string {
	if httpClient.Jar == nil {
		return "[]"
	}
	var session []*http.Cookie
	for _, c := range httpClient.Jar.Cookies(siteURL) {
		if c.Name == "MUSIC_U" || c.Name == "__csrf" {
			session = append(session, c)
		}
	}
	return diagCookies(session)
}

// diagReply notes a request's answer: its HTTP status, NetEase's code and
// message when not 200, how long it took and the cookies it set.
func diagReply(method, path string, start time.Time, res *http.Response, status *apiError, err error) {
	took := time.Since(start).Milliseconds()
	if err != nil {
		diagf("%s %s %dms error: %v", method, path, took, err)
		return
	}
	line := fmt.Sprintf("%s %s http=%d", method, path, res.StatusCode)
	if status != nil {
		line += fmt.Sprintf(" code=%d", status.Code)
		if status.Code != 200 {
			msg := status.Message
			if msg == "" {
				msg = status.Msg
			}
			if len(msg) > 200 {
				msg = msg[:200]
			}
			if msg != "" {
				line += fmt.Sprintf(" msg=%q", msg)
			}
		}
	}
	line += fmt.Sprintf(" %dms", took)
	if set := res.Cookies(); len(set) > 0 {
		line += " set-cookie=" + diagCookies(set)
	}
	diagf("%s", line)
}

// diagPrefix is the start of a reply that is not JSON, as a page of
// NetEase's own might be.
func diagPrefix(body []byte) string {
	if len(body) > 160 {
		body = body[:160]
	}
	return string(body)
}
