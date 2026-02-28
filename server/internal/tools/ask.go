package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// AskTool is a first-class tool that asks the user questions.
type AskTool struct {
	mgr *QuestionManager
}

// NewAskTool creates a new ask tool.
func NewAskTool(mgr *QuestionManager) *AskTool {
	return &AskTool{mgr: mgr}
}

// Definition returns the tool definition.
// Supports both single question (q/mq + a) and multiple questions (questions array).
func (t *AskTool) Definition() ToolDefinition {
	optionItemSchema := map[string]interface{}{
		"oneOf": []interface{}{
			map[string]interface{}{"type": "string"},
			map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"label":       map[string]interface{}{"type": "string"},
					"description": map[string]interface{}{"type": "string"},
					"value":       map[string]interface{}{"type": "string"},
				},
			},
		},
	}

	return ToolDefinition{
		Name: "ask",
		Description: `Ask the user one or more questions.
Preferred format: {"questions":[{"question":"...","type":"radio","options":[...]}]}.
Single-question shorthand: {"q":"...","a":[...]} or {"mq":"...","a":[...]}.
Option items can be strings or objects: {"label":"...","description":"...","value":"..."}.
Inside questions items, use only "question"/"options"/"type".`,
		Icon: "question",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"questions": map[string]interface{}{
					"type":        "array",
					"description": "Preferred. Array of question objects.",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"question": map[string]interface{}{
								"type":        "string",
								"description": "Question text.",
							},
							"type": map[string]interface{}{
								"type":        "string",
								"enum":        []string{"radio", "checkbox"},
								"description": "Selection mode.",
							},
							"options": map[string]interface{}{
								"type":        "array",
								"items":       optionItemSchema,
								"description": "Selectable options.",
							},
						},
						"required": []string{"question", "options"},
					},
				},
				"q": map[string]interface{}{
					"type":        "string",
					"description": "Single-select question text (shorthand).",
				},
				"mq": map[string]interface{}{
					"type":        "string",
					"description": "Multi-select question text (shorthand).",
				},
				"a": map[string]interface{}{
					"type":        "array",
					"items":       optionItemSchema,
					"description": "Options for q/mq shorthand.",
				},
			},
		},
	}
}

// Execute asks the user one or more questions and returns their answers.
func (t *AskTool) Execute(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if t.mgr == nil {
		return nil, fmt.Errorf("question manager not available")
	}

	// Debug: log incoming args with keys
	argsJSON, _ := json.Marshal(args)
	keys := make([]string, 0, len(args))
	for k := range args {
		keys = append(keys, k)
	}
	fmt.Printf("[ask] Execute called with args: %s, keys: %v\n", argsJSON, keys)

	var questions []QuestionItem

	// Check for multi-question format: questions array
	if qArr, ok := args["questions"].([]interface{}); ok && len(qArr) > 0 {
		for i, item := range qArr {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			qText, _ := m["question"].(string)
			qText = strings.TrimSpace(qText)
			if qText == "" {
				continue
			}
			qType, _ := m["type"].(string)
			isMulti := strings.EqualFold(qType, "checkbox") || strings.EqualFold(qType, "multi")

			qOpts := parseQuestionOptions(m["options"])
			if len(qOpts) == 0 {
				continue
			}

			questions = append(questions, QuestionItem{
				ID:          fmt.Sprintf("q%d", i),
				Question:    qText,
				Header:      shortHeader(qText),
				Options:     qOpts,
				MultiSelect: isMulti,
			})
		}
	}

	// Fallback to single question format: q/mq + a
	if len(questions) == 0 {
		question, multiSelect, options := parseAskArgs(args)
		if question == "" {
			return nil, fmt.Errorf("q/mq + a or questions array is required")
		}
		if len(options) == 0 {
			return nil, fmt.Errorf("ask options are required")
		}

		questions = []QuestionItem{{
			ID:          "q0",
			Question:    question,
			Header:      shortHeader(question),
			Options:     options,
			MultiSelect: multiSelect,
		}}
	}

	userID := GetUserID(ctx)
	sessionID := GetSessionID(ctx)

	answers, silent, err := t.mgr.AskQuestions(ctx, userID, sessionID, questions)
	if err != nil {
		return nil, err
	}

	// Build result: simple for LLM + qa wrapper for card renderer
	sel := []string{}
	if len(answers) > 0 {
		sel = answers[0].Selected
		if answers[0].OtherText != "" {
			sel = append(sel, answers[0].OtherText)
		}
	}

	// Build qa array for card renderer - need to look up question text from original questions
	qaList := make([]map[string]interface{}, len(answers))
	for i, ans := range answers {
		// Find the source question by ID so we can return readable labels.
		var qItem *QuestionItem
		for _, q := range questions {
			if q.ID == ans.QuestionID {
				qCopy := q
				qItem = &qCopy
				break
			}
		}
		qText := ""
		optLabels := []string{}
		if qItem != nil {
			qText = qItem.Question
			optLabels = make([]string, 0, len(qItem.Options))
			for _, opt := range qItem.Options {
				if opt.Label != "" {
					optLabels = append(optLabels, opt.Label)
				}
			}
		}
		qaList[i] = map[string]interface{}{
			"q": qText,
			"o": optLabels,
			"a": ans.Selected,
		}
	}

	result := map[string]interface{}{
		"a":  sel,
		"qa": qaList,
	}
	if len(questions) == 1 {
		if questions[0].MultiSelect {
			result["mq"] = questions[0].Question
		} else {
			result["q"] = questions[0].Question
		}
	}
	if silent {
		result["silent"] = true
	}

	jsonResult, _ := json.Marshal(result)
	return string(jsonResult), nil
}

// parseAskArgs extracts question text, multi-select flag, and options from args.
// Supports the primary q/mq/a format and falls back to top-level legacy format.
func parseAskArgs(args map[string]interface{}) (question string, multiSelect bool, options []QuestionOption) {
	// Primary format: q/mq + a
	if q, ok := args["q"].(string); ok && q != "" {
		question = q
	}
	if mq, ok := args["mq"].(string); ok && mq != "" {
		question = mq
		multiSelect = true
	}
	options = parseQuestionOptions(args["a"])
	if len(options) == 0 {
		options = parseQuestionOptions(args["options"])
	}
	if question != "" {
		return
	}

	// Legacy fallback: "questions" array (question/options only) or "question"/"options" at top level
	question, multiSelect, options = parseLegacyArgs(args)
	return
}

// parseLegacyArgs handles the old nested "questions" format for backward compat.
func parseLegacyArgs(args map[string]interface{}) (question string, multiSelect bool, options []QuestionOption) {
	// Try "question" + "options" at top level
	if q, ok := args["question"].(string); ok && q != "" {
		question = strings.TrimSpace(q)
		if ms, ok := args["multi_select"].(bool); ok {
			multiSelect = ms
		}
		// "type":"checkbox" compat
		if typ, ok := args["type"].(string); ok {
			if strings.EqualFold(typ, "checkbox") || strings.EqualFold(typ, "multi") {
				multiSelect = true
			}
		}
		options = parseQuestionOptions(args["options"])
		if len(options) == 0 {
			options = parseQuestionOptions(args["a"])
		}
		return
	}

	// Try "questions" array — take first item only
	questionsRaw, ok := args["questions"]
	if !ok {
		return
	}
	b, err := json.Marshal(questionsRaw)
	if err != nil {
		return
	}
	// Wrap single object
	if len(b) > 0 && b[0] == '{' {
		b = append(append([]byte{'['}, b...), ']')
	}
	var items []map[string]interface{}
	if json.Unmarshal(b, &items) != nil || len(items) == 0 {
		return
	}
	first := items[0]
	if q, ok := first["question"].(string); ok && strings.TrimSpace(q) != "" {
		question = strings.TrimSpace(q)
	}
	if ms, ok := first["multi_select"].(bool); ok {
		multiSelect = ms
	}
	if typ, _ := first["type"].(string); strings.EqualFold(typ, "checkbox") || strings.EqualFold(typ, "multi") {
		multiSelect = true
	}
	options = parseQuestionOptions(first["options"])
	return
}

func shortHeader(question string) string {
	header := strings.TrimSpace(question)
	if len([]rune(header)) <= 12 {
		return header
	}
	return string([]rune(header)[:12])
}

// parseQuestionOptions converts option payloads to QuestionOption.
// Supports:
// - []string
// - []interface{} of strings
// - []interface{} of objects: {label,text,name,title,value,description,hint}
func parseQuestionOptions(v interface{}) []QuestionOption {
	switch arr := v.(type) {
	case []interface{}:
		out := make([]QuestionOption, 0, len(arr))
		for _, item := range arr {
			switch val := item.(type) {
			case string:
				s := strings.TrimSpace(val)
				if s != "" {
					out = append(out, QuestionOption{Label: s, Value: s})
				}
			case map[string]interface{}:
				// {label: "..."} or {text: "..."} or {name: "..."} with optional description/hint
				label := ""
				for _, k := range []string{"label", "text", "name", "title", "value"} {
					if s, ok := val[k].(string); ok && strings.TrimSpace(s) != "" {
						label = strings.TrimSpace(s)
						break
					}
				}
				if label == "" {
					continue
				}
				desc := ""
				for _, k := range []string{"description", "hint"} {
					if s, ok := val[k].(string); ok && strings.TrimSpace(s) != "" {
						desc = strings.TrimSpace(s)
						break
					}
				}
				value := label
				if s, ok := val["value"].(string); ok && strings.TrimSpace(s) != "" {
					value = strings.TrimSpace(s)
				}
				out = append(out, QuestionOption{
					Label:       label,
					Description: desc,
					Value:       value,
				})
			}
		}
		return out
	case []string:
		out := make([]QuestionOption, 0, len(arr))
		for _, s := range arr {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, QuestionOption{Label: s, Value: s})
			}
		}
		return out
	}
	return nil
}

// RegisterAskTool registers the ask tool.
func RegisterAskTool(registry *Registry, mgr *QuestionManager) {
	if mgr == nil {
		return
	}
	registry.Register(NewAskTool(mgr))
}

// GetQuestionManager retrieves the QuestionManager via the registered tool.
func GetQuestionManager(registry *Registry) *QuestionManager {
	tool := registry.Get("ask")
	if tool == nil {
		return nil
	}
	if t, ok := tool.(*AskTool); ok {
		return t.mgr
	}
	return nil
}
