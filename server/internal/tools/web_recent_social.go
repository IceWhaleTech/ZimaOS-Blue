package tools

import (
	"context"
	"fmt"
	neturl "net/url"
	"strings"
)

func (t *WebTool) collectRecentSocialItems(ctx context.Context, args map[string]interface{}, query, source string, sites []string, maxResults int) ([]webRecentNormalizedItem, bool, error) {
	items := make([]webRecentNormalizedItem, 0, maxResults)
	browserAssisted := false
	errs := make([]error, 0, len(sites))
	for _, site := range sites {
		siteItems, assisted, err := t.collectRecentSiteSearchItems(ctx, args, query, site, source, maxResults, true)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", site, err))
		}
		browserAssisted = browserAssisted || assisted
		items = append(items, siteItems...)
	}
	return webRecentSortDedupLimit(items, maxResults), browserAssisted, webRecentJoinErrors(errs...)
}

func webRecentHighCouplingURL(raw string) bool {
	target, err := neturl.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(target.Hostname()))
	switch {
	case host == "x.com", host == "twitter.com", host == "www.twitter.com", host == "www.x.com":
		return true
	case host == "tiktok.com", host == "www.tiktok.com", strings.HasSuffix(host, ".tiktok.com"):
		return true
	case host == "instagram.com", host == "www.instagram.com", strings.HasSuffix(host, ".instagram.com"):
		return true
	case host == "bsky.app", host == "www.bsky.app", strings.HasSuffix(host, ".bsky.app"):
		return true
	default:
		return false
	}
}
