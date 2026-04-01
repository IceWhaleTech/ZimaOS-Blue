package bootstrap

import (
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func registerBrowserApprovalRoutes(v1 *echo.Group, authMiddleware, pageMiddleware echo.MiddlewareFunc, browserSiteStore *tools.BrowserSiteAllowlistStore) {
	if v1 == nil || browserSiteStore == nil {
		return
	}

	browserGroup := v1.Group("/browser", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	browserGroup.GET("/approvals/sites", func(c echo.Context) error {
		userID := resolveRequestUserID(c)
		entries, err := browserSiteStore.List()
		if err != nil {
			return c.JSON(500, map[string]string{"error": "failed to load approved browser sites"})
		}

		type siteEntry struct {
			ID         string `json:"id"`
			Origin     string `json:"origin"`
			AddedAt    string `json:"added_at"`
			LastUsed   string `json:"last_used"`
			ApprovedBy string `json:"approved_by,omitempty"`
		}
		out := make([]siteEntry, 0, len(entries))
		for _, entry := range entries {
			approvedBy := strings.TrimSpace(entry.ApprovedBy)
			if approvedBy != "" && userID != "default" && userID != approvedBy {
				continue
			}
			out = append(out, siteEntry{
				ID:         entry.ID,
				Origin:     entry.Origin,
				AddedAt:    entry.AddedAt.UTC().Format(time.RFC3339),
				LastUsed:   entry.LastUsed.UTC().Format(time.RFC3339),
				ApprovedBy: approvedBy,
			})
		}

		return c.JSON(200, map[string]interface{}{
			"entries": out,
		})
	})

	browserGroup.DELETE("/approvals/sites/:id", func(c echo.Context) error {
		id := strings.TrimSpace(c.Param("id"))
		if id == "" {
			return c.JSON(400, map[string]string{"error": "site id is required"})
		}

		userID := resolveRequestUserID(c)
		entries, err := browserSiteStore.List()
		if err != nil {
			return c.JSON(500, map[string]string{"error": "failed to load approved browser sites"})
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
			return c.JSON(404, map[string]string{"error": "browser site approval not found"})
		}
		if err := browserSiteStore.Delete(id); err != nil {
			return c.JSON(500, map[string]string{"error": "failed to revoke browser site approval"})
		}
		return c.JSON(200, map[string]bool{"deleted": true})
	})
}
