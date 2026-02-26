package tools

import (
	"context"
	"encoding/json"
	"fmt"
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
// Supports both single question (sq/mq + a) and multiple questions (questions array).
func (t *AskTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name: "ask",
		Description: `Ask the user one or more questions.
For single question: use "sq" (single-select/radio) or "mq" (multi-select/checkbox), with "a" as options array.
For multiple questions: use "questions" array, each with "q" (question text), "type" ("radio"/"checkbox"), and "a" (options).

Example single: {"sq": "Preferred language?", "a": ["Python", "Go", "Rust"]}
Example multi: {"questions": [{"q": "Favorite language?", "type": "radio", "a": ["Python", "Go"]}, {"q": "Preferred IDE?", "type": "checkbox", "a": ["VS Code", "Vim"]}]}`,
		Icon: "question",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"sq": map[string]interface{}{
					"type":        "string",
					"description": "Single-select question text (radio buttons). Use for one question.",
				},
				"mq": map[string]interface{}{
					"type":        "string",
					"description": "Multi-select question text (checkboxes). Use for one question.",
				},
				"a": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Options for single question (2-4 strings).",
				},
				"questions": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"q":    map[string]interface{}{"type": "string"},
							"type": map[string]interface{}{"type": "string", "enum": []string{"radio", "checkbox"}},
							"a":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
						},
					},
					"description": "Multiple questions array. Use this for 2+ questions.",
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
			qText, _ := m["q"].(string)
			if qText == "" {
				continue
			}
			qType, _ := m["type"].(string)
			isMulti := qType == "checkbox"

			opts, _ := m["a"].([]interface{})
			qOpts := make([]QuestionOption, len(opts))
			for j, o := range opts {
				if s, ok := o.(string); ok {
					qOpts[j] = QuestionOption{Label: s, Value: s}
				}
			}

			header := qText
			if len([]rune(header)) > 12 {
				header = string([]rune(header)[:12])
			}

			questions = append(questions, QuestionItem{
				ID:          fmt.Sprintf("q%d", i),
				Question:    qText,
				Header:      header,
				Options:     qOpts,
				MultiSelect: isMulti,
			})
		}
	}

	// Fallback to single question format: sq/mq + a
	if len(questions) == 0 {
		question, multiSelect, options := parseAskArgs(args)
		if question == "" {
			return nil, fmt.Errorf("sq/mq + a or questions array is required")
		}

		header := question
		if len([]rune(header)) > 12 {
			header = string([]rune(header)[:12])
		}

		qOpts := make([]QuestionOption, len(options))
		for i, s := range options {
			qOpts[i] = QuestionOption{Label: s, Value: s}
		}
		questions = []QuestionItem{{
			ID:          "q0",
			Question:    question,
			Header:      header,
			Options:     qOpts,
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
		// Find the question text by ID
		qText := ""
		for _, q := range questions {
			if q.ID == ans.QuestionID {
				qText = q.Question
				break
			}
		}
		optLabels := make([]string, len(ans.Selected))
		copy(optLabels, ans.Selected)
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
			result["sq"] = questions[0].Question
		}
	}
	if silent {
		result["silent"] = true
	}

	jsonResult, _ := json.Marshal(result)
	return string(jsonResult), nil
}

// parseAskArgs extracts question text, multi-select flag, and options from args.
// Supports the primary sq/mq/a format and falls back to legacy "questions" format.
func parseAskArgs(args map[string]interface{}) (question string, multiSelect bool, options []string) {
	// Primary format: sq/mq + a
	if sq, ok := args["sq"].(string); ok && sq != "" {
		question = sq
	}
	if mq, ok := args["mq"].(string); ok && mq != "" {
		question = mq
		multiSelect = true
	}
	if aRaw, ok := args["a"]; ok {
		options = toStringSlice(aRaw)
	}
	if question != "" {
		return
	}

	// Legacy fallback: "questions" array or "question"/"options" at top level
	question, multiSelect, options = parseLegacyArgs(args)
	return
}

// parseLegacyArgs handles the old nested "questions" format for backward compat.
func parseLegacyArgs(args map[string]interface{}) (question string, multiSelect bool, options []string) {
	// Try "question" + "options" at top level
	if q, ok := args["question"].(string); ok && q != "" {
		question = q
		if ms, ok := args["multi_select"].(bool); ok {
			multiSelect = ms
		}
		// "type":"checkbox" compat
		if typ, ok := args["type"].(string); ok {
			if typ == "checkbox" || typ == "multi" {
				multiSelect = true
			}
		}
		if aRaw, ok := args["options"]; ok {
			options = toStringSlice(aRaw)
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
	if q, ok := first["question"].(string); ok {
		question = q
	}
	if ms, ok := first["multi_select"].(bool); ok {
		multiSelect = ms
	}
	if typ, _ := first["type"].(string); typ == "checkbox" || typ == "multi" {
		multiSelect = true
	}
	if aRaw, ok := first["options"]; ok {
		options = toStringSlice(aRaw)
	}
	return
}

// toStringSlice converts various option formats to []string.
func toStringSlice(v interface{}) []string {
	switch arr := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(arr))
		for _, item := range arr {
			switch val := item.(type) {
			case string:
				if val != "" {
					out = append(out, val)
				}
			case map[string]interface{}:
				// {label: "..."} or {text: "..."} or {name: "..."}
				for _, k := range []string{"label", "text", "name", "title", "value"} {
					if s, ok := val[k].(string); ok && s != "" {
						out = append(out, s)
						break
					}
				}
			}
		}
		return out
	case []string:
		return arr
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

