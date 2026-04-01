package preview

import (
	"context"
	"database/sql"
	"errors"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

var (
	// ErrUsernameRequired is returned when username is empty.
	ErrUsernameRequired = errors.New("username is required")
	// ErrPasswordRequired is returned when password is empty.
	ErrPasswordRequired = errors.New("password is required")
	// ErrUsersAlreadyExist is returned when preview upgrade is attempted after any user exists.
	ErrUsersAlreadyExist = errors.New("users already exist")
	// ErrAdminAlreadyExists is returned when trying to upgrade but admin already exists.
	ErrAdminAlreadyExists = errors.New("admin already exists")
)

// UpgradeRequest represents a request to upgrade from preview mode.
type UpgradeRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// UpgradeResponse represents the response after successful upgrade.
type UpgradeResponse struct {
	Success        bool             `json:"success"`
	User           *user.User       `json:"user"`
	DataMigrated   bool             `json:"data_migrated"`
	MigratedCounts map[string]int64 `json:"migrated_counts,omitempty"`
}

// UpgradeService handles upgrading from preview mode to normal mode.
type UpgradeService struct {
	userService      *user.Service
	db               *sql.DB
	migrationService *MigrationService
}

// NewUpgradeService creates a new UpgradeService.
func NewUpgradeService(userService *user.Service, db *sql.DB) *UpgradeService {
	return NewUpgradeServiceWithReadDB(userService, db, db)
}

// NewUpgradeServiceWithReadDB creates a new UpgradeService with separate
// write and read database handles for preview migration status checks.
func NewUpgradeServiceWithReadDB(userService *user.Service, writeDB, readDB *sql.DB) *UpgradeService {
	return &UpgradeService{
		userService:      userService,
		db:               writeDB,
		migrationService: NewMigrationServiceWithReadDB(writeDB, readDB),
	}
}

// Upgrade creates an admin user and migrates preview data.
func (s *UpgradeService) Upgrade(ctx context.Context, req *UpgradeRequest) (*UpgradeResponse, error) {
	// Validate input
	if req.Username == "" {
		return nil, ErrUsernameRequired
	}
	if req.Password == "" {
		return nil, ErrPasswordRequired
	}

	// Preview upgrade is only valid before the first user is created.
	adminExists, err := s.userService.AdminExists(ctx)
	if err != nil {
		return nil, err
	}
	if adminExists {
		return nil, ErrAdminAlreadyExists
	}
	userExists, err := s.userService.AnyUserExists(ctx)
	if err != nil {
		return nil, err
	}
	if userExists {
		return nil, ErrUsersAlreadyExist
	}

	// Create admin user
	adminUser, err := s.userService.Create(ctx, &user.CreateUserRequest{
		Username: req.Username,
		Password: req.Password,
		Role:     user.RoleAdmin,
	})
	if err != nil {
		return nil, err
	}

	// Migrate preview data to admin
	dataMigrated := false
	var migratedCounts map[string]int64
	if s.migrationService != nil {
		migrationResult, err := s.migrationService.MigratePreviewData(ctx, adminUser.ID.String())
		if err != nil {
			// Log error but don't fail the upgrade
			// The admin is created, migration is best-effort
		} else {
			dataMigrated = true
			if migrationResult != nil {
				migratedCounts = migrationResult.MigratedCounts
			}
		}
	}

	return &UpgradeResponse{
		Success:        true,
		User:           adminUser,
		DataMigrated:   dataMigrated,
		MigratedCounts: migratedCounts,
	}, nil
}
