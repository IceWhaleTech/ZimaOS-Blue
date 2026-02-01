package tts

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

// DownloadState represents the persisted download state.
type DownloadState struct {
	ModelType    string    `json:"model_type"`
	URL          string    `json:"url"`
	TotalBytes   int64     `json:"total_bytes"`
	CurrentBytes int64     `json:"current_bytes"`
	TempFile     string    `json:"temp_file"`
	StartedAt    time.Time `json:"started_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SherpaDownloadManager manages Sherpa model downloads with resume support.
type SherpaDownloadManager struct {
	ModelDir   string
	OnProgress func(progress SherpaDownloadProgress)

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
func (m *SherpaDownloadManager) stateFilePath() string {
	return filepath.Join(m.ModelDir, ".download_state.json")
}

// completeMarkerPath returns the path to the completion marker for a model.
func (m *SherpaDownloadManager) completeMarkerPath(modelType string) string {
	return filepath.Join(m.ModelDir, fmt.Sprintf(".%s.complete", modelType))
}

// tempFilePath returns the path to the temporary download file.
func (m *SherpaDownloadManager) tempFilePath(modelType string) string {
	return filepath.Join(m.ModelDir, fmt.Sprintf("%s.tar.bz2.part", modelType))
}

// saveState persists the current download state to disk.
func (m *SherpaDownloadManager) saveState(state *DownloadState) error {
	state.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.stateFilePath(), data, 0644)
}

// loadState loads the download state from disk.
func (m *SherpaDownloadManager) loadState() (*DownloadState, error) {
	data, err := os.ReadFile(m.stateFilePath())
	if err != nil {
		return nil, err
	}
	var state DownloadState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// clearState removes the download state file.
func (m *SherpaDownloadManager) clearState() {
	os.Remove(m.stateFilePath())
}

// IsModelComplete checks if a model has been fully downloaded by checking actual files.
func (m *SherpaDownloadManager) IsModelComplete(modelType string) bool {
	// Get model directory
	modelDirs := map[string]string{
		"piper-en":     "vits-piper-en_US-lessac-medium",
		"piper-en-hfc": "vits-piper-en_US-hfc_female-medium",
		"piper-de":     "vits-piper-de_DE-thorsten-medium",
		"piper-es":     "vits-piper-es_ES-davefx-medium",
	}

	dir, ok := modelDirs[modelType]
	if !ok {
		return false
	}

	modelPath := filepath.Join(m.ModelDir, dir)

	// Piper models have different naming: {voice}.onnx instead of model.onnx
	modelFiles := map[string]string{
		"piper-en":     "en_US-lessac-medium.onnx",
		"piper-en-hfc": "en_US-hfc_female-medium.onnx",
		"piper-de":     "de_DE-thorsten-medium.onnx",
		"piper-es":     "es_ES-davefx-medium.onnx",
	}
	onnxFile := "model.onnx"
	if f, ok := modelFiles[modelType]; ok {
		onnxFile = f
	}

	// Check if required files exist
	requiredFiles := []string{onnxFile, "tokens.txt"}
	for _, file := range requiredFiles {
		if _, err := os.Stat(filepath.Join(modelPath, file)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// markComplete creates a completion marker for a model.
func (m *SherpaDownloadManager) markComplete(modelType string) error {
	markerPath := m.completeMarkerPath(modelType)
	return os.WriteFile(markerPath, []byte(time.Now().Format(time.RFC3339)), 0644)
}

// Download downloads the model files with resume support.
func (m *SherpaDownloadManager) Download(ctx context.Context, modelType string) error {
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
	packageName, ok := sherpaModelPackages[modelType]
	if !ok {
		return fmt.Errorf("unknown model type: %s", modelType)
	}

	// Download URL
	url := fmt.Sprintf("%s/%s", SherpaModelBaseURL, packageName)
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
func (m *SherpaDownloadManager) downloadWithResume(ctx context.Context, url, tempFile string, resumeFrom int64, modelType string) error {
	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	// Add Range header for resume
	if resumeFrom > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Handle response status
	switch resp.StatusCode {
	case http.StatusOK:
		// Server doesn't support range, start from beginning
		resumeFrom = 0
		m.currentBytes = 0
		m.totalBytes = resp.ContentLength
	case http.StatusPartialContent:
		// Resume supported
		if m.totalBytes == 0 {
			// Parse Content-Range header to get total size
			// Format: bytes start-end/total
			m.totalBytes = resp.ContentLength + resumeFrom
		}
	default:
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Open file for writing (append if resuming)
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

	// Save initial state
	state := &DownloadState{
		ModelType:    modelType,
		URL:          url,
		TotalBytes:   m.totalBytes,
		CurrentBytes: resumeFrom,
		TempFile:     tempFile,
		StartedAt:    m.startTime,
	}
	m.saveState(state)

	// Create progress reader
	reader := &sherpaProgressReader{
		reader: resp.Body,
		onProgress: func(n int64) {
			m.updateProgress(n)
			// Periodically save state (every 1MB)
			if m.currentBytes%(1024*1024) < n {
				state.CurrentBytes = m.currentBytes
				m.saveState(state)
			}
		},
	}

	// Copy data to file
	_, err = io.Copy(file, reader)
	if err != nil {
		// Save state before returning error for resume
		state.CurrentBytes = m.currentBytes
		m.saveState(state)
		return fmt.Errorf("download interrupted: %w", err)
	}

	return nil
}

// extractTarBz2 extracts a tar.bz2 file.
func (m *SherpaDownloadManager) extractTarBz2(archivePath string) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Decompress bzip2
	bzReader := bzip2.NewReader(file)

	// Extract tar
	tarReader := tar.NewReader(bzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		// Construct target path
		target := filepath.Join(m.ModelDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			// Ensure parent directory exists
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %w", err)
			}

			// Create file
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
func (m *SherpaDownloadManager) updateProgress(n int64) {
	m.mu.Lock()
	m.currentBytes += n
	m.mu.Unlock()

	if m.OnProgress != nil {
		m.OnProgress(m.GetProgress())
	}
}

// GetProgress returns the current download progress.
func (m *SherpaDownloadManager) GetProgress() SherpaDownloadProgress {
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
		eta = formatSherpaDuration(time.Duration(remaining) * time.Second)
	}

	percentage := float64(0)
	if m.totalBytes > 0 {
		percentage = float64(m.currentBytes) / float64(m.totalBytes) * 100
	}

	return SherpaDownloadProgress{
		File:       m.currentFile,
		Downloaded: m.currentBytes,
		Total:      m.totalBytes,
		Percentage: percentage,
		Speed:      speed,
		SpeedHuman: formatSherpaBytes(int64(speed)) + "/s",
		ETA:        eta,
		StartedAt:  m.startTime,
	}
}

// GetSavedProgress returns the saved progress from disk (for page refresh).
func (m *SherpaDownloadManager) GetSavedProgress() *SherpaDownloadProgress {
	state, err := m.loadState()
	if err != nil {
		return nil
	}

	// Check if temp file still exists
	info, err := os.Stat(state.TempFile)
	if err != nil {
		return nil
	}

	percentage := float64(0)
	if state.TotalBytes > 0 {
		percentage = float64(info.Size()) / float64(state.TotalBytes) * 100
	}

	return &SherpaDownloadProgress{
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
func (m *SherpaDownloadManager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

// IsDownloading returns true if a download is in progress.
func (m *SherpaDownloadManager) IsDownloading() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.downloading
}

// HasPendingDownload checks if there's a resumable download.
func (m *SherpaDownloadManager) HasPendingDownload() bool {
	state, err := m.loadState()
	if err != nil {
		return false
	}
	_, err = os.Stat(state.TempFile)
	return err == nil
}

// GetPendingModelType returns the model type of pending download.
func (m *SherpaDownloadManager) GetPendingModelType() string {
	state, err := m.loadState()
	if err != nil {
		return ""
	}
	return state.ModelType
}

// sherpaProgressReader wraps an io.Reader to track progress.
type sherpaProgressReader struct {
	reader     io.Reader
	onProgress func(n int64)
}

func (r *sherpaProgressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.onProgress != nil {
		r.onProgress(int64(n))
	}
	return n, err
}

// formatSherpaBytes formats bytes to human readable string.
func formatSherpaBytes(bytes int64) string {
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

// formatSherpaDuration formats a duration to human readable string.
func formatSherpaDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// SherpaModelStatus represents the model status response.
type SherpaModelStatus struct {
	Ready          bool                    `json:"ready"`
	ModelDir       string                  `json:"model_dir"`
	ModelType      string                  `json:"model_type"`
	Downloading    bool                    `json:"downloading"`
	Progress       *SherpaDownloadProgress `json:"progress,omitempty"`
	HasPending     bool                    `json:"has_pending"`
	PendingModel   string                  `json:"pending_model,omitempty"`
	SavedProgress  *SherpaDownloadProgress `json:"saved_progress,omitempty"`
}

// GetModelStatus returns the current model status.
func (p *SherpaProvider) GetModelStatus() *SherpaModelStatus {
	// Map internal model type to external type
	externalType := p.modelType
	switch p.modelType {
	case "kokoro":
		externalType = "kokoro-en"
	case "piper":
		externalType = "piper-en"
	}

	status := &SherpaModelStatus{
		Ready:       p.IsModelReady(),
		ModelDir:    p.modelDir,
		ModelType:   externalType,
		Downloading: p.downloadMgr.IsDownloading(),
		HasPending:  p.downloadMgr.HasPendingDownload(),
	}

	if status.Downloading {
		progress := p.downloadMgr.GetProgress()
		status.Progress = &progress
	} else if status.HasPending {
		status.PendingModel = p.downloadMgr.GetPendingModelType()
		status.SavedProgress = p.downloadMgr.GetSavedProgress()
	}

	return status
}

// DownloadModel downloads the model files.
func (p *SherpaProvider) DownloadModel(ctx context.Context) error {
	err := p.downloadMgr.Download(ctx, p.modelType)
	if err != nil {
		return err
	}

	// Update model ready status
	p.mu.Lock()
	p.modelReady = p.checkModelReady()
	p.mu.Unlock()

	// Pre-initialize TTS engine after download completes
	if p.modelReady {
		go func() {
			if err := p.initTTS(); err != nil {
				fmt.Printf("Failed to initialize TTS after download: %v\n", err)
			} else {
				fmt.Println("TTS engine initialized after download")
			}
		}()
	}

	return nil
}

// SetOnProgress sets the progress callback for downloads.
func (p *SherpaProvider) SetOnProgress(callback func(progress SherpaDownloadProgress)) {
	p.downloadMgr.OnProgress = callback
}
