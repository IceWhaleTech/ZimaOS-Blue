package downloader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// ModelFile describes a downloadable model file with fallback mirrors.
type ModelFile struct {
	Filename string   `json:"filename"`
	URL      string   `json:"url"`      // Primary URL (usually HuggingFace)
	Mirrors  []string `json:"mirrors"`  // Fallback URLs (hf-mirror, modelscope)
	Size     string   `json:"size"`     // Human-readable size
}

// DownloadProgress tracks download progress for a single file.
type DownloadProgress struct {
	File       string  `json:"file"`
	FileIndex  int     `json:"file_index"`
	TotalFiles int     `json:"total_files"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
	SpeedHuman string  `json:"speed_human"`
	ETA        string  `json:"eta"`
}

// Download states
const (
	StateIdle        = ""
	StateConnecting  = "connecting"
	StateDownloading = "downloading"
	StateError       = "error"
)

// ModelDownloader manages model file downloads with high availability fallback.
// It tries ModelScope → HF-Mirror → HuggingFace in sequence.
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
// It tries ModelScope first (fastest in China), then HF-Mirror, then HuggingFace.
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

		// Try ModelScope first (fastest in China), then mirrors, then primary
		urls := buildURLList(f)
		if err := d.downloadWithFallback(ctx, urls, destPath); err != nil {
			d.mu.Lock()
			d.state = StateError
			d.lastError = fmt.Sprintf("%s: %v", f.Filename, err)
			d.mu.Unlock()
			return fmt.Errorf("download %s: %w", f.Filename, err)
		}
	}
	return nil
}

// buildURLList builds the URL list with ModelScope first for high availability.
// Priority: ModelScope (fastest in China) → HF-Mirror → HuggingFace
func buildURLList(f ModelFile) []string {
	urls := make([]string, 0, len(f.Mirrors)+1)

	// 1. Try ModelScope first (fastest in China)
	for _, mirror := range f.Mirrors {
		if contains(mirror, "modelscope.cn") {
			urls = append(urls, mirror)
		}
	}

	// 2. Try HF-Mirror (backup for China)
	for _, mirror := range f.Mirrors {
		if contains(mirror, "hf-mirror.com") {
			urls = append(urls, mirror)
		}
	}

	// 3. Try HuggingFace (original, may be slow in China)
	urls = append(urls, f.URL)

	// 4. Add any remaining mirrors
	for _, mirror := range f.Mirrors {
		if !contains(mirror, "modelscope.cn") && !contains(mirror, "hf-mirror.com") {
			urls = append(urls, mirror)
		}
	}

	return urls
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		 findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (d *ModelDownloader) downloadWithFallback(ctx context.Context, urls []string, destPath string) error {
	var lastErr error
	for _, u := range urls {
		if err := d.downloadFile(ctx, u, destPath); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

func (d *ModelDownloader) downloadFile(ctx context.Context, url, destPath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %s", resp.Status)
	}

	// Transition from connecting to downloading
	d.mu.Lock()
	d.state = StateDownloading
	d.mu.Unlock()

	out, err := os.Create(destPath + ".tmp")
	if err != nil {
		return err
	}
	defer out.Close()

	d.mu.Lock()
	d.progress.Total = resp.ContentLength
	d.mu.Unlock()

	startTime := time.Now()
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
			}

			// Calculate speed and ETA
			elapsed := time.Since(startTime).Seconds()
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
			os.Remove(destPath + ".tmp")
			return err
		}
	}

	out.Close()
	return os.Rename(destPath+".tmp", destPath)
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
