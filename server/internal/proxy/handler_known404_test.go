package proxy

import (
	"net/http"
	"testing"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/providerpool"
)

func TestTreatKnown404AsModelNotConfigured_CuratedFormats(t *testing.T) {
	tests := []struct {
		name   string
		format providerpool.APIFormat
		want   bool
	}{
		{name: "copilot", format: providerpool.APIFormatCopilot, want: true},
		{name: "anthropic", format: providerpool.APIFormatAnthropic, want: true},
		{name: "cloudcode", format: providerpool.APIFormatCloudCode, want: false},
		{name: "openai", format: providerpool.APIFormatOpenAI, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &providerpool.Provider{APIFormat: tt.format}
			got := treatKnown404AsModelNotConfigured(p, true, http.StatusNotFound)
			if got != tt.want {
				t.Fatalf("treatKnown404AsModelNotConfigured(%s) = %v, want %v", tt.format, got, tt.want)
			}
		})
	}
}

func TestTreatKnown404AsModelNotConfigured_Guards(t *testing.T) {
	p := &providerpool.Provider{APIFormat: providerpool.APIFormatCopilot}

	if treatKnown404AsModelNotConfigured(nil, true, http.StatusNotFound) {
		t.Fatal("nil provider should not be treated as model-not-configured")
	}
	if treatKnown404AsModelNotConfigured(p, false, http.StatusNotFound) {
		t.Fatal("unknown format should not force model-not-configured on 404")
	}
	if treatKnown404AsModelNotConfigured(p, true, http.StatusBadRequest) {
		t.Fatal("non-404 should not force model-not-configured")
	}
}
