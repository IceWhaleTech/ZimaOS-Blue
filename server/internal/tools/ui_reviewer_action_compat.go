package tools

import (
	"fmt"
	"strings"
)

// CanonicalizeUIReviewAction normalizes legacy or natural-language UI review
// actions to the canonical runtime actions.
func CanonicalizeUIReviewAction(action string, url string, image string) (string, error) {
	rawAction := strings.TrimSpace(action)
	canonicalAction := strings.ToLower(rawAction)
	canonicalAction = strings.ReplaceAll(canonicalAction, "-", "_")
	canonicalAction = strings.ReplaceAll(canonicalAction, " ", "_")

	hasURL := strings.TrimSpace(url) != ""
	hasImage := strings.TrimSpace(image) != ""

	if canonicalAction == "" {
		switch {
		case hasURL:
			return "review_url", nil
		case hasImage:
			return "review_image", nil
		default:
			return "", fmt.Errorf("action is required (or provide url/image so it can be inferred)")
		}
	}

	switch canonicalAction {
	case "review_url", "review_image", "check_accessibility":
		return canonicalAction, nil
	case "audit", "review", "inspect", "evaluate", "critique", "score", "rate", "assess":
		if hasImage && !hasURL {
			return "review_image", nil
		}
		return "review_url", nil
	case "image", "screenshot", "image_review", "screenshot_review", "audit_image", "audit_screenshot":
		return "review_image", nil
	case "accessibility", "a11y", "accessibility_check", "check_a11y", "a11y_check":
		return "check_accessibility", nil
	default:
		label := rawAction
		if label == "" {
			label = action
		}
		return "", fmt.Errorf("invalid action: %s (valid: review_url, review_image, check_accessibility)", label)
	}
}
