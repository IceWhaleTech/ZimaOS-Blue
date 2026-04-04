package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	concpool "github.com/sourcegraph/conc/pool"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/a2ui"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/agentcore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/backup"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/bootstrap"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/companion"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack"
	contextpackembed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/contextpack/embedded"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	dbutil "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/database"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/embedding"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/extauth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/formfiller"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/lifecycle"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/metrics"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mfa"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	ocrruntime "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ocr"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/password"
	pdfextract "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/pdf"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/plugin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/reclaim"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sandbox"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sessionaudit"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	skillEmbed "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/embedded"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/speech"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/stt"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tts"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/voice"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/web"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/worker"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workflow"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/workspace"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	ssePkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
)

var (
	version   = "0.10.38"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func applyPendingBackupRestore(dataDir string) (bool, error) {
	mgr, err := backup.NewManager(backup.Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          filepath.Join(dataDir, "backups"),
		SkillsPath:    filepath.Join(dataDir, "workspace", ".claude", "skills"),
	}, dataDir, dataDir)
	if err != nil {
		return false, err
	}
	if !mgr.HasPendingRestore() {
		return false, nil
	}
	_, err = mgr.ApplyPendingRestore(context.Background())
	return true, err
}

func applyPrimaryDatabasePoolConfig(conn *dbutil.SQLiteConn, perfCfg config.DatabasePerfConfig) {
	if conn == nil || conn.Writer == nil {
		return
	}

	maxOpen := perfCfg.PoolSize
	if maxOpen <= 0 {
		maxOpen = 10
	}
	maxIdle := perfCfg.MaxIdleConns
	if maxIdle <= 0 {
		maxIdle = 5
	}
	if maxIdle > maxOpen {
		maxIdle = maxOpen
	}
	connMaxLifetime := perfCfg.ConnMaxLifetime
	if connMaxLifetime <= 0 {
		connMaxLifetime = time.Hour
	}
	connMaxIdleTime := perfCfg.ConnMaxIdleTime
	if connMaxIdleTime <= 0 {
		connMaxIdleTime = 10 * time.Minute
	}

	if conn.Reader == nil || conn.Reader == conn.Writer {
		conn.Writer.SetMaxOpenConns(maxOpen)
		conn.Writer.SetMaxIdleConns(maxIdle)
		conn.Writer.SetConnMaxLifetime(connMaxLifetime)
		conn.Writer.SetConnMaxIdleTime(connMaxIdleTime)
		return
	}

	conn.Writer.SetMaxOpenConns(1)
	conn.Writer.SetMaxIdleConns(1)
	conn.Writer.SetConnMaxLifetime(connMaxLifetime)
	conn.Writer.SetConnMaxIdleTime(connMaxIdleTime)

	conn.Reader.SetMaxOpenConns(maxOpen)
	conn.Reader.SetMaxIdleConns(maxIdle)
	conn.Reader.SetConnMaxLifetime(connMaxLifetime)
	conn.Reader.SetConnMaxIdleTime(connMaxIdleTime)
}

func openPrimaryDatabaseWithStartupRecovery(dataDir string, perfCfg config.DatabasePerfConfig) (*dbutil.SQLiteConn, error) {
	dbPath := filepath.Join(dataDir, "blue.db")

	open := func() (*dbutil.SQLiteConn, error) {
		cacheSize := perfCfg.CacheSize
		if cacheSize == 0 {
			cacheSize = 2000
		}

		if perfCfg.WALMode {
			conn, err := dbutil.OpenSQLite(dbPath, &dbutil.SQLiteOpenOpts{
				MaxReaders:  perfCfg.PoolSize,
				BusyTimeout: 5000,
				CacheSize:   -cacheSize,
				ForeignKeys: true,
			})
			if err != nil {
				return nil, err
			}
			applyPrimaryDatabasePoolConfig(conn, perfCfg)
			_, _ = conn.Writer.Exec("PRAGMA shrink_memory")
			return conn, nil
		}

		db, err := dbutil.OpenSQLiteWithRecovery(dbPath, dbPath, func(db *sql.DB) error {
			pragmas := []string{
				"PRAGMA busy_timeout=5000",
				"PRAGMA journal_mode=DELETE",
				"PRAGMA foreign_keys=ON",
				"PRAGMA synchronous=FULL",
				fmt.Sprintf("PRAGMA cache_size=-%d", cacheSize),
				"PRAGMA wal_autocheckpoint=1000",
			}
			if runtime.GOOS == "darwin" {
				pragmas = append(pragmas,
					"PRAGMA fullfsync=ON",
					"PRAGMA checkpoint_fullfsync=ON",
				)
			}
			for _, pragma := range pragmas {
				if _, err := db.Exec(pragma); err != nil {
					return fmt.Errorf("exec %q: %w", pragma, err)
				}
			}
			db.Exec("PRAGMA shrink_memory")
			return nil
		})
		if err != nil {
			return nil, err
		}
		conn := &dbutil.SQLiteConn{Writer: db, Reader: db}
		applyPrimaryDatabasePoolConfig(conn, perfCfg)
		return conn, nil
	}

	var err error
	if !dbutil.StartupQuickCheckEnabled() {
		if quickErr := dbutil.QuickCheckDatabase(dbPath); quickErr != nil {
			if dbutil.IsSQLiteCorruptionError(quickErr) {
				err = dbutil.WrapSQLiteOpenError(dbPath, quickErr)
				logger.Warn().Err(quickErr).Str("db_path", dbPath).Msg("Primary database quick check reported corruption before startup open")
			} else {
				logger.Warn().Err(quickErr).Str("db_path", dbPath).Msg("Primary database quick check failed before startup open, falling back to normal open path")
			}
		}
	}

	var dbConn *dbutil.SQLiteConn
	if err == nil {
		dbConn, err = open()
	}
	if err == nil {
		return dbConn, nil
	}
	if !dbutil.IsSQLiteCorruptionError(err) {
		return nil, err
	}

	logger.Warn().Err(err).Str("db_path", dbPath).Msg("Primary database open reported corruption, attempting startup auto-recovery")

	var recoverErr error
	mgr, mgrErr := backup.NewManager(backup.Config{
		Enabled:       true,
		RetentionDays: 7,
		Path:          filepath.Join(dataDir, "backups"),
		SkillsPath:    filepath.Join(dataDir, "workspace", ".claude", "skills"),
	}, dataDir, dataDir)
	if mgrErr != nil {
		recoverErr = fmt.Errorf("init backup manager: %w", mgrErr)
		logger.Warn().Err(mgrErr).Str("db_path", dbPath).Msg("Failed to initialize backup manager for corrupted primary database, recreating fresh database instead")
	} else {
		result, autoRecoverErr := mgr.CheckAndAutoRecover(context.Background(), []string{dbPath})
		recoverErr = autoRecoverErr
		if autoRecoverErr == nil {
			if result != nil && len(result.RepairedDatabases) > 0 {
				logger.Info().Strs("repaired_databases", result.RepairedDatabases).Msg("Primary database repaired during startup recovery")
			}
			if result != nil && result.Recovered {
				logger.Info().
					Strs("corrupted_databases", result.CorruptedDatabases).
					Str("backup_id", result.BackupID).
					Msg("Primary database restored from backup during startup recovery")
			}

			dbConn, retryErr := open()
			if retryErr == nil {
				return dbConn, nil
			}
			if !dbutil.IsSQLiteCorruptionError(retryErr) {
				return nil, fmt.Errorf("open database after startup auto-recovery: %w", retryErr)
			}
			recoverErr = retryErr
			logger.Warn().Err(retryErr).Str("db_path", dbPath).Msg("Primary database still failed after startup auto-recovery, recreating from .bak backup")
		} else {
			logger.Warn().Err(autoRecoverErr).Str("db_path", dbPath).Msg("Startup auto-recovery did not produce a usable primary database, recreating a fresh database")
		}
	}

	backupPath, rotateErr := dbutil.RotateCorruptSQLiteDatabase(dbPath)
	if rotateErr != nil {
		return nil, fmt.Errorf("open database: %w (startup auto-recovery failed: %v; rotate corrupt database failed: %v)", err, recoverErr, rotateErr)
	}
	logger.Warn().
		Str("db_path", dbPath).
		Str("backup_path", backupPath).
		Msg("Rotated corrupt primary database to .bak backup and recreating a fresh database")

	dbConn, retryErr := open()
	if retryErr != nil {
		return nil, fmt.Errorf("open database after recreating corrupt primary database: %w", retryErr)
	}
	return dbConn, nil
}

func main() {
	// Fast-path: CLI subcommands bypass cobra to minimize page faults and RSS.
	// All init() functions have already run, but we avoid touching cobra's
	// command tree, flag parsing, and the heavy code paths they pull in.
	// This must run BEFORE macosRequestSTTAuthorization() so IPC calls
	// (e.g. `blue web_query ...`) don't trigger CGo/Speech framework
	// initialization, log output, or any server-side side effects.
	if len(os.Args) > 1 {
		if cliDispatch(os.Args[1:]) {
			return
		}
	}

	// On macOS, request speech recognition authorization on thread 0
	// BEFORE starting the server. runtime.LockOSThread() in macos_init.go
	// pins this goroutine to thread 0 (required by AppKit/TCC).
	if !shouldSkipStartupSTTAuthorization(os.Args[1:]) {
		macosRequestSTTAuthorization()
	}

	// On macOS, the main goroutine (thread 0) must pump the Cocoa run loop
	// for Speech framework callbacks. Run the server on a goroutine and
	// keep thread 0 for the run loop. On non-darwin, macosRunMainRunLoop()
	// is a no-op so we fall through to Execute() directly.
	if isDarwin() {
		go func() {
			Execute()
			// Server shut down — stop the run loop so main() can return
			macosStopMainRunLoop()
		}()
		macosRunMainRunLoop() // blocks thread 0 until StopMainRunLoop()
	} else {
		Execute()
	}
}

const runtimeIdleCheckpointThreshold = 5 * time.Minute

type serverRunOutcome struct {
	RestartRequested bool
}

type serverRestartController struct {
	mu         sync.Mutex
	requested  bool
	suppressed bool
	ch         chan struct{}
}

func newServerRestartController() *serverRestartController {
	return &serverRestartController{
		ch: make(chan struct{}),
	}
}

func (c *serverRestartController) Request() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.suppressed || c.requested {
		return false
	}
	c.requested = true
	close(c.ch)
	return true
}

func (c *serverRestartController) Suppress() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.suppressed = true
	c.requested = false
}

func (c *serverRestartController) Requested() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.requested && !c.suppressed
}

func (c *serverRestartController) C() <-chan struct{} {
	return c.ch
}

var runServerIteration = runServerOnce

func shouldSkipStartupSTTAuthorization(args []string) bool {
	var positional []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--config", "--profile":
			if i+1 < len(args) {
				i++
			}
		case "--", "-h", "--help", "--dev", "--no-color", "--no-intercept", "--json", "-v", "--verbose":
			continue
		default:
			positional = append(positional, args[i])
		}
	}

	if len(positional) == 0 {
		return false
	}
	if positional[0] != "gateway" {
		return false
	}
	if len(positional) < 2 {
		return true
	}
	return positional[1] != "run"
}

// runServer is the CLI server entry point. It can restart the full runtime
// in-process when a backup restore has been staged and needs a clean reload.
func runServer() {
	for {
		outcome := runServerIteration()
		if !outcome.RestartRequested {
			return
		}
		fmt.Fprintln(os.Stderr, "Restarting ZimaOS-Blue runtime to apply staged backup restore...")
	}
}

// runServerOnce runs a single server lifetime.
func runServerOnce() serverRunOutcome {
	// Tune GC for lower memory usage (shared with bluelib)
	bootstrap.TuneGC()
	restartController := newServerRestartController()

	// Load configuration (cfgFile is set by cobra's --config flag)
	cfg, err := config.Load(cfgFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}
	var hotReloader *config.HotReloader

	// Initialize logger
	if err := logger.InitWithMirror(&cfg.Log, filepath.Join(getLogsDir(), "blue.log")); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
		os.Exit(1)
	}

	logger.Info().
		Str("version", version).
		Str("build_time", buildTime).
		Str("git_commit", gitCommit).
		Msg("Starting ZimaOS-Blue")

	// Initialize lifecycle manager
	lm := lifecycle.New()

	// Initialize worker pool
	pool := worker.NewPool(lm.Context(), cfg.Worker.PoolSize)
	logger.Info().Int("pool_size", cfg.Worker.PoolSize).Msg("Worker pool initialized")

	// Initialize database
	dataDir := getDataDir()
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		logger.Fatal().Err(err).Msg("Failed to create data directory")
	}
	previousCleanShutdown, startupIntegrityErr := dbutil.BeginStartupIntegritySession(dataDir)
	if startupIntegrityErr != nil {
		logger.Warn().Err(startupIntegrityErr).Msg("Failed to initialize startup integrity state")
		previousCleanShutdown = false
	}
	// After an unclean shutdown, pay the quick_check cost once during startup
	// so power-loss corruption is surfaced before services begin using blue.db.
	startupQuickCheckEnabled := !previousCleanShutdown
	dbutil.SetStartupQuickCheckEnabled(startupQuickCheckEnabled)
	defer dbutil.SetStartupQuickCheckEnabled(true)
	appliedPendingRestore, err := applyPendingBackupRestore(dataDir)
	if previousCleanShutdown {
		logger.Info().Msg("Skipping proactive startup database scan after previous clean shutdown")
	} else if !appliedPendingRestore {
		logger.Info().Bool("startup_quick_check", startupQuickCheckEnabled).Msg("Primary database open will run quick integrity checks after an unclean shutdown")
	}
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to apply pending backup restore before database initialization")
	}

	dbConn, err := openPrimaryDatabaseWithStartupRecovery(dataDir, cfg.Performance.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer dbConn.Close()
	db := dbConn.Writer
	dbReader := dbConn.Reader

	// Shared kvstore for all config persistence (replaces scattered JSON files)
	sqliteKV, err := kvstore.NewSQLiteStoreWithReadDB(db, dbReader)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize config kvstore")
	}
	configKV := kvstore.NewCachedStore(sqliteKV)

	// Import config.yaml into kvstore (first-run: imports; subsequent: loads from DB)
	configStore := config.NewConfigStore(configKV)
	cfg, err = configStore.LoadOrImport(cfg)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to load config from DB, using YAML defaults")
	}
	applyServerRuntimeOverrides(&cfg.Server)
	hotReloader, err = config.NewHotReloader(cfgFile, cfg, &config.HotReloadConfig{
		Enabled:             true,
		WatchInterval:       5 * time.Second,
		ValidateBeforeApply: true,
	})
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to initialize hot reloader")
		hotReloader = nil
	} else {
		config.SyncHotReloadToStore(hotReloader, configStore)
		if err := hotReloader.Start(); err != nil {
			logger.Warn().Err(err).Msg("Failed to start hot reloader")
		}
		lm.RegisterShutdownHook(func(ctx context.Context) error {
			_ = ctx
			return hotReloader.Stop()
		})
	}
	if result, migrateErr := server.MigrateLegacyProviderSettings(context.Background(), configKV, dataDir); migrateErr != nil {
		logger.Warn().Err(migrateErr).Msg("Failed to migrate legacy provider settings into config store")
	} else if result != nil {
		entry := logger.Info().Str("source", result.SourcePath)
		if result.ArchivedPath != "" {
			entry = entry.Str("archived_path", result.ArchivedPath)
		}
		entry.Msg("Legacy provider settings imported into config store")
	}
	if result, migrateErr := harness.MigrateLegacyStore(context.Background(), db, dataDir); migrateErr != nil {
		logger.Warn().Err(migrateErr).Msg("Failed to migrate legacy harness store")
	} else if result != nil {
		entry := logger.Info().Str("source", result.SourcePath).Int("rows_imported", result.RowsImported)
		if result.ArchivedPath != "" {
			entry = entry.Str("archived_path", result.ArchivedPath)
		}
		entry.Msg("Legacy harness store imported into blue.db")
	}
	runtimeActivity := server.NewRuntimeActivityTracker()
	if cfg.Performance.Database.CheckpointInterval > 0 {
		dbutil.StartIdleAwarePeriodicWALCheckpoint(
			lm.Context(),
			db,
			cfg.Performance.Database.CheckpointInterval,
			runtimeIdleCheckpointThreshold,
			func(ctx context.Context) (bool, string, error) {
				return runtimeActivity.CheckpointIdle(ctx, dbReader, runtimeIdleCheckpointThreshold)
			},
			dbutil.CheckpointTruncate,
			func(err error) {
				logger.Warn().Err(err).Msg("Idle-aware WAL checkpoint failed")
			},
		)
		logger.Info().
			Dur("interval", cfg.Performance.Database.CheckpointInterval).
			Dur("idle_threshold", runtimeIdleCheckpointThreshold).
			Msg("Idle-aware WAL checkpoint enabled")
	} else {
		logger.Info().Msg("Periodic WAL checkpoint disabled")
	}

	// Initialize user repository and service
	userRepo, err := user.NewSQLiteRepositoryWithReadDB(db, dbReader)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize user repository")
	}

	// Create password hasher with secure defaults
	passwordHasher := password.NewHasher(&password.Config{
		Memory:      32 * 1024, // 32 MB (reduced from 64MB for lower memory spikes, still OWASP-compliant)
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	})

	// Create password policy based on config
	passwordPolicy := password.NewPolicy(&password.PolicyConfig{
		MinLength:        cfg.Security.Password.MinLength,
		RequireUppercase: cfg.Security.Password.RequireUppercase,
		RequireLowercase: cfg.Security.Password.RequireLowercase,
		RequireNumber:    cfg.Security.Password.RequireNumber,
		RequireSpecial:   cfg.Security.Password.RequireSpecial,
	})

	userService := user.NewService(userRepo, passwordHasher, passwordPolicy, nil)
	userHandler := user.NewHandler(userService)

	// Initialize permission repository and service
	permissionRepo, err := permission.NewRepositoryWithReadDB(db, dbReader)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize permission repository")
	}
	permissionService := permission.NewService(permissionRepo, userRepo)
	permissionHandler := permission.NewHandler(permissionService, userRepo)

	// Set permission service on user handler
	userHandler.SetPermissionService(permissionService)

	// Initialize memory store for conversations using a dedicated handle so chat
	// pragmas do not leak into the shared primary DB users.
	chatDBPath := filepath.Join(dataDir, "blue.db")
	chatStoreOpts := memory.DefaultChatStoreOptions(chatDBPath)
	chatStoreOpts.Durability = cfg.Session.ChatDBDurability
	chatStoreOpts.CheckpointInterval = 0
	chatStoreOpts.AttachmentExternalStore = cfg.Session.ChatAttachmentExternalStore
	if chatStoreOpts.AttachmentExternalStore {
		chatStoreOpts.AttachmentDir = filepath.Join(dataDir, "message_attachments")
	}
	memoryStore, err := memory.NewStoreWithOptions(chatDBPath, chatStoreOpts)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize memory store")
	}

	// Initialize LLM provider registry
	llmRegistry := llm.NewProviderRegistry()

	// Register default LLM providers from environment variables
	// Only register providers with actual API keys to reduce idle memory

	// OpenAI provider
	openaiKey := os.Getenv("OPENAI_API_KEY")
	if openaiKey != "" {
		openaiBaseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
		llmRegistry.Register(llm.NewOpenAIProvider(openaiKey, openaiBaseURL))
	}

	// Claude provider
	claudeKey := os.Getenv("ANTHROPIC_API_KEY")
	if claudeKey != "" {
		llmRegistry.Register(llm.NewClaudeProvider(claudeKey, ""))
	}

	// Ollama provider is only registered when explicitly configured.
	ollamaURL := strings.TrimSpace(os.Getenv("OLLAMA_URL"))
	if ollamaURL != "" {
		llmRegistry.Register(llm.NewOllamaProvider(ollamaURL))
	}

	// Custom OpenAI-compatible provider (for third-party services like DeepSeek, Together, etc.)
	customKey := os.Getenv("CUSTOM_API_KEY")
	customURL := os.Getenv("CUSTOM_API_URL")
	if customKey != "" || customURL != "" {
		llmRegistry.Register(llm.NewCustomProvider(customKey, customURL))
	}

	// Grok provider (xAI)
	grokKey := os.Getenv("GROK_API_KEY")
	if grokKey != "" {
		llmRegistry.Register(llm.NewGrokProvider(grokKey, ""))
	}

	// Qwen provider (Alibaba Cloud)
	qwenKey := os.Getenv("QWEN_API_KEY")
	if qwenKey != "" {
		llmRegistry.Register(llm.NewQwenProvider(qwenKey, ""))
	}

	// Initialize tools registry and register built-in tools
	toolRegistry := tools.NewRegistry()
	registerServerToolRegistry(toolRegistry, cfg, dataDir)
	logger.Info().Int("count", len(toolRegistry.List())).Msg("Built-in tools registered")

	// Initialize skill registry and register built-in skills
	skillRegistry := skill.NewRegistry()
	if err := builtin.RegisterAll(skillRegistry); err != nil {
		logger.Fatal().Err(err).Msg("Failed to register built-in skills")
	}
	logger.Info().Int("count", skillRegistry.Count()).Msg("Built-in skills registered")

	// Initialize plugin registry and store
	pluginRegistry := plugin.NewRegistry()
	pluginStoreConfig := plugin.DefaultStoreConfig()
	pluginStoreConfig.CacheDir = filepath.Join(getDataDir(), "plugin-cache")
	pluginStore := plugin.NewStore(pluginStoreConfig, pluginRegistry)
	logger.Info().Msg("Plugin store initialized")

	var a2uiManager *a2ui.Manager
	var ocrService *ocrruntime.TesseractService
	var pdfService *pdfextract.Service

	// Initialize chat handler
	chatHandler := server.NewChatHandler(memoryStore, llmRegistry, toolRegistry)
	chatHandler.SetPersistenceOptions(cfg.Session.ChatPersistAsync, cfg.Session.ChatReadLite)
	if cfg.Session.Audit.Enabled {
		auditCfg := sessionaudit.StoreConfig{
			RetentionDays:      cfg.Session.Audit.RetentionDays,
			CleanupInterval:    cfg.Session.Audit.CleanupInterval,
			CleanupBatchSize:   cfg.Session.Audit.CleanupBatchSize,
			Durability:         cfg.Session.ChatDBDurability,
			WALAutoCheckpoint:  4000,
			CheckpointInterval: 0,
		}
		auditDBPath := sessionaudit.ResolveDBPath(dataDir, cfg.Session.Audit.Path)
		var (
			auditStore *sessionaudit.Store
			err        error
		)
		if mkErr := os.MkdirAll(filepath.Dir(auditDBPath), 0o750); mkErr != nil {
			logger.Warn().Err(mkErr).Str("path", auditDBPath).Msg("Failed to create session audit directory")
		} else {
			auditStore, err = sessionaudit.NewSQLiteStore(auditDBPath, auditCfg)
		}
		if err != nil {
			logger.Warn().Err(err).Str("path", auditDBPath).Msg("Failed to initialize session audit store")
		} else if auditStore != nil {
			chatHandler.SetSessionAuditStore(auditStore)
			logger.Info().Str("path", auditDBPath).Int("retention_days", cfg.Session.Audit.RetentionDays).Msg("Session tool payload audit store enabled")
		}
	}
	lm.RegisterShutdownHook(func(ctx context.Context) error {
		_ = ctx
		chatHandler.Shutdown()
		_ = memoryStore.Close()
		return nil
	})

	// Initialize external auth service (for OAuth/OIDC providers)
	extauthService, err := extauth.NewService(&extauth.ServiceConfig{
		Providers:    []*extauth.ProviderConfig{}, // No providers configured by default
		StateStore:   extauth.NewMemoryStateStore(),
		AccountStore: extauth.NewMemoryAccountStore(),
		UserStore:    nil, // TODO: Implement user store adapter
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize external auth service")
	}
	extauthHandler := extauth.NewHandler(extauthService)

	// Initialize JWT service
	jwtService := auth.NewJWTService(&auth.JWTConfig{
		Secret:            cfg.Security.JWT.Secret,
		Expiration:        cfg.Security.JWT.Expiration,
		RefreshExpiration: cfg.Security.JWT.RefreshExpiration,
		Issuer:            cfg.Security.JWT.Issuer,
	})

	// Initialize API Key service (shares main blue.db)
	apiKeyServiceFactory := func() (*auth.APIKeyService, error) {
		return auth.NewAPIKeyServiceWithDB(db)
	}
	if dbConn != nil {
		apiKeyServiceFactory = func() (*auth.APIKeyService, error) {
			return auth.NewAPIKeyServiceWithReadDB(dbConn.Writer, dbConn.Reader)
		}
	}
	apiKeyService, err := apiKeyServiceFactory()
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize API key service")
	}

	// Initialize internal routing API keys for the local proxy/runtime bridge.
	// This must be done after apiKeyService is ready
	// Create three internal API keys for different routing modes:
	// - runtime-auto: Auto mode (system chooses best provider)
	// - runtime-cloud: Cloud mode (force cloud provider)
	// - runtime-local: Local mode (force local runtime)
	var runtimeAutoKey, runtimeCloudKey, runtimeLocalKey string

	if providerpool.GetTrialLicense() != "" {
		// Helper function to create or recreate an API key
		createOrRecreateKey := func(name string, scopes []string) string {
			keyInfo, err := apiKeyService.CreateKey(context.Background(), &auth.CreateKeyRequest{
				UserID: "system",
				Name:   name,
				Scopes: scopes,
			})
			if err != nil {
				// Key might already exist, revoke and recreate
				keys, _ := apiKeyService.ListKeys(context.Background(), "system")
				for _, k := range keys {
					if k.Name == name {
						apiKeyService.RevokeKey(context.Background(), k.ID, "system")
						keyInfo, _ = apiKeyService.CreateKey(context.Background(), &auth.CreateKeyRequest{
							UserID: "system",
							Name:   name,
							Scopes: scopes,
						})
						break
					}
				}
			}
			if keyInfo != nil {
				return keyInfo.Key
			}
			return ""
		}

		// Create three keys for different routing modes
		runtimeAutoKey = createOrRecreateKey("runtime-auto", []string{"chat", "proxy", "route:auto"})
		runtimeCloudKey = createOrRecreateKey("runtime-cloud", []string{"chat", "proxy", "route:cloud"})
		runtimeLocalKey = createOrRecreateKey("runtime-local", []string{"chat", "proxy", "route:local"})

		logger.Info().
			Str("auto_key", runtimeAutoKey[:8]+"...").
			Str("cloud_key", runtimeCloudKey[:8]+"...").
			Str("local_key", runtimeLocalKey[:8]+"...").
			Msg("Created three internal API keys for runtime routing modes")
	}

	// Note: the local coding runtime is not registered as an LLM provider.
	// All chat requests are routed through the proxy, which handles provider selection internally.

	// Initialize auth middleware
	authMiddleware := auth.NewAuthMiddleware(jwtService, apiKeyService)

	// Initialize API Key handler
	apiKeyHandler := auth.NewAPIKeyHandler(apiKeyService)

	// Set JWT service on user handler for token generation
	userHandler.SetJWTService(jwtService)

	// Initialize auto-reply service and handler
	zapLogger, _ := zap.NewProduction()
	a2uiManager = a2ui.NewManager(zapLogger)
	ocrService = ocrruntime.NewTesseractService(zapLogger, ocrruntime.Config{
		ModelDir:     filepath.Join(dataDir, "models", "tesseract"),
		AutoDownload: true,
		WorkerCount:  1,
	})
	pdfService = pdfextract.NewService(zapLogger, ocrService)
	tools.RegisterCanvasTools(toolRegistry, a2uiManager)
	tools.AttachPDFServiceToWebTools(toolRegistry, pdfService)
	tools.RegisterPDFTool(toolRegistry, pdfService)
	autoreplyService := autoreply.NewService(autoreply.DefaultConfig(), zapLogger)
	autoreplyHandler := autoreply.NewHandler(autoreplyService, zapLogger)

	// Initialize independent services in parallel for faster startup
	var (
		metricsCollector  *metrics.Collector
		metricsWriter     *metrics.MetricsWriter
		backupManager     *backup.Manager
		backupHandler     *backup.Handler
		threatDetector    *security.ThreatDetector
		securityHandler   *security.Handler
		mfaHandler        *mfa.Handler
		sandboxManager    *sandbox.Manager
		sandboxHandler    *sandbox.Handler
		cronHandler       *cron.Handler
		browserHandler    *browser.Handler
		sttService        stt.Service
		ttsService        tts.Service
		voiceHandler      *voice.Handler
		workflowHandler   *workflow.Handler
		formfillerStore   *formfiller.Store
		formfillerHandler *formfiller.Handler
		ngrokTunnelMgr    *ngrok.SDKTunnelManager
		ngrokConfigStore  *ngrok.ConfigStore
	)

	// Use conc/pool for safer parallel initialization with automatic panic recovery
	initPool := concpool.New().WithMaxGoroutines(8)

	// Metrics: async init — chat handler nil-checks metricsRecorder, so first
	// requests simply skip recording until metrics is ready.  This avoids
	// blocking server start on metrics initialization + collector goroutine.
	go func() {
		// Metrics collector (collect every 10 seconds, keep 5 minutes of history)
		metricsCollector = metrics.NewCollector(10*time.Second, 30)
		metricsCollector.Start()
		// Metrics writer for detailed API metrics with SQLite persistence.
		// Reuse blue.db by default to reduce auxiliary SQLite files.
		metricsConfig := metrics.DefaultWriterConfig()
		metricsConfig.SharedSQLiteDB = db
		metricsConfig.SharedSQLiteReadDB = dbReader
		metricsWriter = metrics.NewMetricsWriter(nil, metricsConfig)
		metricsWriter.Start()
		logger.Info().Msg("Metrics services initialized")

		// Set metrics recorder on chat handler after initialization
		chatHandler.SetMetricsRecorder(metricsWriter)
		logger.Info().Msg("Metrics recorder set on chat handler")
	}()

	// Group 2: Non-critical services (async initialization for faster startup)

	initPool.Go(func() {
		// Backup manager
		var err error
		backupManager, err = backup.NewManager(backup.Config{
			Enabled:            true,
			RetentionDays:      7,
			Path:               filepath.Join(dataDir, "backups"),
			SkillsPath:         filepath.Join(dataDir, "workspace", ".claude", "skills"),
			AutoBackup:         false,
			AutoBackupInterval: 6 * time.Hour,
			AutoBackupOnChange: false,
			ChangePollInterval: time.Minute,
			ChangeDebounce:     5 * time.Minute,
		}, dataDir, dataDir)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize backup manager")
			return
		}
		backupHandler = backup.NewHandler(backupManager)
		backupHandler.SetRestartFunc(func() error {
			if restartController.Request() {
				logger.Info().Msg("Backup restore staged; restarting CLI runtime in-process")
			}
			return nil
		})
		logger.Info().Msg("Backup manager initialized with runtime auto backup disabled")
	})

	// Browser automation service — lazy init, only when first API call arrives
	// Chromium is very heavy on memory, skip at startup
	logger.Info().Msg("Browser automation will be initialized on first use")

	// TTS service — pick OS-appropriate default provider (async, not needed at startup)
	initPool.Go(func() {
		defaultTTSProvider := tts.ProviderEdge
		if runtime.GOOS == "darwin" {
			defaultTTSProvider = tts.ProviderMacOSNative
		}

		providers := []tts.ProviderConfig{
			{Type: tts.ProviderEdge, Enabled: true},
		}
		if defaultTTSProvider == tts.ProviderMacOSNative {
			providers = append(providers, tts.ProviderConfig{Type: tts.ProviderMacOSNative, Enabled: true})
		}

		var err error
		ttsService, err = tts.NewService(&tts.ServiceConfig{
			DefaultProvider: defaultTTSProvider,
			Providers:       providers,
			DataPath:        dataDir,
		})
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize TTS service, speech features will be limited")
		} else {
			logger.Info().Msgf("TTS service initialized (%s)", defaultTTSProvider)
		}
	})

	// Critical services in parallel pool
	initPool.Go(func() {
		// Security threat detector
		threatDetector = security.NewThreatDetector()
		securityHandler = security.NewHandler(threatDetector)
		logger.Info().Msg("Security handler initialized")
	})

	// Workflow service — lazy init on first API call (avoids cron goroutine + DB queries at startup)
	workflowHandler = workflow.NewLazyHandler(func() *workflow.WorkflowService {
		repo, err := workflow.NewRepositoryWithReadDB(db, dbReader)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize workflow repository")
			return nil
		}
		svc, err := workflow.NewService(nil, repo)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize workflow service")
			return nil
		}
		logger.Info().Msg("Workflow service initialized lazily")
		return svc
	})
	workflowHandler.SetIdleReclaim(cfg.Performance.ResourceReclaim.WorkflowIdleAfter)

	// Async initialization for MFA handler
	go func() {
		mfaHandler = mfa.NewHandler(nil, nil)
		logger.Info().Msg("MFA handler initialized")
	}()

	// Sync initialization for Sandbox manager (must complete before route registration)
	{
		if !cfg.Security.Sandbox.Enabled {
			logger.Info().Msg("Sandbox features disabled by config")
		} else {
			var err error
			sandboxManager, err = bootstrap.NewSandboxManagerFromConfig(cfg)
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to initialize sandbox manager, sandbox features will be disabled")
			} else if sandboxManager == nil {
				logger.Info().Msg("Sandbox features disabled by config")
			} else if !sandboxManager.IsSupported() {
				logger.Warn().Str("reason", sandboxManager.SupportReason()).Msg("Sandbox manager initialized without a supported isolation backend; sandbox features will remain disabled")
				sandboxManager = nil
			} else {
				sandboxHandler = sandbox.NewHandler(sandboxManager)
				logger.Info().Bool("supported", true).Msg("Sandbox handler initialized")
			}
		}
	}

	// Cron service — lazy init on first API call (avoids robfig/cron goroutine at startup)
	cronHandler = cron.NewLazyHandler(func() *cron.Service {
		svc := cron.NewService(cron.DefaultConfig(), zapLogger)
		svc.RegisterBuiltinHandlers()
		if err := svc.Start(); err != nil {
			logger.Warn().Err(err).Msg("Failed to start cron service")
		}
		return svc
	}, zapLogger)
	cronHandler.SetIdleReclaim(cfg.Performance.ResourceReclaim.CronIdleAfter)
	logger.Info().Msg("Cron service configured for lazy initialization")

	cronIPC := sockipc.NewCronIPCAdapter(cron.NewSkillAdapter(cronHandler.GetService))

	// SSE event broker — created early so push service can use it as EventPublisher
	sseBroker := ssePkg.NewBroker()

	// Wire Web Push notification support (shared with bluelib)
	wpSender := bootstrap.InitWebPushSenderWithReadDB(db, dbReader, configKV, zapLogger)

	// Wire push notification service (shared with bluelib)
	pushResult := bootstrap.InitPushService(&bootstrap.PushServiceDeps{
		DB:          db,
		ReadDB:      dbReader,
		MemoryStore: memoryStore,
		CronGetSvc:  cronHandler.GetService,
		SSEBroker:   sseBroker,
		WPSender:    wpSender,
		Logger:      zapLogger,
	})
	var pushIPC sockipc.PushBackend
	var pushSvc *push.Service
	if pushResult != nil {
		pushIPC = pushResult.IPC
		pushSvc = pushResult.Service
	}

	// Wire browser service — lazy init, creates rod service on first use (for IPC only)
	var browserBackend tools.BrowserBackend
	var lazyBrowserSvc func() *browser.RodService
	var acquireBrowserSvc func() (*browser.RodService, func(), error)
	var acquireFallbackBrowserSvc func() (*browser.RodService, func(), error)
	var lightpandaShimSvc *browser.LightpandaService
	var relayInfoProvider func() browser.RelayInfo
	var syncBrowserMonitorRetention func(time.Duration)
	var cleanupBrowserMonitorFrames func()
	{
		type browserRuntime struct {
			lazy           func() *browser.RodService
			peek           func() *browser.RodService
			acquire        func() (*browser.RodService, func(), error)
			peekVisible    func() *browser.RodService
			acquireVisible func() (*browser.RodService, func(), error)
			close          func(context.Context) error
			rodBackend     *tools.RodBrowserBackend
			backend        tools.BrowserBackend
		}

		var browserRuntimeClosers []func(context.Context) error
		var browserRuntimePeekers []func() *browser.RodService
		var browserRuntimeRetentionUpdaters []func(time.Duration)
		initialBrowserMonitorRetention := time.Duration(cfg.Companion.Retention.SessionsDays) * 24 * time.Hour
		cfg.Browser.SessionScreenshotRetention = initialBrowserMonitorRetention
		makeBrowserRuntime := func(baseCfg *browser.Config) browserRuntime {
			headlessCfg := baseCfg.Clone()
			headlessCfg.Headless = true
			headlessRuntime := reclaim.NewManaged[*browser.RodService](
				cfg.Performance.ResourceReclaim.BrowserIdleAfter,
				func() (*browser.RodService, error) {
					return browser.NewService(headlessCfg)
				},
				func(_ context.Context, svc *browser.RodService) error {
					if svc == nil {
						return nil
					}
					return svc.Close()
				},
			)

			visibleCfg := baseCfg.Clone()
			visibleCfg.Headless = false
			visibleRuntime := reclaim.NewManaged[*browser.RodService](
				cfg.Performance.ResourceReclaim.BrowserIdleAfter,
				func() (*browser.RodService, error) {
					return browser.NewService(visibleCfg)
				},
				func(_ context.Context, svc *browser.RodService) error {
					if svc == nil {
						return nil
					}
					return svc.Close()
				},
			)

			updateRetention := func(retention time.Duration) {
				if retention < 0 {
					retention = 0
				}
				headlessCfg.SessionScreenshotRetention = retention
				visibleCfg.SessionScreenshotRetention = retention
				if svc, ok := headlessRuntime.Peek(); ok && svc != nil {
					svc.SetSessionScreenshotRetention(retention)
				}
				if svc, ok := visibleRuntime.Peek(); ok && svc != nil {
					svc.SetSessionScreenshotRetention(retention)
				}
			}

			runtime := browserRuntime{
				lazy: func() *browser.RodService {
					svc, err := headlessRuntime.Get()
					if err != nil {
						logger.Warn().Err(err).Msg("Failed to create browser service")
						return nil
					}
					return svc
				},
				peek: func() *browser.RodService {
					svc, ok := headlessRuntime.Peek()
					if !ok {
						return nil
					}
					return svc
				},
				acquire: headlessRuntime.Acquire,
				peekVisible: func() *browser.RodService {
					svc, ok := visibleRuntime.Peek()
					if !ok {
						return nil
					}
					return svc
				},
				acquireVisible: visibleRuntime.Acquire,
				close: func(ctx context.Context) error {
					if err := visibleRuntime.Close(ctx); err != nil {
						_ = headlessRuntime.Close(ctx)
						return err
					}
					return headlessRuntime.Close(ctx)
				},
			}
			runtime.rodBackend = tools.NewPeekLeaseAwareRodBrowserBackend(
				runtime.peek,
				runtime.acquire,
				runtime.peekVisible,
				runtime.acquireVisible,
			)
			runtime.backend = runtime.rodBackend
			browserRuntimeClosers = append(browserRuntimeClosers, runtime.close)
			browserRuntimePeekers = append(browserRuntimePeekers,
				func() *browser.RodService {
					svc, ok := headlessRuntime.Peek()
					if !ok {
						return nil
					}
					return svc
				},
				func() *browser.RodService {
					svc, ok := visibleRuntime.Peek()
					if !ok {
						return nil
					}
					return svc
				},
			)
			browserRuntimeRetentionUpdaters = append(browserRuntimeRetentionUpdaters, updateRetention)
			return runtime
		}

		var relayServer *browser.RelayServer
		relayInfoProvider = func() browser.RelayInfo {
			if relayServer == nil {
				return browser.RelayInfo{}
			}
			return relayServer.Info()
		}

		if cfg.Browser.RelayEnabled {
			extensionDir, err := browser.EnsureRelayExtensionDir(dataDir)
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to export browser relay extension assets")
			}

			relayServer, err = browser.StartRelayServer(&cfg.Browser, extensionDir)
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to start browser relay server")
			} else {
				info := relayServer.Info()
				logger.Info().
					Str("base_url", info.BaseURL).
					Str("cdp_url", info.CDPURL).
					Str("extension_dir", info.ExtensionDir).
					Msg("Browser relay server started")
				lm.RegisterShutdownHook(func(ctx context.Context) error {
					return relayServer.Close()
				})
			}
		}

		browserHandler = browser.NewLazyHandler(func() browser.Service {
			browserService, err := browser.NewService(&cfg.Browser)
			if err != nil {
				logger.Warn().Err(err).Msg("Failed to initialize browser service lazily")
				return nil
			}
			return browserService
		})
		browserHandler.SetIdleReclaim(cfg.Performance.ResourceReclaim.BrowserIdleAfter)
		browserHandler.SetRelayInfoProvider(relayInfoProvider)

		defaultRuntime := makeBrowserRuntime(&cfg.Browser)
		managedRuntime := defaultRuntime
		if cfg.Browser.ResolvedDriver() != "managed" {
			managedRuntime = makeBrowserRuntime(cfg.Browser.CloneForDriver("managed"))
		}
		relayRuntime := defaultRuntime
		if cfg.Browser.ResolvedDriver() != "relay" {
			relayRuntime = makeBrowserRuntime(cfg.Browser.CloneForDriver("relay"))
		}
		lazyBrowserSvc = defaultRuntime.lazy
		acquireBrowserSvc = defaultRuntime.acquire
		acquireFallbackBrowserSvc = managedRuntime.acquire
		relayPreferredSites := cfg.Browser.ExpandedRelayPreferredSites()
		lightpandaSvc := browser.NewLightpandaService(&cfg.Browser)
		lightpandaShimSvc = lightpandaSvc
		var lightpandaBinaryBackend *tools.LightpandaBinaryBrowserBackend
		if cfg.Browser.Lightpanda.Enabled {
			lightpandaBinaryRuntime := browser.NewLightpandaBinaryRuntime(&cfg.Browser)
			lightpandaBinaryBackend = tools.NewLightpandaBinaryBrowserBackend(lightpandaBinaryRuntime)
			lm.RegisterShutdownHook(func(ctx context.Context) error {
				return lightpandaBinaryRuntime.Stop(ctx)
			})
			go func() {
				path, err := lightpandaSvc.WarmBinary(context.Background())
				if err != nil {
					logger.Warn().Err(err).Msg("Lightpanda browser-lite binary is not ready; Blue will keep using the read-layer shim and Chromium fallback until it becomes available")
					return
				}
				if strings.TrimSpace(path) != "" {
					logger.Info().Str("binary_path", path).Msg("Lightpanda browser-lite binary is ready")
				}
			}()
		}
		browserBackend = tools.NewHybridCapabilityBrowserBackend(
			&cfg.Browser,
			lightpandaSvc,
			lightpandaBinaryBackend,
			managedRuntime.rodBackend,
			relayRuntime.rodBackend,
			func(rawURL string) bool {
				return browser.MatchSitePatternList(rawURL, relayPreferredSites)
			},
			func(ctx context.Context) bool {
				if ctx == nil {
					ctx = context.Background()
				}
				if strings.TrimSpace(cfg.Browser.CDPURL) != "" {
					probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
					defer cancel()
					return browser.ProbeCDPURL(probeCtx, cfg.Browser.CDPURL) == nil
				}
				if cfg.Browser.RelayEnabled {
					return relayInfoProvider().ExtensionConnected
				}
				return false
			},
		)
		if provider, ok := browserBackend.(browser.SessionRouteProvider); ok {
			browserHandler.SetSessionRouteProvider(provider)
		}

		syncBrowserMonitorRetention = func(retention time.Duration) {
			if retention < 0 {
				retention = 0
			}
			cfg.Browser.SessionScreenshotRetention = retention
			if browserHandler != nil {
				if svc, ok := browserHandler.PeekService().(*browser.RodService); ok && svc != nil {
					svc.SetSessionScreenshotRetention(retention)
				}
			}
			for _, updateRetention := range browserRuntimeRetentionUpdaters {
				updateRetention(retention)
			}
		}
		cleanupBrowserMonitorFrames = func() {
			seen := make(map[*browser.RodService]struct{})
			if browserHandler != nil {
				if svc, ok := browserHandler.PeekService().(*browser.RodService); ok && svc != nil {
					seen[svc] = struct{}{}
					svc.CleanupExpiredMonitorFrames()
				}
			}
			for _, peek := range browserRuntimePeekers {
				svc := peek()
				if svc == nil {
					continue
				}
				if _, ok := seen[svc]; ok {
					continue
				}
				seen[svc] = struct{}{}
				svc.CleanupExpiredMonitorFrames()
			}
		}
		syncBrowserMonitorRetention(initialBrowserMonitorRetention)

		for _, closeRuntime := range browserRuntimeClosers {
			closeFn := closeRuntime
			lm.RegisterShutdownHook(func(ctx context.Context) error {
				return closeFn(ctx)
			})
		}
	}

	// TTS/STT services are initialized lazily when chat page is opened
	// This avoids heavy initialization at startup
	logger.Info().Msg("TTS/STT services will be initialized on demand (when chat page is opened)")

	initPool.Go(func() {
		// Form filler store
		var err error
		formfillerStore, err = formfiller.NewStore(filepath.Join(dataDir, "formfiller"))
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize form filler store, form filler features will be disabled")
			return
		}
		formfillerHandler = formfiller.NewHandler(formfillerStore)
		logger.Info().Msg("Form filler handler initialized")
	})

	var companionHandler *companion.Handler
	var companionWSHandler *companion.WebSocketHandler

	if cfg.Companion.Enabled {
		// Companion service — eager init so chat handler can emit events immediately
		companionConfig := companion.DefaultConfig()
		companionConfig.Storage.BasePath = filepath.Join(dataDir, "companion")
		companionConfig.Retention.EventsDays = cfg.Companion.Retention.EventsDays
		companionConfig.Retention.SessionsDays = cfg.Companion.Retention.SessionsDays
		companionConfig.Retention.AlertsDays = cfg.Companion.Retention.AlertsDays
		companionStorage, err := companion.NewJSONLStorage(companionConfig.Storage.BasePath)
		if err != nil {
			logger.Warn().Err(err).Msg("Failed to initialize companion storage")
		}
		if companionStorage != nil {
			companionStreamer := companion.NewEventStreamer(companionConfig)
			if err := companionStreamer.Start(lm.Context()); err != nil {
				logger.Warn().Err(err).Msg("Failed to start companion streamer")
			}
			companionManager := companion.NewManager(companionStorage, companionStreamer, companionConfig)
			companionHandler = companion.NewHandler(companionManager, companionStorage)
			if syncBrowserMonitorRetention != nil {
				companionHandler.SetRetentionChangeHook(func(_ context.Context, retention companion.RetentionConfig) error {
					syncBrowserMonitorRetention(time.Duration(retention.SessionsDays) * 24 * time.Hour)
					return nil
				})
			}
			if syncBrowserMonitorRetention != nil || cleanupBrowserMonitorFrames != nil {
				companionHandler.SetCleanupHook(func(_ context.Context, retention companion.RetentionConfig) error {
					if syncBrowserMonitorRetention != nil {
						syncBrowserMonitorRetention(time.Duration(retention.SessionsDays) * 24 * time.Hour)
					}
					if cleanupBrowserMonitorFrames != nil {
						cleanupBrowserMonitorFrames()
					}
					return nil
				})
			}
			companionWSHandler = companion.NewWebSocketHandler(companionStreamer, companionConfig)
			chatHandler.SetCompanionManager(companionManager)
			lm.RegisterShutdownHook(func(ctx context.Context) error {
				return companionStreamer.Stop()
			})
			logger.Info().Msg("Companion service initialized")
		}
	} else {
		logger.Info().Msg("Companion service disabled by config")
	}

	// Wait for all parallel initializations to complete
	initPool.Wait()

	// Whisper ASR provider will be initialized lazily on first use
	// This significantly speeds up startup time
	whisperModelPath := filepath.Join(dataDir, "whisper-models")

	// Create a lazy-loading STT service that initializes Whisper on first use
	sttService = stt.NewLazyService(whisperModelPath, cfg.Performance.ResourceReclaim.STTIdleAfter)
	logger.Info().Msg("STT service configured for lazy initialization")

	// Register deferred cleanup for metrics services
	if metricsCollector != nil {
		defer metricsCollector.Stop()
	}
	if metricsWriter != nil {
		defer metricsWriter.Stop()
	}

	// Initialize voice service (depends on STT and TTS)
	// Create voice handler even if services are not initialized yet
	// The services will be initialized on demand when chat page is opened
	var voiceService voice.Service
	if sttService != nil && ttsService != nil {
		voiceService = voice.NewService(&voice.ServiceConfig{
			STTService: sttService,
			TTSService: ttsService,
		})
	} else {
		// Create a placeholder service that will work with lazy initialization
		voiceService = voice.NewService(&voice.ServiceConfig{
			STTService: sttService,
			TTSService: ttsService,
		})
	}
	voiceHandler = voice.NewHandler(voiceService)
	logger.Info().Msg("Voice handler initialized")

	// Initialize voice WebSocket handler (created here so shutdown can close connections)
	var voiceWSHandler *voice.WSHandler
	if voiceHandler != nil {
		voiceWSHandler = voice.NewWSHandler(voiceHandler.Service())
	}

	// Initialize ngrok config store (kvstore-based, lazy initialization)
	ngrokConfigStore = ngrok.NewConfigStore(configKV)
	ngrokTunnelMgr = ngrok.NewSDKTunnelManager(nil)
	logger.Info().Msg("Ngrok tunnel services initialized (lightweight)")

	// Note: Metrics recorder is set on chat handler asynchronously after metrics initialization completes

	// Initialize HTTP server
	srv := server.New(&cfg.Server)
	srv.Echo().Use(runtimeActivity.MutationMiddleware())
	server.SetVersion(version)
	srv.RegisterHealthRoutes()

	// Register API routes
	registerAPIRoutes(srv, pool, userHandler, extauthHandler, userService, chatHandler, autoreplyService, autoreplyHandler, metricsCollector, metricsWriter, authMiddleware, apiKeyHandler, apiKeyService, skillRegistry, pluginRegistry, pluginStore, backupHandler, toolRegistry, securityHandler, sandboxHandler, sandboxManager, cronHandler, browserHandler, workflowHandler, mfaHandler, voiceHandler, voiceWSHandler, formfillerHandler, companionHandler, companionWSHandler, ngrokTunnelMgr, ngrokConfigStore, zapLogger, version, buildTime, gitCommit, dataDir, cfg, llmRegistry, dbConn, db, dbReader, memoryStore, jwtService, permissionHandler, sttService, ttsService, a2uiManager, ocrService, pdfService, lm, hotReloader, sseBroker, pushIPC, pushSvc, cronIPC, browserBackend, lazyBrowserSvc, acquireBrowserSvc, acquireFallbackBrowserSvc, lightpandaShimSvc, configKV, configStore)

	// Register shutdown hook for server
	lm.RegisterShutdownHook(func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	// Store global server reference for dynamic TLS start
	server.SetGlobalServer(srv)

	// Register callback so cert generation/upload dynamically starts HTTPS
	security.OnCertReady(func() {
		srv.EnsureTLSStarted()
	})
	if cfg.Performance.ResourceReclaim.Enabled {
		server.OnServerStart(func(port int) {
			_ = port
			lm.Go(func(ctx context.Context) {
				bootstrap.RunStartupMemoryTrimLoop(ctx, zapLogger, bootstrap.StartupMemoryTrimSchedule()...)
			})
		})
		logger.Info().Interface("schedule", bootstrap.StartupMemoryTrimSchedule()).Msg("Startup memory trim scheduled")
	}

	// Start server in background
	lm.Go(func(ctx context.Context) {
		if err := srv.Start(); err != nil {
			logger.Error().Err(err).Msg("Server error")
		}
	})

	// Start HTTPS server if TLS is enabled and certificate is available
	tlsManager := security.GetGlobalTLSManager()
	if tlsManager != nil && tlsManager.GetCertificate() != nil {
		srv.EnsureTLSStarted()
	}

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	select {
	case sig := <-quit:
		restartController.Suppress()
		logger.Info().Str("signal", sig.String()).Msg("Received shutdown signal")
	case <-restartController.C():
		logger.Info().Msg("Received in-process restart request")
	case <-lm.Done():
		restartController.Suppress()
		logger.Info().Msg("Lifecycle manager done")
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Force-cancel on second signal — don't let the process hang
	go func() {
		select {
		case sig := <-quit:
			logger.Warn().Str("signal", sig.String()).Msg("Received second signal, cancelling graceful shutdown")
			cancel()
		case <-shutdownCtx.Done():
		}
	}()

	server.SetReady(false)

	// Close SSE broker first — this makes all SSE handlers return,
	// so httpServer.Shutdown() won't block waiting for long-lived connections.
	sseBroker.Close()

	// Close WebSocket connections so httpServer.Shutdown() doesn't block.
	if companionWSHandler != nil {
		companionWSHandler.Close()
	}
	if voiceWSHandler != nil {
		voiceWSHandler.Close()
	}

	if err := lm.Shutdown(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Shutdown error - some services may not have stopped cleanly")
		// Don't use os.Exit here - let deferred cleanup run
	}

	// Wait for worker pool
	if err := pool.Wait(); err != nil {
		logger.Warn().Err(err).Msg("Worker pool error during shutdown")
	}

	// Clean up cron service if it was initialized
	if cronHandler != nil {
		if svc := cronHandler.PeekService(); svc != nil {
			svc.Stop(shutdownCtx)
			logger.Info().Msg("Cron service stopped")
		}
	}

	// Clean up STT service (important for CGO resources like Whisper)
	if sttService != nil {
		_ = sttService.Close()
		logger.Info().Msg("STT service cleaned up")
	}

	// Clean up TTS service (important for CGO resources like eSpeak-NG)
	if ttsService != nil {
		ttsService.Close()
		logger.Info().Msg("TTS service cleaned up")
	}
	if pdfService != nil {
		_ = pdfService.Close()
		logger.Info().Msg("PDF service cleaned up")
	}

	// Clean up extracted web dist from tmpfs
	web.CleanupDist()

	if err := dbutil.MarkStartupIntegrityClean(dataDir); err != nil {
		logger.Warn().Err(err).Msg("Failed to mark startup integrity state clean")
		logger.Warn().Msg("Leaving startup integrity marker dirty so the next boot performs conservative database checks")
	}

	logger.Info().Msg("ZimaOS-Blue stopped")
	return serverRunOutcome{RestartRequested: restartController.Requested()}
}

func registerAPIRoutes(srv *server.Server, pool *worker.Pool, userHandler *user.Handler, extauthHandler *extauth.Handler, userService *user.Service, chatHandler *server.ChatHandler, autoreplyService *autoreply.Service, autoreplyHandler *autoreply.Handler, metricsCollector *metrics.Collector, metricsWriter *metrics.MetricsWriter, authMiddleware *auth.AuthMiddleware, apiKeyHandler *auth.APIKeyHandler, apiKeyService *auth.APIKeyService, skillRegistry *skill.Registry, pluginRegistry *plugin.Registry, pluginStore *plugin.Store, backupHandler *backup.Handler, toolRegistry *tools.Registry, securityHandler *security.Handler, sandboxHandler *sandbox.Handler, sandboxManager *sandbox.Manager, cronHandler *cron.Handler, browserHandler *browser.Handler, workflowHandler *workflow.Handler, mfaHandler *mfa.Handler, voiceHandler *voice.Handler, voiceWSHandler *voice.WSHandler, formfillerHandler *formfiller.Handler, companionHandler *companion.Handler, companionWSHandler *companion.WebSocketHandler, ngrokTunnelMgr *ngrok.SDKTunnelManager, ngrokConfigStore *ngrok.ConfigStore, zapLogger *zap.Logger, version, buildTime, gitCommit, dataDir string, cfg *config.Config, llmRegistry *llm.ProviderRegistry, dbConn *dbutil.SQLiteConn, db, dbReader *sql.DB, memoryStore *memory.Store, jwtService *auth.JWTService, permissionHandler *permission.Handler, sttService stt.Service, ttsService tts.Service, a2uiManager *a2ui.Manager, ocrService *ocrruntime.TesseractService, pdfService *pdfextract.Service, lm *lifecycle.Manager, hotReloader *config.HotReloader, sseBroker *ssePkg.Broker, pushIPC sockipc.PushBackend, pushSvc *push.Service, cronIPC sockipc.CronBackend, browserBackend tools.BrowserBackend, lazyBrowserSvc func() *browser.RodService, acquireBrowserSvc func() (*browser.RodService, func(), error), acquireFallbackBrowserSvc func() (*browser.RodService, func(), error), lightpandaShimSvc *browser.LightpandaService, configKV kvstore.Store, configStore *config.ConfigStore) {
	e := srv.Echo()
	logger := zapLogger

	// Initialize speech handler
	speechKV := configKV
	asrProvider := "whisper"
	if runtime.GOOS == "darwin" {
		asrProvider = "macos-native"
	} else if runtime.GOOS == "windows" {
		asrProvider = "windows-native"
	}
	speechService := speech.NewService(&speech.Config{
		TTS: speech.TTSConfig{Provider: "edge", Model: ""},
		ASR: speech.ASRConfig{Enabled: true, Provider: asrProvider, EditBeforeSend: true},
	}, sttService, ttsService)
	speechService.SetStatusPrewarmEnabled(!cfg.Performance.ResourceReclaim.SpeechStatusDoesNotPrewarmSTT)
	if ttsService != nil {
		if provider := ttsService.GetProvider(tts.ProviderEdge); provider != nil {
			speechService.SetTTSProvider(provider)
		}
	}
	speechHandler := speech.NewHandler(speechService, speechKV, dataDir)

	// Restore persisted settings from kvstore
	speechHandler.RestoreEditBeforeSend()

	// Restore persisted TTS provider from kvstore
	if ttsService != nil {
		if saved := speechHandler.GetPersistedTTSProvider(); saved != "" {
			if err := ttsService.SetDefaultProvider(tts.ProviderType(saved)); err != nil {
				logger.Warn("Failed to restore persisted TTS provider", zap.String("provider", saved), zap.Error(err))
			} else {
				if p := ttsService.GetProvider(tts.ProviderType(saved)); p != nil {
					speechService.SetTTSProvider(p)
				}
				logger.Info("Restored persisted TTS provider", zap.String("provider", saved))
			}
		}
	}
	// On macOS/Windows, initialize native STT in background — not needed for first request
	logger.Info("ASR provider setup", zap.String("goos", runtime.GOOS), zap.Bool("sttServiceNil", sttService == nil))
	go func() {
		if runtime.GOOS == "darwin" {
			logger.Info("macOS detected, initializing native STT...")
			macosSTT := speech.NewMacOSNativeSTT()
			if err := macosSTT.Initialize(); err == nil {
				speechService.SetASRProvider(macosSTT)
				// Also update voice service to use macOS native STT for /voice/transcribe
				if voiceHandler != nil {
					voiceHandler.Service().SetSTTService(stt.NewServiceFromProvider(macosSTT))
				}
				logger.Info("macOS native STT initialized OK",
					zap.String("providerType", string(macosSTT.Type())),
					zap.String("providerName", macosSTT.Name()))
			} else {
				logger.Warn("macOS native STT init failed, falling back to whisper", zap.Error(err))
				speechService.SetASRPermissionDenied(err.Error())
				if sttService != nil {
					if wp := sttService.PeekWhisperProvider(); wp != nil && wp.IsInitialized() {
						speechService.SetASRProvider(wp)
					}
				}
			}
		} else if runtime.GOOS == "windows" {
			logger.Info("Windows detected, initializing native ASR...")
			windowsASR := speech.NewWindowsNativeASR()
			if windowsASR != nil {
				speechService.SetASRProvider(windowsASR)
				// Also update voice service to use Windows native ASR for /voice/transcribe
				if voiceHandler != nil {
					voiceHandler.Service().SetSTTService(stt.NewServiceFromProvider(windowsASR))
				}
				logger.Info("Windows native ASR initialized OK",
					zap.String("providerType", string(windowsASR.Type())),
					zap.String("providerName", windowsASR.Name()))
			} else {
				logger.Warn("Windows native ASR init failed, falling back to whisper")
				if sttService != nil {
					if wp := sttService.PeekWhisperProvider(); wp != nil && wp.IsInitialized() {
						speechService.SetASRProvider(wp)
					}
				}
			}
		}
		// Log final ASR provider state without eagerly constructing Whisper on Linux.
		if runtime.GOOS == "linux" && sttService != nil {
			if wp := sttService.PeekWhisperProvider(); wp != nil {
				logger.Info("Final ASR provider", zap.String("type", string(wp.Type())), zap.String("name", wp.Name()))
			} else {
				logger.Info("ASR provider will remain lazily initialized on first speech use")
			}
			return
		}
		if p := speechService.GetASRProvider(); p != nil {
			logger.Info("Final ASR provider", zap.String("type", string(p.Type())), zap.String("name", p.Name()))
		} else {
			logger.Warn("No ASR provider configured")
		}
	}()

	// Initialize workspace (SOUL.md, USER.md, IDENTITY.md, etc.)
	workspaceMgr := workspace.NewManager(bootstrap.ResolveWorkspaceDir(dataDir, cfg))
	if err := workspaceMgr.EnsureWorkspace(); err != nil {
		logger.Warn("Failed to initialize workspace", zap.Error(err))
	}
	if err := workspaceMgr.ReleaseContextPacks(contextpackembed.PacksFS); err != nil {
		logger.Warn("Failed to release embedded context packs", zap.Error(err))
	}
	contextRegistry := contextpack.NewRegistry(workspaceMgr.ContextDir())
	if result, migrateErr := contextpack.MigrateLegacyAnnotations(context.Background(), db, dataDir); migrateErr != nil {
		logger.Warn("Failed to migrate legacy context annotation store", zap.Error(migrateErr))
	} else if result != nil {
		fields := []zap.Field{
			zap.String("source", result.SourcePath),
			zap.Int("rows_imported", result.RowsImported),
		}
		if result.ArchivedPath != "" {
			fields = append(fields, zap.String("archived_path", result.ArchivedPath))
		}
		logger.Info("Legacy context annotation store imported into blue.db", fields...)
	}
	contextAnnotationStoreFactory := func() (*contextpack.AnnotationStore, error) {
		return contextpack.NewAnnotationStoreWithDB(db)
	}
	if dbConn != nil {
		contextAnnotationStoreFactory = func() (*contextpack.AnnotationStore, error) {
			return contextpack.NewAnnotationStoreWithReadDB(dbConn.Writer, dbConn.Reader)
		}
	}
	contextAnnotationStore, err := contextAnnotationStoreFactory()
	if err != nil {
		logger.Warn("Failed to initialize context annotation store", zap.Error(err))
	}
	if contextAnnotationStore != nil {
		lm.RegisterShutdownHook(func(ctx context.Context) error {
			return contextAnnotationStore.Close()
		})
	}
	contextResolver := contextpack.NewResolver(contextRegistry, contextAnnotationStore, contextpack.ResolverConfig{MaxFiles: 3, MaxTokens: 1500, SearchLimit: 5})

	workspaceDir := workspaceMgr.Dir() // {dataDir}/workspace/
	systemPromptBuilder := agentcore.NewSystemPromptBuilder(&agentcore.Config{WorkspaceDir: workspaceDir})
	systemPromptBuilder.SetToolRegistry(toolRegistry)
	systemPromptBuilder.SetWorkspace(workspaceMgr)
	systemPromptBuilder.SetContextResolver(contextResolver)
	chatHandler.SetSystemPromptBuilder(systemPromptBuilder)

	// Initialize memory handler (markdown primary, optional dual-write with vector store)
	var memoryHandler *server.MemoryHandler
	memoryHandler = server.NewLazyMemoryHandler(func(h *server.MemoryHandler) error {
		memoryDir := cfg.Memory.MarkdownDir
		if memoryDir == "" {
			memoryDir = workspaceMgr.MemoryDir()
		}
		mdBackend, err := memory.NewPureMarkdownBackend(memoryDir)
		if err != nil {
			logger.Warn("Failed to initialize markdown backend", zap.Error(err))
			return err
		}
		unifiedService := memory.NewUnifiedMemoryService(mdBackend)
		h.SetUnifiedService(unifiedService)

		// Try to set up dual-write backend with vector store + hybrid search
		if cfg.Memory.VectorStore.Enabled {
			if dualBackend := initDualWriteBackend(cfg, dataDir, mdBackend); dualBackend != nil {
				unifiedService.SetBackend(dualBackend)
				logger.Info("Memory service initialized (dual-write: markdown + vector store)")
			} else {
				logger.Info("Memory service initialized (markdown-only, vector store init failed)")
			}
		} else {
			logger.Info("Memory service initialized (markdown backend)", zap.String("dir", memoryDir))
		}

		layeredService, err := memory.NewLayeredMemoryService(unifiedService, memory.LayeredMemoryConfig{
			BaseDir:            memoryDir,
			LongTermDir:        workspaceMgr.Dir(),
			DailyRetentionDays: 30,
		})
		if err != nil {
			logger.Warn("Failed to initialize layered memory service", zap.Error(err))
		} else {
			h.SetLayeredService(layeredService)
		}
		toolsAdapter := memory.NewToolsAdapter(unifiedService)
		tools.RegisterMemoryTools(toolRegistry, toolsAdapter)

		return nil
	})

	// Initialize channel config store
	channelConfigStore := server.NewChannelConfigStore(configKV)

	// Initialize provider pool (SQLite-backed, auto-migrates from JSON files)
	var providerPool *providerpool.Pool
	providerPoolPath := filepath.Join(dataDir, "providerpool")
	ppOpts := []providerpool.PoolOption{
		providerpool.WithDB(db),
		providerpool.WithReadDB(dbReader),
	}
	if cfg.Security.Encryption.Enabled {
		secretEncryptor, encErr := auth.NewEncryptor(&auth.EncryptionConfig{
			KeyPath:    cfg.Security.Encryption.KeyPath,
			Passphrase: cfg.Security.Encryption.Passphrase,
		})
		if encErr != nil {
			logger.Warn("Failed to initialize provider pool secret encryption", zap.Error(encErr))
		} else {
			ppOpts = append(ppOpts, providerpool.WithSecretEncryptor(secretEncryptor))
		}
	}
	providerPool, ppErr := providerpool.NewPool(providerPoolPath, ppOpts...)
	if ppErr != nil {
		logger.Warn("Failed to initialize provider pool", zap.Error(ppErr))
		providerPool = nil
	}

	if providerPool != nil {
		// Stop provider pool background goroutines (health checks, usage tracker) on shutdown
		pp := providerPool
		lm.RegisterShutdownHook(func(ctx context.Context) error {
			pp.Stop()
			return nil
		})

		bootstrap.LoadProvidersFromPool(providerPool, llmRegistry)
		chatHandler.SetProviderPool(providerPool)

		if providerpool.CheckMigrationNeeded(dataDir) {
			result, err := providerpool.MigrateFromLegacy(dataDir, providerPool)
			if err != nil {
				logger.Warn("Failed to migrate legacy provider settings", zap.Error(err))
			} else if result.Migrated > 0 {
				logger.Info("Legacy provider settings migrated", zap.Int("migrated", result.Migrated))
			}
		}

	}

	// Call bootstrap.RegisterAllRoutes with all dependencies
	routeUserRepo, _ := user.NewSQLiteRepositoryWithReadDB(db, dbReader)

	// Initialize gateway runtime + HTTP handler; method handlers are wired in bootstrap routes.
	gatewayRuntime := gateway.NewGateway(gateway.DefaultConfig(), zapLogger)
	gatewayHandler := gateway.NewHandlerWithAuth(gatewayRuntime, zapLogger, jwtService)

	// Create IPC adapters for browser, UI reviewer, and push notification SKILLs
	browserIPC := sockipc.NewToolBrowserIPCAdapter(browserBackend)
	uiReviewerIPC := sockipc.NewUIReviewIPCAdapter(&tools.UIReviewerTool{}) // placeholder, routes.go creates the real one

	deps := &bootstrap.RoutesDeps{
		DB:                 db,
		Config:             cfg,
		DisablePromptGuard: disableIntercepts,
		ServerConfig: &bootstrap.ServerConfig{
			Version:   version,
			BuildTime: buildTime,
			GitCommit: gitCommit,
			DataDir:   dataDir,
			Port:      cfg.Server.Port,
		},
		Services: &bootstrap.Services{
			DB:            db,
			DBConn:        dbConn,
			UserService:   userService,
			UserRepo:      routeUserRepo,
			JWTService:    jwtService,
			MemoryStore:   memoryStore,
			SkillRegistry: skillRegistry,
			ToolRegistry:  toolRegistry,
			WorkerPool:    pool,
			A2UIManager:   a2uiManager,
			OCRService:    ocrService,
			PDFService:    pdfService,
		},
		Logger:           zapLogger,
		Ctx:              lm.Context(),
		MetricsWriter:    metricsWriter,
		MetricsCollector: metricsCollector,
		ChatHandler:      chatHandler,
		PluginRegistry:   pluginRegistry,
		PluginStore:      pluginStore,
		ExtauthHandler:   extauthHandler,
		AutoreplyService: autoreplyService,
		AutoreplyHandler: autoreplyHandler,
		AuthMiddleware:   authMiddleware,
		APIKeyHandler:    apiKeyHandler,
		UserHandler:      userHandler,
		BackupHandler:    backupHandler,
		SecurityHandler:  securityHandler,
		SandboxHandler:   sandboxHandler,
		CronHandler:      cronHandler,
		BrowserHandler:   browserHandler,
		BrowserIPC:       browserIPC,
		UIReviewerIPC:    uiReviewerIPC,
		PushIPC:          pushIPC,
		PushService:      pushSvc,
		CronIPC:          cronIPC,
		RegisterIPCExtensions: func(ipcSrv *sockipc.Server) {
			registerCLIIPCHandlers(ipcSrv, workspaceMgr, contextRegistry, contextAnnotationStore, cfg, configStore, zapLogger)
		},
		WorkflowHandler:    workflowHandler,
		VoiceHandler:       voiceHandler,
		VoiceWSHandler:     voiceWSHandler,
		FormfillerHandler:  formfillerHandler,
		CompanionHandler:   companionHandler,
		CompanionWSHandler: companionWSHandler,
		ProviderPool:       providerPool,
		APIKeyService:      apiKeyService,
		SpeechHandler:      speechHandler,
		STTService:         sttService,
		NgrokTunnelMgr:     ngrokTunnelMgr,
		NgrokConfigStore:   ngrokConfigStore,
		MemoryHandler:      memoryHandler,
		ChannelConfigStore: channelConfigStore,
		ConfigKV:           configKV,
		ConfigStore:        configStore,
		HotReloader:        hotReloader,
		WorkspaceHandler:   workspace.NewHandler(workspaceMgr),
		SSEBroker:          sseBroker,
		Gateway:            gatewayRuntime,
		GatewayHandler:     gatewayHandler,
		// Consolidated init deps
		SkillEmbedFS:              skillEmbed.SkillsFS,
		SandboxManager:            sandboxManager,
		SystemPromptBuilder:       systemPromptBuilder,
		LazyBrowserSvc:            lazyBrowserSvc,
		AcquireBrowserSvc:         acquireBrowserSvc,
		AcquireFallbackBrowserSvc: acquireFallbackBrowserSvc,
		LightpandaShimSvc:         lightpandaShimSvc,
		BrowserBackend:            browserBackend,
	}

	_ = bootstrap.RegisterAllRoutes(e, deps)

	// Register shutdown hooks for closers started during route registration (e.g., sockipc)
	for _, c := range deps.Closers {
		closer := c // capture for closure
		lm.RegisterShutdownHook(func(ctx context.Context) error {
			return closer.Close()
		})
	}
}

func resolveVectorStoreDBPath(dataDir, configuredPath string) string {
	return embedding.ResolveVectorStoreDBPath(dataDir, configuredPath)
}

// initDualWriteBackend creates a DualWriteBackend with VectorStore + HybridSearcher.
// Returns nil if initialization fails (caller should fall back to markdown-only).
func initDualWriteBackend(cfg *config.Config, dataDir string, mdBackend *memory.PureMarkdownBackend) *memory.DualWriteBackend {
	log := logger.Get()

	dbPath := resolveVectorStoreDBPath(dataDir, cfg.Memory.VectorStore.DBPath)

	dims := cfg.Memory.VectorStore.Dimensions

	// Create cybertron embedding provider (lazy — model downloads on first use)
	embCfg := cfg.Embedding
	modelsDir := embedding.PrepareSharedModelCache(dataDir, cfg.Memory.VectorStore.DBPath, embCfg.Model)
	model := embCfg.Model
	if model == "" {
		model = embedding.DefaultCybertronModel
	}
	embProvider := embedding.NewCybertronProvider(embedding.CybertronConfig{
		ModelsDir:  modelsDir,
		Model:      model,
		Dimensions: embCfg.Dimensions,
	})

	// Use configured dims or fallback; actual dims resolved lazily on first embed
	if dims <= 0 {
		if embCfg.Dimensions > 0 {
			dims = embCfg.Dimensions
		} else {
			dims = 512 // BGE-small-zh default
		}
	}

	log.Info().
		Str("provider", embProvider.Name()).
		Str("model", embProvider.Model()).
		Msg("Embedding provider configured (lazy load)")

	// Create VectorStore (after embedding provider so dims are known)
	vs, err := memory.NewVectorStore(memory.VectorStoreConfig{
		DBPath:       dbPath,
		EmbeddingDim: dims,
		MaxChunks:    10000,
		EnableFTS:    true,
	})
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize vector store")
		return nil
	}

	// Create HybridSearcher → MemoryService → DualWriteBackend
	searcher := memory.NewHybridSearcher(vs, embProvider, cfg.Memory)
	memSvc := memory.NewMemoryService(searcher)
	return memory.NewDualWriteBackend(mdBackend, memSvc)
}
