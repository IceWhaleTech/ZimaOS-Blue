package personality

import (
	"testing"
)

func TestServiceCreate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	service := NewService(repo)

	// Act
	p, err := service.Create("Test", "Test desc", "Test prompt")

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if p.ID == "" {
		t.Error("Expected ID to be set")
	}
}

func TestServiceGetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	service := NewService(repo)
	p, _ := service.Create("Test", "Test desc", "Test prompt")

	// Act
	retrieved, err := service.GetByID(p.ID)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if retrieved.Name != "Test" {
		t.Errorf("Expected name Test, got %s", retrieved.Name)
	}
}

func TestServiceDelete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	service := NewService(repo)
	p, _ := service.Create("Test", "Test desc", "Test prompt")

	// Act
	err := service.Delete(p.ID)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify deleted
	_, err = service.GetByID(p.ID)
	if err == nil {
		t.Error("Expected personality to be deleted")
	}
}
