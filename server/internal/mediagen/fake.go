package mediagen

import (
	"context"
	"fmt"
	"strings"

	basetask "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/task"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

const fakeMediaImagePNGBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAAEElEQVR4nGL6//8/IAAA//8GBgMAt2YRIQAAAABJRU5ErkJggg=="

// FakeMediaProvider provides a dev-only local image generator for smoke tests.
type FakeMediaProvider struct{}

// NewFakeMediaProvider creates a local fake media provider.
func NewFakeMediaProvider() *FakeMediaProvider {
	return &FakeMediaProvider{}
}

func (p *FakeMediaProvider) Name() string { return fakeMediaProviderID }

func (p *FakeMediaProvider) SupportedModels() []MediaModelInfo {
	return []MediaModelInfo{
		{
			ID:             fakeMediaModelID,
			Name:           "Fake Image",
			Type:           MediaTypeImage,
			Category:       CategoryT2I,
			Provider:       fakeMediaProviderID,
			SupportedSizes: []string{"1024x1024", "1536x1024", "1024x1536"},
		},
		{
			ID:             fakeMediaModelID,
			Name:           "Fake Image Edit",
			Type:           MediaTypeImage,
			Category:       CategoryI2I,
			Provider:       fakeMediaProviderID,
			SupportedSizes: []string{"1024x1024", "1536x1024", "1024x1536"},
		},
	}
}

func (p *FakeMediaProvider) SupportsType(t MediaType) bool {
	return t == MediaTypeImage
}

func (p *FakeMediaProvider) Generate(_ context.Context, req *MediaRequest) (*MediaTask, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = fakeMediaModelID
	}
	n := req.N
	if n <= 0 {
		n = 1
	}
	prompt := strings.TrimSpace(req.Prompt)
	if prompt == "" {
		prompt = "placeholder image"
	}
	now := timeutil.NowTime()
	results := make([]MediaResult, 0, n)
	for i := 0; i < n; i++ {
		revised := prompt
		if n > 1 {
			revised = fmt.Sprintf("%s (#%d)", prompt, i+1)
		}
		results = append(results, MediaResult{
			B64JSON:       fakeMediaImagePNGBase64,
			ContentType:   "image/png",
			Width:         1,
			Height:        1,
			RevisedPrompt: revised,
		})
	}
	return &MediaTask{
		BaseTask: basetask.BaseTask{
			Status:      TaskStatusSucceeded,
			Progress:    1.0,
			CreatedAt:   now,
			CompletedAt: &now,
		},
		Type:     MediaTypeImage,
		Provider: p.Name(),
		Model:    model,
		Request:  req,
		Response: &MediaResponse{
			Created: now.Unix(),
			Data:    results,
		},
	}, nil
}

func (p *FakeMediaProvider) Poll(_ context.Context, taskID string) (*MediaTask, error) {
	return nil, fmt.Errorf("unknown task: %s", taskID)
}
