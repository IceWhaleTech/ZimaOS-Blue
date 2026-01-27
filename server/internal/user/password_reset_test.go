package user

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewInMemoryPasswordResetStore(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	if store == nil {
		t.Fatal("NewInMemoryPasswordResetStore() returned nil")
	}
}

func TestInMemoryPasswordResetStore_Create(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	ctx := context.Background()

	token := &PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Token:     "test-token",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	err := store.Create(ctx, token)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestInMemoryPasswordResetStore_GetByToken(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	ctx := context.Background()

	token := &PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Token:     "test-token",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	_ = store.Create(ctx, token)

	retrieved, err := store.GetByToken(ctx, "test-token")
	if err != nil {
		t.Fatalf("GetByToken() error = %v", err)
	}

	if retrieved.ID != token.ID {
		t.Errorf("GetByToken() ID = %v, want %v", retrieved.ID, token.ID)
	}
}

func TestInMemoryPasswordResetStore_GetByToken_NotFound(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	ctx := context.Background()

	_, err := store.GetByToken(ctx, "non-existent")
	if err != ErrResetTokenNotFound {
		t.Errorf("GetByToken(not found) error = %v, want %v", err, ErrResetTokenNotFound)
	}
}

func TestInMemoryPasswordResetStore_MarkUsed(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	ctx := context.Background()

	token := &PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Token:     "test-token",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}

	_ = store.Create(ctx, token)

	err := store.MarkUsed(ctx, token.ID)
	if err != nil {
		t.Fatalf("MarkUsed() error = %v", err)
	}

	retrieved, _ := store.GetByToken(ctx, "test-token")
	if retrieved.UsedAt == nil {
		t.Error("MarkUsed() should set UsedAt")
	}
}

func TestInMemoryPasswordResetStore_CountRecentByUser(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	ctx := context.Background()

	userID := uuid.New()

	// Create multiple tokens
	for i := 0; i < 3; i++ {
		token := &PasswordResetToken{
			ID:        uuid.New(),
			UserID:    userID,
			Token:     "token-" + string(rune('0'+i)),
			ExpiresAt: time.Now().Add(time.Hour),
			CreatedAt: time.Now(),
		}
		_ = store.Create(ctx, token)
	}

	count, err := store.CountRecentByUser(ctx, userID, time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatalf("CountRecentByUser() error = %v", err)
	}

	if count != 3 {
		t.Errorf("CountRecentByUser() = %d, want 3", count)
	}
}

func TestInMemoryPasswordResetStore_DeleteExpired(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	ctx := context.Background()

	// Create expired token
	expiredToken := &PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-time.Hour),
		CreatedAt: time.Now().Add(-2 * time.Hour),
	}
	_ = store.Create(ctx, expiredToken)

	// Create valid token
	validToken := &PasswordResetToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Token:     "valid-token",
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedAt: time.Now(),
	}
	_ = store.Create(ctx, validToken)

	err := store.DeleteExpired(ctx)
	if err != nil {
		t.Fatalf("DeleteExpired() error = %v", err)
	}

	// Expired token should be deleted
	_, err = store.GetByToken(ctx, "expired-token")
	if err != ErrResetTokenNotFound {
		t.Error("DeleteExpired() should delete expired token")
	}

	// Valid token should still exist
	_, err = store.GetByToken(ctx, "valid-token")
	if err != nil {
		t.Error("DeleteExpired() should not delete valid token")
	}
}

func TestPasswordResetToken_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		token    *PasswordResetToken
		expected bool
	}{
		{
			name: "valid token",
			token: &PasswordResetToken{
				ExpiresAt: time.Now().Add(time.Hour),
				UsedAt:    nil,
			},
			expected: true,
		},
		{
			name: "expired token",
			token: &PasswordResetToken{
				ExpiresAt: time.Now().Add(-time.Hour),
				UsedAt:    nil,
			},
			expected: false,
		},
		{
			name: "used token",
			token: &PasswordResetToken{
				ExpiresAt: time.Now().Add(time.Hour),
				UsedAt:    func() *time.Time { t := time.Now(); return &t }(),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.token.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestDefaultPasswordResetConfig(t *testing.T) {
	config := DefaultPasswordResetConfig()

	if config == nil {
		t.Fatal("DefaultPasswordResetConfig() returned nil")
	}

	if config.TokenTTL != time.Hour {
		t.Errorf("TokenTTL = %v, want 1h", config.TokenTTL)
	}

	if config.MaxRequestsPerHour != 3 {
		t.Errorf("MaxRequestsPerHour = %d, want 3", config.MaxRequestsPerHour)
	}

	if config.TokenLength != 32 {
		t.Errorf("TokenLength = %d, want 32", config.TokenLength)
	}
}

func TestNewPasswordResetService(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	service := NewPasswordResetService(store, nil)

	if service == nil {
		t.Fatal("NewPasswordResetService() returned nil")
	}
}

func TestPasswordResetService_RequestReset(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	service := NewPasswordResetService(store, nil)
	ctx := context.Background()

	userID := uuid.New()
	token, err := service.RequestReset(ctx, userID)

	if err != nil {
		t.Fatalf("RequestReset() error = %v", err)
	}

	if token == nil {
		t.Fatal("RequestReset() returned nil token")
	}

	if token.UserID != userID {
		t.Errorf("RequestReset() UserID = %v, want %v", token.UserID, userID)
	}

	if token.Token == "" {
		t.Error("RequestReset() Token should not be empty")
	}
}

func TestPasswordResetService_RequestReset_RateLimit(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	config := &PasswordResetConfig{
		TokenTTL:           time.Hour,
		MaxRequestsPerHour: 2,
		TokenLength:        32,
	}
	service := NewPasswordResetService(store, config)
	ctx := context.Background()

	userID := uuid.New()

	// First two requests should succeed
	_, err := service.RequestReset(ctx, userID)
	if err != nil {
		t.Fatalf("RequestReset(1) error = %v", err)
	}

	_, err = service.RequestReset(ctx, userID)
	if err != nil {
		t.Fatalf("RequestReset(2) error = %v", err)
	}

	// Third request should fail
	_, err = service.RequestReset(ctx, userID)
	if err != ErrTooManyResetRequests {
		t.Errorf("RequestReset(3) error = %v, want %v", err, ErrTooManyResetRequests)
	}
}

func TestPasswordResetService_ValidateToken(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	service := NewPasswordResetService(store, nil)
	ctx := context.Background()

	userID := uuid.New()
	token, _ := service.RequestReset(ctx, userID)

	validatedUserID, err := service.ValidateToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("ValidateToken() error = %v", err)
	}

	if validatedUserID != userID {
		t.Errorf("ValidateToken() userID = %v, want %v", validatedUserID, userID)
	}
}

func TestPasswordResetService_ValidateToken_NotFound(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	service := NewPasswordResetService(store, nil)
	ctx := context.Background()

	_, err := service.ValidateToken(ctx, "non-existent")
	if err != ErrResetTokenNotFound {
		t.Errorf("ValidateToken(not found) error = %v, want %v", err, ErrResetTokenNotFound)
	}
}

func TestPasswordResetService_ValidateToken_Expired(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	config := &PasswordResetConfig{
		TokenTTL:           -time.Hour, // Already expired
		MaxRequestsPerHour: 10,
		TokenLength:        32,
	}
	service := NewPasswordResetService(store, config)
	ctx := context.Background()

	userID := uuid.New()
	token, _ := service.RequestReset(ctx, userID)

	_, err := service.ValidateToken(ctx, token.Token)
	if err != ErrResetTokenExpired {
		t.Errorf("ValidateToken(expired) error = %v, want %v", err, ErrResetTokenExpired)
	}
}

func TestPasswordResetService_ValidateToken_Used(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	service := NewPasswordResetService(store, nil)
	ctx := context.Background()

	userID := uuid.New()
	token, _ := service.RequestReset(ctx, userID)

	// Mark as used
	_ = store.MarkUsed(ctx, token.ID)

	_, err := service.ValidateToken(ctx, token.Token)
	if err != ErrResetTokenUsed {
		t.Errorf("ValidateToken(used) error = %v, want %v", err, ErrResetTokenUsed)
	}
}

func TestPasswordResetService_ConsumeToken(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	service := NewPasswordResetService(store, nil)
	ctx := context.Background()

	userID := uuid.New()
	token, _ := service.RequestReset(ctx, userID)

	err := service.ConsumeToken(ctx, token.Token)
	if err != nil {
		t.Fatalf("ConsumeToken() error = %v", err)
	}

	// Second consume should fail
	err = service.ConsumeToken(ctx, token.Token)
	if err != ErrResetTokenUsed {
		t.Errorf("ConsumeToken(second) error = %v, want %v", err, ErrResetTokenUsed)
	}
}

func TestPasswordResetService_Cleanup(t *testing.T) {
	store := NewInMemoryPasswordResetStore()
	service := NewPasswordResetService(store, nil)
	ctx := context.Background()

	err := service.Cleanup(ctx)
	if err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
}

func TestPasswordResetRequest_Fields(t *testing.T) {
	req := PasswordResetRequest{
		Email: "test@example.com",
	}

	if req.Email != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", req.Email)
	}
}

func TestPasswordResetConfirmRequest_Fields(t *testing.T) {
	req := PasswordResetConfirmRequest{
		Token:       "test-token",
		NewPassword: "new-password",
	}

	if req.Token != "test-token" {
		t.Errorf("Token = %v, want test-token", req.Token)
	}

	if req.NewPassword != "new-password" {
		t.Errorf("NewPassword = %v, want new-password", req.NewPassword)
	}
}

func TestPasswordResetResponse_Fields(t *testing.T) {
	resp := PasswordResetResponse{
		Message: "Password reset email sent",
	}

	if resp.Message != "Password reset email sent" {
		t.Errorf("Message = %v, want 'Password reset email sent'", resp.Message)
	}
}
