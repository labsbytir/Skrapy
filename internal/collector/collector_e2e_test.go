package collector

import (
	"context"
	"os"
	"testing"
	"time"
)

// Requires a running Lightpanda: lightpanda serve --port 9333
func TestRunE2E(t *testing.T) {
	cdp := os.Getenv("LIGHTPANDA_CDP")
	if cdp == "" {
		t.Skip("LIGHTPANDA_CDP not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	res, err := Run(ctx, cdp, "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	if res.Title == "" || res.Text == "" {
		t.Fatalf("empty result: %+v", res)
	}
	t.Logf("title=%q text_len=%d", res.Title, len(res.Text))
}