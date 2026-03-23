package drivers

import "testing"

func TestComposeConversationContext(t *testing.T) {
	tests := []struct {
		name string
		meta map[string]interface{}
		want string
	}{
		{
			name: "base context only",
			meta: map[string]interface{}{"context": "recent chat context"},
			want: "recent chat context",
		},
		{
			name: "retry context only",
			meta: map[string]interface{}{"retry_context": "Harness retry guidance"},
			want: "Harness retry guidance",
		},
		{
			name: "base and retry context",
			meta: map[string]interface{}{
				"context":       "recent chat context",
				"retry_context": "Harness retry guidance",
			},
			want: "recent chat context\n\nHarness retry guidance",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := composeConversationContext(tc.meta); got != tc.want {
				t.Fatalf("composeConversationContext() = %q, want %q", got, tc.want)
			}
		})
	}
}
