package mfa

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewInMemoryCredentialStore(t *testing.T) {
	store := NewInMemoryCredentialStore()
	if store == nil {
		t.Fatal("NewInMemoryCredentialStore() returned nil")
	}
}

func TestInMemoryCredentialStore_Create(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		PublicKey: []byte("test-public-key"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}

	err := store.Create(ctx, userID, cred)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
}

func TestInMemoryCredentialStore_Create_Duplicate(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		PublicKey: []byte("test-public-key"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}

	_ = store.Create(ctx, userID, cred)

	err := store.Create(ctx, userID, cred)
	if err != ErrCredentialExists {
		t.Errorf("Create(duplicate) error = %v, want %v", err, ErrCredentialExists)
	}
}

func TestInMemoryCredentialStore_GetByUser(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	// Create multiple credentials
	for i := 0; i < 3; i++ {
		cred := &WebAuthnCredential{
			ID:        []byte("cred-" + string(rune('0'+i))),
			PublicKey: []byte("key-" + string(rune('0'+i))),
			Name:      "Key " + string(rune('0'+i)),
			CreatedAt: time.Now(),
		}
		_ = store.Create(ctx, userID, cred)
	}

	creds, err := store.GetByUser(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUser() error = %v", err)
	}

	if len(creds) != 3 {
		t.Errorf("GetByUser() returned %d credentials, want 3", len(creds))
	}
}

func TestInMemoryCredentialStore_GetByUser_Empty(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	creds, err := store.GetByUser(ctx, userID)
	if err != nil {
		t.Fatalf("GetByUser(empty) error = %v", err)
	}

	if len(creds) != 0 {
		t.Errorf("GetByUser(empty) returned %d credentials, want 0", len(creds))
	}
}

func TestInMemoryCredentialStore_GetByID(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		PublicKey: []byte("test-public-key"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}

	_ = store.Create(ctx, userID, cred)

	retrieved, err := store.GetByID(ctx, userID, []byte("test-cred-id"))
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}

	if retrieved.Name != "Test Key" {
		t.Errorf("GetByID() Name = %v, want Test Key", retrieved.Name)
	}
}

func TestInMemoryCredentialStore_GetByID_NotFound(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	_, err := store.GetByID(ctx, userID, []byte("non-existent"))
	if err != ErrCredentialNotFound {
		t.Errorf("GetByID(not found) error = %v, want %v", err, ErrCredentialNotFound)
	}
}

func TestInMemoryCredentialStore_Update(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		PublicKey: []byte("test-public-key"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}

	_ = store.Create(ctx, userID, cred)

	cred.Name = "Updated Key"
	err := store.Update(ctx, userID, cred)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	retrieved, _ := store.GetByID(ctx, userID, []byte("test-cred-id"))
	if retrieved.Name != "Updated Key" {
		t.Errorf("Update() Name = %v, want Updated Key", retrieved.Name)
	}
}

func TestInMemoryCredentialStore_Update_NotFound(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:   []byte("non-existent"),
		Name: "Test",
	}

	err := store.Update(ctx, userID, cred)
	if err != ErrCredentialNotFound {
		t.Errorf("Update(not found) error = %v, want %v", err, ErrCredentialNotFound)
	}
}

func TestInMemoryCredentialStore_Delete(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		PublicKey: []byte("test-public-key"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}

	_ = store.Create(ctx, userID, cred)

	err := store.Delete(ctx, userID, []byte("test-cred-id"))
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err = store.GetByID(ctx, userID, []byte("test-cred-id"))
	if err != ErrCredentialNotFound {
		t.Error("Delete() should remove credential")
	}
}

func TestInMemoryCredentialStore_Delete_NotFound(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	err := store.Delete(ctx, userID, []byte("non-existent"))
	if err != ErrCredentialNotFound {
		t.Errorf("Delete(not found) error = %v, want %v", err, ErrCredentialNotFound)
	}
}

func TestInMemoryCredentialStore_Count(t *testing.T) {
	store := NewInMemoryCredentialStore()
	ctx := context.Background()
	userID := uuid.New()

	// Initially 0
	count, _ := store.Count(ctx, userID)
	if count != 0 {
		t.Errorf("Count() = %d, want 0", count)
	}

	// Add credentials
	for i := 0; i < 3; i++ {
		cred := &WebAuthnCredential{
			ID:   []byte("cred-" + string(rune('0'+i))),
			Name: "Key " + string(rune('0'+i)),
		}
		_ = store.Create(ctx, userID, cred)
	}

	count, _ = store.Count(ctx, userID)
	if count != 3 {
		t.Errorf("Count() = %d, want 3", count)
	}
}

func TestDefaultCredentialManagerConfig(t *testing.T) {
	config := DefaultCredentialManagerConfig()

	if config == nil {
		t.Fatal("DefaultCredentialManagerConfig() returned nil")
	}

	if config.MaxCredentialsPerUser != 10 {
		t.Errorf("MaxCredentialsPerUser = %d, want 10", config.MaxCredentialsPerUser)
	}
}

func TestNewCredentialManager(t *testing.T) {
	store := NewInMemoryCredentialStore()
	manager := NewCredentialManager(store, nil)

	if manager == nil {
		t.Fatal("NewCredentialManager() returned nil")
	}
}

func TestCredentialManager_AddCredential(t *testing.T) {
	store := NewInMemoryCredentialStore()
	manager := NewCredentialManager(store, nil)
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		PublicKey: []byte("test-public-key"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}

	err := manager.AddCredential(ctx, userID, cred)
	if err != nil {
		t.Fatalf("AddCredential() error = %v", err)
	}
}

func TestCredentialManager_AddCredential_MaxReached(t *testing.T) {
	store := NewInMemoryCredentialStore()
	config := &CredentialManagerConfig{MaxCredentialsPerUser: 2}
	manager := NewCredentialManager(store, config)
	ctx := context.Background()
	userID := uuid.New()

	// Add max credentials
	for i := 0; i < 2; i++ {
		cred := &WebAuthnCredential{
			ID:   []byte("cred-" + string(rune('0'+i))),
			Name: "Key " + string(rune('0'+i)),
		}
		_ = manager.AddCredential(ctx, userID, cred)
	}

	// Try to add one more
	cred := &WebAuthnCredential{
		ID:   []byte("cred-extra"),
		Name: "Extra Key",
	}

	err := manager.AddCredential(ctx, userID, cred)
	if err != ErrMaxCredentialsReached {
		t.Errorf("AddCredential(max) error = %v, want %v", err, ErrMaxCredentialsReached)
	}
}

func TestCredentialManager_ListCredentials(t *testing.T) {
	store := NewInMemoryCredentialStore()
	manager := NewCredentialManager(store, nil)
	ctx := context.Background()
	userID := uuid.New()

	// Add credentials
	for i := 0; i < 3; i++ {
		cred := &WebAuthnCredential{
			ID:        []byte("cred-" + string(rune('0'+i))),
			Name:      "Key " + string(rune('0'+i)),
			CreatedAt: time.Now(),
		}
		_ = manager.AddCredential(ctx, userID, cred)
	}

	creds, err := manager.ListCredentials(ctx, userID)
	if err != nil {
		t.Fatalf("ListCredentials() error = %v", err)
	}

	if len(creds) != 3 {
		t.Errorf("ListCredentials() returned %d credentials, want 3", len(creds))
	}
}

func TestCredentialManager_GetCredential(t *testing.T) {
	store := NewInMemoryCredentialStore()
	manager := NewCredentialManager(store, nil)
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}
	_ = manager.AddCredential(ctx, userID, cred)

	// List to get the encoded ID
	creds, _ := manager.ListCredentials(ctx, userID)
	credID := creds[0].ID

	info, err := manager.GetCredential(ctx, userID, credID)
	if err != nil {
		t.Fatalf("GetCredential() error = %v", err)
	}

	if info.Name != "Test Key" {
		t.Errorf("GetCredential() Name = %v, want Test Key", info.Name)
	}
}

func TestCredentialManager_UpdateCredentialName(t *testing.T) {
	store := NewInMemoryCredentialStore()
	manager := NewCredentialManager(store, nil)
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}
	_ = manager.AddCredential(ctx, userID, cred)

	creds, _ := manager.ListCredentials(ctx, userID)
	credID := creds[0].ID

	err := manager.UpdateCredentialName(ctx, userID, credID, "Updated Key")
	if err != nil {
		t.Fatalf("UpdateCredentialName() error = %v", err)
	}

	info, _ := manager.GetCredential(ctx, userID, credID)
	if info.Name != "Updated Key" {
		t.Errorf("UpdateCredentialName() Name = %v, want Updated Key", info.Name)
	}
}

func TestCredentialManager_DeleteCredential(t *testing.T) {
	store := NewInMemoryCredentialStore()
	manager := NewCredentialManager(store, nil)
	ctx := context.Background()
	userID := uuid.New()

	cred := &WebAuthnCredential{
		ID:        []byte("test-cred-id"),
		Name:      "Test Key",
		CreatedAt: time.Now(),
	}
	_ = manager.AddCredential(ctx, userID, cred)

	creds, _ := manager.ListCredentials(ctx, userID)
	credID := creds[0].ID

	err := manager.DeleteCredential(ctx, userID, credID)
	if err != nil {
		t.Fatalf("DeleteCredential() error = %v", err)
	}

	creds, _ = manager.ListCredentials(ctx, userID)
	if len(creds) != 0 {
		t.Error("DeleteCredential() should remove credential")
	}
}

func TestCredentialManager_UpdateLastUsed(t *testing.T) {
	store := NewInMemoryCredentialStore()
	manager := NewCredentialManager(store, nil)
	ctx := context.Background()
	userID := uuid.New()

	oldTime := time.Now().Add(-time.Hour)
	cred := &WebAuthnCredential{
		ID:         []byte("test-cred-id"),
		Name:       "Test Key",
		CreatedAt:  oldTime,
		LastUsedAt: oldTime,
	}
	_ = manager.AddCredential(ctx, userID, cred)

	err := manager.UpdateLastUsed(ctx, userID, []byte("test-cred-id"))
	if err != nil {
		t.Fatalf("UpdateLastUsed() error = %v", err)
	}

	creds, _ := manager.ListCredentials(ctx, userID)
	if creds[0].LastUsedAt.Before(oldTime.Add(time.Minute)) {
		t.Error("UpdateLastUsed() should update LastUsedAt")
	}
}

func TestCredentialInfo_Fields(t *testing.T) {
	now := time.Now()
	info := CredentialInfo{
		ID:         "test-id",
		Name:       "Test Key",
		CreatedAt:  now,
		LastUsedAt: now,
	}

	if info.ID != "test-id" {
		t.Errorf("ID = %v, want test-id", info.ID)
	}
	if info.Name != "Test Key" {
		t.Errorf("Name = %v, want Test Key", info.Name)
	}
}

func TestListCredentialsResponse_Fields(t *testing.T) {
	resp := ListCredentialsResponse{
		Credentials: []CredentialInfo{
			{ID: "1", Name: "Key 1"},
			{ID: "2", Name: "Key 2"},
		},
		Count:      2,
		MaxAllowed: 10,
	}

	if len(resp.Credentials) != 2 {
		t.Errorf("Credentials length = %d, want 2", len(resp.Credentials))
	}
	if resp.Count != 2 {
		t.Errorf("Count = %d, want 2", resp.Count)
	}
	if resp.MaxAllowed != 10 {
		t.Errorf("MaxAllowed = %d, want 10", resp.MaxAllowed)
	}
}

func TestUpdateCredentialRequest_Fields(t *testing.T) {
	req := UpdateCredentialRequest{
		Name: "New Name",
	}

	if req.Name != "New Name" {
		t.Errorf("Name = %v, want New Name", req.Name)
	}
}

func TestDeleteCredentialRequest_Fields(t *testing.T) {
	req := DeleteCredentialRequest{
		Password: "secret",
	}

	if req.Password != "secret" {
		t.Errorf("Password = %v, want secret", req.Password)
	}
}
