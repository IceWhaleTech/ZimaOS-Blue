package server

import (
	"context"
	"strings"
)

type channelReplyMetadataContextKey string

const channelReplyMetadataKey channelReplyMetadataContextKey = "channel_reply_metadata"

func withChannelReplyMetadata(ctx context.Context, metadata map[string]interface{}) context.Context {
	filtered := channelReplyMetadataSubset(metadata)
	if len(filtered) == 0 {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, channelReplyMetadataKey, filtered)
}

func channelReplyMetadataFromContext(ctx context.Context) map[string]interface{} {
	if ctx == nil {
		return nil
	}
	raw, _ := ctx.Value(channelReplyMetadataKey).(map[string]interface{})
	return channelReplyMetadataSubset(raw)
}

func mergeChannelReplyMetadata(ctx context.Context, metadata map[string]interface{}) map[string]interface{} {
	replyMetadata := channelReplyMetadataFromContext(ctx)
	if len(metadata) == 0 {
		return replyMetadata
	}
	if replyMetadata == nil {
		replyMetadata = make(map[string]interface{}, len(metadata))
	}
	for key, value := range metadata {
		replyMetadata[key] = value
	}
	return replyMetadata
}

func channelReplyMetadataSubset(metadata map[string]interface{}) map[string]interface{} {
	if len(metadata) == 0 {
		return nil
	}
	var out map[string]interface{}
	for _, key := range []string{"context_token", "session_id"} {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		if text, ok := value.(string); ok {
			text = strings.TrimSpace(text)
			if text == "" {
				continue
			}
			value = text
		}
		if out == nil {
			out = make(map[string]interface{}, 2)
		}
		out[key] = value
	}
	return out
}
