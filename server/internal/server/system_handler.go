package server

import (
	"encoding/pem"
	"net/http"
	"runtime"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/sync/singleflight"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cache"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/logger"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/security"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sysinfo"
)

// SystemHandler handles system-related API endpoints
type SystemHandler struct {
	version      string
	buildTime    string
	gitCommit    string
	dataDir      string
	serverConfig *config.ServerConfig

	// singleflight for deduplicating concurrent requests
	sfGroup singleflight.Group

	// cache for expensive system info collection
	sysInfoCache *cache.GenericCache[string]
}

// NewSystemHandler creates a new system handler
func NewSystemHandler(version, buildTime, gitCommit, dataDir string) *SystemHandler {
	return &SystemHandler{
		version:   version,
		buildTime: buildTime,
		gitCommit: gitCommit,
		dataDir:   dataDir,
		sysInfoCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    10,
			DefaultTTL: 5 * time.Second, // System info can change, short TTL
		}, "system_info"),
	}
}

// NewSystemHandlerWithConfig creates a new system handler with server config
func NewSystemHandlerWithConfig(version, buildTime, gitCommit, dataDir string, serverConfig *config.ServerConfig) *SystemHandler {
	return &SystemHandler{
		version:      version,
		buildTime:    buildTime,
		gitCommit:    gitCommit,
		dataDir:      dataDir,
		serverConfig: serverConfig,
		sysInfoCache: cache.NewGenericCacheWithStats(cache.Config{
			MaxSize:    10,
			DefaultTTL: 5 * time.Second, // System info can change, short TTL
		}, "system_info_detailed"),
	}
}

// RegisterRoutes registers system routes on the given group
func (h *SystemHandler) RegisterRoutes(g *echo.Group) {
	g.GET("/system/info", h.GetInfo)
	g.GET("/system/logs", h.GetLogs)
	g.POST("/system/logs", h.WriteLog)
	g.GET("/system/config", h.GetConfig)
	g.PUT("/system/config", h.UpdateConfig)
	g.POST("/system/restart", h.RestartService)
	g.GET("/system/certificate", h.GetCertificate)
}

// SystemInfo represents system information
type SystemInfo struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	BuildTime string `json:"build_time"`
	GitCommit string `json:"git_commit"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	// Extended system information
	System *sysinfo.Info `json:"system,omitempty"`
}

// GetInfo returns system information
func (h *SystemHandler) GetInfo(c echo.Context) error {
	// Check if detailed info is requested
	detailed := c.QueryParam("detailed") == "true"

	info := SystemInfo{
		Version:   h.version,
		GoVersion: runtime.Version(),
		BuildTime: h.buildTime,
		GitCommit: h.gitCommit,
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	if detailed {
		// Try cache first for expensive sysinfo.Collect()
		cacheKey := "detailed_sysinfo"
		if cached, ok := h.sysInfoCache.Get(cacheKey); ok {
			if sysInfo, ok := cached.(*sysinfo.Info); ok {
				info.System = sysInfo
				return c.JSON(http.StatusOK, info)
			}
		}

		// Use singleflight to deduplicate concurrent requests
		result, _, _ := h.sfGroup.Do("collect_sysinfo", func() (interface{}, error) {
			sysInfo := sysinfo.Collect()
			h.sysInfoCache.Put(cacheKey, sysInfo)
			return sysInfo, nil
		})

		if result != nil {
			info.System = result.(*sysinfo.Info)
		}
	}

	return c.JSON(http.StatusOK, info)
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp string                 `json:"timestamp"`
	Level     string                 `json:"level"`
	Message   string                 `json:"message"`
	Source    string                 `json:"source,omitempty"`
	Fields    map[string]interface{} `json:"fields,omitempty"`
}

// LogsResponse represents the response for log queries
type LogsResponse struct {
	Entries    []LogEntry `json:"entries"`
	TotalCount int        `json:"total_count"`
	Offset     int        `json:"offset"`
	Limit      int        `json:"limit"`
}

// GetLogs returns recent log entries from the in-memory ring buffer
func (h *SystemHandler) GetLogs(c echo.Context) error {
	// Parse query parameters
	level := c.QueryParam("level")
	search := c.QueryParam("search")
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")
	startTimeStr := c.QueryParam("start_time")
	endTimeStr := c.QueryParam("end_time")

	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Parse time filters
	var startTime, endTime *time.Time
	if startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		}
	}
	if endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &t
		}
	}

	// Get log buffer
	buffer := logger.GetBuffer()
	if buffer == nil {
		return c.JSON(http.StatusOK, LogsResponse{
			Entries:    []LogEntry{},
			TotalCount: 0,
			Offset:     offset,
			Limit:      limit,
		})
	}

	// Query logs from buffer
	bufferEntries := buffer.Query(level, search, limit, offset, startTime, endTime)

	// Convert to response format
	entries := make([]LogEntry, len(bufferEntries))
	for i, e := range bufferEntries {
		entries[i] = LogEntry{
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Level:     e.Level,
			Message:   e.Message,
			Source:    e.Caller,
			Fields:    e.Fields,
		}
	}

	return c.JSON(http.StatusOK, entries)
}

// WriteLogRequest represents a client log write request
type WriteLogRequest struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	Source  string `json:"source"`
}

// WriteLog writes a client-side log entry
func (h *SystemHandler) WriteLog(c echo.Context) error {
	var req WriteLogRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Log based on level
	source := req.Source
	if source == "" {
		source = "web-client"
	}

	switch req.Level {
	case "error":
		logger.Error().Str("source", source).Msg(req.Message)
	case "warn":
		logger.Warn().Str("source", source).Msg(req.Message)
	default:
		logger.Info().Str("source", source).Msg(req.Message)
	}

	return c.JSON(http.StatusOK, map[string]bool{"success": true})
}

// ConfigResponse represents the config response
type ConfigResponse struct {
	Server   ServerConfigResponse   `json:"server"`
	Log      LogConfigResponse      `json:"log"`
	Security SecurityConfigResponse `json:"security"`
}

// ServerConfigResponse represents server config
type ServerConfigResponse struct {
	Host             string `json:"host"`
	Port             int    `json:"port"`
	ActualPort       int    `json:"actual_port"`
	PortAutoFallback bool   `json:"port_auto_fallback"`
}

// LogConfigResponse represents log config
type LogConfigResponse struct {
	Level  string `json:"level"`
	Format string `json:"format"`
}

// SecurityConfigResponse represents security config (safe fields only)
type SecurityConfigResponse struct {
	JWTExpiration string `json:"jwt_expiration"`
}

// GetConfig returns the current configuration (safe fields only)
func (h *SystemHandler) GetConfig(c echo.Context) error {
	// Get configured values or defaults
	host := "0.0.0.0"
	port := 23456
	portAutoFallback := true

	if h.serverConfig != nil {
		host = h.serverConfig.Host
		port = h.serverConfig.Port
		portAutoFallback = h.serverConfig.PortAutoFallback
	}

	return c.JSON(http.StatusOK, ConfigResponse{
		Server: ServerConfigResponse{
			Host:             host,
			Port:             port,
			ActualPort:       GetActualPort(),
			PortAutoFallback: portAutoFallback,
		},
		Log: LogConfigResponse{
			Level:  "info",
			Format: "json",
		},
		Security: SecurityConfigResponse{
			JWTExpiration: "24h",
		},
	})
}

// UpdateConfig updates the configuration
func (h *SystemHandler) UpdateConfig(c echo.Context) error {
	// Configuration updates are not supported via API for security reasons
	// Use the config file and hot-reload instead
	return c.JSON(http.StatusMethodNotAllowed, map[string]interface{}{
		"success": false,
		"message": "Configuration updates via API are not supported. Please modify the config file and use /api/v1/config/reload to apply changes.",
	})
}

// RestartService triggers a service restart
func (h *SystemHandler) RestartService(c echo.Context) error {
	// Service restart is not supported via API for security reasons
	// Use systemctl or the container orchestrator instead
	return c.JSON(http.StatusMethodNotAllowed, map[string]interface{}{
		"success": false,
		"message": "Service restart via API is not supported for security reasons. Please use systemctl or your container orchestrator.",
	})
}

// CertificateResponse represents the certificate export response
type CertificateResponse struct {
	Certificate string `json:"certificate"`
	IsSelfSigned bool   `json:"is_self_signed"`
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	NotBefore   string `json:"not_before"`
	NotAfter    string `json:"not_after"`
}

// GetCertificate exports the server's TLS certificate for client trust
func (h *SystemHandler) GetCertificate(c echo.Context) error {
	tlsMgr := security.GetGlobalTLSManager()
	certInfo := tlsMgr.GetCertificateInfo()

	if certInfo == nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "No certificate available",
		})
	}

	// Read certificate file
	cert := tlsMgr.GetCertificate()
	if cert == nil || len(cert.Certificate) == 0 {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to load certificate",
		})
	}

	// Encode certificate to PEM format
	certPEM := ""
	for _, certBytes := range cert.Certificate {
		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certBytes,
		}
		certPEM += string(pem.EncodeToMemory(block))
	}

	return c.JSON(http.StatusOK, CertificateResponse{
		Certificate: certPEM,
		IsSelfSigned: certInfo.IsSelfSigned,
		Subject:     certInfo.Subject,
		Issuer:      certInfo.Issuer,
		NotBefore:   certInfo.NotBefore.String(),
		NotAfter:    certInfo.NotAfter.String(),
	})
}

