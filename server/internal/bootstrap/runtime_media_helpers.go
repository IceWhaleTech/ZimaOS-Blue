package bootstrap

import (
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/auth"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/config"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/mediagen"
)

func resolveRequestUserID(c echo.Context) string {
	if claims := auth.GetUserFromContext(c); claims != nil {
		if userID := strings.TrimSpace(claims.UserID); userID != "" {
			return userID
		}
	}
	if userID := strings.TrimSpace(c.QueryParam("user_id")); userID != "" {
		return userID
	}
	return "default"
}

func fallbackFirstProvider(chain []string) string {
	if len(chain) == 0 {
		return ""
	}
	return strings.TrimSpace(chain[0])
}

func convertFallbackPublicSpaces(spaces []config.MediaFallbackPublicSpaceConfig) []mediagen.FallbackPublicSpacePreset {
	out := make([]mediagen.FallbackPublicSpacePreset, 0, len(spaces))
	for _, preset := range spaces {
		categories := make([]mediagen.MediaCategory, 0, len(preset.Categories))
		for _, category := range preset.Categories {
			if trimmed := strings.TrimSpace(category); trimmed != "" {
				categories = append(categories, mediagen.MediaCategory(trimmed))
			}
		}
		success := make([]mediagen.FallbackResultSelector, 0, len(preset.SuccessSelectors))
		for _, selector := range preset.SuccessSelectors {
			success = append(success, mediagen.FallbackResultSelector{
				Selectors: append([]string(nil), selector.Selectors...),
				Attribute: selector.Attribute,
				Kind:      selector.Kind,
			})
		}
		out = append(out, mediagen.FallbackPublicSpacePreset{
			ID:                      preset.ID,
			DisplayName:             preset.DisplayName,
			URL:                     preset.URL,
			Categories:              categories,
			ReadySelectors:          append([]string(nil), preset.ReadySelectors...),
			PromptSelectors:         append([]string(nil), preset.PromptSelectors...),
			NegativePromptSelectors: append([]string(nil), preset.NegativePromptSelectors...),
			UploadSelectors:         append([]string(nil), preset.UploadSelectors...),
			SubmitSelectors:         append([]string(nil), preset.SubmitSelectors...),
			SuccessSelectors:        success,
			ProcessingSelectors:     append([]string(nil), preset.ProcessingSelectors...),
			ErrorSelectors:          append([]string(nil), preset.ErrorSelectors...),
			PollInterval:            preset.PollInterval,
			StallTimeout:            preset.StallTimeout,
			MaxRuntime:              preset.MaxRuntime,
			Timeout:                 preset.Timeout,
		})
	}
	return out
}
