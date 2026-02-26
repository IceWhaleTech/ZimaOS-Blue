// Package cardproto provides helpers for blue CLI subcommands to emit
// typeless cards via the stdout card protocol. Cards are written as
// single-line __CARD__{json}__END__ markers that the exec tool's stdout
// scanner extracts and forwards to the frontend SSE stream in real-time.
package cardproto

import (
	"encoding/json"
	"fmt"
	"os"
)

const (
	prefix = "__CARD__"
	suffix = "__END__"
)

// Emit writes a card to stdout using the card protocol.
// The card is JSON-serialised and wrapped in __CARD__...__END__ markers.
func Emit(card map[string]interface{}) {
	data, err := json.Marshal(card)
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stdout, "%s%s%s\n", prefix, data, suffix)
}

// Progress emits a progress card with the given type, step, name, and status.
func Progress(cardType, step, name, status string) {
	Emit(map[string]interface{}{
		"type":   cardType,
		"step":   step,
		"name":   name,
		"status": status,
	})
}

// Alert emits an alert card.
func Alert(variant, titleKey, message string) {
	Emit(map[string]interface{}{
		"type":      "alert",
		"variant":   variant,
		"title_key": titleKey,
		"message":   message,
	})
}
