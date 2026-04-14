package bootstrap

import (
	"context"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/autoreply"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

type routeRuntimeContractChannelOptions struct {
	api                   *echo.Group
	authMiddleware        echo.MiddlewareFunc
	requirePagePermission func(string) echo.MiddlewareFunc
	config                *config.Config
	serverPort            int
	logger                *zap.Logger
	chat                  *serverpkg.ChatHandler
	channelConfigStore    *serverpkg.ChannelConfigStore
	channelTaskWatcher    *mediagen.ChannelTaskWatcher
	ngrokTunnelMgr        *ngrok.SDKTunnelManager
	tunnelHandler         runtimeChannelTunnelSetup
	jwtService            *auth.JWTService
	autoreplyService      *autoreply.Service
	mgmtTool              *tools.MgmtTool
}

type runtimeChannelSender interface {
	Send(ctx context.Context, channelName string, msg channel.OutgoingMessage) error
	SendWithID(ctx context.Context, channelName string, msg channel.OutgoingMessage) (string, error)
	UpdateMessage(ctx context.Context, channelName string, chatID string, messageID string, msg channel.OutgoingMessage) error
}

type runtimeChannelChatTarget interface {
	SetChannelSender(sender func(ctx context.Context, channelName string, out channel.OutgoingMessage) error)
	SetChannelSenderWithID(sender func(ctx context.Context, channelName string, out channel.OutgoingMessage) (string, error))
	SetChannelMessageUpdater(updater func(ctx context.Context, channelName string, chatID string, messageID string, out channel.OutgoingMessage) error)
}

type runtimeChannelChatProcessor interface {
	ProcessChannelMessage(ctx context.Context, msg channel.Message) (string, error)
}

type runtimeChannelWatcherTarget interface {
	SetNotifier(notifier mediagen.ChannelNotifier)
	SetURLResolver(resolver mediagen.URLResolver)
}

type runtimeChannelHandlerTarget interface {
	SetHandler(handler channel.MessageHandler)
}
type runtimeChannelAutoreplyTarget interface {
	Match(ctx context.Context, message, channel, userID, username, chatID string) (string, *autoreply.Rule, error)
}

type runtimeChannelLifecycleManager interface {
	Register(ch channel.Channel) error
	StartChannel(ctx context.Context, name string) error
}

type runtimeChannelFactory interface {
	CreateChannel(cfg *serverpkg.ChannelConfig) (channel.Channel, error)
}

type runtimeChannelConfigUpdater interface {
	Set(id string, cfg *serverpkg.ChannelConfig) error
}
