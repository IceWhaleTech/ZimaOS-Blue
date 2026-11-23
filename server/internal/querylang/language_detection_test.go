package querylang

import (
	"strings"
	"testing"
)

func TestDetectQueryLanguage(t *testing.T) {
	for _, tt := range []struct{ query, want string }{
		{"", ""},
		{"\xff", ""},
		{strings.Repeat("photos de chiens qui jouent sur la plage ", 110), ""},
		{"photos of dogs playing on the beach", "en"},
		{"Finde alle Fotos von roten Autos ohne Katzen im Hintergrund", "de"},
		{"photos de chiens qui jouent sur la plage", "fr"},
		{"去年没有猫的狗照片", "zh"},
		{"東京都の猫の写真", "ja"},
		{"고양이 사진", "ko"},
		{"φωτογραφίες σκύλων", "el"},
		{"നായയുടെ ചിത്രം", "ml"},
		{"cat", ""},
		{"cat chat Katze", ""},
		{"IMG_1234.JPG", ""},
		{"ＩＭＧ＿１２３４．ＪＰＧ", ""},
		{"phоtos de chiens", ""}, // Cyrillic о remains mixed script.
		{"photos de chiens qui jouent sur la plage.jpg", ""},
		{`"photos de chiens qui jouent sur la plage"`, ""},
		{"photos/de/chiens/qui/jouent/sur/la/plage", ""},
		{"2026-10-30", ""},
		{"صور للكلاب تلعب على الشاطئ", ""},
		{"photos of dogs playing on the beach 猫", ""},
	} {
		t.Run(tt.query, func(t *testing.T) {
			if got := DetectQueryLanguage(tt.query); got != tt.want {
				t.Fatalf("language=%q want=%q", got, tt.want)
			}
		})
	}
}
