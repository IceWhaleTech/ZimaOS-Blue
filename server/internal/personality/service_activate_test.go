package personality

import (
	"testing"
)

func TestServiceActivatePersonality(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	service := NewService(repo)
	p1, _ := service.Create("P1", "Desc1", "Prompt1")
	_, _ = service.Create("P2", "Desc2", "Prompt2")

	// Act
	err := service.Activate(p1.ID)

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify p1 is active
	active, err := service.GetActive()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if active.ID != p1.ID {
		t.Errorf("Expected active personality %s, got %s", p1.ID, active.ID)
	}
}

func TestServiceGetActive(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewRepository(db)
	service := NewService(repo)
	p, _ := service.Create("Test", "Desc", "Prompt")
	service.Activate(p.ID)

	// Act
	active, err := service.GetActive()

	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if active.ID != p.ID {
		t.Errorf("Expected %s, got %s", p.ID, active.ID)
	}
}
