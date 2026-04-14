// Package bootstrap provides shared server initialization logic
package bootstrap

import (
	"database/sql"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
	"go.uber.org/zap"
)

// Services holds all initialized services
type Services struct {
	DB            *sql.DB
	DBConn        *database.SQLiteConn // Read-write separated connection
	RuntimeDBConn *database.SQLiteConn // Separate DB for harness/agent tables (reduces startup time)
	Config        *config.Config
	Logger        *zap.Logger
	UserService   *user.Service
	UserRepo      *user.SQLiteRepository
	JWTService    *auth.JWTService
	APIKeyService *auth.APIKeyService
	MemoryStore   *memory.Store
	A2UIManager   *a2ui.Manager
	OCRService    *ocrruntime.TesseractService
	PDFService    *pdfextract.Service
	LLMRegistry   *llm.ProviderRegistry
	ToolRegistry  *tools.Registry
	SkillRegistry *skill.Registry
	MgmtTool      *tools.MgmtTool
	WorkerPool    *worker.Pool
	DataDir       string
}

// InitServices initializes all core services
func InitServices(cfg *ServerConfig, appCfg *config.Config, logger *zap.Logger) (*Services, error) {
	trace := NewStartupTrace("bootstrap.init_services", logger)
	trace.Mark("enter")

	s := &Services{
		Config:  appCfg,
		Logger:  logger,
		DataDir: cfg.DataDir,
	}

	if err := initServicesDatabaseAndIdentity(s, cfg, appCfg, trace); err != nil {
		return nil, err
	}

	if err := initServicesRuntimeDatabase(s, cfg, trace); err != nil {
		return nil, err
	}

	if err := initServicesMemoryAndMedia(s, cfg, appCfg, trace); err != nil {
		return nil, err
	}

	initServicesRuntimeRegistries(s, cfg, appCfg, trace)
	initServicesWorkerPool(s, trace)

	trace.Mark("complete")
	return s, nil
}
