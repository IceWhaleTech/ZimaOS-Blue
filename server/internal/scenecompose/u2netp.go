package scenecompose

import (
	"context"
	"errors"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
)

const (
	u2netpModelID         = "u2netp"
	defaultIdleTimeout    = 90 * time.Second
	defaultPollIntervalMS = 1500
)

type U2NetPModelManager struct {
	modelDir   string
	dataDir    string
	statusURL  string
	downloader *downloader.ModelDownloader

	mu              sync.Mutex
	session         *onnx.DynamicSession
	inputName       string
	outputName      string
	inputWidth      int
	inputHeight     int
	idleTimeout     time.Duration
	idleTimer       *time.Timer
	lastError       string
	onDownloadStart func(context.Context)
}

func NewU2NetPModelManager(modelDir, statusURL string) *U2NetPModelManager {
	manager := &U2NetPModelManager{
		modelDir:    modelDir,
		dataDir:     filepath.Dir(modelDir),
		statusURL:   strings.TrimSpace(statusURL),
		downloader:  downloader.NewModelDownloader(modelDir),
		idleTimeout: defaultIdleTimeout,
	}
	return manager
}

func (m *U2NetPModelManager) ModelID() string {
	return u2netpModelID
}

func (m *U2NetPModelManager) IsReady() bool {
	if m == nil {
		return false
	}
	_, err := os.Stat(m.modelPath())
	return err == nil
}

func (m *U2NetPModelManager) modelPath() string {
	return filepath.Join(m.modelDir, "u2netp.onnx")
}

func (m *U2NetPModelManager) GetStatus() *ModelStatus {
	if m == nil {
		return &ModelStatus{ModelID: u2netpModelID, Status: "error", State: "error", Error: "model manager unavailable"}
	}
	files := []ModelFileStatus{
		{
			Filename:   "u2netp.onnx",
			Downloaded: fileExists(m.modelPath()),
			Size:       "~4.6 MB",
		},
	}
	if onnx.RuntimeTgzFilename() != "" {
		files = append(files, ModelFileStatus{
			Filename:   onnx.RuntimeTgzFilename(),
			Downloaded: onnx.RuntimeLibPath(m.dataDir) != "",
			Size:       "~30 MB",
		})
	}

	_, errText := m.downloader.GetState()
	downloading := m.downloader.IsDownloading()
	ready := m.IsReady()
	progress := m.downloader.GetProgress()

	m.mu.Lock()
	lastError := strings.TrimSpace(m.lastError)
	m.mu.Unlock()

	status := "not_downloaded"
	switch {
	case ready:
		status = "ready"
	case downloading:
		status = "downloading"
	case strings.TrimSpace(errText) != "" || lastError != "":
		status = "error"
	}
	errorText := firstNonEmpty(strings.TrimSpace(errText), lastError)

	var mapped *ModelDownloadProgress
	if progress != nil {
		mapped = &ModelDownloadProgress{
			File:       progress.File,
			FileIndex:  progress.FileIndex,
			TotalFiles: progress.TotalFiles,
			Downloaded: progress.Downloaded,
			Total:      progress.Total,
			Percentage: progress.Percentage,
			SpeedHuman: progress.SpeedHuman,
			ETA:        progress.ETA,
		}
	}
	return &ModelStatus{
		ModelID:     u2netpModelID,
		Status:      status,
		Ready:       ready,
		Downloading: downloading,
		State:       status,
		Error:       errorText,
		Progress:    mapped,
		Files:       files,
	}
}

func (m *U2NetPModelManager) EnsureReadyAsync(ctx context.Context) {
	if m == nil {
		return
	}
	if m.onDownloadStart != nil {
		m.onDownloadStart(ctx)
	}
	if m.IsReady() || m.downloader.IsDownloading() {
		return
	}
	go func() {
		if err := m.downloader.Download(context.Background(), m.downloadFiles()); err != nil && !errors.Is(err, context.Canceled) {
			m.mu.Lock()
			m.lastError = err.Error()
			m.mu.Unlock()
		}
	}()
}

func (m *U2NetPModelManager) downloadFiles() []downloader.ModelFile {
	files := make([]downloader.ModelFile, 0, 2)
	if onnx.RuntimeLibPath(m.dataDir) == "" {
		tgz := onnx.RuntimeTgzFilename()
		url := onnx.RuntimeTgzURL()
		if tgz != "" && url != "" {
			dataDir := m.dataDir
			files = append(files, downloader.ModelFile{
				Filename: tgz,
				URL:      url,
				Mirrors:  onnx.RuntimeTgzMirrors(),
				Size:     "~30 MB",
				PostProcess: func(path string) error {
					libPath, err := onnx.ExtractRuntimeFromTgz(path, dataDir)
					if err != nil {
						return err
					}
					onnx.SetLibraryPath(libPath)
					onnx.SetDataDir(dataDir)
					return nil
				},
			})
		}
	}
	files = append(files, downloader.ModelFile{
		Filename: "u2netp.onnx",
		URL:      "https://huggingface.co/coilabs/u2netp-q/resolve/main/model.onnx",
		Mirrors: []string{
			"https://hf-mirror.com/coilabs/u2netp-q/resolve/main/model.onnx",
		},
		Size: "~4.6 MB",
	})
	return files
}

func (m *U2NetPModelManager) Cutout(ctx context.Context, src image.Image) (*CutoutResult, error) {
	if m == nil {
		return nil, errors.New("u2netp manager unavailable")
	}
	if !m.IsReady() {
		m.EnsureReadyAsync(ctx)
		return nil, errors.New("u2netp model not ready")
	}
	result, err := m.inferMask(ctx, src)
	if err != nil {
		m.mu.Lock()
		m.lastError = err.Error()
		m.mu.Unlock()
		return nil, err
	}
	if result != nil {
		m.touchActivity()
	}
	return result, nil
}

func (m *U2NetPModelManager) touchActivity() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idleTimeout <= 0 {
		m.idleTimeout = defaultIdleTimeout
	}
	if m.idleTimer != nil {
		m.idleTimer.Reset(m.idleTimeout)
		return
	}
	m.idleTimer = time.AfterFunc(m.idleTimeout, m.unloadSession)
}

func (m *U2NetPModelManager) unloadSession() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session != nil {
		_ = m.session.Close()
		m.session = nil
	}
	m.inputName = ""
	m.outputName = ""
	m.inputWidth = 0
	m.inputHeight = 0
}

func (m *U2NetPModelManager) SetDownloadStartHook(fn func(context.Context)) {
	if m == nil {
		return
	}
	m.onDownloadStart = fn
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
