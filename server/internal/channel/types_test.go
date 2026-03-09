package channel

import (
	"reflect"
	"testing"
)

func TestConfigIncludesNextcloudTalk(t *testing.T) {
	field, ok := reflect.TypeOf(Config{}).FieldByName("NextcloudTalk")
	if !ok {
		t.Fatal("Config.NextcloudTalk field missing")
	}
	if got := field.Tag.Get("yaml"); got != "nextcloudtalk" {
		t.Fatalf("yaml tag = %q, want nextcloudtalk", got)
	}

	cfg := DefaultConfig()
	if cfg.NextcloudTalk.Enabled {
		t.Fatal("DefaultConfig().NextcloudTalk.Enabled = true, want false")
	}
	if cfg.NextcloudTalk.ServerURL != "" || cfg.NextcloudTalk.Username != "" || cfg.NextcloudTalk.Password != "" || cfg.NextcloudTalk.RoomToken != "" {
		t.Fatalf("DefaultConfig().NextcloudTalk = %#v, want zero-value credentials", cfg.NextcloudTalk)
	}
}
