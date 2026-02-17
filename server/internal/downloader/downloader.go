package downloader

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type Downloader struct {
	URL     string
	ETag    string
	Content string
	Timeout time.Duration
}

func (d *Downloader) Fetch() (string, string, error) {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}
	ipv4Client := createIPv4Client(timeout)

	var lastErr error
	for _, url := range ExpandGitHubURL(d.URL) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			if isIPv6RelatedError(err) {
				resp, err = ipv4Client.Do(req)
				if err != nil {
					lastErr = err
					continue
				}
			} else {
				lastErr = err
				continue
			}
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			continue
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}
		d.ETag = resp.Header.Get("ETag")
		d.Content = string(bodyBytes)
		return d.Content, d.ETag, nil
	}
	return "", "", lastErr
}

func isIPv6RelatedError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "timeout") || strings.Contains(s, "no such host") ||
		strings.Contains(s, "network is unreachable") || strings.Contains(s, "connection refused") ||
		strings.Contains(s, "i/o timeout") || strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "context deadline")
}

func createIPv4Client(timeout time.Duration) *http.Client {
	dialer := &net.Dialer{Timeout: timeout, KeepAlive: 30 * time.Second}
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _ string, addr string) (net.Conn, error) {
				return dialer.DialContext(ctx, "tcp4", addr)
			},
		},
	}
}

func (d *Downloader) FetchIfChanged() (string, string, error) {
	timeout := d.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	httpClient := &http.Client{Timeout: timeout}

	var lastErr error
	for _, url := range ExpandGitHubURL(d.URL) {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if d.ETag != "" {
			req.Header.Set("If-None-Match", d.ETag)
		}
		resp, err := httpClient.Do(req)
		if err != nil {
			if isIPv6RelatedError(err) {
				resp, err = createIPv4Client(timeout).Do(req)
				if err != nil {
					lastErr = err
					continue
				}
			} else {
				lastErr = err
				continue
			}
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusNotModified {
			return d.Content, d.ETag, nil
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			continue
		}
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = err
			continue
		}
		d.Content = string(bodyBytes)
		d.ETag = resp.Header.Get("ETag")
		return d.Content, d.ETag, nil
	}
	return "", "", lastErr
}
