package stt

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SherpaASRDownloadState represents the persisted download state.
type SherpaASRDownloadState struct {
	ModelType    string    `json:"model_type"`
	URL          string    `json:"url"`
	TotalBytes   int64     `json:"total_bytes"`
	CurrentBytes int64     `json:"current_bytes"`
	TempFile     string    `json:"temp_file"`
	StartedAt    time.Time `json:"started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SherpaASRDownloadProgress represents the current download progress.
type SherpaASRDownloadProgress struct {
	File       string    `json:"file"`
	Downloaded int64     `json:"downloaded"`
	Total      int64     `json:"total"`
	Percentage float64   `json:"percentage"`
	Speed      float64   `json:"speed"`
	SpeedHuman string    `json:"speed_human"`
	ETA        string    `json:"eta"`
	StartedAt  time.Time `json:"started_at"`
}

// SherpaASRDownloadManager manages Sherpa ASR model downloads with resume support.
type SherpaASRDownloadManager struct {
	ModelDir   string
	OnProgress func(progress SherpaASRDownloadProgress)

	mu           sync.Mutex
	downloading  bool
	cancelFunc   context.CancelFunc
	currentBytes int64
	totalBytes   int64
	currentFile  string
	startTime    time.Time
	modelType    string
}

// stateFilePath returns the path to the download state file.
func (m *SherpaASRDownloadManager) stateFilePath() string {
	return filepath.Join(m.ModelDir, ".asr_download_state.json")
}

// completeMarkerPath returns the path to the completion marker for a model.
func (m *SherpaASRDownloadManager) completeMarkerPath(modelType string) string {
	return filepath.Join(m.ModelDir, fmt.Sprintf(".%s.complete", modelType))
}

// tempFilePath returns the path to the temporary download file.
func (m *SherpaASRDownloadManager) tempFilePath(modelType string) string {
	return filepath.Join(m.ModelDir, fmt.Sprintf("%s.tar.bz2.part", modelType))
}

// saveState persists the current download state to disk.
func (m *SherpaASRDownloadManager) saveState(state *SherpaASRDownloadState) error {
	state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.stateFilePath(), data, 0644)
}

// loadState loads the download state from disk.
func (m *SherpaASRDownloadManager) loadState() (*SherpaASRDownloadState, error) {
	data, err := os.ReadFile(m.stateFilePath())
	if err != nil {
		return nil, err
	}
	var state SherpaASRDownloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// clearState removes the download state file.
func (m *SherpaASRDownloadManager) clearState() {
	os.Remove(m.stateFilePath())
}

// IsModelComplete checks if a model has been fully downloaded.
func (m *SherpaASRDownloadManager) IsModelComplete(modelType string) bool {
	markerPath := m.completeMarkerPath(modelType)
	_, err := os.Stat(markerPath)
	return err == nil
}

// markComplete creates a completion marker for a model.
func (m *SherpaASRDownloadManager) markComplete(modelType string) error {
	markerPath := m.completeMarkerPath(modelType)
	return os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0644)
}

// Download downloads the model files with resume support.
func (m *SherpaASRDownloadManager) Download(ctx context.Context, modelType string) error {
	m.mu.Lock()
	if m.downloading {
		m.mu.Unlock()
		return fmt.Errorf("download already in progress")
	}
	m.downloading = true
	ctx, m.cancelFunc = context.WithCancel(ctx)
	m.startTime = time.Now()
	m.modelType = modelType
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.downloading = false
		m.cancelFunc = nil
		m.mu.Unlock()
	}()

	// Ensure model directory exists
	if err := os.MkdirAll(m.ModelDir, 0755); err != nil {
		return fmt.Errorf("failed to create model directory: %w", err)
	}

	// Check if already complete
	if m.IsModelComplete(modelType) {
		return nil
	}

	// Get package name for model type
	packageName, ok := sherpaASRModelPackages[modelType]
	if !ok {
		return fmt.Errorf("unknown model type: %s", modelType)
	}

	// Download URL
	url := fmt.Sprintf("%s/%s", SherpaASRModelBaseURL, packageName)
	m.currentFile = packageName

	// Try to resume or start fresh
	tempFile := m.tempFilePath(modelType)
	var resumeFrom int64 = 0

	// Check for existing partial download
	if state, err := m.loadState(); err == nil && state.ModelType == modelType {
		if info, err := os.Stat(state.TempFile); err == nil {
			resumeFrom = info.Size()
			m.currentBytes = resumeFrom
			m.totalBytes = state.TotalBytes
			m.startTime = state.StartedAt
		}
	}

	// Download with resume support
	if err := m.downloadWithResume(ctx, url, tempFile, resumeFrom, modelType); err != nil {
		return err
	}

	// Extract the downloaded file
	if err := m.extractTarBz2(tempFile); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	// Cleanup
	os.Remove(tempFile)
	m.clearState()

	// Mark as complete
	if err := m.markComplete(modelType); err != nil {
		return fmt.Errorf("failed to mark complete: %w", err)
	}

	return nil
}

// downloadWithResume downloads a file with HTTP Range support for resuming.
func (m *SherpaASRDownloadManager) downloadWithResume(ctx context.Context, url, tempFile string, resumeFrom int64, modelType string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if resumeFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		resumeFrom = 0
		m.currentBytes = 0
		m.totalBytes = resp.ContentLength
	case http.StatusPartialContent:
		if m.totalBytes == 0 {
			m.totalBytes = resp.ContentLength + resumeFrom
		}
	default:
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	var file *os.File
	if resumeFrom > 0 {
		file, err = os.OpenFile(tempFile, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		file, err = os.Create(tempFile)
	}
	if err != nil {
		return fmt.Errorf("failed to open temp file: %w", err)
	}
	defer file.Close()

	state := &SherpaASRDownloadState{
		ModelType:    modelType,
		URL:          url,
		TotalBytes:   m.totalBytes,
		CurrentBytes: resumeFrom,
		TempFile:     tempFile,
		StartedAt:    m.startTime,
	}
	m.saveState(state)

	reader := &asrProgressReader{
		reader: resp.Body,
		onProgress: func(n int64) {
			m.updateProgress(n)
			if m.currentBytes%(1024*1024) < n {
				state.CurrentBytes = m.currentBytes
				m.saveState(state)
			}
		},
	}

	_, err = io.Copy(file, reader)
	if err != nil {
		state.CurrentBytes = m.currentBytes
		m.saveState(state)
		return fmt.Errorf("download interrupted: %w", err)
	}

	return nil
}

// extractTarBz2 extracts a tar.bz2 file.
func (m *SherpaASRDownloadManager) extractTarBz2(archivePath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	bzReader := bzip2.NewReader(file)
	tarReader := tar.NewReader(bzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		target := filepath.Join(m.ModelDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			outFile, err := os.Create(target)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()
		}
	}

	return nil
}

// updateProgress updates the current progress.
func (m *SherpaASRDownloadManager) updateProgress(n int64) {
	m.mu.Lock()
	m.currentBytes += n
	m.mu.Unlock()

	if m.OnProgress != nil {
		m.OnProgress(m.GetProgress())
	}
}

// GetProgress returns the current download progress.
func (m *SherpaASRDownloadManager) GetProgress() SherpaASRDownloadProgress {
	m.mu.Lock()
	defer m.mu.Unlock()

	elapsed := time.Since(m.startTime).Seconds()
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(m.currentBytes) / elapsed
	}

	eta := ""
	if speed > 0 && m.totalBytes > m.currentBytes {
		remaining := float64(m.totalBytes-m.currentBytes) / speed
		eta = formatASRDuration(time.Duration(remaining) * time.Second)
	}

	percentage := float64(0)
	if m.totalBytes > 0 {
		percentage = float64(m.currentBytes) / float64(m.totalBytes) * 100
	}

	return SherpaASRDownloadProgress{
		File:       m.currentFile,
		Downloaded: m.currentBytes,
		Total:      m.totalBytes,
		Percentage: percentage,
		Speed:      speed,
		SpeedHuman: formatASRBytes(int64(speed)) + "/s",
		ETA:        eta,
		StartedAt:  m.startTime,
	}
}

// GetSavedProgress returns the saved progress from disk.
func (m *SherpaASRDownloadManager) GetSavedProgress() *SherpaASRDownloadProgress {
	state, err := m.loadState()
	if err != nil {
		return nil
	}

	info, err := os.Stat(state.TempFile)
	if err != nil {
		return nil
	}

	percentage := float64(0)
	if state.TotalBytes > 0 {
		percentage = float64(info.Size()) / float64(state.TotalBytes) * 100
	}

	return &SherpaASRDownloadProgress{
		File:       filepath.Base(state.TempFile),
		Downloaded: info.Size(),
		Total:      state.TotalBytes,
		Percentage: percentage,
		Speed:      0,
		SpeedHuman: "paused",
		ETA:        "resumable",
		StartedAt:  state.StartedAt,
	}
}

// Cancel cancels the current download.
func (m *SherpaASRDownloadManager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

// IsDownloading returns true if a download is in progress.
func (m *SherpaASRDownloadManager) IsDownloading() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.downloading
}

// HasPendingDownload checks if there's a resumable download.
func (m *SherpaASRDownloadManager) HasPendingDownload() bool {
	state, err := m.loadState()
	if err != nil {
		return false
	}
	_, err = os.Stat(state.TempFile)
	return err == nil
}

// GetPendingModelType returns the model type of pending download.
func (m *SherpaASRDownloadManager) GetPendingModelType() string {
	state, err := m.loadState()
	if err != nil {
		return ""
	}
	return state.ModelType
}

// DeleteModel deletes a downloaded model.
func (m *SherpaASRDownloadManager) DeleteModel(modelType string) error {
	// Remove completion marker
	os.Remove(m.completeMarkerPath(modelType))

	// Remove model directory
	modelDir := m.getModelDir(modelType)
	return os.RemoveAll(modelDir)
}

// getModelDir returns the model directory for a specific model type.
func (m *SherpaASRDownloadManager) getModelDir(modelType string) string {
	switch modelType {
	case "whisper-tiny":
		return filepath.Join(m.ModelDir, "sherpa-onnx-whisper-tiny")
	case "whisper-base":
		return filepath.Join(m.ModelDir, "sherpa-onnx-whisper-base")
	case "zipformer-en":
		return filepath.Join(m.ModelDir, "sherpa-onnx-streaming-zipformer-en-2023-06-26")
	case "paraformer-zh":
		return filepath.Join(m.ModelDir, "sherpa-onnx-paraformer-zh-2023-09-14")
	case "sensevoice-small":
		return filepath.Join(m.ModelDir, "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17")
	default:
		return filepath.Join(m.ModelDir, modelType)
	}
}

// asrProgressReader wraps an io.Reader to track progress.
type asrProgressReader struct {
	reader     io.Reader
	onProgress func(n int64)
}

func (r *asrProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.onProgress != nil {
		r.onProgress(int64(n))
	}
	return n, err
}

// formatASRBytes formats bytes to human readable string.
func formatASRBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// formatASRDuration formats a duration to human readable string.
func formatASRDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
