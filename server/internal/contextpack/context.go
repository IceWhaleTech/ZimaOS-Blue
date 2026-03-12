package contextpack

import "context"

type ctxKey string

const (
	queryKey         ctxKey = "contextpack_query"
	selectedSkillKey ctxKey = "contextpack_selected_skill"
	budgetTokensKey  ctxKey = "contextpack_budget_tokens"
)

func WithPromptQuery(ctx context.Context, query string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, queryKey, query)
}

func PromptQueryFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(queryKey).(string)
	return v
}

func WithSelectedSkill(ctx context.Context, skill string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, selectedSkillKey, skill)
}

func SelectedSkillFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	v, _ := ctx.Value(selectedSkillKey).(string)
	return v
}

func WithBudgetTokens(ctx context.Context, tokens int) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, budgetTokensKey, tokens)
}

func BudgetTokensFromContext(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	v, _ := ctx.Value(budgetTokensKey).(int)
	return v
}
