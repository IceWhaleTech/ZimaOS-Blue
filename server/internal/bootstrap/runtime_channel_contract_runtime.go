package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/ngrok"
	serverpkg "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/server"
)

func newRouteRuntimeChannelManager(options routeRuntimeContractChannelOptions) *channel.Manager {
	return channel.NewManager(
		resolveRouteRuntimeChannelConfig(options.config, options.channelConfigStore),
		options.logger,
	)
}

func resolveRouteRuntimeChannelConfig(cfg *config.Config, store *serverpkg.ChannelConfigStore) channel.Config {
	channelCfg := channel.DefaultConfig()
	if cfg != nil {
		channelCfg = cfg.Channels
	}
	if store != nil {
		if settings, ok := store.GetSettings(); ok {
			channelCfg = serverpkg.ApplyChannelSettings(channelCfg, settings)
		}
	}
	return channelCfg
}

func bindRouteRuntimeChannelChatTarget(chat runtimeChannelChatTarget, manager runtimeChannelSender) {
	if !routeRuntimeHasValue(chat) || manager == nil {
		return
	}
	chat.SetChannelSender(func(ctx context.Context, channelName string, out channel.OutgoingMessage) error {
		return manager.Send(ctx, channelName, out)
	})
	chat.SetChannelSenderWithID(func(ctx context.Context, channelName string, out channel.OutgoingMessage) (string, error) {
		return manager.SendWithID(ctx, channelName, out)
	})
	chat.SetChannelMessageUpdater(func(ctx context.Context, channelName string, chatID string, messageID string, out channel.OutgoingMessage) error {
		return manager.UpdateMessage(ctx, channelName, chatID, messageID, out)
	})
}

func bindRouteRuntimeChannelWatcherTarget(
	watcher runtimeChannelWatcherTarget,
	manager runtimeChannelSender,
	resolver mediagen.URLResolver,
) {
	if !routeRuntimeHasValue(watcher) || manager == nil {
		return
	}
	watcher.SetNotifier(func(ctx context.Context, channelName string, msg channel.OutgoingMessage) error {
		return manager.Send(ctx, channelName, msg)
	})
	if resolver != nil {
		watcher.SetURLResolver(resolver)
	}
}

func newRouteRuntimeChannelURLResolver(tunnelMgr *ngrok.SDKTunnelManager, serverPort int) mediagen.URLResolver {
	if tunnelMgr == nil {
		return nil
	}
	return &mediagen.SimpleURLResolver{
		TunnelGetURL: tunnelMgr.GetURL,
		ServerPort:   serverPort,
	}
}

func bindRouteRuntimeChannelHandler(
	target runtimeChannelHandlerTarget,
	chat runtimeChannelChatProcessor,
	autoreplySvc runtimeChannelAutoreplyTarget,
) {
	if !routeRuntimeHasValue(target) {
		return
	}
	target.SetHandler(func(ctx context.Context, msg channel.Message) (*channel.OutgoingMessage, error) {
		if autoreplySvc != nil {
			response, rule, err := autoreplySvc.Match(ctx, msg.Content, msg.ChannelName, msg.UserID, "", msg.ChatID)
			if err == nil && response != "" && rule != nil {
				out := buildRuntimeChannelReply(msg, response)
				return &out, nil
			}
		}
		if chat != nil {
			aiResponse, err := chat.ProcessChannelMessage(ctx, msg)
			if err != nil {
				return nil, err
			}
			if aiResponse != "" {
				out := buildRuntimeChannelReply(msg, aiResponse)
				return &out, nil
			}
		}
		return nil, nil
	})
}
