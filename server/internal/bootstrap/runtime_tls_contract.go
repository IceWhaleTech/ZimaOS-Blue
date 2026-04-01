package bootstrap

import (
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
)

type routeRuntimeContractTLSOptions struct {
	e        *echo.Echo
	config   *config.Config
	dataDir  string
	configKV kvstore.Store
	logger   *zap.Logger
}

func (binding *runtimeContractBinding) BindTLSRuntime(options routeRuntimeContractTLSOptions) {
	if binding == nil {
		return
	}
	bindRouteRuntimeTLS(options)
}

func bindRouteRuntimeTLS(options routeRuntimeContractTLSOptions) {
	if options.e == nil || options.config == nil {
		return
	}

	certsDir := filepath.Join(options.dataDir, "certs")
	acmeDir := strings.TrimSpace(options.config.Server.TLS.ACMEDir)
	if acmeDir == "" {
		acmeDir = filepath.Join(certsDir, "acme")
	}
	security.SetGlobalTLSManagerConfig(&security.TLSManagerConfig{
		CertFile:     filepath.Join(certsDir, "server.crt"),
		KeyFile:      filepath.Join(certsDir, "server.key"),
		ACMEDir:      acmeDir,
		SelfSigned:   options.config.Server.TLS.SelfSigned,
		ACMEEmail:    options.config.Server.TLS.ACMEEmail,
		ACMEDomains:  strings.Split(options.config.Server.TLS.ACMEDomains, ","),
		ACMEProvider: options.config.Server.TLS.ACMEProvider,
		AutoCert:     options.config.Server.TLS.AutoCert,
		HTTPSOnly:    options.config.Server.TLS.Enabled,
		HTTPSPort:    options.config.Server.TLS.Port,
	})

	tlsManager := security.GetGlobalTLSManager()
	if tlsManager == nil {
		return
	}

	if options.configKV != nil {
		tlsManager.SetKVStore(options.configKV)
	}
	if err := tlsManager.LoadSettings(); err != nil && options.logger != nil {
		options.logger.Warn("Failed to load persisted TLS settings", zap.Error(err))
	}
	if err := tlsManager.LoadCertificate(); err != nil {
		if options.logger != nil {
			options.logger.Debug("No existing TLS certificate found", zap.Error(err))
		}
	} else if options.logger != nil {
		options.logger.Info("TLS certificate loaded from disk")
	}

	if options.config.Server.TLS.AutoCert &&
		strings.TrimSpace(options.config.Server.TLS.ACMEEmail) != "" &&
		strings.TrimSpace(options.config.Server.TLS.ACMEDomains) != "" {
		domains := splitRouteRuntimeDomains(options.config.Server.TLS.ACMEDomains)
		if err := tlsManager.RequestACMECertificate(&security.ACMEConfig{
			Email:    options.config.Server.TLS.ACMEEmail,
			Domains:  domains,
			Provider: options.config.Server.TLS.ACMEProvider,
			CacheDir: acmeDir,
		}); err != nil {
			if options.logger != nil {
				options.logger.Warn("Failed to initialize ACME auto-renewal", zap.Error(err))
			}
		} else if options.logger != nil {
			options.logger.Info("ACME auto-renewal initialized", zap.Strings("domains", domains))
		}
	}

	options.e.Use(tlsManager.HTTPSRedirectMiddleware())
}

func splitRouteRuntimeDomains(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
