package deepresearch

import (
	"fmt"
	"strings"
)

type Planner interface {
	Plan(query string, mode Mode, lang string) []Task
}

type HeuristicPlanner struct{}

func NewHeuristicPlanner() *HeuristicPlanner {
	return &HeuristicPlanner{}
}

func (p *HeuristicPlanner) Plan(query string, mode Mode, lang string) []Task {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	maxTasks := 3
	if mode == ModeDeep {
		maxTasks = 5
	} else if mode == ModeFast {
		maxTasks = 2
	}

	tasks := make([]Task, 0, maxTasks)
	base := planQueriesForLang(q, lang)

	for i := 0; i < maxTasks && i < len(base); i++ {
		tasks = append(tasks, Task{
			ID:       fmt.Sprintf("task_%d", i+1),
			Question: base[i],
			Priority: i + 1,
			Depth:    1,
			Status:   "pending",
		})
	}
	return tasks
}
