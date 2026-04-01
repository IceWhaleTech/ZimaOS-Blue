package bootstrap

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/videogen"
)

func newRouteRuntimeMediaConfigStore(
	deps *RoutesDeps,
	dataDir string,
	logger *zap.Logger,
) mediagen.MediaConfigStore {
	if deps == nil || deps.Services == nil {
		return mediagen.NewConfigStore(filepath.Join(dataDir, "media"))
	}

	services := deps.Services
	sqliteMediaStoreFactory := func() (*mediagen.SQLiteConfigStore, error) {
		return mediagen.NewSQLiteConfigStore(services.DB)
	}
	if services.DBConn != nil {
		sqliteMediaStoreFactory = func() (*mediagen.SQLiteConfigStore, error) {
			return mediagen.NewSQLiteConfigStoreWithReadDB(services.DBConn.Writer, services.DBConn.Reader)
		}
	}

	sqliteMediaStore, err := sqliteMediaStoreFactory()
	if err != nil {
		if logger != nil {
			logger.Warn("Failed to create SQLite media config store, falling back to JSON", zap.Error(err))
		}
		return mediagen.NewConfigStore(filepath.Join(dataDir, "media"))
	}
	if migrateErr := sqliteMediaStore.MigrateFromJSON(filepath.Join(dataDir, "media")); migrateErr != nil && logger != nil {
		logger.Warn("Failed to migrate media config from JSON", zap.Error(migrateErr))
	}
	return sqliteMediaStore
}

func bindRouteRuntimeMediaFallback(
	mediaManager *mediagen.Manager,
	mediaStorage *mediagen.MediaStorage,
	dataDir string,
	deps *RoutesDeps,
	services *Services,
	locale string,
) {
	if mediaManager == nil || deps == nil || deps.Config == nil {
		return
	}
	if deps.AcquireBrowserSvc == nil && deps.LazyBrowserSvc == nil && deps.AcquireFallbackBrowserSvc == nil {
		return
	}

	fallbackCfg := deps.Config.Media.Fallback
	webSearchCfg := buildWebSearchConfig(deps.Config)
	webSearchCfg.Provider = fallbackFirstProvider(fallbackCfg.SearchProviderChain)
	webSearchCfg.Providers = append([]string(nil), fallbackCfg.SearchProviderChain...)
	webSearchCfg.MaxResults = fallbackCfg.SearchMaxResults
	webSearchTool := tools.NewWebSearchTool(webSearchCfg)

	renderPort := serverpkg.GetActualPort()
	if renderPort == 0 && deps.ServerConfig != nil {
		renderPort = deps.ServerConfig.Port
	}

	browserFactory := func() mediagen.FallbackBrowserService {
		switch {
		case deps.AcquireFallbackBrowserSvc != nil:
			return newLeaseAwareFallbackBrowserAdapter(deps.AcquireFallbackBrowserSvc)
		case deps.AcquireBrowserSvc != nil:
			return newLeaseAwareFallbackBrowserAdapter(deps.AcquireBrowserSvc)
		case deps.LazyBrowserSvc != nil:
			return deps.LazyBrowserSvc()
		default:
			return nil
		}
	}

	fallbackEngine := mediagen.NewFallbackEngine(mediagen.FallbackConfig{
		Enabled:             fallbackCfg.Enabled,
		SearchProviderChain: append([]string(nil), fallbackCfg.SearchProviderChain...),
		SearchMaxResults:    fallbackCfg.SearchMaxResults,
		ScreenshotWidth:     fallbackCfg.ScreenshotWidth,
		ScreenshotHeight:    fallbackCfg.ScreenshotHeight,
		ComplexPromptChars:  fallbackCfg.ComplexPromptChars,
		RenderBaseURL:       fmt.Sprintf("http://127.0.0.1:%d", renderPort),
		DataDir:             dataDir,
		U2NetPStatusURL:     "/api/v1/media/fallback/models/u2netp/status",
		NativeVideo: mediagen.FallbackNativeVideoConfig{
			Enabled:            fallbackCfg.NativeVideo.Enabled,
			FPS:                fallbackCfg.NativeVideo.FPS,
			DefaultDurationSec: fallbackCfg.NativeVideo.DefaultDurationSec,
			MaxDurationSec:     fallbackCfg.NativeVideo.MaxDurationSec,
			PollInterval:       fallbackCfg.NativeVideo.PollInterval,
			StallTimeout:       fallbackCfg.NativeVideo.StallTimeout,
			MaxRuntime:         fallbackCfg.NativeVideo.MaxRuntime,
			HelperPath:         fallbackCfg.NativeVideo.HelperPath,
			AudioMode:          fallbackCfg.NativeVideo.AudioMode,
		},
		PublicSpaces: convertFallbackPublicSpaces(fallbackCfg.PublicSpaces),
	}, mediaStorage, mediagen.NewToolWebSearcher(webSearchTool), browserFactory, locale)
	fallbackEngine.SetScenePlannerLLM(newProviderRegistryLLMCaller(services.LLMRegistry))
	if deps.SpeechHandler != nil {
		fallbackEngine.SetTTSService(deps.SpeechHandler.Service().GetTTSService())
	}
	fallbackEngine.SetNativeVideoGenerator(videogen.NewHelperRunner(fallbackCfg.NativeVideo.HelperPath))
	mediaManager.SetFallbackEngine(fallbackEngine)
}

func newRouteRuntimeMediaTaskStore(services *Services) (*mediagen.TaskStore, error) {
	if services == nil {
		return nil, fmt.Errorf("services unavailable")
	}
	taskStoreFactory := mediagen.NewTaskStore
	if services.DBConn != nil {
		taskStoreFactory = func(db *sql.DB) (*mediagen.TaskStore, error) {
			return mediagen.NewTaskStoreWithReadDB(services.DBConn.Writer, services.DBConn.Reader)
		}
	}
	taskStore, err := taskStoreFactory(services.DB)
	if err != nil {
		return nil, err
	}
	return taskStore, nil
}
