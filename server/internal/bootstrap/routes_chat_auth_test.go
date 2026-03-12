package bootstrap

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func TestChatRoutesOptionalAuthenticateCarriesUserContext(t *testing.T) {
	store, err := memory.NewStore(filepath.Join(t.TempDir(), "chat.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	handler := server.NewChatHandler(store, llm.NewProviderRegistry(), tools.NewRegistry())
	defer handler.Close()

	jwtSvc := auth.NewJWTService(&auth.JWTConfig{
		Secret:            strings.Repeat("a", 32),
		Expiration:        time.Hour,
		RefreshExpiration: 2 * time.Hour,
		Issuer:            "test",
	})
	middleware := auth.NewAuthMiddleware(jwtSvc, nil)

	claims := &auth.UserClaims{
		UserID:   "preview-user",
		Username: "preview",
		Role:     "user",
	}
	token, err := jwtSvc.GenerateAccessToken(claims)
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	e := echo.New()
	api := e.Group("/api/v1")
	chatGroup := api.Group("")
	chatGroup.Use(middleware.OptionalAuthenticate())
	handler.RegisterRoutes(chatGroup)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/conversations", bytes.NewBufferString(`{"title":"Preview Reminder"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var conv memory.Conversation
	if err := json.Unmarshal(rec.Body.Bytes(), &conv); err != nil {
		t.Fatalf("decode conversation: %v", err)
	}
	if conv.UserID != claims.UserID {
		t.Fatalf("conversation user_id = %q, want %q", conv.UserID, claims.UserID)
	}
}
