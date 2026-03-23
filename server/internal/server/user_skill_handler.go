package server

import (
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skillbundle"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// UserSkillHandler handles per-user skill enable/disable.
type UserSkillHandler struct {
	db        *sql.DB
	skillsDir string // {dataDir}/workspace/.claude/skills/
}

// NewUserSkillHandler creates a new user skill handler and runs migrations.
func NewUserSkillHandler(db *sql.DB, skillsDir string) (*UserSkillHandler, error) {
	h := &UserSkillHandler{db: db, skillsDir: skillsDir}
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
	ID   string
	Name string
}

// listInstalledSkills scans the skills directory for installed skills.
func (h *UserSkillHandler) listInstalledSkills() []installedSkillInfo {
	if h.skillsDir == "" {
		return nil
	}
	entries, err := os.ReadDir(h.skillsDir)
	if err != nil {
		return nil
	}
	var skills []installedSkillInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillDir := filepath.Join(h.skillsDir, entry.Name())
		entryDoc, err := skillbundle.FindEntryDocumentInDir(skillDir)
		if err != nil {
			continue
		}
		name := entry.Name()
		// Try to parse name from frontmatter
		if data, err := os.ReadFile(entryDoc.Path); err == nil {
			if parsed := parseFrontmatterField(string(data), "name"); parsed != "" {
				name = parsed
			}
		}
		skills = append(skills, installedSkillInfo{ID: entry.Name(), Name: name})
	}
	return skills
}

// parseFrontmatterField extracts a field value from YAML frontmatter.
func parseFrontmatterField(content, field string) string {
	if !strings.HasPrefix(content, "---") {
		return ""
	}
	end := strings.Index(content[3:], "---")
	if end < 0 {
		return ""
	}
	fm := content[3 : 3+end]
	for _, line := range strings.Split(fm, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, field+":") {
			val := strings.TrimSpace(strings.TrimPrefix(line, field+":"))
			if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') {
				val = val[1 : len(val)-1]
			}
			return val
		}
	}
	return ""
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
	rows, err := h.db.QueryContext(c.Request().Context(),
		"SELECT skill_id, enabled FROM user_skill_configs WHERE user_id = ?", userID,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var skillID string
			var enabled bool
			if err := rows.Scan(&skillID, &enabled); err == nil {
				userConfigs[skillID] = enabled
				userToggled[skillID] = true
			}
		}
	}

	result := make([]UserSkillResponse, 0, len(skills))
	for _, s := range skills {
		enabled := true // default enabled
		toggled := false
		if v, ok := userConfigs[s.ID]; ok {
			enabled = v
			toggled = true
		}
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

	var req ToggleRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	now := timeutil.NowTime()

	// Upsert
	_, err := h.db.ExecContext(c.Request().Context(),
		`INSERT INTO user_skill_configs (id, user_id, skill_id, enabled, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, skill_id) DO UPDATE SET enabled = ?, updated_at = ?`,
		uuid.New().String(), userID, skillID, req.Enabled, now, now,
		req.Enabled, now,
	)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update skill config")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"skill_id": skillID,
		"enabled":  req.Enabled,
	})
}
