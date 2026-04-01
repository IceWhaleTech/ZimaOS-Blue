package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

func callAuxiliarySmallModel(ctx context.Context, runtime smallmodel.Runtime, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if runtime == nil || !runtime.Ready() {
		return nil, smallmodel.ErrNotReady
	}
	prompt, err := renderAuxiliarySmallModelPrompt(req)
	if err != nil {
		return nil, err
	}
	resp, err := runtime.Generate(ctx, smallmodel.GenerateRequest{
		Prompt:      prompt,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil || strings.TrimSpace(resp.Text) == "" {
		return nil, fmt.Errorf("small model returned empty response")
	}
	return &llm.ChatResponse{
		Model:      smallmodel.ModelID,
		Provider:   "smallmodel",
		ProviderID: "smallmodel",
		Message: llm.Message{
			Role:    llm.RoleAssistant,
			Content: strings.TrimSpace(resp.Text),
		},
	}, nil
}

func shouldFallbackFromSmallModel(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return true
}
