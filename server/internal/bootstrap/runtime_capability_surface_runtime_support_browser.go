package bootstrap

import (
	"context"
	"encoding/json"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/browser"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/harness"
)

type browserOverviewTaskProjectionAdapter struct {
	service *harness.UserTaskProjectionService
}

func (a browserOverviewTaskProjectionAdapter) List(
	ctx context.Context,
	query browser.BrowserOverviewTaskQuery,
) ([]map[string]any, error) {
	if a.service == nil {
		return []map[string]any{}, nil
	}
	projections, err := a.service.List(ctx, harness.UserTaskProjectionFilter{
		UserID:         query.UserID,
		ConversationID: query.ConversationID,
		Scope:          query.Scope,
		Limit:          query.Limit,
	})
	if err != nil {
		return nil, err
	}
	result := make([]map[string]any, 0, len(projections))
	for _, projection := range projections {
		payload, err := projectionToBrowserOverviewMap(projection)
		if err != nil {
			return nil, err
		}
		result = append(result, payload)
	}
	return result, nil
}

func projectionToBrowserOverviewMap(projection harness.UserTaskProjection) (map[string]any, error) {
	raw, err := json.Marshal(projection)
	if err != nil {
		return nil, err
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}
