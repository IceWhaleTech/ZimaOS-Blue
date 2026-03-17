package formfiller

import (
	"errors"
	"strings"
)

var ErrInvalidDomain = errors.New("invalid site domain")

// normalizeDomain validates a site-mapping domain before it is used as a cache key or filename.
func normalizeDomain(domain string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(domain))
	if normalized == "" || len(normalized) > 253 {
		return "", ErrInvalidDomain
	}
	if strings.Contains(normalized, "..") || strings.ContainsAny(normalized, `/\:`) {
		return "", ErrInvalidDomain
	}

	labels := strings.Split(normalized, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 {
			return "", ErrInvalidDomain
		}
		for i, r := range label {
			isLower := r >= 'a' && r <= 'z'
			isDigit := r >= '0' && r <= '9'
			isHyphen := r == '-'
			if !isLower && !isDigit && !isHyphen {
				return "", ErrInvalidDomain
			}
			if isHyphen && (i == 0 || i == len(label)-1) {
				return "", ErrInvalidDomain
			}
		}
	}

	return normalized, nil
}
