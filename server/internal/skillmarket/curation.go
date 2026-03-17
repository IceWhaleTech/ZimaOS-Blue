package skillmarket

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

func (s *Service) syncCurations(ctx context.Context) error {
	cfg, state, err := s.loadCuratedConfig(ctx)
	if err != nil {
		_ = s.store.UpsertCurationSyncState(ctx, CurationSyncState{
			ID:        "default",
			LastError: err.Error(),
			UpdatedAt: timeutil.NowTime(),
		})
		return err
	}

	entries := make(map[string]SkillCuration)
	for _, skillID := range cfg.Hidden {
		skillID = normalizeSkillID(skillID)
		if skillID == "" {
			continue
		}
		entry := entries[skillID]
		entry.SkillID = skillID
		entry.Hidden = true
		entries[skillID] = entry
	}
	for _, item := range cfg.Featured {
		skillID := normalizeSkillID(item.SkillID)
		if skillID == "" {
			continue
		}
		entry := entries[skillID]
		entry.SkillID = skillID
		entry.FeaturedRank = item.Rank
		if strings.TrimSpace(item.Label) != "" {
			entry.Label = item.Label
		}
		if strings.TrimSpace(item.Reason) != "" {
			entry.Reason = item.Reason
		}
		entries[skillID] = entry
	}
	for _, item := range cfg.Boosts {
		skillID := normalizeSkillID(item.SkillID)
		if skillID == "" {
			continue
		}
		entry := entries[skillID]
		entry.SkillID = skillID
		entry.BoostWeight = item.Weight
		if strings.TrimSpace(item.Label) != "" && entry.Label == "" {
			entry.Label = item.Label
		}
		if strings.TrimSpace(item.Reason) != "" && entry.Reason == "" {
			entry.Reason = item.Reason
		}
		entries[skillID] = entry
	}
	curations := make([]SkillCuration, 0, len(entries))
	for _, entry := range entries {
		curations = append(curations, entry)
	}
	if err := s.store.ReplaceCurations(ctx, curations); err != nil {
		return err
	}
	for _, seed := range cfg.AdditionalSeeds {
		if err := s.ingestAdditionalSeed(ctx, seed); err != nil && s.logger != nil {
			s.logger.Warn("skillmarket additional seed failed", zap.String("type", seed.Type), zap.String("value", seed.Value), zap.Error(err))
		}
	}
	return s.store.UpsertCurationSyncState(ctx, state)
}

func (s *Service) loadCuratedConfig(ctx context.Context) (*CuratedConfig, CurationSyncState, error) {
	candidates := make([]string, 0, 1+len(s.cfg.CuratedConfigURLs))
	if strings.TrimSpace(s.cfg.CuratedConfigPath) != "" {
		candidates = append(candidates, s.cfg.CuratedConfigPath)
	}
	candidates = append(candidates, s.cfg.CuratedConfigURLs...)
	lastErr := ""
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		data, sourceURL, err := s.readCuratedCandidate(ctx, candidate)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		var cfg CuratedConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			lastErr = err.Error()
			continue
		}
		return &cfg, CurationSyncState{
			ID:            "default",
			SourceURL:     sourceURL,
			Checksum:      checksumBytes(data),
			LastSuccessAt: timeutil.NowTime(),
			UpdatedAt:     timeutil.NowTime(),
		}, nil
	}
	if lastErr == "" {
		lastErr = "no curated config source succeeded"
	}
	return nil, CurationSyncState{}, fmt.Errorf("%s", lastErr)
}

func (s *Service) readCuratedCandidate(ctx context.Context, candidate string) ([]byte, string, error) {
	if !strings.HasPrefix(candidate, "http://") && !strings.HasPrefix(candidate, "https://") {
		data, err := os.ReadFile(candidate)
		if err != nil {
			return nil, "", err
		}
		return data, candidate, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, candidate, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("curated config status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, "", err
	}
	return data, candidate, nil
}

func (s *Service) ingestAdditionalSeed(ctx context.Context, seed AdditionalSeed) error {
	switch strings.TrimSpace(seed.Type) {
	case "github_repo":
		parts := strings.Split(strings.Trim(strings.TrimSpace(seed.Value), "/"), "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid github_repo seed: %s", seed.Value)
		}
		return s.discoverGitHubRepoSeed(ctx, parts[0], parts[1])
	case "skill_url":
		return s.discoverSkillURLSeed(ctx, seed.Value)
	default:
		return fmt.Errorf("unsupported additional seed type: %s", seed.Type)
	}
}

func checksumBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
