package downloader

import (
	"context"
	"fmt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ModelFile describes a downloadable model file with fallback mirrors.
type ModelFile struct {
	Filename    string                  `json:"filename"`
	URL         string                  `json:"url"`     // Primary URL (usually HuggingFace)
	Mirrors     []string                `json:"mirrors"` // Fallback URLs (prefer modelscope, then hf-mirror)
	Size        string                  `json:"size"`    // Human-readable size
	PostProcess func(path string) error `json:"-"`       // Optional post-download hook (e.g. extract tgz)
}

// DownloadProgress tracks download progress for a single file.
type DownloadProgress struct {
	File            string  `json:"file"`
	FileIndex       int     `json:"file_index"`
	TotalFiles      int     `json:"total_files"`
	Downloaded      int64   `json:"downloaded"`
	Total           int64   `json:"total"`
	Percentage      float64 `json:"percentage"`
	SpeedHuman      string  `json:"speed_human"`
	DownloadedHuman string  `json:"downloaded_human"`
	ETA             string  `json:"eta"`
}

// Download states
const (
	StateIdle        = ""
	StateConnecting  = "connecting"
	StateDownloading = "downloading"
	StateError       = "error"
)

// ModelDownloader manages model file downloads with high availability fallback.
// It tries ModelScope → HuggingFace (hf.co) → HF-Mirror in sequence.
type ModelDownloader struct {
	destDir     string
	downloading bool
	state       string
	lastError   string
	progress    *DownloadProgress
	cancelFunc  context.CancelFunc
	mu          sync.Mutex
}

// NewModelDownloader creates a new model downloader.
func NewModelDownloader(destDir string) *ModelDownloader {
	return &ModelDownloader{destDir: destDir}
}

// IsDownloading returns true if a download is in progress.
func (d *ModelDownloader) IsDownloading() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.downloading
}

// GetProgress returns the current download progress.
func (d *ModelDownloader) GetProgress() *DownloadProgress {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.progress == nil {
		return nil
	}
	// Return a copy to avoid race conditions
	p := *d.progress
	return &p
}

// GetState returns the current download state.
func (d *ModelDownloader) GetState() (state string, err string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.state, d.lastError
}

// Download downloads a list of model files sequentially.
// It tries ModelScope first (fastest in China), then HuggingFace (hf.co), then HF-Mirror.
func (d *ModelDownloader) Download(ctx context.Context, files []ModelFile) error {
	d.mu.Lock()
	if d.downloading {
		d.mu.Unlock()
		return fmt.Errorf("download already in progress")
	}
	d.downloading = true
	d.state = StateConnecting
	d.lastError = ""
	d.progress = &DownloadProgress{TotalFiles: len(files)}
	ctx, d.cancelFunc = context.WithCancel(ctx)
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.downloading = false
		if d.state != StateError {
			d.state = StateIdle
		}
		d.cancelFunc = nil
		d.mu.Unlock()
	}()

	if err := os.MkdirAll(d.destDir, 0755); err != nil {
		d.mu.Lock()
		d.state = StateError
		d.lastError = err.Error()
		d.mu.Unlock()
		return err
	}

	for i, f := range files {
		destPath := fmt.Sprintf("%s/%s", d.destDir, f.Filename)
		// Skip already downloaded files
		if _, err := os.Stat(destPath); err == nil {
			continue
		}

		d.mu.Lock()
		d.state = StateConnecting
		d.progress.File = f.Filename
		d.progress.FileIndex = i
		d.progress.Downloaded = 0
		d.progress.Total = 0
		d.progress.Percentage = 0
		d.progress.SpeedHuman = ""
		d.progress.ETA = ""
		d.mu.Unlock()

		// Try ModelScope first (fastest in China), then primary, then mirrors.
		urls := buildURLList(f)
		log.Printf("[ModelDownloader] Downloading %s, trying %d URLs in order:", f.Filename, len(urls))
		for idx, u := range urls {
			log.Printf("[ModelDownloader]   %d. %s", idx+1, u)
		}
		if err := d.downloadWithFallback(ctx, urls, destPath); err != nil {
			// Context canceled = user-initiated cancel, not an error
			if ctx.Err() == context.Canceled {
				d.mu.Lock()
				d.state = StateIdle
				d.lastError = ""
				d.mu.Unlock()
				return nil
			}
			d.mu.Lock()
			d.state = StateError
			d.lastError = fmt.Sprintf("%s: %v", f.Filename, err)
			d.mu.Unlock()
			return fmt.Errorf("download %s: %w", f.Filename, err)
		}

		// Run post-process hook if defined (e.g. extract tgz)
		if f.PostProcess != nil {
			if err := f.PostProcess(destPath); err != nil {
				d.mu.Lock()
				d.state = StateError
				d.lastError = fmt.Sprintf("%s post-process: %v", f.Filename, err)
				d.mu.Unlock()
				return fmt.Errorf("post-process %s: %w", f.Filename, err)
			}
		}
	}
	return nil
}

// buildURLList builds the URL list with ModelScope first for high availability.
// Priority: ModelScope (fastest in China) -> HuggingFace (hf.co) -> HF-Mirror.
func buildURLList(f ModelFile) []string {
	urls := make([]string, 0, len(f.Mirrors)+1)

	// 1. Try ModelScope first (fastest in China)
	for _, mirror := range f.Mirrors {
		if contains(mirror, "modelscope.cn") {
			urls = append(urls, mirror)
		}
	}

	// 2. Try HuggingFace primary (hf.co)
	urls = append(urls, f.URL)

	// 3. Try HF-Mirror (backup for China)
	for _, mirror := range f.Mirrors {
		if contains(mirror, "hf-mirror.com") {
			urls = append(urls, mirror)
		}
	}

	// 4. Add any remaining mirrors
	for _, mirror := range f.Mirrors {
		if !contains(mirror, "modelscope.cn") && !contains(mirror, "hf-mirror.com") {
			urls = append(urls, mirror)
		}
	}

	return urls
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func (d *ModelDownloader) downloadWithFallback(ctx context.Context, urls []string, destPath string) error {
	var lastErr error
	for idx, u := range urls {
		log.Printf("[ModelDownloader] Attempt %d/%d: trying %s", idx+1, len(urls), u)
		if err := d.downloadFile(ctx, u, destPath); err == nil {
			log.Printf("[ModelDownloader] ✓ Successfully downloaded from %s", u)
			return nil
		} else {
			log.Printf("[ModelDownloader] ✗ Failed to download from %s: %v", u, err)
			lastErr = err
		}
	}
	log.Printf("[ModelDownloader] All %d URLs failed, last error: %v", len(urls), lastErr)
	return lastErr
}

func (d *ModelDownloader) downloadFile(ctx context.Context, url, destPath string) error {
	log.Printf("[ModelDownloader] Creating HTTP request for %s", url)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("[ModelDownloader] Failed to create request: %v", err)
		return err
	}

	log.Printf("[ModelDownloader] Sending HTTP request...")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("[ModelDownloader] HTTP request failed: %v", err)
		return err
	}
	defer resp.Body.Close()

	log.Printf("[ModelDownloader] HTTP response: %s (Content-Length: %d)", resp.Status, resp.ContentLength)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	// Transition from connecting to downloading
	d.mu.Lock()
	d.state = StateDownloading
	d.mu.Unlock()

	// Ensure parent directory exists for nested files (e.g. voices/af_heart.bin)
	if dir := filepath.Dir(destPath); dir != d.destDir {
		log.Printf("[ModelDownloader] Creating subdirectory: %s", dir)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("[ModelDownloader] Failed to create subdirectory: %v", err)
			return fmt.Errorf("create subdirectory: %w", err)
		}
	}

	log.Printf("[ModelDownloader] Creating temporary file: %s.tmp", destPath)
	out, err := os.Create(destPath + ".tmp")
	if err != nil {
		log.Printf("[ModelDownloader] Failed to create temp file: %v", err)
		return err
	}
	defer out.Close()

	d.mu.Lock()
	d.progress.Total = resp.ContentLength
	d.mu.Unlock()

	startTime := timeutil.NowTime()
	buf := make([]byte, 32*1024)
	var downloaded int64

	for {
		select {
		case <-ctx.Done():
			os.Remove(destPath + ".tmp")
			return ctx.Err()
		default:
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				os.Remove(destPath + ".tmp")
				return writeErr
			}
			downloaded += int64(n)

			// Update progress
			d.mu.Lock()
			d.progress.Downloaded = downloaded
			if d.progress.Total > 0 {
				d.progress.Percentage = float64(downloaded) / float64(d.progress.Total) * 100
			} else {
				// Content-Length unknown: show downloaded bytes, percentage stays 0
				d.progress.DownloadedHuman = formatSize(downloaded)
			}

			// Calculate speed and ETA
			elapsed := timeutil.SinceTime(startTime).Seconds()
			if elapsed > 0 {
				speed := float64(downloaded) / elapsed
				d.progress.SpeedHuman = formatSpeed(speed)
				if speed > 0 && d.progress.Total > 0 {
					remaining := float64(d.progress.Total-downloaded) / speed
					d.progress.ETA = formatDuration(time.Duration(remaining) * time.Second)
				}
			}
			d.mu.Unlock()
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("[ModelDownloader] Read error: %v", err)
			os.Remove(destPath + ".tmp")
			return err
		}
	}

	out.Close()
	log.Printf("[ModelDownloader] Download complete, renaming %s.tmp to %s", destPath, destPath)
	if err := os.Rename(destPath+".tmp", destPath); err != nil {
		log.Printf("[ModelDownloader] Failed to rename file: %v", err)
		return err
	}
	log.Printf("[ModelDownloader] File saved successfully: %s", destPath)
	return nil
}

// Cancel cancels the current download.
func (d *ModelDownloader) Cancel() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.downloading || d.cancelFunc == nil {
		return fmt.Errorf("no download in progress")
	}

	d.cancelFunc()
	return nil
}

// formatSpeed formats bytes/sec to human-readable string.
func formatSpeed(bytesPerSec float64) string {
	if bytesPerSec < 1024 {
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	} else if bytesPerSec < 1024*1024 {
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	} else {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/1024/1024)
	}
}

// formatSize formats bytes to human-readable string.
func formatSize(bytes int64) string {
	if bytes < 1024 {
		return fmt.Sprintf("%d B", bytes)
	} else if bytes < 1024*1024 {
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	} else {
		return fmt.Sprintf("%.1f MB", float64(bytes)/1024/1024)
	}
}

// formatDuration formats duration to human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	} else {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}
