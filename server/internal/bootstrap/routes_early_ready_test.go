package bootstrap

import (
	"context"
	"database/sql"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

type earlyReadyStop struct{}

func TestRegisterAllRoutes_EarlyReadyIncludesSettingsAndTunnelRoutes(t *testing.T) {
	tmp := t.TempDir()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	userRepo, err := user.NewSQLiteRepository(db)
	if err != nil {
		t.Fatalf("new user repo: %v", err)
	}
	userService := user.NewService(userRepo, nil, nil, nil)
	userHandler := user.NewHandler(userService)

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	broker := sse.NewBroker()
	defer broker.Close()

	e := echo.New()
	stop := earlyReadyStop{}
	deps := &RoutesDeps{
		Config: &config.Config{},
		ServerConfig: &ServerConfig{
			DataDir: tmp,
			Port:    80,
			Version: "test",
			Mode:    "embedded",
		},
		Services: &Services{
			DB:           db,
			UserService:  userService,
			UserRepo:     userRepo,
			JWTService:   jwtSvc,
			ToolRegistry: tools.NewRegistry(),
			DataDir:      tmp,
		},
		Logger:         zap.NewNop(),
		Ctx:            ctx,
		ChatHandler:    &serverpkg.ChatHandler{},
		UserHandler:    userHandler,
		AuthMiddleware: auth.NewAuthMiddleware(jwtSvc, nil),
		ConfigKV:       kvstore.NewMemoryStore(),
		SSEBroker:      broker,
		OnEarlyReady: func() {
			type routeKey struct {
				method string
				path   string
			}

			routes := make([]routeKey, 0, len(e.Routes()))
			for _, route := range e.Routes() {
				routes = append(routes, routeKey{method: route.Method, path: route.Path})
			}

			expected := []routeKey{
				{method: "GET", path: "/api/v1/settings"},
				{method: "PUT", path: "/api/v1/settings"},
				{method: "PATCH", path: "/api/v1/settings"},
				{method: "GET", path: "/api/v1/voice-wake/status"},
				{method: "POST", path: "/api/v1/voice-wake/restart"},
				{method: "GET", path: "/api/v1/events"},
				{method: "POST", path: "/api/v1/approval/resolve"},
				{method: "GET", path: "/api/v1/tunnel/providers"},
				{method: "GET", path: "/api/v1/claudecode/config"},
			}
			for _, route := range expected {
				if !slices.Contains(routes, route) {
					t.Fatalf("expected early-ready route %s %s to be registered; got routes=%v", route.method, route.path, routes)
				}
			}

			panic(stop)
		},
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected early-ready sentinel panic to stop deferred bootstrap")
		}
		if recovered != stop {
			panic(recovered)
		}
	}()

	RegisterAllRoutes(e, deps)
}
