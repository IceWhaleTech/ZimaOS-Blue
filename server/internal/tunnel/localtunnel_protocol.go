// Package tunnel: localtunnel client protocol (compatible with https://github.com/localtunnel/localtunnel).
// Protocol: GET host/?new (or host/subdomain) for tunnel info, then TCP proxy to assigned remote_ip:remote_port.

package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	localtunnelDefaultHost = "https://localtunnel.me"
	localtunnelDialTimeout = 15 * time.Second
	localtunnelHTTPTimeout = 15 * time.Second
)

// localtunnelInfo is the JSON response from GET host/?new or host/subdomain.
type localtunnelInfo struct {
	ID           string `json:"id"`
	IP           string `json:"ip"`
	Port         int    `json:"port"`
	URL          string `json:"url"`
	CachedURL    string `json:"cached_url,omitempty"`
	MaxConnCount int    `json:"max_conn_count"`
}

// requestLocaltunnel requests a new tunnel from the server. subdomain can be empty for random.
func requestLocaltunnel(ctx context.Context, host string, subdomain string) (*localtunnelInfo, error) {
	base, err := url.Parse(host)
	if err != nil {
		return nil, fmt.Errorf("localtunnel host: %w", err)
	}
	if base.Path == "" {
		base.Path = "/"
	}
	var requestURL string
	if subdomain != "" {
		requestURL = base.ResolveReference(&url.URL{Path: subdomain}).String()
	} else {
		// GET https://localtunnel.me/?new to request a new random tunnel
		requestURL = base.ResolveReference(&url.URL{Path: "/", RawQuery: "new"}).String()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: localtunnelHTTPTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("localtunnel request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("localtunnel server returned %d: %s", resp.StatusCode, string(body))
	}

	var info localtunnelInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("localtunnel response: %w", err)
	}
	if info.URL == "" || info.Port == 0 {
		return nil, fmt.Errorf("localtunnel invalid response: missing url or port")
	}
	if info.MaxConnCount <= 0 {
		info.MaxConnCount = 1
	}
	return &info, nil
}

// runLocaltunnelClient runs the TCP proxy: dial remote (localtunnel server), then local, and copy bidirectionally.
// onURL is called once with the public URL when the first connection is established.
func runLocaltunnelClient(ctx context.Context, info *localtunnelInfo, localPort int, onURL func(string)) error {
	remoteAddr := fmt.Sprintf("%s:%d", info.IP, info.Port)
	onURL(info.URL)

	numConns := info.MaxConnCount
	if numConns > 4 {
		numConns = 4
	}

	var wg sync.WaitGroup
	for i := 0; i < numConns; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				remote, err := net.DialTimeout("tcp", remoteAddr, localtunnelDialTimeout)
				if err != nil {
					select {
					case <-ctx.Done():
						return
					default:
						time.Sleep(2 * time.Second)
					}
					continue
				}

				local, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", localPort), 5*time.Second)
				if err != nil {
					remote.Close()
					time.Sleep(1 * time.Second)
					continue
				}

				bidirectionalCopy(remote, local)

				select {
				case <-ctx.Done():
					return
				default:
					// connection closed, reopen after short delay
					time.Sleep(500 * time.Millisecond)
				}
			}
		}()
	}

	wg.Wait()
	return ctx.Err()
}

// bidirectionalCopy is in bore_protocol.go (shared).
