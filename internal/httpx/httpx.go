// Package httpx holds the capped, timed-out HTTP fetch shared across the app.
package httpx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	UserAgent = "wisp/0.1"

	DefaultTimeout = 15 * time.Second

	MaxBytes = 30 << 20 // bounds a misbehaving server, not real feed sizes — some podcast feeds with hundreds of episodes exceed 10MB
)

// FetchCapped refuses bodies over maxBytes, checked by actual length read.
func FetchCapped(ctx context.Context, url string, maxBytes int64) (body []byte, contentType string, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err = io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > maxBytes {
		return nil, "", fmt.Errorf("response exceeds %d bytes", maxBytes)
	}
	return body, resp.Header.Get("Content-Type"), nil
}
