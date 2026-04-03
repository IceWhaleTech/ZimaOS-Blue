package server

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/optimization"
)

type AgentcoreRunnerStatus = optimization.Status
type AgentcoreRunnerPrepareRequest = optimization.PrepareRequest

type agentcoreRunnerManager interface {
	GetStatus(ctx context.Context) optimization.Status
	Prepare(ctx context.Context, req optimization.PrepareRequest) (optimization.Status, error)
}

type agentcoreRunnerLastRunReader interface {
	GetLastOptimizationRun(ctx context.Context) (optimization.OptimizationRunRecord, error)
}

func (h *SettingsHandler) SetAgentcoreRunnerManager(manager agentcoreRunnerManager) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.agentcoreRunnerManager = manager
}

func (h *SettingsHandler) AgentcoreRunnerOptimizationManager() *optimization.Manager {
	h.mu.RLock()
	defer h.mu.RUnlock()
	manager, _ := h.agentcoreRunnerManager.(*optimization.Manager)
	return manager
}

func (h *SettingsHandler) GetExperimentalAgentcoreRunnerEnabled() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.settings.ExperimentalAgentcoreRunnerEnabled != nil && *h.settings.ExperimentalAgentcoreRunnerEnabled
}

func (h *SettingsHandler) GetExperimentalAgentcoreRunnerRepoURL() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return strings.TrimSpace(h.settings.ExperimentalAgentcoreRunnerRepoURL)
}

func (h *SettingsHandler) GetExperimentalAgentcoreRunnerRef() string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return strings.TrimSpace(h.settings.ExperimentalAgentcoreRunnerRef)
}

func (h *SettingsHandler) GetAgentcoreRunnerStatus(c echo.Context) error {
	status, err := h.agentcoreRunnerStatus(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, status)
}

func (h *SettingsHandler) GetAgentcoreRunnerLastRun(c echo.Context) error {
	h.mu.RLock()
	manager := h.agentcoreRunnerManager
	h.mu.RUnlock()
	reader, _ := manager.(agentcoreRunnerLastRunReader)
	if reader == nil {
		return c.JSON(http.StatusOK, nil)
	}
	record, err := reader.GetLastOptimizationRun(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, record)
}

func (h *SettingsHandler) PrepareAgentcoreRunner(c echo.Context) error {
	h.mu.RLock()
	manager := h.agentcoreRunnerManager
	repoURL := strings.TrimSpace(h.settings.ExperimentalAgentcoreRunnerRepoURL)
	ref := strings.TrimSpace(h.settings.ExperimentalAgentcoreRunnerRef)
	h.mu.RUnlock()
	if manager == nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "agentcore runner manager not initialized"})
	}
	status, err := manager.Prepare(c.Request().Context(), optimization.PrepareRequest{
		RepoURL: repoURL,
		Ref:     ref,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
	status.Enabled = h.GetExperimentalAgentcoreRunnerEnabled()
	if status.RepoURL == "" {
		status.RepoURL = repoURL
	}
	if status.ResolvedRef == "" {
		status.ResolvedRef = ref
	}
	return c.JSON(http.StatusOK, status)
}

func (h *SettingsHandler) agentcoreRunnerStatus(ctx context.Context) (optimization.Status, error) {
	h.mu.RLock()
	manager := h.agentcoreRunnerManager
	enabled := h.settings.ExperimentalAgentcoreRunnerEnabled != nil && *h.settings.ExperimentalAgentcoreRunnerEnabled
	repoURL := strings.TrimSpace(h.settings.ExperimentalAgentcoreRunnerRepoURL)
	ref := strings.TrimSpace(h.settings.ExperimentalAgentcoreRunnerRef)
	h.mu.RUnlock()

	status := optimization.Status{
		Enabled:     enabled,
		RepoURL:     repoURL,
		ResolvedRef: ref,
	}
	if manager == nil {
		return status, nil
	}
	current := manager.GetStatus(ctx)
	current.Enabled = enabled
	if current.RepoURL == "" {
		current.RepoURL = repoURL
	}
	if current.ResolvedRef == "" {
		current.ResolvedRef = ref
	}
	return current, nil
}
