package browser

import (
	"net/url"
	"strings"
)

const (
	// SitePresetBrowserCommon expands a browser-heavy site bundle covering the
	// common community, blog, video, travel, review, and shopping targets users
	// frequently ask Blue to handle.
	SitePresetBrowserCommon = "browser_common"
	// SitePresetRelayFocused keeps only the highest-value sites for relay:
	// login-heavy, anti-bot-prone, or strongly session-dependent destinations.
	SitePresetRelayFocused = "relay_focused"
)

var trustedSitePresetOrigins = map[string][]string{
	SitePresetBrowserCommon: {
		"https://github.com",
		"https://news.ycombinator.com",
		"https://www.reddit.com",
		"https://x.com",
		"https://twitter.com",
		"https://www.linkedin.com",
		"https://medium.com",
		"https://substack.com",
		"https://dev.to",
		"https://hashnode.com",
		"https://www.zhihu.com",
		"https://www.xiaohongshu.com",
		"https://juejin.cn",
		"https://www.v2ex.com",
		"https://weibo.com",
		"https://okjike.com",
		"https://36kr.com",
		"https://www.thepaper.cn",
		"https://www.reuters.com",
		"https://www.bloomberg.com",
		"https://www.youtube.com",
		"https://www.bilibili.com",
		"https://www.douyin.com",
		"https://www.tiktok.com",
		"https://www.taobao.com",
		"https://www.tmall.com",
		"https://www.jd.com",
		"https://mobile.yangkeduo.com",
		"https://www.amazon.com",
		"https://www.amazon.co.jp",
		"https://www.ebay.com",
		"https://www.walmart.com",
		"https://www.etsy.com",
		"https://www.aliexpress.com",
		"https://www.coupang.com",
		"https://www.meituan.com",
		"https://www.dianping.com",
		"https://www.ctrip.com",
		"https://www.trip.com",
		"https://www.qunar.com",
		"https://www.mafengwo.cn",
		"https://www.fliggy.com",
		"https://xueqiu.com",
		"https://finance.yahoo.com",
		"https://www.barchart.com",
		"https://weread.qq.com",
		"https://www.zhipin.com",
		"https://www.chaoxing.com",
		"https://book.douban.com",
	},
	SitePresetRelayFocused: {
		"https://github.com",
		"https://x.com",
		"https://twitter.com",
		"https://www.linkedin.com",
		"https://www.zhihu.com",
		"https://www.xiaohongshu.com",
		"https://weibo.com",
		"https://okjike.com",
		"https://www.douyin.com",
		"https://www.tiktok.com",
		"https://www.bilibili.com",
		"https://www.taobao.com",
		"https://www.tmall.com",
		"https://www.jd.com",
		"https://mobile.yangkeduo.com",
		"https://www.amazon.com",
		"https://www.meituan.com",
		"https://www.dianping.com",
		"https://www.ctrip.com",
		"https://www.trip.com",
		"https://xueqiu.com",
		"https://weread.qq.com",
		"https://www.zhipin.com",
		"https://www.chaoxing.com",
	},
}

var relayPreferredSitePresetPatterns = map[string][]string{
	SitePresetBrowserCommon: {
		"github.com",
		"news.ycombinator.com",
		"reddit.com",
		"x.com",
		"twitter.com",
		"linkedin.com",
		"medium.com",
		"substack.com",
		"dev.to",
		"hashnode.com",
		"zhihu.com",
		"xiaohongshu.com",
		"juejin.cn",
		"v2ex.com",
		"weibo.com",
		"okjike.com",
		"36kr.com",
		"thepaper.cn",
		"reuters.com",
		"bloomberg.com",
		"youtube.com",
		"youtu.be",
		"bilibili.com",
		"douyin.com",
		"tiktok.com",
		"taobao.com",
		"tmall.com",
		"jd.com",
		"yangkeduo.com",
		"amazon.com",
		"amazon.co.jp",
		"ebay.com",
		"walmart.com",
		"etsy.com",
		"aliexpress.com",
		"coupang.com",
		"meituan.com",
		"dianping.com",
		"ctrip.com",
		"trip.com",
		"qunar.com",
		"mafengwo.cn",
		"fliggy.com",
		"xueqiu.com",
		"finance.yahoo.com",
		"barchart.com",
		"weread.qq.com",
		"zhipin.com",
		"chaoxing.com",
		"douban.com",
	},
	SitePresetRelayFocused: {
		"github.com",
		"x.com",
		"twitter.com",
		"linkedin.com",
		"zhihu.com",
		"xiaohongshu.com",
		"weibo.com",
		"okjike.com",
		"douyin.com",
		"tiktok.com",
		"bilibili.com",
		"taobao.com",
		"tmall.com",
		"jd.com",
		"yangkeduo.com",
		"amazon.com",
		"meituan.com",
		"dianping.com",
		"ctrip.com",
		"trip.com",
		"xueqiu.com",
		"weread.qq.com",
		"zhipin.com",
		"chaoxing.com",
	},
}

// ExpandTrustedSitePresets resolves built-in trusted site preset names into exact origins.
func ExpandTrustedSitePresets(presets []string) []string {
	return expandPresetEntries(trustedSitePresetOrigins, presets)
}

// ExpandRelayPreferredSitePresets resolves built-in relay site preset names into
// host/origin match patterns.
func ExpandRelayPreferredSitePresets(presets []string) []string {
	return expandPresetEntries(relayPreferredSitePresetPatterns, presets)
}

func expandPresetEntries(catalog map[string][]string, presets []string) []string {
	if len(presets) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, preset := range presets {
		key := strings.ToLower(strings.TrimSpace(preset))
		for _, entry := range catalog[key] {
			trimmed := strings.TrimSpace(entry)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			out = append(out, trimmed)
		}
	}
	return out
}

// MatchSitePatternList reports whether the raw URL matches any configured host/origin pattern.
func MatchSitePatternList(rawURL string, patterns []string) bool {
	for _, pattern := range patterns {
		if MatchSitePattern(rawURL, pattern) {
			return true
		}
	}
	return false
}

// MatchSitePattern matches either an exact origin pattern (scheme://host[:port])
// or a host pattern such as "zhihu.com", which also matches subdomains.
func MatchSitePattern(rawURL, pattern string) bool {
	origin, host := parseSiteTarget(rawURL)
	if host == "" {
		return false
	}
	normalized := normalizeSitePattern(pattern)
	if normalized == "" {
		return false
	}
	if strings.Contains(normalized, "://") {
		return origin == normalized
	}
	return host == normalized || strings.HasSuffix(host, "."+normalized)
}

func normalizeSitePattern(raw string) string {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, "://") {
		origin, _ := parseSiteTarget(raw)
		return origin
	}
	raw = strings.TrimPrefix(raw, "*.")
	if raw == "" {
		return ""
	}
	if strings.ContainsAny(raw, "/?#") {
		parsed, err := url.Parse("https://" + raw)
		if err != nil {
			return ""
		}
		raw = strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	}
	raw = strings.Trim(raw, ".")
	return raw
}

func parseSiteTarget(raw string) (origin string, host string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", ""
	}
	scheme := strings.ToLower(strings.TrimSpace(parsed.Scheme))
	if scheme != "http" && scheme != "https" {
		return "", ""
	}
	host = strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "" {
		return "", ""
	}
	port := strings.TrimSpace(parsed.Port())
	origin = scheme + "://" + host
	switch {
	case port == "":
	case scheme == "http" && port == "80":
	case scheme == "https" && port == "443":
	default:
		origin += ":" + port
	}
	return origin, host
}
