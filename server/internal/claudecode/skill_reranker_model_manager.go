package claudecode

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/downloader"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/onnx"
)

const defaultSkillRerankerRepo = "cross-encoder/ms-marco-MiniLM-L-6-v2"

// SkillRerankerModelManager manages ONNX reranker model download and load validation.
type SkillRerankerModelManager struct {
	modelDir   string
	repo       string
	downloader *downloader.ModelDownloader
	mu         sync.Mutex
}

func NewSkillRerankerModelManager(dataDir, repo string) *SkillRerankerModelManager {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		repo = defaultSkillRerankerRepo
	}
	modelDir := filepath.Join(dataDir, "skill-reranker")
	return &SkillRerankerModelManager{
		modelDir:   modelDir,
		repo:       repo,
		downloader: downloader.NewModelDownloader(modelDir),
	}
}

func (m *SkillRerankerModelManager) ModelPath() string {
	return filepath.Join(m.modelDir, "model.onnx")
}

func (m *SkillRerankerModelManager) IsReady() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := os.Stat(m.ModelPath()); err != nil {
		return false
	}
	return m.validateModelLoad() == nil
}

func (m *SkillRerankerModelManager) EnsureReady(ctx context.Context, allowDownload bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureReadyInner(ctx, allowDownload)
}

func (m *SkillRerankerModelManager) ensureReadyInner(ctx context.Context, allowDownload bool) error {
	if _, err := os.Stat(m.ModelPath()); err == nil {
		if err := m.validateModelLoad(); err == nil {
			return nil
		}
		// Corrupted or incompatible local model; remove and re-download.
		_ = os.Remove(m.ModelPath())
	}
	if !allowDownload {
		return fmt.Errorf("skill reranker model is not ready and auto download is disabled")
	}

	if err := os.MkdirAll(m.modelDir, 0o755); err != nil {
		return fmt.Errorf("create skill reranker dir: %w", err)
	}

	files := []downloader.ModelFile{m.modelFile()}
	if err := m.downloader.Download(ctx, files); err != nil {
		return fmt.Errorf("download skill reranker model: %w", err)
	}
	if err := m.validateModelLoad(); err != nil {
		_ = os.Remove(m.ModelPath())
		return fmt.Errorf("load-check skill reranker model: %w", err)
	}
	return nil
}

func (m *SkillRerankerModelManager) modelFile() downloader.ModelFile {
	hfURL := fmt.Sprintf("https://huggingface.co/%s/resolve/main/model.onnx", m.repo)
	modelscopeURL := fmt.Sprintf("https://modelscope.cn/models/%s/resolve/master/model.onnx", m.repo)
	hfMirrorURL := fmt.Sprintf("https://hf-mirror.com/%s/resolve/main/model.onnx", m.repo)

	return downloader.ModelFile{
		Filename: "model.onnx",
		URL:      hfURL,
		Mirrors: []string{
			modelscopeURL,
			hfMirrorURL,
		},
		Size: "~90MB",
	}
}

// validateModelLoad checks whether downloaded model can be opened by ONNX Runtime.
// This is the acceptance criterion instead of checksum.
func (m *SkillRerankerModelManager) validateModelLoad() error {
	modelPath := m.ModelPath()
	if _, err := os.Stat(modelPath); err != nil {
		return err
	}

	dataPath := filepath.Dir(m.modelDir)
	onnx.SetDataDir(dataPath)
	if libPath := onnx.RuntimeLibPath(dataPath); libPath != "" {
		onnx.SetLibraryPath(libPath)
	}

	inputNameSets := [][]string{
		{"input_ids", "attention_mask", "token_type_ids"},
		{"input_ids", "attention_mask"},
		{"input_ids"},
	}
	outputNameSets := [][]string{
		{"logits"},
		{"output_0"},
		{"output"},
	}

	var lastErr error
	for _, in := range inputNameSets {
		for _, out := range outputNameSets {
			s, err := onnx.NewDynamicSession(modelPath, in, out)
			if err == nil {
				_ = s.Close()
				return nil
			}
			lastErr = err
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unknown model load error")
	}
	return lastErr
}

func (m *SkillRerankerModelManager) WarmupAsync(allowDownload bool) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		_ = m.EnsureReady(ctx, allowDownload)
	}()
}
