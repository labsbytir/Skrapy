package collector

import (
	"context"
	"fmt"
	"time"

	"github.com/chromedp/chromedp"
)

type Result struct {
	URL     string `json:"url"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Fetched time.Time `json:"fetched"`
}

// Run connects to a Lightpanda CDP endpoint and fetches the given URL.
func Run(ctx context.Context, cdpURL, target string) (Result, error) {
	alloc, cancel := chromedp.NewRemoteAllocator(ctx, cdpURL)
	defer cancel()
	taskCtx, cancelTask := chromedp.NewContext(alloc)
	defer cancelTask()

	var title, text string
	err := chromedp.Run(taskCtx,
		chromedp.Navigate(target),
		chromedp.Title(&title),
		chromedp.Text("body", &text, chromedp.ByQuery),
	)
	if err != nil {
		return Result{}, fmt.Errorf("collect %s: %w", target, err)
	}
	return Result{URL: target, Title: title, Text: text, Fetched: time.Now()}, nil
}