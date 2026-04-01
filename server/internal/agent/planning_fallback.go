package agent

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
)

func fallbackPlanResult(goal string) *planResult {
	plan := fallbackRuntimePlan(goal)
	if len(plan.Subtasks) == 0 {
		return nil
	}

	steps := make([]PlanStep, 0, len(plan.Subtasks))
	for i, subtask := range plan.Subtasks {
		steps = append(steps, PlanStep{
			Index:       i,
			Description: strings.TrimSpace(subtask),
			Status:      StepStatusPending,
		})
	}

	return &planResult{
		Goal:            plan.Goal,
		Raw:             plan,
		Steps:           steps,
		SuccessCriteria: plan.SuccessCriteria,
		FallbackPlan:    plan.FallbackPlan,
	}
}

func fallbackRuntimePlan(goal string) runtimePlan {
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return runtimePlan{}
	}

	if skill, ok := routingcue.InferSkill(goal); ok {
		switch skill {
		case "web_query":
			return fillRuntimePlanDefaults(runtimePlan{
				Goal: goal,
				Subtasks: []string{
					"Identify the official public source that should answer the request.",
					"Use web_query to retrieve the latest relevant public documentation and capture evidence.",
					"Summarize the verified result with the source or explain the evidence limit.",
				},
				SuccessCriteria: []string{
					"official public documentation is retrieved or directly verified",
					"the response cites the verified source or clearly states the evidence limitation",
				},
				FallbackPlan: []string{
					"retry once with an official vendor-docs domain constraint",
					"if live evidence is still unavailable, stop and explain the limitation without inventing facts",
				},
			})
		case "analyze":
			return fillRuntimePlanDefaults(runtimePlan{
				Goal: goal,
				Subtasks: []string{
					"Collect the referenced URL or provided public text that needs analysis.",
					"Use analyze to extract the key facts from that public content without switching to unrelated local workspace files.",
					"Summarize the findings and note any missing source evidence or access limits.",
				},
				SuccessCriteria: []string{
					"the referenced public content is inspected or its access limitation is made explicit",
					"the final summary is grounded in the analyzed source content",
				},
				FallbackPlan: []string{
					"retry once with the exact URL or source text boundary clarified",
					"if the source is still unavailable, report the missing public evidence clearly and stop",
				},
			})
		case "workspace_local":
			return fillRuntimePlanDefaults(runtimePlan{
				Goal: goal,
				Subtasks: []string{
					"Read the relevant local workspace files needed for the request.",
					"Extract the key facts from the local content without switching to unrelated web sources.",
					"Summarize the findings and note any missing local evidence.",
				},
				SuccessCriteria: []string{
					"the relevant local workspace content is inspected",
					"the final summary is grounded in local file content",
				},
				FallbackPlan: []string{
					"inspect the nearest matching local files or paths once",
					"if the needed files are missing, report that clearly and stop",
				},
			})
		case "reminder":
			return fillRuntimePlanDefaults(runtimePlan{
				Goal: goal,
				Subtasks: []string{
					"Extract the reminder time, message, and scheduling intent from the request.",
					"Create the reminder with the reminder capability using the parsed schedule details.",
					"Confirm the reminder details or explain any scheduling limit.",
				},
				SuccessCriteria: []string{
					"the reminder is created with the requested schedule or the blocking issue is explained",
					"the final response confirms the scheduled reminder details",
				},
				FallbackPlan: []string{
					"retry once with a normalized absolute time interpretation",
					"if the time is still not schedulable, ask one targeted clarifying question or stop with the constraint",
				},
			})
		case "browser":
			return fillRuntimePlanDefaults(runtimePlan{
				Goal: goal,
				Subtasks: []string{
					"Open the requested page in the browser capability.",
					"Inspect the page state needed for the request.",
					"Report the verified browser result or blocker.",
				},
			})
		case "ui_reviewer":
			return fillRuntimePlanDefaults(runtimePlan{
				Goal: goal,
				Subtasks: []string{
					"Inspect the provided UI surface or screenshot.",
					"Evaluate the interface for usability and accessibility issues.",
					"Report the prioritized findings with concrete evidence.",
				},
			})
		}
	}

	lower := strings.ToLower(goal)
	if strings.Contains(lower, "workspace") || strings.Contains(lower, "readme") {
		return fillRuntimePlanDefaults(runtimePlan{
			Goal: goal,
			Subtasks: []string{
				"Inspect the relevant local workspace context.",
				"Carry out the minimal local action needed to satisfy the request.",
				"Verify the result and summarize the outcome.",
			},
		})
	}

	return fillRuntimePlanDefaults(runtimePlan{
		Goal: goal,
		Subtasks: []string{
			"Inspect the available context and determine the minimal valid path.",
			"Execute the next concrete action needed to make progress.",
			"Verify the outcome and summarize the result or blocker.",
		},
	})
}
