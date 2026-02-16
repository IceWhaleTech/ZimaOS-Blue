package pruner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const hfMirrorHost = "hf-mirror.com"

// PrunerModelInfo describes a downloadable pruner model file.
type PrunerModelInfo struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
	Size     string `json:"size"`
}

// Download states
const (
	StateIdle        = ""
	StateConnecting  = "connecting"
	StateDownloading = "downloading"
	StateError       = "error"
)

// prunerModelFiles lists all files needed for the ONNX pruner.
var prunerModelFiles = []PrunerModelInfo{
	{Filename: "model.onnx", URL: "https://huggingface.co/ayanami-kitasan/code-pruner/resolve/main/model.onnx", Size: "1.4 GB"},
	{Filename: "vocab.json", URL: "https://huggingface.co/ayanami-kitasan/code-pruner/resolve/main/vocab.json", Size: "2.8 MB"},
	{Filename: "merges.txt", URL: "https://huggingface.co/ayanami-kitasan/code-pruner/resolve/main/merges.txt", Size: "1.7 MB"},
}

// ModelDownloadProgress tracks download progress.
type ModelDownloadProgress struct {
	File       string  `json:"file"`
	FileIndex  int     `json:"file_index"`
	TotalFiles int     `json:"total_files"`
	Downloaded int64   `json:"downloaded"`
	Total      int64   `json:"total"`
	Percentage float64 `json:"percentage"`
	SpeedHuman string  `json:"speed_human"`
	ETA        string  `json:"eta"`
}

// PrunerModelStatus represents the model state.
type PrunerModelStatus struct {
	Ready       bool                   `json:"ready"`
	Downloading bool                   `json:"downloading"`
	State       string                 `json:"state,omitempty"`
	Error       string                 `json:"error,omitempty"`
	Progress    *ModelDownloadProgress `json:"progress,omitempty"`
	Files       []PrunerFileStatus     `json:"files,omitempty"`
}

// PrunerFileStatus shows per-file download status.
type PrunerFileStatus struct {
	Filename   string `json:"filename"`
	Downloaded bool   `json:"downloaded"`
	Size       string `json:"size"`
}

// PrunerModelManager manages pruner model downloads.
type PrunerModelManager struct {
	modelDir    string
	downloading bool
	state       string
	lastError   string
	progress    *ModelDownloadProgress
	cancelFunc  context.CancelFunc
	mu          sync.Mutex
}

// NewPrunerModelManager creates a new model manager.
func NewPrunerModelManager(modelDir string) *PrunerModelManager {
	return &PrunerModelManager{modelDir: modelDir}
}

// ModelDir returns the model directory path.
func (m *PrunerModelManager) ModelDir() string { return m.modelDir }

// IsReady returns true if all model files are downloaded.
func (m *PrunerModelManager) IsReady() bool {
	for _, f := range prunerModelFiles {
		path := filepath.Join(m.modelDir, f.Filename)
		if _, err := os.Stat(path); err != nil {
			return false
		}
	}
	return true
}

// GetStatus returns the current model status.
func (m *PrunerModelManager) GetStatus() PrunerModelStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	files := make([]PrunerFileStatus, len(prunerModelFiles))
	allReady := true
	for i, f := range prunerModelFiles {
		path := filepath.Join(m.modelDir, f.Filename)
		_, err := os.Stat(path)
		downloaded := err == nil
		if !downloaded {
			allReady = false
		}
		files[i] = PrunerFileStatus{
			Filename:   f.Filename,
			Downloaded: downloaded,
			Size:       f.Size,
		}
	}

	return PrunerModelStatus{
		Ready:       allReady,
		Downloading: m.downloading,
		State:       m.state,
		Error:       m.lastError,
		Progress:    m.progress,
		Files:       files,
	}
}

// Download starts downloading all model files in sequence.
func (m *PrunerModelManager) Download(ctx context.Context) error {
	m.mu.Lock()
	if m.downloading {
		m.mu.Unlock()
		return fmt.Errorf("download already in progress")
	}
	m.downloading = true
	m.state = StateConnecting
	m.lastError = ""
	m.progress = &ModelDownloadProgress{TotalFiles: len(prunerModelFiles)}
	ctx, m.cancelFunc = context.WithCancel(ctx)
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		if m.state != StateError {
			m.state = StateIdle
		}
		m.cancelFunc = nil
		m.mu.Unlock()
	}()

	if err := os.MkdirAll(m.modelDir, 0755); err != nil {
		m.mu.Lock()
		m.state = StateError
		m.lastError = err.Error()
		m.mu.Unlock()
		return err
	}

	for i, f := range prunerModelFiles {
		destPath := filepath.Join(m.modelDir, f.Filename)
		// Skip already downloaded files
		if _, err := os.Stat(destPath); err == nil {
			continue
		}

		m.mu.Lock()
		m.state = StateConnecting
		m.progress.File = f.Filename
		m.progress.FileIndex = i
		m.progress.Downloaded = 0
		m.progress.Total = 0
		m.progress.Percentage = 0
		m.progress.SpeedHuman = ""
		m.progress.ETA = ""
		m.mu.Unlock()

		if err := m.downloadFileWithMirror(ctx, f.URL, destPath); err != nil {
			m.mu.Lock()
			m.state = StateError
			m.lastError = fmt.Sprintf("%s: %v", f.Filename, err)
			m.mu.Unlock()
			return fmt.Errorf("download %s: %w", f.Filename, err)
		}
	}
	return nil
}

func (m *PrunerModelManager) downloadFileWithMirror(ctx context.Context, url, destPath string) error {
	err := m.downloadFile(ctx, url, destPath)
	if err == nil {
		return nil
	}
	// If original HuggingFace URL failed, try hf-mirror
	mirrorURL := strings.Replace(url, "huggingface.co", hfMirrorHost, 1)
	if mirrorURL == url {
		return err
	}
	return m.downloadFile(ctx, mirrorURL, destPath)
}

func (m *PrunerModelManager) downloadFile(ctx context.Context, url, destPath string) error {
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
	m.mu.Lock()
	m.state = StateDownloading
	m.mu.Unlock()

	out, err := os.Create(destPath + ".tmp")
	if err != nil {
		return err
	}
	defer out.Close()

	m.mu.Lock()
	m.progress.Total = resp.ContentLength
	m.mu.Unlock()

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
			out.Write(buf[:n])
			downloaded += int64(n)

			m.mu.Lock()
			m.progress.Downloaded = downloaded
			if m.progress.Total > 0 {
				m.progress.Percentage = float64(downloaded) / float64(m.progress.Total) * 100
			}
			elapsed := time.Since(startTime).Seconds()
			if elapsed > 0 {
				speed := float64(downloaded) / elapsed
				m.progress.SpeedHuman = formatDownloadSpeed(speed)
				if speed > 0 && m.progress.Total > 0 {
					remaining := float64(m.progress.Total-downloaded) / speed
					m.progress.ETA = formatDownloadDuration(remaining)
				}
			}
			m.mu.Unlock()
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			os.Remove(destPath + ".tmp")
			return err
		}
	}

	return os.Rename(destPath+".tmp", destPath)
}

// CancelDownload cancels the current download.
func (m *PrunerModelManager) CancelDownload() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

func formatDownloadSpeed(bytesPerSec float64) string {
	if bytesPerSec >= 1024*1024 {
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/1024/1024)
	}
	return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
}

func formatDownloadDuration(seconds float64) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", int(seconds))
	}
	return fmt.Sprintf("%dm%ds", int(seconds)/60, int(seconds)%60)
}
