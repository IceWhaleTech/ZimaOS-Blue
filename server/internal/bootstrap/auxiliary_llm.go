package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/smallmodel"
)

var (
	errAuxiliaryLLMUnavailable = errors.New("auxiliary llm backend not available")
	errSmallModelUnsupported   = errors.New("small model request unsupported")
)

type llmChatCaller interface {
	Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error)
}

type auxiliaryLLMCaller struct {
	mu         sync.RWMutex
	smallModel smallmodel.Runtime
	fallback   llmChatCaller
}

func newAuxiliaryLLMCaller() *auxiliaryLLMCaller {
	return &auxiliaryLLMCaller{}
}

func (c *auxiliaryLLMCaller) Name() string {
	return "smallmodel"
}

func (c *auxiliaryLLMCaller) Models() []string {
	return []string{smallmodel.ModelID}
}

func (c *auxiliaryLLMCaller) SetSmallModel(rt smallmodel.Runtime) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.smallModel = rt
}

func (c *auxiliaryLLMCaller) SetFallback(fallback llmChatCaller) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fallback = fallback
}

func (c *auxiliaryLLMCaller) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.mu.RLock()
	runtime := c.smallModel
	fallback := c.fallback
	c.mu.RUnlock()

	if runtime != nil && runtime.Ready() {
		resp, err := callAuxiliarySmallModel(ctx, runtime, req)
		if err == nil {
			return resp, nil
		}
		if !shouldFallbackFromSmallModel(err) || fallback == nil {
			return nil, err
		}
	}

	if fallback == nil {
		return nil, errAuxiliaryLLMUnavailable
	}
	return fallback.Chat(ctx, req)
}

func (c *auxiliaryLLMCaller) ChatStream(_ context.Context, _ llm.ChatRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("auxiliary llm does not support streaming")
}

func (c *auxiliaryLLMCaller) ChatStreamCallback(_ context.Context, _ llm.ChatRequest, _ llm.StreamCallback) error {
	return fmt.Errorf("auxiliary llm does not support streaming")
}

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

func renderAuxiliarySmallModelPrompt(req llm.ChatRequest) (string, error) {
	if req.Stream || len(req.Tools) > 0 || strings.TrimSpace(req.PreviousResponseID) != "" {
		return "", errSmallModelUnsupported
	}

	var sb strings.Builder
	writeSection := func(title, body string) {
		body = strings.TrimSpace(body)
		if body == "" {
			return
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(title)
		sb.WriteString(":\n")
		sb.WriteString(body)
	}

	writeSection("System", req.Instructions)
	for _, msg := range req.Messages {
		rendered, err := renderAuxiliarySmallModelMessage(msg)
		if err != nil {
			return "", err
		}
		if rendered == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(rendered)
	}
	if sb.Len() == 0 {
		return "", fmt.Errorf("small model request has no prompt content")
	}
	sb.WriteString("\n\nAssistant:\n")
	return sb.String(), nil
}

func renderAuxiliarySmallModelMessage(msg llm.Message) (string, error) {
	if len(msg.ToolCalls) > 0 {
		return "", errSmallModelUnsupported
	}

	parts := make([]string, 0, 1+len(msg.ContentParts))
	if content := strings.TrimSpace(msg.Content); content != "" {
		parts = append(parts, content)
	}
	for _, part := range msg.ContentParts {
		partType := strings.TrimSpace(strings.ToLower(part.Type))
		switch partType {
		case "", "text":
			if text := strings.TrimSpace(part.Text); text != "" {
				parts = append(parts, text)
			}
		default:
			return "", errSmallModelUnsupported
		}
	}
	if len(parts) == 0 {
		return "", nil
	}

	roleLabel := auxiliaryRoleLabel(msg)
	return fmt.Sprintf("%s:\n%s", roleLabel, strings.Join(parts, "\n\n")), nil
}

func auxiliaryRoleLabel(msg llm.Message) string {
	switch msg.Role {
	case llm.RoleSystem:
		return "System"
	case llm.RoleAssistant:
		return "Assistant"
	case llm.RoleTool:
		if name := strings.TrimSpace(msg.ToolName); name != "" {
			return "Tool (" + name + ")"
		}
		return "Tool"
	default:
		return "User"
	}
}
