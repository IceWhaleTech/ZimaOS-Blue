package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/gateway"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

func registerRouteRuntimeGatewayMethods(gw *gateway.Gateway, options routeRuntimeContractGatewayOptions) {
	if gw == nil {
		return
	}

	browserGatewayTool := tools.NewBrowserTool()
	if options.browserBackend != nil {
		browserGatewayTool.SetBackend(options.browserBackend)
	}
	if mediaDir := strings.TrimSpace(options.mediaDir); mediaDir != "" {
		browserGatewayTool.SetMediaDir(mediaDir)
	}

	gw.RegisterHandler("chat.send", func(ctx context.Context, conn *gateway.Connection, msg *gateway.Message) (*gateway.Message, error) {
		if options.chat == nil {
			return nil, fmt.Errorf("chat handler not configured")
		}
		var req struct {
			ConversationID string `json:"conversation_id"`
			Content        string `json:"content"`
		}
		if err := decodeRouteRuntimeGatewayPayload(msg, &req); err != nil {
			return nil, err
		}
		respContent, err := options.chat.GatewaySend(ctx, req.ConversationID, req.Content, conn.UserID)
		if err != nil {
			return nil, err
		}
		return gatewayRouteRuntimeMessageWithPayload(map[string]interface{}{
			"conversation_id": req.ConversationID,
			"content":         respContent,
		})
	})

	gw.RegisterHandler("chat.abort", func(_ context.Context, _ *gateway.Connection, msg *gateway.Message) (*gateway.Message, error) {
		if options.chat == nil {
			return nil, fmt.Errorf("chat handler not configured")
		}
		var req struct {
			ConversationID string `json:"conversation_id"`
		}
		if err := decodeRouteRuntimeGatewayPayload(msg, &req); err != nil {
			return nil, err
		}
		streamID, cancelled := options.chat.CancelConversationStream(req.ConversationID)
		return gatewayRouteRuntimeMessageWithPayload(map[string]interface{}{
			"conversation_id": req.ConversationID,
			"stream_id":       streamID,
			"cancelled":       cancelled,
		})
	})

	gw.RegisterHandler("browser.request", func(ctx context.Context, _ *gateway.Connection, msg *gateway.Message) (*gateway.Message, error) {
		if options.browserBackend == nil {
			return nil, fmt.Errorf("browser backend not configured")
		}
		var req struct {
			Action string                 `json:"action"`
			Params map[string]interface{} `json:"params"`
		}
		if err := decodeRouteRuntimeGatewayPayload(msg, &req); err != nil {
			return nil, err
		}
		action := strings.TrimSpace(req.Action)
		if action == "" {
			return nil, fmt.Errorf("action is required")
		}

		args := make(map[string]interface{}, len(req.Params)+1)
		args["action"] = action
		for key, value := range req.Params {
			args[key] = value
		}

		result, err := browserGatewayTool.Execute(ctx, args)
		if err != nil {
			return nil, err
		}
		return gatewayRouteRuntimeMessageWithPayload(map[string]interface{}{
			"action": strings.ToLower(action),
			"result": result,
		})
	})

	gw.RegisterHandler("hooks.wake", func(ctx context.Context, _ *gateway.Connection, msg *gateway.Message) (*gateway.Message, error) {
		if options.pluginRegistry == nil {
			return nil, fmt.Errorf("plugin registry not configured")
		}
		var req struct {
			HookID  string      `json:"hook_id"`
			Payload interface{} `json:"payload"`
		}
		if err := decodeRouteRuntimeGatewayPayload(msg, &req); err != nil {
			return nil, err
		}
		if strings.TrimSpace(req.HookID) == "" {
			return nil, fmt.Errorf("hook_id is required")
		}
		if err := options.pluginRegistry.TriggerHook(ctx, req.HookID, req.Payload); err != nil {
			return nil, err
		}
		return gatewayRouteRuntimeMessageWithPayload(map[string]interface{}{
			"hook_id": req.HookID,
			"woke":    true,
		})
	})
}

func decodeRouteRuntimeGatewayPayload(msg *gateway.Message, out interface{}) error {
	if msg == nil {
		return fmt.Errorf("nil message")
	}
	if out == nil {
		return fmt.Errorf("nil payload target")
	}
	if len(msg.Payload) == 0 {
		return fmt.Errorf("payload is required")
	}
	if err := json.Unmarshal(msg.Payload, out); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}
	return nil
}

func gatewayRouteRuntimeMessageWithPayload(payload interface{}) (*gateway.Message, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return &gateway.Message{Payload: raw}, nil
}
