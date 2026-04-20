package mediagen

import (
	"testing"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/i18n"
	task "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
)

func TestChannelSource(t *testing.T) {
	tests := []struct {
		channelName string
		chatID      string
		want        string
	}{
		{"telegram", "12345", "channel:telegram:12345"},
		{"discord", "guild:chan", "channel:discord:guild:chan"},
		{"slack", "", "channel:slack:"},
	}
	for _, tt := range tests {
		got := ChannelSource(tt.channelName, tt.chatID)
		if got != tt.want {
			t.Errorf("ChannelSource(%q, %q) = %q, want %q", tt.channelName, tt.chatID, got, tt.want)
		}
	}
}

func TestIsChannelSource(t *testing.T) {
	tests := []struct {
		source string
		want   bool
	}{
		{"channel:telegram:123", true},
		{"channel:discord:", true},
		{"web", false},
		{"ipc", false},
		{"", false},
	}
	for _, tt := range tests {
		got := isChannelSource(tt.source)
		if got != tt.want {
			t.Errorf("isChannelSource(%q) = %v, want %v", tt.source, got, tt.want)
		}
	}
}

func TestFormatChannelResult(t *testing.T) {
	lang := i18n.LangEnUS

	tests := []struct {
		name        string
		task        *MediaTask
		wantContent string
		wantAttach  int
		wantType    channel.MessageType
	}{
		{
			name:        "nil task",
			task:        nil,
			wantContent: "",
		},
		{
			name: "failed task",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusFailed, Error: "rate limit exceeded"},
				Category: "t2i",
			},
			wantContent: "❌ Media generation failed: rate limited, please try again later",
		},
		{
			name: "cancelled task",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusCancelled},
			},
			wantContent: "🚫 Media generation was cancelled.",
		},
		{
			name: "succeeded image",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusSucceeded},
				Model:    "nano-banana-pro",
				Category: "t2i",
				Response: &MediaResponse{
					Data: []MediaResult{{URL: "https://example.com/cat.png"}},
				},
			},
			wantContent: "✅ Image generated (nano-banana-pro)",
			wantAttach:  1,
			wantType:    channel.MessageTypeImage,
		},
		{
			name: "succeeded video",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusSucceeded},
				Model:    "wan2.6-t2v",
				Category: "t2v",
				Response: &MediaResponse{
					Data: []MediaResult{{URL: "https://example.com/video.mp4"}},
				},
			},
			wantContent: "✅ Video generated (wan2.6-t2v)",
			wantAttach:  1,
			wantType:    channel.MessageTypeVideo,
		},
		{
			name: "succeeded no output",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusSucceeded},
				Response: &MediaResponse{Data: []MediaResult{}},
			},
			wantContent: "✅ Generation complete, but no output was returned.",
		},
		{
			name: "succeeded image with duration",
			task: func() *MediaTask {
				created := time.Now().Add(-65 * time.Second)
				completed := time.Now()
				return &MediaTask{
					BaseTask: task.BaseTask{Status: TaskStatusSucceeded, CreatedAt: created, CompletedAt: &completed},
					Model:    "nano-banana-pro",
					Category: "t2i",
					Response: &MediaResponse{
						Data: []MediaResult{{URL: "https://example.com/cat.png"}},
					},
				}
			}(),
			wantContent: "✅ Image generated (nano-banana-pro)  ⏱ 1m5s",
			wantAttach:  1,
			wantType:    channel.MessageTypeImage,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatChannelResult(tt.task, lang, nil, nil)
			if got.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", got.Content, tt.wantContent)
			}
			if len(got.Attachments) != tt.wantAttach {
				t.Errorf("Attachments count = %d, want %d", len(got.Attachments), tt.wantAttach)
			}
			if tt.wantAttach > 0 && got.Attachments[0].Type != tt.wantType {
				t.Errorf("Attachment type = %q, want %q", got.Attachments[0].Type, tt.wantType)
			}
		})
	}
}

func TestParseChannelSource(t *testing.T) {
	tests := []struct {
		source      string
		wantChannel string
		wantChat    string
	}{
		{"channel:telegram:12345", "telegram", "12345"},
		{"channel:discord:guild:chan", "discord", "guild:chan"},
		{"channel:slack:", "slack", ""},
		{"web", "", ""},
		{"", "", ""},
	}
	for _, tt := range tests {
		ch, chat := parseChannelSource(tt.source)
		if ch != tt.wantChannel || chat != tt.wantChat {
			t.Errorf("parseChannelSource(%q) = (%q, %q), want (%q, %q)",
				tt.source, ch, chat, tt.wantChannel, tt.wantChat)
		}
	}
}
