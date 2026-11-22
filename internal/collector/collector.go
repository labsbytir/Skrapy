package collector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

type Result struct {
	URL      string            `json:"url"`
	FinalURL string            `json:"finalUrl"`
	Status   int               `json:"status"`
	Title    string            `json:"title"`
	Meta     map[string]string `json:"meta"`
	Headings []string          `json:"headings"`
	Text     string            `json:"text"`
	Links    []string          `json:"links"`
	Forms    []Form            `json:"forms"`
	JSONLD   string            `json:"jsonld"`
	Error    string            `json:"error,omitempty"`
	Fetched  time.Time         `json:"fetched"`
}

// extractionJS returns the page HTML plus the current URL in one round trip.
const extractionJS = `(() => ({url: location.href, html: document.documentElement.outerHTML}))()`

// Run connects to a Lightpanda CDP endpoint and fetches the given URL.
func Run(ctx context.Context, cdpURL, target string) (Result, error) {
	alloc, cancel := chromedp.NewRemoteAllocator(ctx, cdpURL)
	defer cancel()
	taskCtx, cancelTask := chromedp.NewContext(alloc)
	defer cancelTask()

	var out struct {
		URL  string `json:"url"`
		HTML string `json:"html"`
	}
	err := chromedp.Run(taskCtx,
		chromedp.Navigate(target),
		chromedp.Evaluate(extractionJS, &out),
	)
	if err != nil {
		return Result{URL: target, Error: err.Error(), Fetched: time.Now()},
			fmt.Errorf("collect %s: %w", target, err)
	}
	res := ParseDocument(target, out.URL, 0, out.HTML)
	if res.FinalURL == "" || !strings.HasPrefix(res.FinalURL, "http") {
		res.FinalURL = target
	}
	return res, nil
}
