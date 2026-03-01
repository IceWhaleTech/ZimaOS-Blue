package server

import (
	"context"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/kvstore"
)

const (
	shadowQualitySamplesKVKey   = "small_model:shadow_quality_samples"
	defaultShadowQualitySamples = 512
)

// ShadowQualitySample stores one shadow-vs-main quality comparison sample.
type ShadowQualitySample struct {
	Scene        string    `json:"scene"`
	MainDigest   string    `json:"main_digest,omitempty"`
	ShadowDigest string    `json:"shadow_digest,omitempty"`
	Delta        float64   `json:"delta"`
	CreatedAt    time.Time `json:"created_at"`
}

// ShadowQualityStore persists recent shadow quality samples for rollout gates.
type ShadowQualityStore struct {
	mu         sync.Mutex
	kv         kvstore.Store
	maxSamples int
	samples    []ShadowQualitySample
}

func NewShadowQualityStore(kv kvstore.Store, maxSamples int) *ShadowQualityStore {
	if maxSamples <= 0 {
		maxSamples = defaultShadowQualitySamples
	}
	s := &ShadowQualityStore{
		kv:         kv,
		maxSamples: maxSamples,
		samples:    make([]ShadowQualitySample, 0, maxSamples),
	}
	s.load()
	return s
}

func (s *ShadowQualityStore) load() {
	if s == nil || s.kv == nil {
		return
	}
	var persisted []ShadowQualitySample
	if err := s.kv.GetJSON(context.Background(), shadowQualitySamplesKVKey, &persisted); err != nil {
		return
	}
	if len(persisted) > s.maxSamples {
		persisted = persisted[len(persisted)-s.maxSamples:]
	}
	for i := range persisted {
		persisted[i] = normalizeShadowSample(persisted[i])
	}
	s.samples = append(s.samples[:0], persisted...)
}

func (s *ShadowQualityStore) Append(sample ShadowQualitySample) error {
	if s == nil {
		return nil
	}
	sample = normalizeShadowSample(sample)
	if sample.Scene == "" {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.samples = append(s.samples, sample)
	if len(s.samples) > s.maxSamples {
		s.samples = append([]ShadowQualitySample(nil), s.samples[len(s.samples)-s.maxSamples:]...)
	}
	if s.kv == nil {
		return nil
	}
	return s.kv.SetJSON(context.Background(), shadowQualitySamplesKVKey, s.samples, 0)
}

func (s *ShadowQualityStore) Snapshot() []ShadowQualitySample {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ShadowQualitySample, len(s.samples))
	copy(out, s.samples)
	return out
}

func (s *ShadowQualityStore) Reset() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = s.samples[:0]
	if s.kv == nil {
		return nil
	}
	if err := s.kv.Delete(context.Background(), shadowQualitySamplesKVKey); err != nil && err != kvstore.ErrKeyNotFound {
		return err
	}
	return nil
}

func normalizeShadowSample(sample ShadowQualitySample) ShadowQualitySample {
	sample.Scene = strings.TrimSpace(sample.Scene)
	sample.MainDigest = truncateRunes(strings.TrimSpace(strings.Join(strings.Fields(sample.MainDigest), " ")), 160)
	sample.ShadowDigest = truncateRunes(strings.TrimSpace(strings.Join(strings.Fields(sample.ShadowDigest), " ")), 160)
	if math.IsNaN(sample.Delta) || math.IsInf(sample.Delta, 0) {
		sample.Delta = 1
	}
	if sample.Delta < 0 {
		sample.Delta = 0
	}
	if sample.Delta > 1 {
		sample.Delta = 1
	}
	if sample.CreatedAt.IsZero() {
		sample.CreatedAt = time.Now().UTC()
	} else {
		sample.CreatedAt = sample.CreatedAt.UTC()
	}
	return sample
}
