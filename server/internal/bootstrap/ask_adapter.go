package bootstrap

import (
	"context"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/skill/builtin"
	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/tools"
)

// questionManagerAskAdapter adapts tools.QuestionManager to builtin.AskQuestioner.
type questionManagerAskAdapter struct {
	mgr *tools.QuestionManager
}

func (a *questionManagerAskAdapter) AskQuestions(ctx context.Context, userID, sessionID string, questions []builtin.AskQuestionItem) ([]builtin.AskQuestionAnswerResult, bool, error) {
	items := make([]tools.QuestionItem, 0, len(questions))
	for _, q := range questions {
		opts := make([]tools.QuestionOption, 0, len(q.Options))
		for _, o := range q.Options {
			opts = append(opts, tools.QuestionOption{Label: o.Label, Value: o.Value})
		}
		items = append(items, tools.QuestionItem{
			ID:          q.ID,
			Question:    q.Question,
			Header:      q.Header,
			Options:     opts,
			MultiSelect: q.MultiSelect,
		})
	}

	answers, silent, err := a.mgr.AskQuestions(ctx, userID, sessionID, items)
	if err != nil {
		return nil, false, err
	}

	out := make([]builtin.AskQuestionAnswerResult, 0, len(answers))
	for _, ans := range answers {
		out = append(out, builtin.AskQuestionAnswerResult{
			QuestionID: ans.QuestionID,
			Selected:   ans.Selected,
			OtherText:  ans.OtherText,
		})
	}
	return out, silent, nil
}
