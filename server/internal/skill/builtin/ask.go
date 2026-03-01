package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill"
)

// AskQuestioner is the interface for asking user questions.
type AskQuestioner interface {
	AskQuestions(ctx context.Context, userID, sessionID string, questions []AskQuestionItem) ([]AskQuestionAnswerResult, bool, error)
}

// AskQuestionItem is a single question in ask skill payload.
type AskQuestionItem struct {
	ID          string
	Question    string
	Detail      string
	Header      string
	Options     []AskQuestionOption
	MultiSelect bool
}

// AskQuestionOption is a single selectable option.
type AskQuestionOption struct {
	Label       string
	Value       string
	Description string
}

// AskQuestionAnswerResult is the answer payload returned by ask backend.
type AskQuestionAnswerResult struct {
	QuestionID string   `json:"question_id"`
	Selected   []string `json:"selected"`
	OtherText  string   `json:"other_text,omitempty"`
}

// Ask is a built-in skill that asks one or more multiple-choice questions.
type Ask struct {
	manifest   *skill.Manifest
	questioner AskQuestioner
}

// NewAsk creates a new ask skill.
func NewAsk() *Ask {
	return &Ask{
		manifest: &skill.Manifest{
			ID:          "ask",
			Name:        "Ask",
			Version:     "1.0.0",
			Description: "Ask the user one or more multiple-choice questions and wait for their response.",
			Category:    "system",
			Icon:        "question",
			Tags:        []string{"ask", "question", "input", "interaction"},
			Inputs: []skill.Parameter{
				{
					Name:        "questions",
					Type:        "array",
					Description: "Preferred. Question array. Each item: {question, detail?, type: radio|checkbox, options: [...]}",
					Required:    false,
				},
				{
					Name:        "q",
					Type:        "string",
					Description: "Single-select question text (shorthand).",
					Required:    false,
				},
				{
					Name:        "detail",
					Type:        "string",
					Description: "Optional extra detail for q/mq shorthand or question items.",
					Required:    false,
				},
				{
					Name:        "mq",
					Type:        "string",
					Description: "Multi-select question text (shorthand).",
					Required:    false,
				},
				{
					Name:        "a",
					Type:        "array",
					Description: "Options for q/mq shorthand. Supports string options or objects with label/description/value.",
					Required:    false,
				},
			},
		},
	}
}

func (a *Ask) SetQuestioner(q AskQuestioner) { a.questioner = q }

func (a *Ask) Manifest() *skill.Manifest { return a.manifest }

func (a *Ask) Validate(input map[string]any) error {
	_, err := parseAskQuestions(input)
	return err
}

func (a *Ask) Execute(ctx context.Context, input map[string]any) (*skill.Result, error) {
	if a.questioner == nil {
		return skill.NewErrorResult(fmt.Errorf("ask backend not configured")), nil
	}

	questions, err := parseAskQuestions(input)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	userID := skill.GetUserID(ctx)
	answers, silent, err := a.questioner.AskQuestions(ctx, userID, "", questions)
	if err != nil {
		return skill.NewErrorResult(err), nil
	}

	selected := []string{}
	if len(answers) > 0 {
		selected = append(selected, answers[0].Selected...)
		if answers[0].OtherText != "" {
			selected = append(selected, answers[0].OtherText)
		}
	}

	return skill.NewResult(map[string]any{
		"answers":  answers,
		"selected": selected,
		"silent":   silent,
	}), nil
}

func parseAskQuestions(input map[string]any) ([]AskQuestionItem, error) {
	if raw, ok := input["questions"]; ok {
		items, err := parseQuestionsInput(raw)
		if err != nil {
			return nil, err
		}
		if len(items) > 0 {
			return items, nil
		}
	}

	if q, ok := input["q"].(string); ok && strings.TrimSpace(q) != "" {
		options := parseOptionsFromInput(input, "a")
		if len(options) == 0 {
			return nil, fmt.Errorf("a must include at least 1 value for q")
		}
		question := strings.TrimSpace(q)
		detail := extractAskQuestionDetail(input)
		return []AskQuestionItem{
			{
				ID:          "q0",
				Question:    question,
				Detail:      detail,
				Header:      shortHeader(question),
				Options:     options,
				MultiSelect: false,
			},
		}, nil
	}
	if mq, ok := input["mq"].(string); ok && strings.TrimSpace(mq) != "" {
		options := parseOptionsFromInput(input, "a")
		if len(options) == 0 {
			return nil, fmt.Errorf("a must include at least 1 value for mq")
		}
		question := strings.TrimSpace(mq)
		detail := extractAskQuestionDetail(input)
		return []AskQuestionItem{
			{
				ID:          "q0",
				Question:    question,
				Detail:      detail,
				Header:      shortHeader(question),
				Options:     options,
				MultiSelect: true,
			},
		}, nil
	}

	question, _ := input["question"].(string)
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("provide questions[] or q/mq + a")
	}
	options := parseOptionsFromInput(input, "options")
	if len(options) == 0 {
		return nil, fmt.Errorf("options must include at least 1 value")
	}

	return []AskQuestionItem{
		{
			ID:          "q0",
			Question:    question,
			Detail:      extractAskQuestionDetail(input),
			Header:      shortHeader(question),
			Options:     options,
			MultiSelect: false,
		},
	}, nil
}

func parseQuestionsInput(raw any) ([]AskQuestionItem, error) {
	parsed, err := parseJSONOrValue(raw)
	if err != nil {
		return nil, err
	}
	if parsed == nil {
		return nil, nil
	}

	list, ok := asAnySlice(parsed)
	if !ok {
		return nil, fmt.Errorf("invalid questions JSON: must be an array")
	}

	items := make([]AskQuestionItem, 0, len(list))
	for i, entry := range list {
		obj, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		question := firstNonEmptyString(obj, "question")
		if question == "" {
			continue
		}
		options := parseOptionsFromQuestion(obj)
		if len(options) == 0 {
			return nil, fmt.Errorf("question %d has no options", i+1)
		}
		qType := firstNonEmptyString(obj, "type")
		isMulti := strings.EqualFold(qType, "checkbox") || strings.EqualFold(qType, "multi")
		items = append(items, AskQuestionItem{
			ID:          fmt.Sprintf("q%d", i),
			Question:    question,
			Detail:      extractAskQuestionDetail(obj),
			Header:      shortHeader(question),
			Options:     options,
			MultiSelect: isMulti,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("questions is empty")
	}
	return items, nil
}

func parseOptionsFromQuestion(q map[string]any) []AskQuestionOption {
	raw, ok := q["options"]
	if !ok {
		return nil
	}
	if opts := parseOptionPayload(raw); len(opts) > 0 {
		return opts
	}
	return nil
}

func parseOptionsFromInput(input map[string]any, key string) []AskQuestionOption {
	for _, candidate := range append([]string{key}, optionAliases(key)...) {
		raw, ok := input[candidate]
		if !ok {
			continue
		}
		if opts := parseOptionPayload(raw); len(opts) > 0 {
			return opts
		}
	}
	return nil
}

func parseOptionPayload(raw any) []AskQuestionOption {
	parsed, err := parseJSONOrValue(raw)
	if err != nil || parsed == nil {
		return nil
	}

	switch v := parsed.(type) {
	case string:
		return parseCSVOptions(v)
	case []string:
		out := make([]AskQuestionOption, 0, len(v))
		for _, option := range v {
			opt := strings.TrimSpace(option)
			if opt != "" {
				out = append(out, AskQuestionOption{Label: opt, Value: opt})
			}
		}
		return out
	default:
		list, ok := asAnySlice(v)
		if !ok {
			return nil
		}
		return parseOptionArray(list)
	}
}

func parseJSONOrValue(raw any) (any, error) {
	switch v := raw.(type) {
	case nil:
		return nil, nil
	case string:
		s := unwrapQuotedString(strings.TrimSpace(v))
		if s == "" {
			return nil, nil
		}
		if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") {
			var parsed any
			if err := json.Unmarshal([]byte(s), &parsed); err == nil {
				return parsed, nil
			}
		}
		return s, nil
	default:
		return raw, nil
	}
}

func unwrapQuotedString(s string) string {
	if len(s) >= 2 {
		first := s[0]
		last := s[len(s)-1]
		if (first == '\'' && last == '\'') || (first == '"' && last == '"') {
			return strings.TrimSpace(s[1 : len(s)-1])
		}
	}
	return s
}

func asAnySlice(v any) ([]any, bool) {
	switch t := v.(type) {
	case []any:
		return t, true
	case map[string]any:
		return []any{t}, true
	default:
		return nil, false
	}
}

func parseOptionArray(items []any) []AskQuestionOption {
	out := make([]AskQuestionOption, 0, len(items))
	for _, item := range items {
		switch v := item.(type) {
		case string:
			label := strings.TrimSpace(v)
			if label != "" {
				out = append(out, AskQuestionOption{Label: label, Value: label})
			}
		case map[string]any:
			if opt, ok := parseOptionObject(v); ok {
				out = append(out, opt)
			}
		}
	}
	return out
}

func parseOptionObject(v map[string]any) (AskQuestionOption, bool) {
	label := firstNonEmptyString(v, "label", "text", "name", "title", "value")
	if label == "" {
		return AskQuestionOption{}, false
	}
	value := firstNonEmptyString(v, "value")
	if value == "" {
		value = label
	}
	desc := firstNonEmptyString(v, "description", "hint")
	return AskQuestionOption{
		Label:       label,
		Value:       value,
		Description: desc,
	}, true
}

func extractAskQuestionDetail(m map[string]any) string {
	return firstNonEmptyString(m, "detail", "details", "extra_detail", "description", "hint")
}

func firstNonEmptyString(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if raw, ok := m[key]; ok {
			if s, ok := raw.(string); ok {
				trimmed := strings.TrimSpace(s)
				if trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func optionAliases(key string) []string {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "options":
		return []string{"option", "a"}
	case "option":
		return []string{"options", "a"}
	case "a":
		return []string{"options", "option"}
	default:
		return nil
	}
}

func parseCSVOptions(s string) []AskQuestionOption {
	parts := strings.Split(s, ",")
	out := make([]AskQuestionOption, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, AskQuestionOption{Label: v, Value: v})
		}
	}
	return out
}

func shortHeader(s string) string {
	rs := []rune(strings.TrimSpace(s))
	if len(rs) <= 12 {
		return string(rs)
	}
	return string(rs[:12])
}
