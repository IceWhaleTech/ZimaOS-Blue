package connection

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// ConnectionType represents the type of connection.
type ConnectionType string

const (
	ConnectionTypeHTTP      ConnectionType = "http"
	ConnectionTypeWebSocket ConnectionType = "websocket"
	ConnectionTypeSSE       ConnectionType = "sse"
)

// Connection represents an active connection.
type Connection struct {
	ID           string         `json:"id"`
	Type         ConnectionType `json:"type"`
	ClientIP     string         `json:"client_ip"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Path         string         `json:"path"`
	Method       string         `json:"method,omitempty"`
	ConnectedAt  time.Time      `json:"connected_at"`
	LastActivity time.Time      `json:"last_activity"`
	RequestCount int            `json:"request_count"`
	BytesSent    int64          `json:"bytes_sent"`
	BytesRecv    int64          `json:"bytes_recv"`
	Status       string         `json:"status"` // active, idle, closed
	Metadata     map[string]any `json:"metadata,omitempty"`
	GeoLocation  *GeoLocation   `json:"geo_location,omitempty"`
}

// GeoLocation represents geographic location information for an IP.
type GeoLocation struct {
	Country     string  `json:"country,omitempty"`
	CountryCode string  `json:"country_code,omitempty"`
	Region      string  `json:"region,omitempty"`
	City        string  `json:"city,omitempty"`
	Latitude    float64 `json:"latitude,omitempty"`
	Longitude   float64 `json:"longitude,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	ISP         string  `json:"isp,omitempty"`
	IsPrivate   bool    `json:"is_private"`
}

// ConnectionStats represents connection statistics.
type ConnectionStats struct {
	TotalConnections   int            `json:"total_connections"`
	ActiveHTTP         int            `json:"active_http"`
	ActiveWebSocket    int            `json:"active_websocket"`
	ActiveSSE          int            `json:"active_sse"`
	TotalRequests      int64          `json:"total_requests"`
	TotalBytesSent     int64          `json:"total_bytes_sent"`
	TotalBytesRecv     int64          `json:"total_bytes_recv"`
	ConnectionsByIP    map[string]int `json:"connections_by_ip"`
	AvgRequestDuration time.Duration  `json:"avg_request_duration"`
	LastUpdated        time.Time      `json:"last_updated"`
}

// Manager manages active connections.
type Manager struct {
	mu          sync.RWMutex
	connections map[string]*Connection
	stats       ConnectionStats
	maxConns    int
	idleTimeout time.Duration
}

// NewManager creates a new connection manager.
func NewManager(maxConns int, idleTimeout time.Duration) *Manager {
	if maxConns <= 0 {
		maxConns = 10000
	}
	if idleTimeout <= 0 {
		idleTimeout = 30 * time.Second // Default to 30 seconds for keep-alive
	}

	m := &Manager{
		connections: make(map[string]*Connection),
		maxConns:    maxConns,
		idleTimeout: idleTimeout,
		stats: ConnectionStats{
			ConnectionsByIP: make(map[string]int),
			LastUpdated:     timeutil.NowTime(),
		},
	}

	// Start cleanup goroutine
	go m.cleanupLoop()

	return m
}

// RegisterConnection registers a new connection.
func (m *Manager) RegisterConnection(connType ConnectionType, clientIP, userAgent, path, method string) *Connection {
	m.mu.Lock()
	defer m.mu.Unlock()

	conn := &Connection{
		ID:           uuid.New().String(),
		Type:         connType,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
		Path:         path,
		Method:       method,
		ConnectedAt:  timeutil.NowTime(),
		LastActivity: timeutil.NowTime(),
		RequestCount: 1,
		Status:       "active",
		Metadata:     make(map[string]any),
	}

	m.connections[conn.ID] = conn
	m.stats.TotalConnections++
	m.stats.TotalRequests++
	m.stats.ConnectionsByIP[clientIP]++

	switch connType {
	case ConnectionTypeHTTP:
		m.stats.ActiveHTTP++
	case ConnectionTypeWebSocket:
		m.stats.ActiveWebSocket++
	case ConnectionTypeSSE:
		m.stats.ActiveSSE++
	}

	m.stats.LastUpdated = timeutil.NowTime()

	return conn
}

// UpdateConnection updates connection activity.
func (m *Manager) UpdateConnection(id string, bytesSent, bytesRecv int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, ok := m.connections[id]; ok {
		conn.LastActivity = timeutil.NowTime()
		conn.RequestCount++
		conn.BytesSent += bytesSent
		conn.BytesRecv += bytesRecv
		conn.Status = "active"

		m.stats.TotalRequests++
		m.stats.TotalBytesSent += bytesSent
		m.stats.TotalBytesRecv += bytesRecv
		m.stats.LastUpdated = timeutil.NowTime()
	}
}

// UnregisterConnection removes a connection.
func (m *Manager) UnregisterConnection(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, ok := m.connections[id]; ok {
		conn.Status = "closed"

		switch conn.Type {
		case ConnectionTypeHTTP:
			m.stats.ActiveHTTP--
		case ConnectionTypeWebSocket:
			m.stats.ActiveWebSocket--
		case ConnectionTypeSSE:
			m.stats.ActiveSSE--
		}

		m.stats.ConnectionsByIP[conn.ClientIP]--
		if m.stats.ConnectionsByIP[conn.ClientIP] <= 0 {
			delete(m.stats.ConnectionsByIP, conn.ClientIP)
		}

		delete(m.connections, id)
		m.stats.LastUpdated = timeutil.NowTime()
	}
}

// GetConnection retrieves a connection by ID.
func (m *Manager) GetConnection(id string) *Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if conn, ok := m.connections[id]; ok {
		return conn
	}
	return nil
}

// ListConnections returns all active connections.
func (m *Manager) ListConnections(connType ConnectionType) []*Connection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Connection, 0)
	for _, conn := range m.connections {
		if connType == "" || conn.Type == connType {
			result = append(result, conn)
		}
	}
	return result
}

// GetStats returns connection statistics.
func (m *Manager) GetStats() ConnectionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Create a copy of stats
	stats := m.stats
	stats.TotalConnections = len(m.connections)
	stats.ConnectionsByIP = make(map[string]int)
	for ip, count := range m.stats.ConnectionsByIP {
		stats.ConnectionsByIP[ip] = count
	}

	return stats
}

// cleanupLoop periodically removes idle connections.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.cleanupIdleConnections()
	}
}

// cleanupIdleConnections removes connections that have been idle too long.
func (m *Manager) cleanupIdleConnections() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := timeutil.NowTime()
	for id, conn := range m.connections {
		// Only cleanup HTTP connections (WS/SSE are long-lived)
		if conn.Type == ConnectionTypeHTTP && now.Sub(conn.LastActivity) > m.idleTimeout {
			conn.Status = "closed"
			m.stats.ActiveHTTP--
			m.stats.ConnectionsByIP[conn.ClientIP]--
			if m.stats.ConnectionsByIP[conn.ClientIP] <= 0 {
				delete(m.stats.ConnectionsByIP, conn.ClientIP)
			}
			delete(m.connections, id)
		}
	}
	m.stats.LastUpdated = now
}

// Handler handles connection-related API endpoints.
type Handler struct {
	manager *Manager
}

// NewHandler creates a new connection handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the connection API routes.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.GET("/active", h.ListActiveConnections)
	g.GET("/stats", h.GetStats)
	g.GET("/:id", h.GetConnection)
	g.GET("/:id/geo", h.GetConnectionGeo)
	g.POST("/geo/lookup", h.LookupIPGeo)
}

// ListActiveConnections handles GET /api/v1/connections/active
func (h *Handler) ListActiveConnections(c echo.Context) error {
	connType := ConnectionType(c.QueryParam("type"))

	connections := h.manager.ListConnections(connType)

	return c.JSON(http.StatusOK, map[string]any{
		"connections": connections,
		"total":       len(connections),
	})
}

// GetStats handles GET /api/v1/connections/stats
func (h *Handler) GetStats(c echo.Context) error {
	stats := h.manager.GetStats()
	return c.JSON(http.StatusOK, stats)
}

// GetConnection handles GET /api/v1/connections/:id
func (h *Handler) GetConnection(c echo.Context) error {
	id := c.Param("id")

	conn := h.manager.GetConnection(id)
	if conn == nil {
		return echo.NewHTTPError(http.StatusNotFound, "connection not found")
	}

	return c.JSON(http.StatusOK, conn)
}

// GetConnectionGeo handles GET /api/v1/connections/:id/geo
// Returns geographic location for a specific connection's IP (on-demand lookup).
func (h *Handler) GetConnectionGeo(c echo.Context) error {
	id := c.Param("id")

	conn := h.manager.GetConnection(id)
	if conn == nil {
		return echo.NewHTTPError(http.StatusNotFound, "connection not found")
	}

	// Get geo location for the connection's IP
	geo := GetGeoLocation(conn.ClientIP)

	// Optionally cache it in the connection metadata
	h.manager.mu.Lock()
	if existingConn, ok := h.manager.connections[id]; ok {
		existingConn.GeoLocation = geo
	}
	h.manager.mu.Unlock()

	return c.JSON(http.StatusOK, map[string]any{
		"connection_id": id,
		"client_ip":     conn.ClientIP,
		"ip_type":       GetIPType(conn.ClientIP),
		"platform":      ParseUserAgentPlatform(conn.UserAgent),
		"geo_location":  geo,
	})
}

// LookupIPGeo handles POST /api/v1/connections/geo/lookup
// Allows manual IP geolocation lookup without requiring a connection.
func (h *Handler) LookupIPGeo(c echo.Context) error {
	var req struct {
		IP string `json:"ip"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.IP == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "ip is required")
	}

	geo := GetGeoLocation(req.IP)

	return c.JSON(http.StatusOK, map[string]any{
		"ip":           req.IP,
		"ip_type":      GetIPType(req.IP),
		"geo_location": geo,
	})
}

// Middleware creates an Echo middleware for tracking connections.
func (m *Manager) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Determine connection type
			connType := ConnectionTypeHTTP
			if c.Request().Header.Get("Upgrade") == "websocket" {
				connType = ConnectionTypeWebSocket
			} else if c.Request().Header.Get("Accept") == "text/event-stream" {
				connType = ConnectionTypeSSE
			}

			// Register connection
			conn := m.RegisterConnection(
				connType,
				c.RealIP(),
				c.Request().UserAgent(),
				c.Request().URL.Path,
				c.Request().Method,
			)

			// Store connection ID in context
			c.Set("connection_id", conn.ID)

			// Execute handler
			err := next(c)

			// Update connection stats
			// Note: For HTTP, we unregister after response
			// For WS/SSE, they should call UnregisterConnection when done
			if connType == ConnectionTypeHTTP {
				// Get response size if available
				resp := c.Response()
				m.UpdateConnection(conn.ID, resp.Size, c.Request().ContentLength)

				// For short-lived HTTP requests, mark as completed but keep for stats
				m.mu.Lock()
				if c, ok := m.connections[conn.ID]; ok {
					c.Status = "completed"
				}
				m.mu.Unlock()
			}

			return err
		}
	}
}

// GetGeoLocation returns geographic location for an IP address.
// It first checks if the IP is private/local, then uses cached data or external lookup.
func GetGeoLocation(ipStr string) *GeoLocation {
	geo := &GeoLocation{}

	// Parse IP
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return geo
	}

	// Check if private/local IP
	if isPrivateIP(ip) {
		geo.IsPrivate = true
		geo.Country = "Local Network"
		geo.CountryCode = "LAN"
		return geo
	}

	// Check for localhost
	if ip.IsLoopback() {
		geo.IsPrivate = true
		geo.Country = "Localhost"
		geo.CountryCode = "LO"
		return geo
	}

	// For public IPs, we return basic info
	// Full geolocation would require an external service like MaxMind GeoIP
	geo.IsPrivate = false
	geo.Country = "Unknown"
	geo.CountryCode = "XX"

	return geo
}

// isPrivateIP checks if an IP address is private/internal.
func isPrivateIP(ip net.IP) bool {
	// Check for IPv4 private ranges
	privateRanges := []string{
		"10.0.0.0/8",     // Class A private
		"172.16.0.0/12",  // Class B private
		"192.168.0.0/16", // Class C private
		"169.254.0.0/16", // Link-local
		"127.0.0.0/8",    // Loopback
		"::1/128",        // IPv6 loopback
		"fc00::/7",       // IPv6 unique local
		"fe80::/10",      // IPv6 link-local
	}

	for _, cidr := range privateRanges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// GetIPType returns a human-readable type for the IP address.
func GetIPType(ipStr string) string {
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return "invalid"
	}

	if ip.IsLoopback() {
		return "loopback"
	}
	if isPrivateIP(ip) {
		return "private"
	}
	if ip.IsMulticast() {
		return "multicast"
	}
	if ip.IsUnspecified() {
		return "unspecified"
	}

	// Check if IPv4 or IPv6
	if ip.To4() != nil {
		return "public_ipv4"
	}
	return "public_ipv6"
}

// ParseUserAgentPlatform extracts platform info from User-Agent string.
func ParseUserAgentPlatform(ua string) string {
	ua = strings.ToLower(ua)

	if strings.Contains(ua, "windows") {
		return "Windows"
	}
	if strings.Contains(ua, "macintosh") || strings.Contains(ua, "mac os") {
		return "macOS"
	}
	if strings.Contains(ua, "linux") {
		if strings.Contains(ua, "android") {
			return "Android"
		}
		return "Linux"
	}
	if strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		return "iOS"
	}
	if strings.Contains(ua, "android") {
		return "Android"
	}

	return "Unknown"
}
