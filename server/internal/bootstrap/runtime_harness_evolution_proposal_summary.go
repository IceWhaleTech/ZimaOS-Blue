package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selfreflect"
)

type harnessEvolutionProposalSummaryProvider struct {
	service *selfreflect.Service
}

func newHarnessEvolutionProposalSummaryProvider(
	service *selfreflect.Service,
) harness.EvolutionProposalSummaryProvider {
	if service == nil {
		return nil
	}
	return harnessEvolutionProposalSummaryProvider{service: service}
}

func (p harnessEvolutionProposalSummaryProvider) PendingProposalCount(
	ctx context.Context,
	ownerUserID string,
) (int, error) {
	if p.service == nil {
		return 0, nil
	}
	proposals, err := p.service.ListProposals(ctx, selfreflect.ProposalFilter{
		OwnerUserID: ownerUserID,
		Statuses:    []selfreflect.ProposalStatus{selfreflect.ProposalStatusPending},
		Limit:       500,
	})
	if err != nil {
		return 0, err
	}
	return len(proposals), nil
}
