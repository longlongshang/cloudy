package app

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// A supporter may have a picture on the wall: support/avatars/<seat>.png,
// named in supporters.json ("avatar"). The app has the ones it was built
// with; the others it reads from GitHub, or the mirrors, as the wall, and
// keeps in its cache folder, so they show at once the next time and still
// show offline. Read once a run: a picture changed shows the run after.

// supportRemoteDir is where the wall's files are kept on GitHub.
const supportRemoteDir = "https://raw.githubusercontent.com/" + releaseRepo + "/main/internal/app/support/"

// supportAvatarMost is the most a picture may weigh.
const supportAvatarMost = 1 << 20

// supportAvatars are the pictures read, by their path, and those asked
// for this run.
type supportAvatars struct {
	bmps  map[string]*ui.Bitmap
	asked map[string]bool
}

// supportAvatarPath reports whether path names a picture of the wall: in
// avatars/, a PNG or a JPEG, and nowhere else.
func supportAvatarPath(path string) bool {
	if !strings.HasPrefix(path, "avatars/") || len(path) > 80 ||
		strings.Contains(path, "..") || strings.ContainsAny(path, `\:?#%`) {
		return false
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}

// supportAvatar is s's picture, or nil while it is not read or where it
// has none: its round shows the initial then.
func (a *app) supportAvatar(s supporter) *ui.Bitmap {
	path := s.Avatar
	if path == "" || !supportAvatarPath(path) {
		return nil
	}
	av := &a.support.avatars
	if bmp := av.bmps[path]; bmp != nil {
		return bmp
	}
	if av.asked[path] {
		return nil
	}
	if av.asked == nil {
		av.asked, av.bmps = map[string]bool{}, map[string]*ui.Bitmap{}
	}
	av.asked[path] = true
	if b, err := supportFiles.ReadFile("support/" + path); err == nil {
		if bmp := supportAvatarBitmap(path, b); bmp != nil {
			av.bmps[path] = bmp
			return bmp
		}
	}
	// Without a window, as in tests, nothing would take what is read.
	if a.win == nil {
		return nil
	}
	go a.readSupportAvatar(path)
	return nil
}

// readSupportAvatar shows the picture kept in the cache, then reads it
// again from GitHub, or the mirrors, keeping what it gets.
func (a *app) readSupportAvatar(path string) {
	show := func(b []byte) bool {
		bmp := supportAvatarBitmap(path, b)
		if bmp == nil {
			return false
		}
		a.post(func() {
			a.support.avatars.bmps[path] = bmp
			a.redraw()
		})
		return true
	}
	kept := supportAvatarCache(path)
	old, _ := os.ReadFile(kept)
	if len(old) > 0 {
		show(old)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	b, err := fetchSupportAvatar(ctx, path)
	if err != nil {
		log.Printf("support: %s: %v", path, err)
		return
	}
	if bytes.Equal(b, old) || !show(b) {
		return
	}
	if kept != "" && os.MkdirAll(filepath.Dir(kept), 0o755) == nil {
		os.WriteFile(kept, b, 0o644)
	}
}

// supportAvatarCache is where the picture at path is kept, or "".
func supportAvatarCache(path string) string {
	dir, err := mygo.App.Path(mygo.PathCache)
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "support", filepath.FromSlash(path))
}

// fetchSupportAvatar reads the picture at path from GitHub, then each
// mirror; tests swap it.
var fetchSupportAvatar = func(ctx context.Context, path string) ([]byte, error) {
	var last error
	for _, prefix := range append([]string{""}, updateMirrors...) {
		b, err := fetchSupportFile(ctx, prefix+supportRemoteDir+path)
		if err == nil {
			return b, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	return nil, last
}

func fetchSupportFile(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s", res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, supportAvatarMost+1))
	if err != nil {
		return nil, err
	}
	if len(b) > supportAvatarMost {
		return nil, fmt.Errorf("larger than %d bytes", supportAvatarMost)
	}
	return b, nil
}

// supportAvatarBitmap decodes a picture, nil if it is none.
func supportAvatarBitmap(path string, b []byte) *ui.Bitmap {
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		log.Printf("support: %s: %v", path, err)
		return nil
	}
	return ui.NewBitmap(img)
}
