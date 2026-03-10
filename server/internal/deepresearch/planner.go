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
	if looksLikePersonTimelineResearch(q, lang) {
		return planPersonResearchTasks(q, mode, lang)
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
		axis := "overview"
		switch i {
		case 1:
			axis = "latest"
		case 2:
			axis = "official"
		case 3:
			axis = "comparison"
		case 4:
			axis = "best_practices"
		}
		tasks = append(tasks, Task{
			ID:       fmt.Sprintf("task_%d", i+1),
			Question: base[i],
			Priority: i + 1,
			Depth:    1,
			Status:   "pending",
			Axis:     axis,
			Category: "generic_research",
		})
	}
	return tasks
}
