package bootstrap

import (
	"context"
	"database/sql"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/cron"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/inject"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/memory"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/push"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sockipc"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/sse"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/webpush"
)

// InitWebPushSender creates a Web Push sender with VAPID keys.
// Returns nil if initialization fails (non-fatal).
func InitWebPushSender(db *sql.DB, kv kvstore.Store, logger *zap.Logger) *webpush.Sender {
	return InitWebPushSenderWithReadDB(db, db, kv, logger)
}

// InitWebPushSenderWithReadDB creates a Web Push sender with separate write
// and read database handles.
func InitWebPushSenderWithReadDB(writeDB, readDB *sql.DB, kv kvstore.Store, logger *zap.Logger) *webpush.Sender {
	vapidPriv, vapidPub, err := webpush.GetOrCreateVAPIDKeys(kv)
	if err != nil {
		logger.Warn("Failed to initialize VAPID keys", zap.Error(err))
		return nil
	}
	wpStore, err := webpush.NewStoreWithReadDB(writeDB, readDB)
	if err != nil {
		logger.Warn("Failed to initialize webpush store", zap.Error(err))
		return nil
	}
	return webpush.NewSender(vapidPriv, vapidPub, wpStore, logger)
}

// PushServiceDeps holds dependencies for InitPushService.
type PushServiceDeps struct {
	DB          *sql.DB
	ReadDB      *sql.DB
	MemoryStore *memory.Store
	CronGetSvc  func() *cron.Service // lazy cron getter (may return nil)
	SSEBroker   *sse.Broker
	WPSender    *webpush.Sender // optional, may be nil
	Logger      *zap.Logger
}

// PushServiceResult holds the outputs of InitPushService.
type PushServiceResult struct {
	IPC     sockipc.PushBackend // for IPC registration
	Service *push.Service       // for skill/tool wiring
}

// InitPushService creates and wires the push notification service.
// Returns nil if initialization fails.
func InitPushService(deps *PushServiceDeps) *PushServiceResult {
	pushStore, err := push.NewStoreWithReadDB(deps.DB, deps.ReadDB)
	if err != nil {
		deps.Logger.Warn("Failed to initialize push store", zap.Error(err))
		return nil
	}

	pushSvc := push.NewService(pushStore, deps.Logger)

	// Wire cron for scheduled firing
	cronAdapter := push.NewCronAdapter(deps.CronGetSvc)
	pushSvc.SetCron(cronAdapter)

	// Wire message injector for conversation delivery
	pushSvc.SetMessageInjector(inject.NewMemoryStoreInjector(deps.MemoryStore))

	// Wire SSE event publisher
	if deps.SSEBroker != nil {
		pushSvc.SetEventPublisher(deps.SSEBroker)
	}

	// Wire native OS notifier
	pushSvc.SetNotifier(push.NewNotifier(deps.Logger))

	// Wire Web Push sender
	if deps.WPSender != nil {
		pushSvc.SetWebPushSender(deps.WPSender)
	}

	// Restore pending push notifications from previous session
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := pushSvc.RestorePending(ctx); err != nil {
			deps.Logger.Warn("Failed to restore pending push notifications", zap.Error(err))
		} else {
			deps.Logger.Info("Pending push notifications restored")
		}
	}()

	return &PushServiceResult{
		IPC:     sockipc.NewPushIPCAdapter(pushSvc),
		Service: pushSvc,
	}
}
