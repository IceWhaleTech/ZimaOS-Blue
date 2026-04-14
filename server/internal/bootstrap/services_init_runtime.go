package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
)

func initServicesMemoryAndMedia(s *Services, cfg *ServerConfig, appCfg *config.Config, trace *StartupTrace) error {
	chatDBPath := filepath.Join(cfg.DataDir, "blue.db")
	opts := memory.DefaultChatStoreOptions(chatDBPath)
	opts.Durability = appCfg.Session.ChatDBDurability
	opts.CheckpointInterval = 0
	opts.AttachmentExternalStore = appCfg.Session.ChatAttachmentExternalStore
	if opts.AttachmentExternalStore {
		opts.AttachmentDir = memory.DefaultChatAttachmentDir(cfg.DataDir)
	}
	store, err := memory.NewStoreWithOptions(chatDBPath, opts)
	if err != nil {
		return fmt.Errorf("failed to initialize memory store: %w", err)
	}
	s.MemoryStore = store
	trace.Mark("memory_store_ready")
	s.A2UIManager = a2ui.NewManager(s.Logger)
	trace.Mark("a2ui_ready")
	s.OCRService = ocrruntime.NewTesseractService(s.Logger, ocrruntime.Config{
		ModelDir:     filepath.Join(cfg.DataDir, "models", "tesseract"),
		AutoDownload: true,
		WorkerCount:  1,
	})
	s.PDFService = pdfextract.NewService(s.Logger, s.OCRService, pdfextract.ServiceConfig{
		RuntimeDir:   filepath.Join(cfg.DataDir, "models", "pdfium"),
		AutoDownload: true,
	})
	trace.Mark("ocr_pdf_ready")
	return nil
}

func initServicesRuntimeRegistries(s *Services, cfg *ServerConfig, appCfg *config.Config, trace *StartupTrace) {
	s.LLMRegistry = llm.NewProviderRegistry()
	registerLLMProviders(s.LLMRegistry, appCfg)
	trace.Mark("llm_registry_ready")
	s.ToolRegistry = tools.NewRegistry()
	registerBuiltinRuntimeTools(s.ToolRegistry, cfg, appCfg)
	tools.AttachPDFServiceToWebTools(s.ToolRegistry, s.PDFService)
	tools.RegisterFactoryToolDefinitions(s.ToolRegistry)
	tools.RegisterAgentTools(s.ToolRegistry, appCfg)
	tools.RegisterSessionTools(s.ToolRegistry, sessionListAdapter{store: s.MemoryStore})
	tools.RegisterCanvasTools(s.ToolRegistry, s.A2UIManager)
	tools.RegisterPDFTool(s.ToolRegistry, s.PDFService)
	trace.Mark("tool_registry_ready")
	s.SkillRegistry = skill.NewRegistry()
	builtin.RegisterAll(s.SkillRegistry)
	trace.Mark("skill_registry_ready")
}

func initServicesWorkerPool(s *Services, trace *StartupTrace) {
	s.WorkerPool = worker.NewPool(context.Background(), 10)
	trace.Mark("worker_pool_ready")
}
