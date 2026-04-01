package server

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillmanifest"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// UserSkillHandler handles per-user skill enable/disable.
type UserSkillHandler struct {
	db        *sql.DB
	readDB    *sql.DB
	skillsDir string // active managed install dir, typically {dataDir}/workspace/.claude/skills/
}

// NewUserSkillHandler creates a new user skill handler and runs migrations.
func NewUserSkillHandler(db *sql.DB, skillsDir string) (*UserSkillHandler, error) {
	return NewUserSkillHandlerWithReadDB(db, db, skillsDir)
}

// NewUserSkillHandlerWithReadDB creates a new user skill handler with separate
// write and read database handles.
func NewUserSkillHandlerWithReadDB(writeDB, readDB *sql.DB, skillsDir string) (*UserSkillHandler, error) {
	if readDB == nil {
		readDB = writeDB
	}
	h := &UserSkillHandler{db: writeDB, readDB: readDB, skillsDir: skillsDir}
	if err := h.migrate(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *UserSkillHandler) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS user_skill_configs (
		id TEXT PRIMARY KEY,
		user_id TEXT NOT NULL,
		skill_id TEXT NOT NULL,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL,
		UNIQUE(user_id, skill_id)
	);
	CREATE INDEX IF NOT EXISTS idx_user_skill_user_id ON user_skill_configs(user_id);
	`
	_, err := h.db.Exec(schema)
	return err
}

// RegisterRoutes registers the user skill API routes.
func (h *UserSkillHandler) RegisterRoutes(g *echo.Group) {
	g.GET("", h.List)
	g.PUT("/:id", h.Toggle)
}

type userSkillConfigRow struct {
	SkillID string `json:"skill_id" zorm:"skill_id"`
	Enabled bool   `json:"enabled" zorm:"enabled"`
}

func (h *UserSkillHandler) writeTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, h.db, "user_skill_configs")
}

func (h *UserSkillHandler) readTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, h.readDB, "user_skill_configs")
}

// UserSkillResponse represents a skill with user-specific enabled state.
type UserSkillResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Category    string `json:"category"`
	Author      string `json:"author"`
	Enabled     bool   `json:"enabled"`
	Installed   bool   `json:"installed"`
	UserToggled bool   `json:"user_toggled"` // true if user has explicit config
}

// installedSkillInfo holds minimal info parsed from SKILL.md frontmatter.
type installedSkillInfo struct {
	ID            string
	Name          string
	Aliases       []string
	UserInvocable bool
}

func (h *UserSkillHandler) skillRoots() []string {
	if strings.TrimSpace(h.skillsDir) == "" {
		return nil
	}
	return skillmanifest.ResolvePeerRootsForManagedDir(h.skillsDir)
}

// listInstalledSkills scans the active managed directory plus compatible peer
// roots for installed skills.
func (h *UserSkillHandler) listInstalledSkills() []installedSkillInfo {
	roots := h.skillRoots()
	if len(roots) == 0 {
		return nil
	}
	var skills []installedSkillInfo
	seen := make(map[string]struct{})
	for _, root := range roots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			skillDir := filepath.Join(root, entry.Name())
			bundle, err := skillmanifest.ValidateInstalledDir(skillDir, "", skillmanifest.Options{})
			if err != nil {
				continue
			}
			id := userSkillCanonicalID(bundle.Document, entry.Name())
			if _, ok := seen[id]; ok {
				continue
			}
			if !bundle.Document.UserInvocable {
				continue
			}
			seen[id] = struct{}{}
			name := firstString(strings.TrimSpace(bundle.Document.Name), id, entry.Name())
			skills = append(skills, installedSkillInfo{
				ID:            id,
				Name:          name,
				Aliases:       userSkillAliases(id, bundle.Document, entry.Name()),
				UserInvocable: bundle.Document.UserInvocable,
			})
		}
	}
	return skills
}

func userSkillCanonicalID(doc skillmanifest.Document, dirName string) string {
	for _, candidate := range []string{
		strings.TrimSpace(doc.ID),
		strings.TrimSpace(doc.Name),
		strings.TrimSpace(dirName),
	} {
		if candidate == "" {
			continue
		}
		if normalized, err := normalizedSkillID(candidate); err == nil {
			return normalized
		}
	}
	return firstString(strings.TrimSpace(doc.ID), strings.TrimSpace(doc.Name), strings.TrimSpace(dirName))
}

func userSkillAliases(id string, doc skillmanifest.Document, dirName string) []string {
	seen := make(map[string]struct{}, 8)
	aliases := make([]string, 0, 8)
	add := func(values ...string) {
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			aliases = append(aliases, value)
		}
	}

	add(id)
	add(skillIDAliases(id)...)
	for _, raw := range []string{
		strings.TrimSpace(doc.ID),
		strings.TrimSpace(doc.Name),
		strings.TrimSpace(dirName),
	} {
		add(raw)
		add(skillIDAliases(raw)...)
	}
	return aliases
}

func lookupUserSkillConfig(userConfigs map[string]bool, userToggled map[string]bool, aliases []string) (bool, bool) {
	for _, alias := range aliases {
		if enabled, ok := userConfigs[alias]; ok {
			return enabled, true
		}
		if userToggled[alias] {
			return false, true
		}
	}
	return true, false
}

func (h *UserSkillHandler) resolveInstalledSkillID(rawID string) (string, bool, error) {
	rawID = strings.TrimSpace(rawID)
	roots := h.skillRoots()
	if rawID == "" || len(roots) == 0 {
		return rawID, false, nil
	}
	resolved, ok, err := skillmanifest.FindAnyByCandidatesStrict(skillmanifest.CandidateIDs(rawID), roots, "", skillmanifest.Options{})
	if err != nil {
		return rawID, false, err
	}
	if !ok {
		return rawID, false, nil
	}
	return userSkillCanonicalID(resolved.Document, filepath.Base(resolved.EntryDir)), true, nil
}

// List returns all installed skills with user-specific enabled state.
func (h *UserSkillHandler) List(c echo.Context) error {
	userID := getUserIDFromContext(c)
	if userID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}

	// Get installed skills from filesystem
	skills := h.listInstalledSkills()

	// Get user's skill configs
	userConfigs := make(map[string]bool)
	userToggled := make(map[string]bool)
	var rows []userSkillConfigRow
	_, err := h.readTable(c.Request().Context()).Select(
		&rows,
		z.Fields("skill_id", "enabled"),
		z.Where(z.Eq("user_id", userID)),
	)
	if err == nil {
		for _, row := range rows {
			userConfigs[row.SkillID] = row.Enabled
			userToggled[row.SkillID] = true
		}
	}

	result := make([]UserSkillResponse, 0, len(skills))
	for _, s := range skills {
		enabled, toggled := lookupUserSkillConfig(userConfigs, userToggled, s.Aliases)
		result = append(result, UserSkillResponse{
			ID:          s.ID,
			Name:        s.Name,
			Enabled:     enabled,
			Installed:   true,
			UserToggled: toggled,
		})
	}

	return c.JSON(http.StatusOK, result)
}

// ToggleRequest is the request body for toggling a skill.
type ToggleRequest struct {
	Enabled bool `json:"enabled"`
}

// Toggle sets the enabled state for a skill for the current user.
func (h *UserSkillHandler) Toggle(c echo.Context) error {
	claims := auth.GetUserFromContext(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "authentication required")
	}
	userID := claims.UserID
	skillID := c.Param("id")
	if resolvedID, ok, err := h.resolveInstalledSkillID(skillID); err != nil {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	} else if ok {
		skillID = resolvedID
	}

	var req ToggleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	now := timeutil.NowTime()

	// Upsert
	_, err := h.writeTable(c.Request().Context()).Insert(map[string]interface{}{
		"id":         uuid.New().String(),
		"user_id":    userID,
		"skill_id":   skillID,
		"enabled":    boolToSQLiteInt(req.Enabled),
		"created_at": now,
		"updated_at": now,
	}, z.OnConflictDoUpdateSet(
		[]string{"user_id", "skill_id"},
		[]string{"enabled", "updated_at"},
	))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update skill config")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"skill_id": skillID,
		"enabled":  req.Enabled,
	})
}
