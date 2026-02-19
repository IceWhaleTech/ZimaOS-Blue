package pruner

import (
	"context"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
)

// PrunerModelInfo describes a downloadable pruner model file.
type PrunerModelInfo struct {
	Filename string   `json:"filename"`
	URL      string   `json:"url"`
	Mirrors  []string `json:"-"` // fallback URLs (hf-mirror, modelscope, etc.)
	Size     string   `json:"size"`
}

// prunerModelFiles lists all files needed for the ONNX pruner.
var prunerModelFiles = []PrunerModelInfo{
	{Filename: "model.onnx", URL: "https://huggingface.co/orca-zhang/code-pruner-onnx/resolve/main/model.onnx",
		Mirrors: []string{
			"https://hf-mirror.com/orca-zhang/code-pruner-onnx/resolve/main/model.onnx",
			"https://modelscope.cn/models/orcazhang/code-pruner-onnx/resolve/master/model.onnx",
		}, Size: "607 MB"},
	{Filename: "vocab.json", URL: "https://huggingface.co/orca-zhang/code-pruner-onnx/resolve/main/vocab.json",
		Mirrors: []string{
			"https://hf-mirror.com/orca-zhang/code-pruner-onnx/resolve/main/vocab.json",
			"https://modelscope.cn/models/orcazhang/code-pruner-onnx/resolve/master/vocab.json",
		}, Size: "2.6 MB"},
	{Filename: "merges.txt", URL: "https://huggingface.co/orca-zhang/code-pruner-onnx/resolve/main/merges.txt",
		Mirrors: []string{
			"https://hf-mirror.com/orca-zhang/code-pruner-onnx/resolve/main/merges.txt",
			"https://modelscope.cn/models/orcazhang/code-pruner-onnx/resolve/master/merges.txt",
		}, Size: "1.6 MB"},
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

// PrunerModelManager manages pruner model downloads via the unified downloader.
type PrunerModelManager struct {
	modelDir   string
	dataPath   string
	downloader *downloader.ModelDownloader
}

// NewPrunerModelManager creates a new model manager.
func NewPrunerModelManager(modelDir string) *PrunerModelManager {
	return &PrunerModelManager{
		modelDir:   modelDir,
		dataPath:   filepath.Dir(modelDir),
		downloader: downloader.NewModelDownloader(modelDir),
	}
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

	state, lastError := m.downloader.GetState()
	downloading := m.downloader.IsDownloading()

	var progress *ModelDownloadProgress
	if p := m.downloader.GetProgress(); p != nil {
		progress = &ModelDownloadProgress{
			File:       p.File,
			FileIndex:  p.FileIndex,
			TotalFiles: p.TotalFiles,
			Downloaded: p.Downloaded,
			Total:      p.Total,
			Percentage: p.Percentage,
			SpeedHuman: p.SpeedHuman,
			ETA:        p.ETA,
		}
	}

	return PrunerModelStatus{
		Ready:       allReady,
		Downloading: downloading,
		State:       state,
		Error:       lastError,
		Progress:    progress,
		Files:       files,
	}
}

// toDownloaderFiles converts pruner model files to unified downloader format.
func toDownloaderFiles() []downloader.ModelFile {
	files := make([]downloader.ModelFile, len(prunerModelFiles))
	for i, f := range prunerModelFiles {
		files[i] = downloader.ModelFile{
			Filename: f.Filename,
			URL:      f.URL,
			Mirrors:  f.Mirrors,
			Size:     f.Size,
		}
	}
	return files
}

// Download starts downloading all model files via the unified downloader.
func (m *PrunerModelManager) Download(ctx context.Context) error {
	var files []downloader.ModelFile

	// ONNX Runtime tgz first (if not already extracted)
	if onnx.RuntimeLibPath(m.dataPath) == "" {
		tgzFilename := onnx.RuntimeTgzFilename()
		tgzURL := onnx.RuntimeTgzURL()
		if tgzFilename != "" && tgzURL != "" {
			dataPath := m.dataPath
			files = append(files, downloader.ModelFile{
				Filename: tgzFilename,
				URL:      tgzURL,
				Mirrors:  onnx.RuntimeTgzMirrors(),
				Size:     "~30MB",
				PostProcess: func(tgzPath string) error {
					libPath, err := onnx.ExtractRuntimeFromTgz(tgzPath, dataPath)
					if err != nil {
						return err
					}
					onnx.SetLibraryPath(libPath)
					onnx.SetDataDir(dataPath)
					return nil
				},
			})
		}
	}

	files = append(files, toDownloaderFiles()...)
	return m.downloader.Download(ctx, files)
}

// CancelDownload cancels the current download.
func (m *PrunerModelManager) CancelDownload() {
	_ = m.downloader.Cancel()
}
