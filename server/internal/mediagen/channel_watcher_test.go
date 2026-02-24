package mediagen

import (
	"testing"

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
	tests := []struct {
		name string
		task *MediaTask
		want string
	}{
		{
			name: "nil task",
			task: nil,
			want: "",
		},
		{
			name: "failed task",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusFailed, Error: "rate limit exceeded"},
				Category: "t2i",
			},
			want: "❌ Media generation failed: rate limit exceeded",
		},
		{
			name: "cancelled task",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusCancelled},
			},
			want: "🚫 Media generation was cancelled.",
		},
		{
			name: "succeeded image",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusSucceeded},
				Model:    "dall-e-3",
				Category: "t2i",
				Response: &MediaResponse{
					Data: []MediaResult{{URL: "https://example.com/cat.png"}},
				},
			},
			want: "✅ Image generated (dall-e-3)\nhttps://example.com/cat.png",
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
			want: "✅ Video generated (wan2.6-t2v)\nhttps://example.com/video.mp4",
		},
		{
			name: "succeeded no output",
			task: &MediaTask{
				BaseTask: task.BaseTask{Status: TaskStatusSucceeded},
				Response: &MediaResponse{Data: []MediaResult{}},
			},
			want: "✅ Generation complete, but no output was returned.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatChannelResult(tt.task)
			if got != tt.want {
				t.Errorf("FormatChannelResult() = %q, want %q", got, tt.want)
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
