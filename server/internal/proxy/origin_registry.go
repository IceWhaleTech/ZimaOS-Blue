package proxy

import (
	"path/filepath"
	"strings"
	"sync"
)

// OriginRegistry resolves model IDs to their origin (cloud/edge/local)
// using glob patterns with an LRU cache for repeated lookups.
type OriginRegistry struct {
	patterns []originPattern
	cache    sync.Map // model(lowercase) → ModelOrigin
}

type originPattern struct {
	glob   string
	origin ModelOrigin
}

// NewOriginRegistry creates a registry from a map of glob→origin strings.
func NewOriginRegistry(patterns map[string]string) *OriginRegistry {
	r := &OriginRegistry{}
	for glob, origin := range patterns {
		r.patterns = append(r.patterns, originPattern{
			glob:   strings.ToLower(glob),
			origin: ModelOrigin(origin),
		})
	}
	return r
}

// Resolve returns the origin for a model ID. Defaults to OriginCloud.
// Results are cached after first lookup.
func (r *OriginRegistry) Resolve(model string) ModelOrigin {
	lower := strings.ToLower(model)
	if v, ok := r.cache.Load(lower); ok {
		return v.(ModelOrigin)
	}
	origin := OriginCloud
	for _, p := range r.patterns {
		if matched, _ := filepath.Match(p.glob, lower); matched {
			origin = p.origin
			break
		}
	}
	r.cache.Store(lower, origin)
	return origin
}
