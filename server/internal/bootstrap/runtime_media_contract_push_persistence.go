package bootstrap

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/permission"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/webpush"
	z "github.com/IceWhaleTech/zorm"
	"github.com/google/uuid"
)

func bindRouteRuntimeMediaWebPush(
	v1 *echo.Group,
	deps *RoutesDeps,
	mediaManager *mediagen.Manager,
	locale string,
	logger *zap.Logger,
	services *Services,
	requirePagePermission func(string) echo.MiddlewareFunc,
) {
	if deps == nil || mediaManager == nil || services == nil {
		return
	}

	wpPriv, wpPub, wpErr := webpush.GetOrCreateVAPIDKeys(deps.ConfigKV)
	if wpErr != nil {
		if logger != nil {
			logger.Warn("Failed to initialize VAPID keys", zap.Error(wpErr))
		}
		return
	}

	wpStoreFactory := webpush.NewStore
	if services.DBConn != nil {
		wpStoreFactory = func(db *sql.DB) (*webpush.Store, error) {
			return webpush.NewStoreWithReadDB(services.DBConn.Writer, services.DBConn.Reader)
		}
	}
	wpStore, wpStoreErr := wpStoreFactory(services.DB)
	if wpStoreErr != nil {
		if logger != nil {
			logger.Warn("Failed to initialize webpush store", zap.Error(wpStoreErr))
		}
		return
	}

	if v1 != nil {
		wpHandler := webpush.NewHandler(wpStore, wpPub)
		authMiddleware := routeRuntimeAuthMiddleware(deps.AuthMiddleware)
		pageMiddleware := routeRuntimePageMiddleware(requirePagePermission, permission.PageProfile)
		wpHandler.RegisterRoutes(v1.Group("/webpush", filterRouteMiddlewares(authMiddleware, pageMiddleware)...))
	}

	wpSender := webpush.NewSender(wpPriv, wpPub, wpStore, logger)
	isCN := strings.HasPrefix(locale, "zh")
	mediaManager.SetOnTaskDone(func(taskID, status, model, imageURL string) {
		var title, body string
		if isCN {
			title = "媒体生成"
			if status == "failed" {
				body = model + " 生成失败"
			} else {
				body = model + " 生成完成"
			}
		} else {
			title = "Media Generation"
			if status == "failed" {
				body = model + " failed"
			} else {
				body = model + " completed"
			}
		}
		go wpSender.SendToAll(context.Background(), title, body, imageURL)
	})
}

func configureRouteRuntimeMediaPersistence(
	handler *mediagen.Handler,
	deps *RoutesDeps,
	services *Services,
) {
	if handler == nil || deps == nil || services == nil {
		return
	}

	resolveMediaConversationScope := func(ctx context.Context, conversationID string) (string, error) {
		if services.MemoryStore == nil {
			return "", nil
		}
		claims, _ := ctx.Value(auth.UserContextKey).(*auth.UserClaims)
		if claims != nil && claims.Role != "admin" {
			if _, err := services.MemoryStore.GetConversation(ctx, conversationID, claims.UserID); err != nil {
				if err == memory.ErrNotFound {
					return "", echo.NewHTTPError(http.StatusNotFound, "conversation not found")
				}
				return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
			}
			return claims.UserID, nil
		}
		conv, err := services.MemoryStore.GetConversation(ctx, conversationID)
		if err != nil {
			if err == memory.ErrNotFound {
				return "", echo.NewHTTPError(http.StatusNotFound, "conversation not found")
			}
			return "", echo.NewHTTPError(http.StatusInternalServerError, "failed to get conversation")
		}
		if claims == nil && strings.TrimSpace(conv.UserID) != "" {
			return "", echo.NewHTTPError(http.StatusNotFound, "conversation not found")
		}
		return strings.TrimSpace(conv.UserID), nil
	}
	handler.SetResolveConversationScope(resolveMediaConversationScope)

	handler.SetAddMessage(func(ctx context.Context, conversationID, role, content string) (string, error) {
		scopedUserID, err := resolveMediaConversationScope(ctx, conversationID)
		if err != nil {
			return "", err
		}
		if services.MemoryStore != nil {
			msg, err := services.MemoryStore.AddMessage(ctx, conversationID, memory.Message{Role: role, Content: content}, scopedUserID)
			if err != nil {
				return "", err
			}
			return msg.ID, nil
		}
		msgID := uuid.New().String()
		now := time.Now().UTC().Format(time.RFC3339Nano)
		_, err = z.TableContext(ctx, deps.DB, "messages").Insert(z.V{
			"id":              msgID,
			"conversation_id": conversationID,
			"role":            role,
			"content":         content,
			"created_at":      now,
		})
		if err != nil {
			return "", err
		}
		return msgID, nil
	})

	handler.SetUpdateTitle(func(ctx context.Context, conversationID, title string) error {
		scopedUserID, err := resolveMediaConversationScope(ctx, conversationID)
		if err != nil {
			return err
		}
		if services.MemoryStore != nil {
			return services.MemoryStore.UpdateConversationTitle(ctx, conversationID, title, scopedUserID)
		}
		_, err = z.TableContext(ctx, deps.DB, "conversations").Update(
			z.V{
				"title":      title,
				"updated_at": time.Now().UTC().Format(time.RFC3339Nano),
			},
			z.Where(z.Eq("id", conversationID)),
		)
		return err
	})
}
