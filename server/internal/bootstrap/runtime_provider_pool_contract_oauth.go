package bootstrap

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool/oauth"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"go.uber.org/zap"
)

func bindRouteRuntimeProviderPoolOAuth(
	options routeRuntimeContractProviderPoolOptions,
	handler *providerpool.Handler,
) *oauth.LazyManager {
	if handler == nil {
		return nil
	}

	oauthManager := oauth.NewLazyManager(func() (*oauth.Manager, error) {
		oauthStore, err := newRouteRuntimeOAuthStore(options.writeDB, options.readDB)
		if err != nil {
			if options.logger != nil {
				options.logger.Warn("Failed to initialize OAuth store", zap.Error(err))
			}
			return nil, err
		}

		legacyPath := filepath.Join(options.dataDir, "providers", "providers", "oauth_tokens.json")
		if err := oauthStore.MigrateFromJSON(legacyPath); err != nil {
			if options.logger != nil {
				options.logger.Warn("Failed to migrate legacy OAuth tokens", zap.Error(err))
			}
		} else {
			_ = os.Remove(legacyPath)
		}
		if err := oauthStore.Deduplicate(); err != nil && options.logger != nil {
			options.logger.Warn("Failed to deduplicate OAuth tokens", zap.Error(err))
		}
		return oauth.NewManager(oauthStore), nil
	})
	oauthManager.OnReady(func(*oauth.Manager) {
		if options.logger != nil {
			options.logger.Info("OAuth manager initialized for provider pool")
		}
	})
	handler.SetOAuthManager(oauthManager)
	if options.trace != nil {
		options.trace.Mark("provider_pool_oauth_manager_wired")
	}

	if options.e != nil {
		handler.RegisterOAuthCallbackRoute(options.e, oauth.AllProviderConfigs())
		if options.trace != nil {
			options.trace.Mark("provider_pool_oauth_callbacks_registered")
		}
	}

	serverpkg.OnServerStart(func(port int) {
		if port > 0 {
			oauthManager.SetPort(port)
			if options.logger != nil {
				options.logger.Info("OAuth redirect port set to server port", zap.Int("port", port))
			}
		}
	})
	serverpkg.OnServerStart(func(int) {
		oauthManager.StartAutoRefresh(context.Background(), 5*time.Minute)
	})
	if options.logger != nil {
		options.logger.Info("OAuth manager scheduled for background initialization")
	}
	if options.trace != nil {
		options.trace.Mark("provider_pool_oauth_background_scheduled")
	}

	return oauthManager
}

func newRouteRuntimeOAuthStore(writeDB, readDB *sql.DB) (*oauth.SQLiteTokenStore, error) {
	if readDB == nil {
		readDB = writeDB
	}
	return oauth.NewSQLiteTokenStoreWithReadDB(writeDB, readDB)
}
