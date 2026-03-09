package teams

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/channel"
)

func TestNew(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	if ch == nil {
		t.Fatal("expected channel to be created")
	}
	if ch.Name() != "teams" {
		t.Errorf("expected name 'teams', got '%s'", ch.Name())
	}
	if ch.Type() != "teams" {
		t.Errorf("expected type 'teams', got '%s'", ch.Type())
	}
}

func TestChannel_Info(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)
	info := ch.Info()

	if info.Name != "teams" {
		t.Errorf("expected name 'teams', got '%s'", info.Name)
	}
	if info.Type != "teams" {
		t.Errorf("expected type 'teams', got '%s'", info.Type)
	}
	if info.Status != channel.StatusDisconnected {
		t.Errorf("expected status 'disconnected', got '%s'", info.Status)
	}
	if !info.Enabled {
		t.Error("expected enabled to be true")
	}
}

func TestChannel_IsConnected(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	if ch.IsConnected() {
		t.Error("expected channel to not be connected initially")
	}
}

func TestChannel_Messages(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)
	messages := ch.Messages()

	if messages == nil {
		t.Error("expected messages channel to not be nil")
	}
}

func TestChannel_Stop_NotStarted(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Stop(ctx)
	if err != nil {
		t.Errorf("expected no error stopping non-started channel, got: %v", err)
	}
}

func TestChannel_Start_Success(t *testing.T) {
	// Create mock OAuth server
	oauthServer := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/botframework.com/oauth2/v2.0/token" {
			response := map[string]interface{}{
				"access_token": "test-access-token",
				"token_type":   "Bearer",
				"expires_in":   3600,
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		http.NotFound(w, r)
	}))
	defer oauthServer.Close()

	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := NewWithOptions(cfg, logger, oauthServer.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Start(ctx)
	if err != nil {
		t.Fatalf("expected no error starting channel, got: %v", err)
	}

	if !ch.IsConnected() {
		t.Error("expected channel to be connected after start")
	}

	// Clean up
	ch.Stop(ctx)
}

func TestChannel_Start_AuthFailure(t *testing.T) {
	// Create mock OAuth server that returns error
	oauthServer := newTCP4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		response := map[string]interface{}{
			"error":             "invalid_client",
			"error_description": "Invalid client credentials",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer oauthServer.Close()

	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "invalid-app-id",
		AppPassword: "invalid-password",
	}

	ch := NewWithOptions(cfg, logger, oauthServer.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Start(ctx)
	if err == nil {
		t.Fatal("expected error starting channel with invalid credentials")
	}

	if ch.IsConnected() {
		t.Error("expected channel to not be connected after auth failure")
	}
}

func TestChannel_Send_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := ch.Send(ctx, channel.OutgoingMessage{
		ChatID:  "test-chat",
		Content: "Hello",
	})

	if err == nil {
		t.Error("expected error sending message when not initialized")
	}
}

func TestChannel_SendStreaming_NotInitialized(t *testing.T) {
	logger := zap.NewNop()
	cfg := channel.TeamsConfig{
		Enabled:     true,
		AppID:       "test-app-id",
		AppPassword: "test-app-password",
	}

	ch := New(cfg, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	content := make(chan string)
	done := make(chan struct{})

	err := ch.SendStreaming(ctx, "test-chat", "", content, done)

	if err == nil {
		t.Error("expected error sending streaming message when not initialized")
	}
}

func TestChannel_isTeamAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedTeams []string
		teamID       string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedTeams: []string{},
			teamID:       "any-team",
			expected:     true,
		},
		{
			name:         "team ID in allowed list",
			allowedTeams: []string{"team-1", "team-2"},
			teamID:       "team-1",
			expected:     true,
		},
		{
			name:         "team ID not in allowed list",
			allowedTeams: []string{"team-1", "team-2"},
			teamID:       "team-3",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			cfg := channel.TeamsConfig{
				Enabled:      true,
				AppID:        "test-app-id",
				AppPassword:  "test-app-password",
				AllowedTeams: tt.allowedTeams,
			}

			ch := New(cfg, logger)
			result := ch.isTeamAllowed(tt.teamID)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestChannel_isUserAllowed(t *testing.T) {
	tests := []struct {
		name         string
		allowedUsers []string
		userID       string
		expected     bool
	}{
		{
			name:         "empty allowed list allows all",
			allowedUsers: []string{},
			userID:       "any-user",
			expected:     true,
		},
		{
			name:         "user ID in allowed list",
			allowedUsers: []string{"user-1", "user-2"},
			userID:       "user-1",
			expected:     true,
		},
		{
			name:         "user ID not in allowed list",
			allowedUsers: []string{"user-1", "user-2"},
			userID:       "user-3",
			expected:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop()
			cfg := channel.TeamsConfig{
				Enabled:      true,
				AppID:        "test-app-id",
				AppPassword:  "test-app-password",
				AllowedUsers: tt.allowedUsers,
			}

			ch := New(cfg, logger)
			result := ch.isUserAllowed(tt.userID)

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestChannel_ConvertActivity_PreservesCardMetadataAndTimestamp(t *testing.T) {
	ch := New(channel.TeamsConfig{Enabled: true, AppID: "app", AppPassword: "pw"}, zap.NewNop())

	activity := &Activity{
		Type:       "message",
		ID:         "activity-1",
		Timestamp:  "2026-03-08T09:10:11Z",
		ServiceURL: "https://smba.trafficmanager.net/amer/",
		TextFormat: "xml",
		ReplyToID:  "parent-1",
		Conversation: &ConversationAccount{
			ID:      "conv-1",
			Name:    "Team Chat",
			IsGroup: true,
		},
		From: &ChannelAccount{ID: "user-1", Name: "Alice"},
		ChannelData: map[string]interface{}{
			"team": map[string]interface{}{"id": "team-1"},
		},
		Attachments: []ActivityAttachment{{
			ContentType: "application/vnd.microsoft.card.adaptive",
			Content: map[string]interface{}{
				"type":    "AdaptiveCard",
				"version": "1.4",
			},
		}},
	}

	msg := ch.convertActivity(activity)
	if msg.Type != channel.MessageTypeCard {
		t.Fatalf("Type = %q, want card", msg.Type)
	}
	if msg.ChatID != "https://smba.trafficmanager.net/amer/|conv-1" {
		t.Fatalf("ChatID = %q", msg.ChatID)
	}
	if msg.ReplyToID != "parent-1" {
		t.Fatalf("ReplyToID = %q, want parent-1", msg.ReplyToID)
	}
	if msg.Timestamp.Format(time.RFC3339) != "2026-03-08T09:10:11Z" {
		t.Fatalf("Timestamp = %s", msg.Timestamp.Format(time.RFC3339))
	}
	if msg.Metadata["team_id"] != "team-1" {
		t.Fatalf("team_id = %v", msg.Metadata["team_id"])
	}
	if msg.Metadata["textFormat"] != "xml" {
		t.Fatalf("textFormat = %v", msg.Metadata["textFormat"])
	}
	attachments, ok := msg.Metadata["attachments"].([]map[string]interface{})
	if !ok || len(attachments) != 1 {
		t.Fatalf("attachments = %#v", msg.Metadata["attachments"])
	}
	if attachments[0]["contentType"] != "application/vnd.microsoft.card.adaptive" {
		t.Fatalf("contentType = %v", attachments[0]["contentType"])
	}
	if _, ok := attachments[0]["content"].(map[string]interface{}); !ok {
		t.Fatalf("content = %#v", attachments[0]["content"])
	}
}

func TestChannel_ConvertActivity_MapsMediaAttachmentKinds(t *testing.T) {
	ch := New(channel.TeamsConfig{Enabled: true, AppID: "app", AppPassword: "pw"}, zap.NewNop())
	msg := ch.convertActivity(&Activity{
		Type:       "message",
		ID:         "activity-2",
		ServiceURL: "https://smba.trafficmanager.net/amer/",
		Conversation: &ConversationAccount{
			ID: "conv-2",
		},
		Attachments: []ActivityAttachment{{
			ContentType: "image/png",
			ContentURL:  "https://example.com/image.png",
			Name:        "image.png",
		}},
	})
	if msg.Type != channel.MessageTypeImage {
		t.Fatalf("Type = %q, want image", msg.Type)
	}
	if len(msg.Attachments) != 1 || msg.Attachments[0].URL != "https://example.com/image.png" {
		t.Fatalf("Attachments = %#v", msg.Attachments)
	}
}

func TestChannel_ConvertActivity_PreservesEntitiesAndMentions(t *testing.T) {
	ch := New(channel.TeamsConfig{Enabled: true, AppID: "app", AppPassword: "pw"}, zap.NewNop())
	msg := ch.convertActivity(&Activity{
		Type:       "message",
		ID:         "activity-mentions",
		ServiceURL: "https://smba.trafficmanager.net/amer/",
		Conversation: &ConversationAccount{
			ID: "conv-mentions",
		},
		Entities: []map[string]interface{}{{
			"type": "mention",
			"text": "<at>Alice</at>",
			"mentioned": map[string]interface{}{
				"id":   "29:user-1",
				"name": "Alice",
			},
		}},
	})

	entities, ok := msg.Metadata["entities"].([]map[string]interface{})
	if !ok || len(entities) != 1 {
		t.Fatalf("entities = %#v", msg.Metadata["entities"])
	}
	mentionIDs, ok := msg.Metadata["mention_ids"].([]string)
	if !ok || len(mentionIDs) != 1 || mentionIDs[0] != "29:user-1" {
		t.Fatalf("mention_ids = %#v", msg.Metadata["mention_ids"])
	}
	mentions, ok := msg.Metadata["mentions"].([]map[string]interface{})
	if !ok || len(mentions) != 1 {
		t.Fatalf("mentions = %#v", msg.Metadata["mentions"])
	}
	if mentions[0]["name"] != "Alice" || mentions[0]["id"] != "29:user-1" {
		t.Fatalf("first mention = %#v", mentions[0])
	}
}

func TestChannel_ConvertActivity_InvokePreservesValueAndFallbackContent(t *testing.T) {
	ch := New(channel.TeamsConfig{Enabled: true, AppID: "app", AppPassword: "pw"}, zap.NewNop())
	msg := ch.convertActivity(&Activity{
		Type:       "invoke",
		ID:         "invoke-1",
		Timestamp:  "2026-03-08T10:11:12Z",
		ServiceURL: "https://smba.trafficmanager.net/amer/",
		ChannelID:  "msteams",
		Name:       "adaptiveCard/action",
		Value: map[string]interface{}{
			"verb":   "approve",
			"ticket": "T-1",
		},
		Conversation: &ConversationAccount{ID: "conv-invoke", Name: "Ops", IsGroup: true},
		From:         &ChannelAccount{ID: "user-1", Name: "Alice"},
	})

	if msg.Content != "approve" {
		t.Fatalf("Content = %q, want approve", msg.Content)
	}
	if msg.Metadata["activity_type"] != "invoke" {
		t.Fatalf("activity_type = %v", msg.Metadata["activity_type"])
	}
	if msg.Metadata["activity_name"] != "adaptiveCard/action" {
		t.Fatalf("activity_name = %v", msg.Metadata["activity_name"])
	}
	if msg.Metadata["channel_id"] != "msteams" {
		t.Fatalf("channel_id = %v", msg.Metadata["channel_id"])
	}
	value, ok := msg.Metadata["value"].(map[string]interface{})
	if !ok || value["ticket"] != "T-1" {
		t.Fatalf("value = %#v", msg.Metadata["value"])
	}
}

func TestChannel_HandleActivity_AllowsInvokeActivities(t *testing.T) {
	ch := New(channel.TeamsConfig{Enabled: true, AppID: "app", AppPassword: "pw"}, zap.NewNop())
	ch.HandleActivity(&Activity{
		Type:       "invoke",
		ID:         "invoke-2",
		ServiceURL: "https://smba.trafficmanager.net/amer/",
		Name:       "adaptiveCard/action",
		Value: map[string]interface{}{
			"action": "approve",
		},
		Conversation: &ConversationAccount{ID: "conv-2"},
		From:         &ChannelAccount{ID: "user-2", Name: "Bob"},
	})

	select {
	case msg := <-ch.Messages():
		if msg.ID != "invoke-2" {
			t.Fatalf("ID = %q, want invoke-2", msg.ID)
		}
		if msg.Content != "approve" {
			t.Fatalf("Content = %q, want approve", msg.Content)
		}
		if msg.Metadata["activity_type"] != "invoke" {
			t.Fatalf("activity_type = %v", msg.Metadata["activity_type"])
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for invoke activity")
	}
}
