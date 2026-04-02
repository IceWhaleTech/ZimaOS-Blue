package bootstrap

import (
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func registerExecApprovalRoutes(execGroup *echo.Group, execApprovals *tools.ApprovalManager) {
	if execGroup == nil || execApprovals == nil {
		return
	}

	execGroup.GET("/approvals/pending", func(c echo.Context) error {
		sessionID := strings.TrimSpace(c.QueryParam("session_id"))
		var req *tools.ApprovalRequest
		if sessionID != "" {
			req = execApprovals.GetPendingBySession(sessionID)
		}
		if req == nil {
			userID := resolveRequestUserID(c)
			req = execApprovals.GetPending(userID)
		}
		if req == nil {
			return c.JSON(200, map[string]interface{}{"pending": false})
		}
		return c.JSON(200, map[string]interface{}{"pending": true, "approval": req})
	})

	execGroup.POST("/approvals/:id", func(c echo.Context) error {
		id := c.Param("id")
		var body struct {
			Decision    string `json:"decision"`
			BindingHash string `json:"binding_hash"`
		}
		if err := c.Bind(&body); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid body"})
		}
		decision := tools.ApprovalDecision(body.Decision)
		if decision != tools.ApprovalAllowOnce && decision != tools.ApprovalAllowAlways && decision != tools.ApprovalDeny {
			return c.JSON(400, map[string]string{"error": "invalid decision; use allow-once, allow-always, or deny"})
		}
		if !execApprovals.ResolveApprovalWithBinding(id, decision, body.BindingHash) {
			return c.JSON(404, map[string]string{"error": "approval not found or expired"})
		}
		return c.JSON(200, map[string]string{"status": string(decision)})
	})
}

func registerExecDirectoryApprovalRoutes(execGroup *echo.Group, execDirStore *tools.DirAllowlistStore) {
	if execGroup == nil || execDirStore == nil {
		return
	}

	execGroup.GET("/approvals/directories", func(c echo.Context) error {
		userID := resolveRequestUserID(c)
		entries, err := execDirStore.List()
		if err != nil {
			return c.JSON(500, map[string]string{"error": "failed to load approved directories"})
		}

		type directoryEntry struct {
			ID         string `json:"id"`
			Path       string `json:"path"`
			AddedAt    string `json:"added_at"`
			LastUsed   string `json:"last_used"`
			ApprovedBy string `json:"approved_by,omitempty"`
		}
		out := make([]directoryEntry, 0, len(entries))
		for _, entry := range entries {
			approvedBy := strings.TrimSpace(entry.ApprovedBy)
			if approvedBy != "" && userID != "default" && userID != approvedBy {
				continue
			}
			out = append(out, directoryEntry{
				ID:         entry.ID,
				Path:       entry.Path,
				AddedAt:    entry.AddedAt.UTC().Format(time.RFC3339),
				LastUsed:   entry.LastUsed.UTC().Format(time.RFC3339),
				ApprovedBy: approvedBy,
			})
		}

		return c.JSON(200, map[string]interface{}{
			"entries": out,
		})
	})

	execGroup.DELETE("/approvals/directories/:id", func(c echo.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.JSON(400, map[string]string{"error": "directory id is required"})
		}

		userID := resolveRequestUserID(c)
		entries, err := execDirStore.List()
		if err != nil {
			return c.JSON(500, map[string]string{"error": "failed to load approved directories"})
		}
		found := false
		for _, entry := range entries {
			if entry.ID != id {
				continue
			}
			found = true
			approvedBy := strings.TrimSpace(entry.ApprovedBy)
			if approvedBy != "" && userID != "default" && userID != approvedBy {
				return c.JSON(403, map[string]string{"error": "forbidden"})
			}
			break
		}
		if !found {
			return c.JSON(404, map[string]string{"error": "directory approval not found"})
		}
		if err := execDirStore.Delete(id); err != nil {
			return c.JSON(500, map[string]string{"error": "failed to revoke directory approval"})
		}
		return c.JSON(200, map[string]bool{"deleted": true})
	})
}
