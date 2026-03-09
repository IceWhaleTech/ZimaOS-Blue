package plugin

import "testing"

func TestInferPluginTags_ChannelTags(t *testing.T) {
	tests := []struct {
		name   string
		id     string
		wantCh string
	}{
		{name: "nextcloudtalk", id: "nextcloudtalk-bot", wantCh: "nextcloudtalk"},
		{name: "mattermost", id: "mattermost-helper", wantCh: "mattermost"},
		{name: "msteams alias", id: "msteams-bridge", wantCh: "msteams"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tags := inferPluginTags(tt.id)
			if len(tags) < 2 {
				t.Fatalf("tags = %#v, want channel tags", tags)
			}
			if tags[0] != "channel" || tags[1] != tt.wantCh {
				t.Fatalf("tags = %#v, want [channel %s ...]", tags, tt.wantCh)
			}
		})
	}
}
