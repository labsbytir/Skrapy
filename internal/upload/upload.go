package upload

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Uploader posts chunk files to the Cloudflare upload Worker.
type Uploader struct {
	Endpoint string
	APIKey   string
	Client   *http.Client
}

func New(endpoint, apiKey string) *Uploader {
	return &Uploader{
		Endpoint: endpoint,
		APIKey:   apiKey,
		Client:   &http.Client{Timeout: 5 * time.Minute},
	}
}

// Upload posts a single chunk. batchID must be unique per collect run.
func (u *Uploader) Upload(batchID string, chunk Chunk) error {
	f, err := os.Open(chunk.Path)
	if err != nil {
		return err
	}
	defer f.Close()

	req, err := http.NewRequest("POST", u.Endpoint+"/upload", f)
	if err != nil {
		return err
	}
	req.ContentLength = chunk.Bytes
	req.Header.Set("x-api-key", u.APIKey)
	req.Header.Set("x-batch-id", batchID)
	req.Header.Set("x-chunk-index", fmt.Sprintf("%d", chunk.Index))
	req.Header.Set("x-chunk-total", fmt.Sprintf("%d", chunk.Total))
	req.Header.Set("x-source", "skrapy")
	req.Header.Set("content-type", "application/jsonl")

	resp, err := u.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
