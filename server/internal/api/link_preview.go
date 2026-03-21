package api

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/net/html"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/network"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// LinkPreviewHandler handles link preview API requests.
type LinkPreviewHandler struct {
	client *http.Client
}

// LinkPreviewResponse represents the link preview metadata.
type LinkPreviewResponse struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	Favicon     string `json:"favicon,omitempty"`
	SiteName    string `json:"siteName,omitempty"`
	URL         string `json:"url"`
}

// NewLinkPreviewHandler creates a new link preview handler.
func NewLinkPreviewHandler() *LinkPreviewHandler {
	return &LinkPreviewHandler{
		client: newLinkPreviewHTTPClient(),
	}
}

func newLinkPreviewHTTPClient() *http.Client {
	transport := network.NewPooledTransport(false)
	baseDial := transport.DialContext
	if baseDial == nil {
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		baseDial = dialer.DialContext
	}

	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host := address
		if h, _, err := net.SplitHostPort(address); err == nil && h != "" {
			host = h
		}
		if err := tools.GuardOutboundHost(ctx, host, false); err != nil {
			return nil, err
		}
		return baseDial(ctx, network, address)
	}

	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return http.ErrUseLastResponse
		}
		return tools.GuardOutboundHost(req.Context(), req.URL.Hostname(), false)
	}

	return client
}

// RegisterRoutes registers link preview routes.
func (h *LinkPreviewHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/link-preview", h.GetLinkPreview)
}

// GetLinkPreview fetches and returns metadata for a given URL.
// @Summary Get link preview
// @Description Fetches Open Graph and meta tags from a URL to generate a preview
// @Tags utils
// @Produce json
// @Param url query string true "URL to fetch preview for"
// @Success 200 {object} LinkPreviewResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/link-preview [get]
func (h *LinkPreviewHandler) GetLinkPreview(c echo.Context) error {
	targetURL := c.QueryParam("url")
	if targetURL == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "url parameter is required",
		})
	}

	// Validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "invalid URL",
		})
	}
	if err := tools.GuardOutboundURL(c.Request().Context(), parsedURL.String(), false); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{
			"error": "URL target is not allowed",
		})
	}

	// Fetch the page
	ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", targetURL, nil)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to create request",
		})
	}

	// Set a browser-like User-Agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; LinkPreview/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := h.client.Do(req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "failed to fetch URL",
		})
	}
	defer resp.Body.Close()

	// Limit response size to 1MB
	limitedReader := io.LimitReader(resp.Body, 1024*1024)

	// Parse HTML
	preview := h.parseHTML(limitedReader, parsedURL)
	preview.URL = targetURL

	return c.JSON(http.StatusOK, preview)
}

// parseHTML extracts Open Graph and meta tags from HTML content.
func (h *LinkPreviewHandler) parseHTML(r io.Reader, baseURL *url.URL) *LinkPreviewResponse {
	preview := &LinkPreviewResponse{}

	doc, err := html.Parse(r)
	if err != nil {
		return preview
	}

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.Data {
			case "title":
				if preview.Title == "" && n.FirstChild != nil {
					preview.Title = strings.TrimSpace(n.FirstChild.Data)
				}
			case "meta":
				h.parseMeta(n, preview)
			case "link":
				h.parseLink(n, preview, baseURL)
			}
		}

		// Stop parsing after head section for efficiency
		if n.Type == html.ElementNode && n.Data == "body" {
			return
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}

	traverse(doc)

	// Resolve relative URLs
	h.resolveURLs(preview, baseURL)

	// Generate favicon URL if not found
	if preview.Favicon == "" {
		preview.Favicon = baseURL.Scheme + "://" + baseURL.Host + "/favicon.ico"
	}

	return preview
}

// parseMeta extracts metadata from a meta tag.
func (h *LinkPreviewHandler) parseMeta(n *html.Node, preview *LinkPreviewResponse) {
	var property, name, content string

	for _, attr := range n.Attr {
		switch attr.Key {
		case "property":
			property = attr.Val
		case "name":
			name = attr.Val
		case "content":
			content = attr.Val
		}
	}

	// Open Graph tags
	switch property {
	case "og:title":
		if preview.Title == "" || strings.HasPrefix(property, "og:") {
			preview.Title = content
		}
	case "og:description":
		if preview.Description == "" || strings.HasPrefix(property, "og:") {
			preview.Description = content
		}
	case "og:image":
		if preview.Image == "" {
			preview.Image = content
		}
	case "og:site_name":
		preview.SiteName = content
	}

	// Twitter cards
	switch name {
	case "twitter:title":
		if preview.Title == "" {
			preview.Title = content
		}
	case "twitter:description":
		if preview.Description == "" {
			preview.Description = content
		}
	case "twitter:image":
		if preview.Image == "" {
			preview.Image = content
		}
	case "description":
		if preview.Description == "" {
			preview.Description = content
		}
	}
}

// parseLink extracts favicon from link tags.
func (h *LinkPreviewHandler) parseLink(n *html.Node, preview *LinkPreviewResponse, baseURL *url.URL) {
	var rel, href string

	for _, attr := range n.Attr {
		switch attr.Key {
		case "rel":
			rel = attr.Val
		case "href":
			href = attr.Val
		}
	}

	// Check for favicon
	if preview.Favicon == "" {
		relLower := strings.ToLower(rel)
		if strings.Contains(relLower, "icon") {
			preview.Favicon = href
		}
	}
}

// resolveURLs converts relative URLs to absolute URLs.
func (h *LinkPreviewHandler) resolveURLs(preview *LinkPreviewResponse, baseURL *url.URL) {
	if preview.Image != "" && !isAbsoluteURL(preview.Image) {
		preview.Image = resolveURL(baseURL, preview.Image)
	}
	if preview.Favicon != "" && !isAbsoluteURL(preview.Favicon) {
		preview.Favicon = resolveURL(baseURL, preview.Favicon)
	}
}

// isAbsoluteURL checks if a URL is absolute.
func isAbsoluteURL(u string) bool {
	return strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "//")
}

// resolveURL resolves a relative URL against a base URL.
func resolveURL(base *url.URL, ref string) string {
	if strings.HasPrefix(ref, "//") {
		return base.Scheme + ":" + ref
	}

	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}

	resolved := base.ResolveReference(refURL)
	return resolved.String()
}

// truncateString truncates a string to a maximum length.
func truncateString(s string, maxLen int) string {
	// Remove extra whitespace
	space := regexp.MustCompile(`\s+`)
	s = space.ReplaceAllString(strings.TrimSpace(s), " ")

	if len(s) <= maxLen {
		return s
	}

	// Find last space before maxLen
	lastSpace := strings.LastIndex(s[:maxLen], " ")
	if lastSpace > 0 {
		return s[:lastSpace] + "..."
	}
	return s[:maxLen] + "..."
}
