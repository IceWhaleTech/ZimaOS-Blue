//go:build kokoro && linux && cgo

package tts

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
)

// kokoroRequiredFiles is the full list of files that must be present for Kokoro to be ready.
var kokoroRequiredFiles = []string{
	"model_quantized.onnx",
	"voices/af_heart.bin",
	"voices/bf_emma.bin",
	"voices/jf_alpha.bin",
	"voices/zf_xiaobei.bin",
	"voices/ef_dora.bin",
	"voices/ff_siwis.bin",
	"voices/hf_alpha.bin",
	"voices/if_sara.bin",
	"voices/pf_dora.bin",
}

// KokoroModelManager manages Kokoro ONNX model downloads.
// Thread safety is provided by the underlying ModelDownloader;
// all fields here (modelPath, dataPath, downloader) are immutable after construction.
type KokoroModelManager struct {
	downloader *downloader.ModelDownloader
	modelPath  string
	dataPath   string
}

// NewKokoroModelManager creates a new Kokoro model manager
func NewKokoroModelManager(dataPath string) *KokoroModelManager {
	destDir := filepath.Join(dataPath, "models", "kokoro")
	return &KokoroModelManager{
		downloader: downloader.NewModelDownloader(destDir),
		modelPath:  filepath.Join(destDir, "model_quantized.onnx"),
		dataPath:   dataPath,
	}
}

// GetModelStatus returns the current model status.
// Thread-safe: reads immutable fields and calls thread-safe downloader methods.
func (m *KokoroModelManager) GetModelStatus() map[string]interface{} {
	ready := true
	destDir := filepath.Dir(m.modelPath)
	for _, f := range kokoroRequiredFiles {
		if _, err := os.Stat(filepath.Join(destDir, f)); err != nil {
			ready = false
			break
		}
	}

	state, errMsg := m.downloader.GetState()
	return map[string]interface{}{
		"ready":       ready,
		"path":        m.modelPath,
		"state":       state,
		"error":       errMsg,
		"progress":    m.downloader.GetProgress(),
		"downloading": m.downloader.IsDownloading(),
	}
}

// DownloadModel starts downloading the Kokoro model.
// The actual download is handled by ModelDownloader which is internally thread-safe.
func (m *KokoroModelManager) DownloadModel(ctx context.Context) error {
	// Create destination directory
	destDir := filepath.Dir(m.modelPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create kokoro directory: %w", err)
	}

	// Build file list — ONNX Runtime tgz first (if not already extracted)
	var files []downloader.ModelFile

	if onnx.RuntimeLibPath(m.dataPath) == "" {
		tgzFilename := onnx.RuntimeTgzFilename()
		tgzURL := onnx.RuntimeTgzURL()
		if tgzFilename != "" && tgzURL != "" {
			dataPath := m.dataPath // capture for closure
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

	const repo = "onnx-community/Kokoro-82M-v1.1-zh-ONNX"

	// Model file
	files = append(files,
		downloader.ModelFile{
			Filename: "model_quantized.onnx",
			URL:      "https://huggingface.co/" + repo + "/resolve/main/onnx/model_quantized.onnx",
			Mirrors: []string{
				"https://hf-mirror.com/" + repo + "/resolve/main/onnx/model_quantized.onnx",
				"https://modelscope.cn/models/" + repo + "/resolve/main/onnx/model_quantized.onnx",
			},
			Size: "~127MB",
		},
	)

	// Voice files — one per supported language (all female)
	for _, f := range kokoroRequiredFiles[1:] { // skip model_quantized.onnx
		files = append(files, downloader.ModelFile{
			Filename: f,
			URL:      "https://huggingface.co/" + repo + "/resolve/main/" + f,
			Mirrors: []string{
				"https://hf-mirror.com/" + repo + "/resolve/main/" + f,
				"https://modelscope.cn/models/" + repo + "/resolve/main/" + f,
			},
			Size: "~524KB",
		})
	}

	return m.downloader.Download(ctx, files)
}

// CancelDownload cancels the current download.
func (m *KokoroModelManager) CancelDownload() {
	m.downloader.Cancel()
}

// IsReady checks if all Kokoro model files are present.
func (m *KokoroModelManager) IsReady() bool {
	destDir := filepath.Dir(m.modelPath)
	for _, f := range kokoroRequiredFiles {
		if _, err := os.Stat(filepath.Join(destDir, f)); err != nil {
			return false
		}
	}
	return true
}
