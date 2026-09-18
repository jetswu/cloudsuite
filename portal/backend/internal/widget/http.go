package widget

// http.go: minimal HTTP plumbing shared by the Stalwart / Nextcloud / Odoo
// widget clients.

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"time"
)

type resp struct {
	status int
	body   []byte
}

// post sends an HTTP request (body may be nil) and reads the response up to
// 1 MiB.
func (s *Service) post(ctx context.Context, url, contentType string, body []byte, headers map[string]string) (*resp, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, reader)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{Timeout: 15 * time.Second}
	hres, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer hres.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(hres.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	return &resp{status: hres.StatusCode, body: raw}, nil
}

func basicAuth(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}
