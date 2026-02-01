package skillstore

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	// MaxReadmeSize is the maximum size of README content to store (50KB)
	MaxReadmeSize = 50 * 1024
	// ReadmeFetchTimeout is the timeout for fetching README content
	ReadmeFetchTimeout = 10 * time.Second
	// MaxConcurrentFetches is the maximum number of concurrent README fetches
	MaxConcurrentFetches = 5
)

// ReadmeFetcher fetches and processes README content for skills
type ReadmeFetcher struct {
	store      *Store
	httpClient *http.Client
	logger     *slog.Logger
	queue      chan *Skill
	wg         sync.WaitGroup
	stopCh     chan struct{}
}

// NewReadmeFetcher creates a new README fetcher
func NewReadmeFetcher(store *Store, logger *slog.Logger) *ReadmeFetcher {
	return &ReadmeFetcher{
		store: store,
		httpClient: &http.Client{
			Timeout: ReadmeFetchTimeout,
		},
		logger: logger,
		queue:  make(chan *Skill, 100),
		stopCh: make(chan struct{}),
	}
}

// Start starts the background README fetcher workers
func (f *ReadmeFetcher) Start(ctx context.Context) {
	for i := 0; i < MaxConcurrentFetches; i++ {
		f.wg.Add(1)
		go f.worker(ctx)
	}
}

// Stop stops the README fetcher
func (f *ReadmeFetcher) Stop() {
	close(f.stopCh)
	f.wg.Wait()
}

// Enqueue adds a skill to the README fetch queue
func (f *ReadmeFetcher) Enqueue(skill *Skill) {
	select {
	case f.queue <- skill:
	default:
		// Queue is full, skip
		if f.logger != nil {
			f.logger.Debug("readme fetch queue full, skipping", "skill", skill.ID)
		}
	}
}

// EnqueueBatch adds multiple skills to the README fetch queue
func (f *ReadmeFetcher) EnqueueBatch(skills []*Skill) {
	for _, skill := range skills {
		// Only enqueue skills without README content
		if skill.Readme == "" && skill.Homepage != "" {
			f.Enqueue(skill)
		}
	}
}

// worker processes skills from the queue
func (f *ReadmeFetcher) worker(ctx context.Context) {
	defer f.wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case <-f.stopCh:
			return
		case skill := <-f.queue:
			if skill == nil {
				continue
			}
			f.fetchAndStore(ctx, skill)
		}
	}
}

// fetchAndStore fetches README content and stores it
func (f *ReadmeFetcher) fetchAndStore(ctx context.Context, skill *Skill) {
	readme, err := f.FetchReadme(ctx, skill)
	if err != nil {
		if f.logger != nil {
			f.logger.Debug("failed to fetch readme", "skill", skill.ID, "error", err)
		}
		return
	}

	if readme == "" {
		return
	}

	// Update skill in database
	skill.Readme = readme
	if err := f.store.UpdateReadme(ctx, skill.ID, readme); err != nil {
		if f.logger != nil {
			f.logger.Error("failed to update readme", "skill", skill.ID, "error", err)
		}
	}
}

// FetchReadme fetches README content for a skill
func (f *ReadmeFetcher) FetchReadme(ctx context.Context, skill *Skill) (string, error) {
	// Try source URL first (usually raw GitHub URL)
	if skill.DownloadURL != "" {
		readme, err := f.fetchURL(ctx, skill.DownloadURL)
		if err == nil && readme != "" {
			return readme, nil
		}
	}

	// Try homepage
	if skill.Homepage != "" {
		readme, err := f.fetchURL(ctx, skill.Homepage)
		if err == nil && readme != "" {
			return readme, nil
		}
	}

	return "", nil
}

// fetchURL fetches content from a URL
func (f *ReadmeFetcher) fetchURL(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", "ZimaOS-Echo/1.0")
	req.Header.Set("Accept", "text/plain, text/markdown, text/html, */*")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", nil
	}

	// Read limited content
	limitedReader := io.LimitReader(resp.Body, MaxReadmeSize+1024)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", err
	}

	content := string(body)

	// Detect content type and process
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		content = HTMLToText(content)
	}

	// Truncate if needed
	content = TruncateContent(content, MaxReadmeSize)

	return content, nil
}

// HTMLToText converts HTML content to plain text
func HTMLToText(html string) string {
	// Remove script and style tags with content
	scriptRegex := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRegex.ReplaceAllString(html, "")

	styleRegex := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRegex.ReplaceAllString(html, "")

	// Remove HTML comments
	commentRegex := regexp.MustCompile(`(?s)<!--.*?-->`)
	html = commentRegex.ReplaceAllString(html, "")

	// Replace common block elements with newlines
	blockTags := []string{"div", "p", "br", "li", "tr", "h1", "h2", "h3", "h4", "h5", "h6", "section", "article"}
	for _, tag := range blockTags {
		openRegex := regexp.MustCompile(`(?i)<` + tag + `[^>]*>`)
		html = openRegex.ReplaceAllString(html, "\n")
		closeRegex := regexp.MustCompile(`(?i)</` + tag + `>`)
		html = closeRegex.ReplaceAllString(html, "\n")
	}

	// Remove all remaining HTML tags
	tagRegex := regexp.MustCompile(`<[^>]+>`)
	html = tagRegex.ReplaceAllString(html, "")

	// Decode common HTML entities
	html = decodeHTMLEntities(html)

	// Clean up whitespace
	html = cleanWhitespace(html)

	return strings.TrimSpace(html)
}

// decodeHTMLEntities decodes common HTML entities
func decodeHTMLEntities(s string) string {
	entities := map[string]string{
		"&nbsp;":  " ",
		"&amp;":   "&",
		"&lt;":    "<",
		"&gt;":    ">",
		"&quot;":  "\"",
		"&apos;":  "'",
		"&#39;":   "'",
		"&mdash;": "—",
		"&ndash;": "–",
		"&copy;":  "©",
		"&reg;":   "®",
		"&trade;": "™",
	}

	for entity, char := range entities {
		s = strings.ReplaceAll(s, entity, char)
	}

	// Decode numeric entities
	numericRegex := regexp.MustCompile(`&#(\d+);`)
	s = numericRegex.ReplaceAllStringFunc(s, func(match string) string {
		if n, err := parseNumericEntity(match); err == nil {
			if n > 0 && n < 128 {
				return string(rune(n))
			}
		}
		return match
	})

	return s
}

// parseNumericEntity parses a numeric HTML entity
func parseNumericEntity(s string) (int, error) {
	re := regexp.MustCompile(`&#(\d+);`)
	matches := re.FindStringSubmatch(s)
	if len(matches) < 2 {
		return 0, nil
	}
	var num int
	for _, c := range matches[1] {
		num = num*10 + int(c-'0')
	}
	return num, nil
}

// cleanWhitespace cleans up excessive whitespace
func cleanWhitespace(s string) string {
	// Replace multiple spaces with single space
	spaceRegex := regexp.MustCompile(`[ \t]+`)
	s = spaceRegex.ReplaceAllString(s, " ")

	// Replace multiple newlines with double newline
	newlineRegex := regexp.MustCompile(`\n{3,}`)
	s = newlineRegex.ReplaceAllString(s, "\n\n")

	// Trim spaces from each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = strings.Join(lines, "\n")

	return s
}

// TruncateContent truncates content to maxSize bytes
func TruncateContent(content string, maxSize int) string {
	if len(content) <= maxSize {
		return content
	}

	// Truncate at word boundary
	truncated := content[:maxSize]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxSize/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}
