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

func initServicesMemoryAndMedia(
	s *Services,
	cfg *ServerConfig,
	appCfg *config.Config, trace *StartupTrace,
) error {
	chatDBPath := filepath.Join(cfg.DataDir, "blue.db")
	chatStoreOpts := memory.DefaultChatStoreOptions(chatDBPath)
	chatStoreOpts.Durability = appCfg.Session.ChatDBDurability
	chatStoreOpts.CheckpointInterval = 0
	chatStoreOpts.AttachmentExternalStore = appCfg.Session.ChatAttachmentExternalStore
	if chatStoreOpts.AttachmentExternalStore {
		chatStoreOpts.AttachmentDir = filepath.Join(cfg.DataDir, "message_attachments")
	}
	store, err := memory.NewStoreWithOptions(chatDBPath, chatStoreOpts)
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
	s.PDFService = pdfextract.NewService(s.Logger, s.OCRService)
	trace.Mark("ocr_pdf_ready")
	return nil
}

func initServicesRuntimeRegistries(
	s *Services,
	cfg *ServerConfig,
	appCfg *config.Config,
	trace *StartupTrace,
) {
	s.LLMRegistry = llm.NewProviderRegistry()
	registerLLMProviders(s.LLMRegistry, appCfg)
	trace.Mark("llm_registry_ready")

	s.ToolRegistry = tools.NewRegistry()
	tools.RegisterBuiltinToolsWithRuntimeConfig(
		s.ToolRegistry,
		buildWebSearchConfig(appCfg),
		buildWebFetchConfig(appCfg),
		resolveBuiltinToolAllowedPaths(appCfg, cfg.DataDir),
		0,
		tools.BuiltinRuntimeConfig{
			DataDir:      cfg.DataDir,
			WorkspaceDir: ResolveWorkspaceDir(cfg.DataDir, appCfg),
			Ripgrep:      appCfg.ToolCalling.Ripgrep,
			SkillDynamicExposureEnabled: func() bool {
				return true
			},
		},
	)
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
