package tools

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	parallelSearchEndpoint    = "https://search.parallel.ai/mcp"
	parallelSearchProtocol    = "2025-03-26"
	parallelSearchUserAgent   = "ZimaOS-Blue (+https://github.com/IceWhaleTech/ZimaOS-Blue)"
	parallelSearchMaxResponse = 2 * 1024 * 1024
)

type parallelRPCResponse struct {
	JSONRPC string
	ID      int
	Result  json.RawMessage
	Error   *struct {
		Code    int
		Message string
	}
}

type parallelSearchClient struct {
	http     *http.Client
	endpoint string
	key      string
	session  string
}

// searchParallel adapts the anonymous Search MCP to Blue's existing result contract.
// Only provider-specific credentials and endpoints apply, so selecting Parallel
// cannot forward a key or self-hosted URL configured for another provider.
func (w *WebSearchTool) searchParallel(ctx context.Context, query string, maxResults int, _ string) (*WebSearchResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, w.config.Timeout)
	defer cancel()
	setting := w.providerSetting("parallel")
	endpoint := strings.TrimSpace(setting.BaseURL)
	if endpoint == "" {
		endpoint = parallelSearchEndpoint
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("invalid Parallel MCP endpoint")
	}
	client := parallelSearchClient{http: w.httpClient, endpoint: endpoint, key: strings.TrimSpace(setting.APIKey)}
	raw, err := client.request(ctx, 1, "initialize", map[string]interface{}{
		"protocolVersion": parallelSearchProtocol, "capabilities": map[string]interface{}{}, "clientInfo": map[string]string{"name": "ZimaOS-Blue", "version": "1"},
	})
	if err != nil {
		return nil, err
	}
	var initialized struct{ ProtocolVersion string }
	if err := json.Unmarshal(raw, &initialized); err != nil || initialized.ProtocolVersion != parallelSearchProtocol {
		return nil, errors.New("unsupported Parallel MCP protocol version")
	}
	if _, err := client.request(ctx, 0, "notifications/initialized", nil); err != nil {
		return nil, err
	}
	raw, err = client.request(ctx, 2, "tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var discovery struct{ Tools []struct{ Name string } }
	if err := json.Unmarshal(raw, &discovery); err != nil {
		return nil, fmt.Errorf("decode Parallel MCP discovery: %w", err)
	}
	found := false
	for _, tool := range discovery.Tools {
		if tool.Name == "web_search" {
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("Parallel MCP web_search tool is unavailable")
	}
	session := w.parallelSession
	if conversation := GetSessionID(ctx); conversation != "" {
		sum := sha256.Sum256([]byte(conversation))
		session = hex.EncodeToString(sum[:])
	}
	raw, err = client.request(ctx, 3, "tools/call", map[string]interface{}{"name": "web_search", "arguments": map[string]interface{}{
		"objective": query, "search_queries": []string{query}, "session_id": session,
	}})
	if err != nil {
		return nil, err
	}
	var result struct {
		StructuredContent json.RawMessage
		Content           []struct {
			Type string
			Text string
		}
		IsError bool
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("decode Parallel MCP tool result: %w", err)
	}
	if result.IsError {
		return nil, errors.New("Parallel MCP web_search returned a tool error")
	}
	payload := result.StructuredContent
	if len(payload) == 0 || string(payload) == "null" {
		for _, content := range result.Content {
			if content.Type == "text" {
				payload = json.RawMessage(content.Text)
				break
			}
		}
	}
	var search struct {
		Results *[]struct {
			Title    string
			URL      string
			Excerpts []string
		}
	}
	if len(payload) == 0 || json.Unmarshal(payload, &search) != nil || search.Results == nil {
		return nil, errors.New("invalid Parallel MCP search response")
	}
	response := &WebSearchResponse{Query: query, Provider: "parallel", Results: []WebSearchResult{}}
	for _, item := range *search.Results {
		u, err := url.Parse(item.URL)
		if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
			continue
		}
		response.Results = append(response.Results, WebSearchResult{Title: item.Title, URL: item.URL, Description: strings.Join(item.Excerpts, "\n\n"), Source: "parallel"})
		if len(response.Results) >= maxResults {
			break
		}
	}
	response.TotalCount = len(response.Results)
	return response, nil
}

// request implements the finite Streamable HTTP exchange needed by Search MCP.
// One client belongs to one search; sessions are never shared across concurrent calls.
func (c *parallelSearchClient) request(ctx context.Context, id int, method string, params interface{}) (json.RawMessage, error) {
	envelope := map[string]interface{}{"jsonrpc": "2.0", "method": method}
	if id != 0 {
		envelope["id"] = id
	}
	if params != nil {
		envelope["params"] = params
	}
	body, err := json.Marshal(envelope)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("User-Agent", parallelSearchUserAgent)
	if c.session != "" {
		req.Header.Set("Mcp-Session-Id", c.session)
	}
	if method != "initialize" {
		req.Header.Set("MCP-Protocol-Version", parallelSearchProtocol)
	}
	if c.key != "" {
		req.Header.Set("Authorization", "Bearer "+c.key)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Parallel MCP %s: %w", method, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Do not retry free-tier quotas or echo remote bodies/credentials into tool output.
		return nil, &webSearchProviderError{StatusCode: resp.StatusCode, Message: fmt.Sprintf("Parallel MCP %s returned HTTP %d", method, resp.StatusCode)}
	}
	if method == "initialize" {
		c.session = resp.Header.Get("Mcp-Session-Id")
	}
	if id == 0 {
		return nil, nil
	}
	reader := io.LimitReader(resp.Body, parallelSearchMaxResponse+1)
	if strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return readParallelSearchEvent(reader, id)
	}
	raw, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	if len(raw) > parallelSearchMaxResponse {
		return nil, errors.New("Parallel MCP response exceeds limit")
	}
	return decodeParallelSearchRPC(raw, id)
}

func decodeParallelSearchRPC(raw []byte, id int) (json.RawMessage, error) {
	var response parallelRPCResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, fmt.Errorf("decode Parallel MCP response: %w", err)
	}
	if response.JSONRPC != "2.0" || response.ID != id {
		return nil, errors.New("unexpected Parallel MCP response identity")
	}
	if response.Error != nil {
		return nil, fmt.Errorf("Parallel MCP RPC error %d", response.Error.Code)
	}
	if len(response.Result) == 0 || string(response.Result) == "null" {
		return nil, errors.New("missing Parallel MCP result")
	}
	return response.Result, nil
}

func readParallelSearchEvent(reader io.Reader, id int) (json.RawMessage, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), parallelSearchMaxResponse)
	var data []string
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" && len(data) > 0 {
			raw := []byte(strings.Join(data, "\n"))
			var event struct {
				Method string
				ID     *int
			}
			if json.Unmarshal(raw, &event) == nil && event.ID == nil && event.Method != "" {
				data = nil
				continue
			}
			return decodeParallelSearchRPC(raw, id)
		}
		if strings.HasPrefix(line, "data:") {
			data = append(data, strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " "))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read Parallel MCP stream: %w", err)
	}
	if len(data) > 0 {
		return decodeParallelSearchRPC([]byte(strings.Join(data, "\n")), id)
	}
	return nil, errors.New("Parallel MCP stream ended without a result")
}
