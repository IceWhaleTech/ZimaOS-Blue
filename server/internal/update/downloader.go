package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Downloader handles binary downloads
type Downloader struct {
	httpClient  *http.Client
	storagePath string
	progress    float64
	onProgress  func(float64)
}

// NewDownloader creates a new downloader
func NewDownloader(storagePath string) *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		storagePath: storagePath,
	}
}

// SetProgressCallback sets the progress callback
func (d *Downloader) SetProgressCallback(fn func(float64)) {
	d.onProgress = fn
}

// Download downloads a binary from URL and verifies checksum
func (d *Downloader) Download(ctx context.Context, url, expectedChecksum string) (string, error) {
	if err := os.MkdirAll(d.storagePath, 0755); err != nil {
		return "", fmt.Errorf("failed to create storage dir: %w", err)
	}

	tmpFile := filepath.Join(d.storagePath, "update.tmp")
	finalFile := filepath.Join(d.storagePath, "update.bin")

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(tmpFile)
	if err != nil {
		return "", err
	}
	defer out.Close()

	hasher := sha256.New()
	writer := io.MultiWriter(out, hasher)

	total := resp.ContentLength
	var downloaded int64
	buf := make([]byte, 32*1024)

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := writer.Write(buf[:n]); werr != nil {
				os.Remove(tmpFile)
				return "", werr
			}
			downloaded += int64(n)
			if total > 0 && d.onProgress != nil {
				d.progress = float64(downloaded) / float64(total) * 100
				d.onProgress(d.progress)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(tmpFile)
			return "", err
		}
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	if expectedChecksum != "" && checksum != expectedChecksum {
		os.Remove(tmpFile)
		return "", fmt.Errorf("checksum mismatch: expected %s, got %s", expectedChecksum, checksum)
	}

	if err := os.Rename(tmpFile, finalFile); err != nil {
		os.Remove(tmpFile)
		return "", err
	}

	return finalFile, nil
}

// Progress returns current download progress
func (d *Downloader) Progress() float64 {
	return d.progress
}
