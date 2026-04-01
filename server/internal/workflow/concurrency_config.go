package workflow

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	workflowConcurrentSlotPollInterval = 100 * time.Millisecond

	workflowMaxConcurrentEnv       = "ZIMA_WORKFLOW_MAX_CONCURRENT_EXECUTIONS"
	legacyWorkflowMaxConcurrentEnv = "BLUE_WORKFLOW_MAX_CONCURRENT_EXECUTIONS"
)

func resolveDefaultMaxConcurrentExecutions() int {
	for _, envName := range []string{workflowMaxConcurrentEnv, legacyWorkflowMaxConcurrentEnv} {
		raw := strings.TrimSpace(os.Getenv(envName))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			continue
		}
		return value
	}
	return 10
}
