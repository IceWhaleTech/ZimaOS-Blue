package bootstrap

import (
	"path/filepath"
	"strings"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
)

func (binding *runtimeContractBinding) BindMediaRuntime(
	options routeRuntimeContractMediaOptions,
) routeRuntimeContractMediaResult {
	if binding == nil {
		return routeRuntimeContractMediaResult{}
	}
	return bindRouteRuntimeMedia(options)
}

func bindRouteRuntimeMedia(options routeRuntimeContractMediaOptions) routeRuntimeContractMediaResult {
	if options.deps == nil || options.deps.Services == nil {
		return routeRuntimeContractMediaResult{}
	}

	deps := options.deps
	services := deps.Services
	logger := options.logger
	if logger == nil {
		logger = deps.Logger
	}
	dataDir := strings.TrimSpace(options.dataDir)
	if dataDir == "" && deps.ServerConfig != nil {
		dataDir = deps.ServerConfig.DataDir
	}
	workspaceDir := ResolveWorkspaceDir(dataDir, deps.Config)

	mediaGenDir := filepath.Join(dataDir, "media", "generated")
	mediaStorage := mediagen.NewMediaStorage(mediaGenDir, "/api/media/generated")
	deps.MediaStorage = mediaStorage
	if err := mediaStorage.EnsureDirs(); err != nil && logger != nil {
		logger.Warn("Failed to create media generation dirs", zap.Error(err))
	}

	mediaConfigStore := newRouteRuntimeMediaConfigStore(deps, dataDir, logger)
	mediagen.MigrateFromProviderPool(filepath.Join(dataDir, "providerpool"), mediaConfigStore)

	locale := readLocaleFromKV(deps.ConfigKV)
	mediaManager := mediagen.NewManager(mediaStorage, mediaConfigStore, locale)
	mediaManager.InitConfigs()
	bindRouteRuntimeMediaFallback(mediaManager, mediaStorage, dataDir, deps, services, locale)
	deps.MediaManager = mediaManager

	taskStore, taskStoreErr := newRouteRuntimeMediaTaskStore(services)
	if taskStoreErr != nil {
		if logger != nil {
			logger.Warn("Failed to initialize media task store", zap.Error(taskStoreErr))
		}
	} else {
		mediaManager.SetTaskStore(taskStore)
		mediaManager.RecoverTasks()
	}
	deps.Closers = append(deps.Closers, mediaManager)

	channelWatcher := mediagen.NewChannelTaskWatcher(mediaManager, nil, locale)
	channelWatcher.RecoverChannelTasks()
	deps.ChannelTaskWatcher = channelWatcher
	deps.Closers = append(deps.Closers, channelWatcher)

	bindRouteRuntimeMediaWebPush(options.v1, deps, mediaManager, locale, logger, services, options.requirePagePermission)

	if deps.SSEBroker != nil {
		mediaManager.SetEventPublisher(deps.SSEBroker)
	}
	if deps.ChatHandler != nil {
		deps.ChatHandler.SetMediaInterceptor(mediagen.NewInterceptor(mediaManager, channelWatcher))
	}

	mediaHandler := mediagen.NewHandler(mediaManager, mediaStorage, locale)
	configureRouteRuntimeMediaPersistence(mediaHandler, deps, services)

	if options.v1 != nil {
		mediaGroup := options.v1.Group("/media")
		if deps.AuthMiddleware != nil {
			mediaGroup.Use(deps.AuthMiddleware.OptionalAuthenticate())
		}
		mediaHandler.RegisterRoutes(mediaGroup)
	}
	if options.e != nil {
		mediaHandler.RegisterStorageRoutes(options.e)
	}
	if logger != nil {
		logger.Info("Media generation routes registered")
	}

	result := routeRuntimeContractMediaResult{}
	sockPath := resolveIPCSocketPath(dataDir)
	ipcSrv := sockipc.NewServer(sockPath, logger)
	registerRouteRuntimeMediaIPC(ipcSrv, deps, services, mediaManager, workspaceDir, logger)
	if err := ipcSrv.Start(); err != nil {
		if logger != nil {
			logger.Warn("Failed to start sockipc server", zap.Error(err))
		}
		return result
	}

	deps.Closers = append(deps.Closers, ipcSrv)
	if logger != nil {
		logger.Info("Socket IPC server started", zap.String("path", sockPath))
	}
	result.ipcServer = ipcSrv
	return result
}
