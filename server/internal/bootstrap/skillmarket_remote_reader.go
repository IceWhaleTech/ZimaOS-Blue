package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmarket"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type skillMarketWebReader struct {
	readTool  *tools.WebReadTool
	crawlTool *tools.WebCrawlTool
}

func newSkillMarketRemoteReader(registry *tools.Registry) skillmarket.RemoteReader {
	if registry == nil {
		return nil
	}
	readTool := tools.GetWebReadTool(registry)
	crawlTool := tools.GetWebCrawlTool(registry)
	if readTool == nil && crawlTool == nil {
		return nil
	}
	return &skillMarketWebReader{
		readTool:  readTool,
		crawlTool: crawlTool,
	}
}

func (r *skillMarketWebReader) ReadURL(ctx context.Context, req skillmarket.RemoteReadRequest) (*skillmarket.RemoteReadResult, error) {
	if r == nil || r.readTool == nil {
		return nil, fmt.Errorf("web_read runtime not available")
	}
	args := buildSkillMarketReadArgs(req)
	raw, err := r.readTool.Execute(ctx, args)
	if err != nil {
		return nil, err
	}
	payload, err := stringifyToolJSON(raw)
	if err != nil {
		return nil, err
	}
	var result skillmarket.RemoteReadResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func buildSkillMarketReadArgs(req skillmarket.RemoteReadRequest) map[string]interface{} {
	args := map[string]interface{}{
		"url":    req.URL,
		"format": "text",
	}
	if req.Format != "" {
		args["format"] = req.Format
	}
	if req.MaxChars > 0 {
		args["max_chars"] = req.MaxChars
	}
	if len(req.Headers) > 0 {
		args["headers"] = req.Headers
	}
	if req.WantRawHTML {
		// Catalog discovery expects HTML-capable reads and should never pay for
		// browser/proxy fallback when a direct HTTP fetch is sufficient.
		args["lane"] = "http"
		args["disable_internal_fallbacks"] = true
	}
	return args
}

func (r *skillMarketWebReader) CrawlSite(ctx context.Context, req skillmarket.RemoteCrawlRequest) (*skillmarket.RemoteCrawlResult, error) {
	if r == nil || r.crawlTool == nil {
		return nil, fmt.Errorf("web_crawl runtime not available")
	}
	args := map[string]interface{}{
		"seeds": req.Seeds,
	}
	if len(req.AllowedHosts) > 0 {
		args["allowed_hosts"] = req.AllowedHosts
	}
	if len(req.Headers) > 0 {
		args["headers"] = req.Headers
	}
	if req.MaxDepth > 0 {
		args["max_depth"] = req.MaxDepth
	}
	if req.MaxPages > 0 {
		args["max_pages"] = req.MaxPages
	}
	if req.MaxRetries > 0 {
		args["max_retries"] = req.MaxRetries
	}
	if req.PageMaxChars > 0 {
		args["page_max_chars"] = req.PageMaxChars
	}
	if req.Checkpoint != nil {
		args["checkpoint"] = req.Checkpoint
	}
	raw, err := r.crawlTool.Execute(ctx, args)
	if err != nil {
		return nil, err
	}
	payload, err := stringifyToolJSON(raw)
	if err != nil {
		return nil, err
	}
	var result skillmarket.RemoteCrawlResult
	if err := json.Unmarshal([]byte(payload), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func stringifyToolJSON(value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}
