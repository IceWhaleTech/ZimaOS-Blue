package bootstrap

import "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"

func buildRuntimeChannelReply(msg channel.Message, content string) channel.OutgoingMessage {
	out := channel.OutgoingMessage{ChatID: msg.ChatID, Content: content}
	if metadata := runtimeChannelReplyMetadata(msg.Metadata); len(metadata) > 0 {
		out.Metadata = metadata
	}
	return out
}

func runtimeChannelReplyMetadata(metadata map[string]interface{}) map[string]interface{} {
	if len(metadata) == 0 {
		return nil
	}
	var out map[string]interface{}
	for _, key := range []string{"context_token", "session_id"} {
		value, ok := metadata[key]
		if !ok {
			continue
		}
		if out == nil {
			out = make(map[string]interface{}, 2)
		}
		out[key] = value
	}
	return out
}
