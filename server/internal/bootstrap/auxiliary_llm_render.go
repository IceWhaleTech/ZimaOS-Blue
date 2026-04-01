package bootstrap

import (
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/llm"
)

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
