package harness

import (
	"context"
	"fmt"
	"strings"
)

type EvolutionOverview struct {
	SkillID      string                             `json:"skill_id,omitempty"`
	Revisions    EvolutionOverviewRevisionCounts    `json:"revisions"`
	Instructions EvolutionOverviewInstructionCounts `json:"instructions"`
}

type EvolutionOverviewRevisionCounts struct {
	Accepted int `json:"accepted"`
}

type EvolutionOverviewInstructionCounts struct {
	Pending int `json:"pending"`
}

func (c *Controller) BuildEvolutionOverview(
	ctx context.Context,
	skillID string,
	ownerUserID string,
) (*EvolutionOverview, error) {
	if c == nil || c.store == nil {
		return nil, fmt.Errorf("harness controller is not configured")
	}
	return c.store.BuildEvolutionOverview(ctx, skillID, ownerUserID)
}

func (s *SQLiteStore) BuildEvolutionOverview(
	ctx context.Context,
	skillID string,
	_ string,
) (*EvolutionOverview, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("harness store is not configured")
	}

	overview := &EvolutionOverview{
		SkillID: strings.TrimSpace(skillID),
	}
	if overview.SkillID == "" {
		return overview, nil
	}

	if err := s.loadEvolutionAcceptedRevisionCount(ctx, overview); err != nil {
		return nil, err
	}
	return overview, nil
}

func (s *SQLiteStore) loadEvolutionAcceptedRevisionCount(
	ctx context.Context,
	overview *EvolutionOverview,
) error {
	return s.db.QueryRowContext(
		ctx,
		`
		SELECT COUNT(*)
		FROM harness_skill_revisions
		WHERE skill_id = ?
		  AND status = ?
	`,
		overview.SkillID,
		string(SkillRevisionStatusAccepted),
	).Scan(&overview.Revisions.Accepted)
}
