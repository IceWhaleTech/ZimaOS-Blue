package deepresearch

import (
	"fmt"
	"strings"
)

type Planner interface {
	Plan(query string, mode Mode) []Task
}

type HeuristicPlanner struct{}

func NewHeuristicPlanner() *HeuristicPlanner {
	return &HeuristicPlanner{}
}

func (p *HeuristicPlanner) Plan(query string, mode Mode) []Task {
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
	base := []string{
		q,
		fmt.Sprintf("%s latest updates", q),
		fmt.Sprintf("%s official documentation", q),
		fmt.Sprintf("%s benchmark comparison", q),
		fmt.Sprintf("%s best practices", q),
	}

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
