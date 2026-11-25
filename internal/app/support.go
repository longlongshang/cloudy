package app

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/egoist/mygo/ui"
)

// What the support page holds that is not code: the thanks wall and the
// pay codes, in support/ (its README says how to fill it). The wall is
// read from the repository too, so that a name added there shows without
// a new version; the codes are only what the app was built with.
//
//go:embed support
var supportFiles embed.FS

// supportData is support/supporters.json: the wall, and the numbers the
// page shows of the work (0 hides one). Issues and PRs are as GitHub
// counted them when the file was written: the page asks GitHub for them
// as they are now (support_counts.go) and shows these where it cannot.
type supportData struct {
	Updates    int         `json:"updates"`
	Feedback   int         `json:"feedback"`
	Issues     int         `json:"issues,omitempty"`
	PRs        int         `json:"prs,omitempty"`
	Supporters []supporter `json:"supporters"`
}

// A supporter is a name on the wall, newest first, and the seat it keeps:
// No, from 1, in the order they came. Note is what one said, kept in the
// file for 1.3.7, which shows it; the wall shows names alone.
type supporter struct {
	No   int    `json:"no,omitempty"`
	Name string `json:"name"`
	Note string `json:"note,omitempty"`
	// Avatar is the picture its round shows, as avatars/3.png in support/
	// (support_avatars.go): the initial where it has none.
	Avatar string `json:"avatar,omitempty"`
}

// supportRemote is where the wall is kept, read straight from GitHub or
// through the update mirrors, which serve its raw files as well.
const supportRemote = supportRemoteDir + "supporters.json"

// supportState is the support page: shown in place of 关于, the code
// chosen, the wall as read, and the code shown large.
type supportState struct {
	page    bool
	method  int // 0 微信, 1 支付宝
	data    supportData
	read    bool // data holds the embedded wall, at least
	fetched bool // the wall was asked of GitHub, this run
	all     bool // the whole wall shows, in a dialog
	large   bool // the code shows large, in a dialog
	shown   bool // the code is uncovered: each visit asks for it again
	name    int  // the seat whose name, cut short in its row, shows whole
	avatars supportAvatars
	counts  supportCounts
}

// supportWallRows is how many names the page shows before 查看全部.
const supportWallRows = 12

// supportRowSeats is how many seats a row of the seat map holds, and
// supportBlock how many the wall offers at a time: the first 100, then,
// those taken, the next 100.
const (
	supportRowSeats = 25
	supportBlock    = 100
)

// seated is the wall in seat order, the first comer first. A supporter
// the file gives no seat, or one another took, gets the first seat free,
// the earlier comers (lower in the file) first.
func (d supportData) seated() []supporter {
	taken := map[int]bool{}
	var out, unseated []supporter
	for _, s := range d.Supporters {
		if s.No > 0 && !taken[s.No] {
			taken[s.No] = true
			out = append(out, s)
		} else {
			unseated = append(unseated, s)
		}
	}
	free := 1
	for i := len(unseated) - 1; i >= 0; i-- {
		for taken[free] {
			free++
		}
		s := unseated[i]
		s.No, taken[free] = free, true
		out = append(out, s)
	}
	slices.SortFunc(out, func(x, y supporter) int { return x.No - y.No })
	return out
}

// supportSeats is how many seats there are for seated (in seat order) and
// the first one free: always one, the hall growing by a block when full.
func supportSeats(seated []supporter) (seats, next int) {
	next = 1
	for _, s := range seated {
		if s.No == next {
			next++
		}
	}
	most := next
	if n := len(seated); n > 0 {
		most = max(most, seated[n-1].No)
	}
	return (most + supportBlock - 1) / supportBlock * supportBlock, next
}

// seatPlace is the row and the seat in it of number no, from 1.
func seatPlace(no int) (row, seat int) {
	return (no-1)/supportRowSeats + 1, (no-1)%supportRowSeats + 1
}

// parseSupport reads supporters.json, dropping names left empty.
func parseSupport(b []byte) (supportData, error) {
	var d supportData
	if err := json.Unmarshal(b, &d); err != nil {
		return supportData{}, err
	}
	kept := d.Supporters[:0]
	for _, s := range d.Supporters {
		s.Name, s.Note = strings.TrimSpace(s.Name), strings.TrimSpace(s.Note)
		if s.Name != "" {
			kept = append(kept, s)
		}
	}
	d.Supporters = kept
	return d, nil
}

// embeddedSupport is the wall the app was built with.
func embeddedSupport() supportData {
	b, err := supportFiles.ReadFile("support/supporters.json")
	if err != nil {
		return supportData{}
	}
	d, err := parseSupport(b)
	if err != nil {
		log.Printf("support: %v", err)
	}
	return d
}

// fetchSupport asks for the wall as it is now, from GitHub and then each
// mirror; tests swap it.
var fetchSupport = func(ctx context.Context) (supportData, error) {
	var last error
	for _, prefix := range append([]string{""}, updateMirrors...) {
		d, err := fetchSupportFrom(ctx, prefix+supportRemote)
		if err == nil {
			return d, nil
		}
		last = err
		if ctx.Err() != nil {
			break
		}
	}
	return supportData{}, last
}

func fetchSupportFrom(ctx context.Context, url string) (supportData, error) {
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return supportData{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return supportData{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return supportData{}, fmt.Errorf("support: %s", res.Status)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return supportData{}, err
	}
	return parseSupport(b)
}

// supportWall is the wall to show: the embedded one at first, then, once,
// what GitHub holds, which replaces it when it is read.
func (a *app) supportWall() *supportData {
	st := &a.support
	if !st.read {
		st.read, st.data = true, embeddedSupport()
	}
	// Without a window, as in tests, nothing would take what is read.
	if !st.fetched && a.win != nil {
		st.fetched = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()
			d, err := fetchSupport(ctx)
			if err != nil {
				log.Printf("support: %v", err)
				return
			}
			a.post(func() {
				a.support.data = d
				a.redraw()
			})
		}()
	}
	return &st.data
}

// supportCodes are the pay codes, nil where the app has none.
var supportCodes = sync.OnceValue(func() [2]*ui.Bitmap {
	var out [2]*ui.Bitmap
	for i, name := range []string{"wechat.png", "alipay.png"} {
		b, err := supportFiles.ReadFile("support/" + name)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			log.Printf("support: %s: %v", name, err)
			continue
		}
		out[i] = ui.NewBitmap(img)
	}
	return out
})

// openSupport shows the support page, in the settings under 关于.
func (a *app) openSupport() {
	a.support.page, a.support.all, a.support.shown = true, false, false
	a.openSettingsAt(settingsAbout)
}

// supportInitial is the letter of a name its round shows: the first
// letter or digit, past the marks some names are dressed in (꧁꫞「…).
func supportInitial(name string) string {
	first := ""
	for _, r := range name {
		if first == "" && !unicode.IsSpace(r) {
			first = string(r)
		}
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return string(unicode.ToUpper(r))
		}
	}
	if first != "" {
		return first
	}
	return "?"
}

// supportTints are the colors of the rounds, chosen by the name.
var supportTints = []ui.Color{ui.Hex("#5b7bd5"), ui.Hex("#c58a3a"), ui.Hex("#4a9a7a"), ui.Hex("#9a5bb5"), ui.Hex("#d0605e"), ui.Hex("#3f8fb0")}

func supportTint(name string) ui.Color {
	h := uint32(2166136261)
	for _, b := range []byte(name) {
		h = (h ^ uint32(b)) * 16777619
	}
	return supportTints[h%uint32(len(supportTints))]
}
