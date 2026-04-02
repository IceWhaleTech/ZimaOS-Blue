package bootstrap

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func registerAskUserQuestionRoutes(v1 *echo.Group, authMiddleware, pageMiddleware echo.MiddlewareFunc, questionMgr *tools.QuestionManager) {
	if v1 == nil || questionMgr == nil {
		return
	}

	askGroup := v1.Group("/ask-user-question", filterRouteMiddlewares(authMiddleware, pageMiddleware)...)
	askGroup.GET("/pending", func(c echo.Context) error {
		sessionID := strings.TrimSpace(c.QueryParam("session_id"))
		var req *tools.QuestionRequest
		if sessionID != "" {
			req = questionMgr.GetPendingBySession(sessionID)
		}
		if req == nil {
			userID := resolveRequestUserID(c)
			req = questionMgr.GetPending(userID)
		}
		if req == nil {
			return c.JSON(200, map[string]interface{}{"pending": false})
		}
		return c.JSON(200, map[string]interface{}{"pending": true, "question": req})
	})
	askGroup.POST("/:id/answer", func(c echo.Context) error {
		id := c.Param("id")
		var body struct {
			Answers []tools.QuestionAnswerResult `json:"answers"`
		}
		if err := c.Bind(&body); err != nil {
			return c.JSON(400, map[string]string{"error": "invalid body"})
		}
		if !questionMgr.ResolveAnswer(id, body.Answers) {
			return c.JSON(404, map[string]string{"error": "question not found or expired"})
		}
		return c.JSON(200, map[string]string{"status": "ok"})
	})
	askGroup.POST("/:id/dismiss", func(c echo.Context) error {
		id := c.Param("id")
		if !questionMgr.DismissQuestion(id) {
			return c.JSON(404, map[string]string{"error": "question not found or expired"})
		}
		return c.JSON(200, map[string]string{"status": "dismissed"})
	})
}
