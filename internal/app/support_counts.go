package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

// The issues solved and the pull requests merged, as the support page's
// head shows them: asked of GitHub's search once a run, the first time the
// page shows, and where GitHub cannot be reached (its API has no mirror),
// as supporters.json last wrote them down.

// supportCounts are the numbers GitHub gave this run, 0 where it gave none.
type supportCounts struct {
	asked       bool
	issues, prs int
}

// supportIssueQuery and supportPRQuery are what the numbers count: issues
// closed as done, and pull requests merged, by anyone.
const (
	supportIssueQuery = "repo:" + releaseRepo + " is:issue is:closed reason:completed"
	supportPRQuery    = "repo:" + releaseRepo + " is:pr is:merged"
)

// supportIssuesPage and supportPRsPage are where a click on a number goes.
const (
	supportIssuesPage = "https://github.com/" + releaseRepo + "/issues?q=is%3Aissue+is%3Aclosed+reason%3Acompleted"
	supportPRsPage    = "https://github.com/" + releaseRepo + "/pulls?q=is%3Apr+is%3Amerged"
)

// supportWorkCounts are the issues and pull requests to show: GitHub's
// numbers once read, else the file's, and it asks for GitHub's the first
// time.
func (a *app) supportWorkCounts(d *supportData) (issues, prs int) {
	st := &a.support
	if !st.counts.asked && a.win != nil {
		st.counts.asked = true
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			issues, err := fetchSupportCount(ctx, supportIssueQuery)
			if err != nil {
				log.Printf("support: issues: %v", err)
				return
			}
			prs, err := fetchSupportCount(ctx, supportPRQuery)
			if err != nil {
				log.Printf("support: pull requests: %v", err)
				return
			}
			a.post(func() {
				a.support.counts.issues, a.support.counts.prs = issues, prs
				a.redraw()
			})
		}()
	}
	issues, prs = d.Issues, d.PRs
	if st.counts.issues > 0 {
		issues = st.counts.issues
	}
	if st.counts.prs > 0 {
		prs = st.counts.prs
	}
	return issues, prs
}

// fetchSupportCount is how many GitHub's search finds for query; tests
// swap it.
var fetchSupportCount = func(ctx context.Context, query string) (int, error) {
	u := "https://api.github.com/search/issues?per_page=1&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("%s", res.Status)
	}
	var out struct {
		Total int `json:"total_count"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out); err != nil {
		return 0, err
	}
	return out.Total, nil
}
