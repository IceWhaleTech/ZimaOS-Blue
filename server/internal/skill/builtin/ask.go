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
	Header      string
	Options     []AskQuestionOption
	MultiSelect bool
}

// AskQuestionOption is a single selectable option.
type AskQuestionOption struct {
	Label string
	Value string
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
					Name:        "question",
					Type:        "string",
					Description: "Single question text.",
					Required:    false,
				},
				{
					Name:        "options",
					Type:        "string",
					Description: "Options for single question. Prefer JSON string array, e.g. [\"A\",\"B\"]. Aliases: option, a.",
					Required:    false,
				},
				{
					Name:        "questions",
					Type:        "string",
					Description: "JSON array of questions: [{\"question\":\"...\",\"options\":[\"A\",\"B\"]}] (also supports q/a aliases).",
					Required:    false,
				},
			},
		},
	}
}

func (a *Ask) SetQuestioner(q AskQuestioner) { a.questioner = q }

func (a *Ask) Manifest() *skill.Manifest { return a.manifest }

func (a *Ask) Validate(input map[string]any) error {
	_, hasQ := input["q"]
	_, hasMQ := input["mq"]
	if !hasQ && !hasMQ {
		return fmt.Errorf("q or mq is required")
	}
	return nil
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
		if s, ok := raw.(string); ok && strings.TrimSpace(s) != "" {
			var payload []struct {
				Q        string   `json:"q"`
				Type     string   `json:"type"`
				A        []string `json:"a"`
				Question string   `json:"question"`
				Options  []string `json:"options"`
			}
			if err := json.Unmarshal([]byte(s), &payload); err != nil {
				return nil, fmt.Errorf("invalid questions JSON: %w", err)
			}
			items := make([]AskQuestionItem, 0, len(payload))
			for i, q := range payload {
				question := strings.TrimSpace(q.Question)
				if question == "" {
					question = strings.TrimSpace(q.Q)
				}
				if question == "" {
					continue
				}
				optionsArr := q.Options
				if len(optionsArr) == 0 {
					optionsArr = q.A
				}
				options := toQuestionOptions(optionsArr)
				if len(options) == 0 {
					return nil, fmt.Errorf("question %d has no options", i+1)
				}
				header := shortHeader(question)
				isMulti := strings.EqualFold(strings.TrimSpace(q.Type), "checkbox")
				items = append(items, AskQuestionItem{
					ID:          fmt.Sprintf("q%d", i),
					Question:    question,
					Header:      header,
					Options:     options,
					MultiSelect: isMulti,
				})
			}
			if len(items) == 0 {
				return nil, fmt.Errorf("questions is empty")
			}
			return items, nil
		}
	}

	if q, ok := input["q"].(string); ok && strings.TrimSpace(q) != "" {
		options := parseOptionsFromInput(input, "a")
		if len(options) < 2 {
			return nil, fmt.Errorf("a must include at least 2 values for q")
		}
		question := strings.TrimSpace(q)
		return []AskQuestionItem{
			{
				ID:          "q0",
				Question:    question,
				Header:      shortHeader(question),
				Options:     toQuestionOptions(options),
				MultiSelect: false,
			},
		}, nil
	}
	if mq, ok := input["mq"].(string); ok && strings.TrimSpace(mq) != "" {
		options := parseOptionsFromInput(input, "a")
		if len(options) < 2 {
			return nil, fmt.Errorf("a must include at least 2 values for mq")
		}
		question := strings.TrimSpace(mq)
		return []AskQuestionItem{
			{
				ID:          "q0",
				Question:    question,
				Header:      shortHeader(question),
				Options:     toQuestionOptions(options),
				MultiSelect: true,
			},
		}, nil
	}

	question, _ := input["question"].(string)
	question = strings.TrimSpace(question)
	if question == "" {
		return nil, fmt.Errorf("question is required")
	}
	options := parseOptionsFromInput(input, "options")
	if len(options) < 2 {
		return nil, fmt.Errorf("options must include at least 2 values")
	}

	return []AskQuestionItem{
		{
			ID:          "q0",
			Question:    question,
			Header:      shortHeader(question),
			Options:     toQuestionOptions(options),
			MultiSelect: false,
		},
	}, nil
}

func parseOptionsFromInput(input map[string]any, key string) []string {
	raw := trimmedString(input, key)
	if raw == "" {
		for _, alias := range optionAliases(key) {
			raw = trimmedString(input, alias)
			if raw != "" {
				break
			}
		}
	}
	if raw == "" {
		return nil
	}
	var arr []string
	if strings.HasPrefix(raw, "[") && strings.HasSuffix(raw, "]") {
		if err := json.Unmarshal([]byte(raw), &arr); err == nil {
			return normalizeOptions(arr)
		}
	}
	return parseCSVOptions(raw)
}

func trimmedString(input map[string]any, key string) string {
	raw, _ := input[key].(string)
	return strings.TrimSpace(raw)
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

func parseCSVOptions(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func normalizeOptions(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func toQuestionOptions(values []string) []AskQuestionOption {
	out := make([]AskQuestionOption, 0, len(values))
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		out = append(out, AskQuestionOption{Label: v, Value: v})
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
