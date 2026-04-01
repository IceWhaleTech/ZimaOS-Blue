package bootstrap

import (
	"fmt"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/password"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/user"
)

func initServicesDatabaseAndIdentity(
	s *Services,
	cfg *ServerConfig,
	appCfg *config.Config,
	trace *StartupTrace,
) error {
	if err := os.MkdirAll(cfg.DataDir, 0o750); err != nil {
		return fmt.Errorf("failed to create data directory: %w", err)
	}
	trace.Mark("data_dir_ready")

	dbPath := filepath.Join(cfg.DataDir, "blue.db")
	dbConn, err := openPrimaryDatabase(cfg, s.Logger)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	s.DB = dbConn.Writer
	s.DBConn = dbConn
	trace.Mark("db_opened", zap.String("db_path", dbPath))

	var userRepo *user.SQLiteRepository
	if s.DBConn != nil {
		userRepo, err = user.NewSQLiteRepositoryWithReadDB(s.DBConn.Writer, s.DBConn.Reader)
	} else {
		userRepo, err = user.NewSQLiteRepository(s.DB)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize user repository: %w", err)
	}
	s.UserRepo = userRepo
	trace.Mark("user_repo_ready")

	passwordHasher := password.NewHasher(&password.Config{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	})
	passwordPolicy := password.NewPolicy(&password.PolicyConfig{
		MinLength:        appCfg.Security.Password.MinLength,
		RequireUppercase: appCfg.Security.Password.RequireUppercase,
		RequireLowercase: appCfg.Security.Password.RequireLowercase,
		RequireNumber:    appCfg.Security.Password.RequireNumber,
		RequireSpecial:   appCfg.Security.Password.RequireSpecial,
	})
	s.UserService = user.NewService(userRepo, passwordHasher, passwordPolicy, nil)
	trace.Mark("user_service_ready")

	s.JWTService = auth.NewJWTService(&auth.JWTConfig{
		Secret:            appCfg.Security.JWT.Secret,
		Expiration:        appCfg.Security.JWT.Expiration,
		RefreshExpiration: appCfg.Security.JWT.RefreshExpiration,
		Issuer:            appCfg.Security.JWT.Issuer,
	})
	trace.Mark("jwt_ready")

	if s.DBConn != nil {
		s.APIKeyService, err = auth.NewAPIKeyServiceWithReadDB(s.DBConn.Writer, s.DBConn.Reader)
	} else {
		s.APIKeyService, err = auth.NewAPIKeyServiceWithDB(s.DB)
	}
	if err != nil {
		return fmt.Errorf("failed to initialize API key service: %w", err)
	}
	trace.Mark("api_key_service_ready")
	return nil
}
