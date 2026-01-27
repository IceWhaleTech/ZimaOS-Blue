package workflow

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// WorkflowService implements the Service interface.
type WorkflowService struct {
	config     *Config
	repo       *Repository
	engine     *Engine
	cron       *cron.Cron
	cronJobs   map[string]cron.EntryID
	cronMu     sync.RWMutex
	webhooks   map[string]string // path -> workflowID
	webhookMu  sync.RWMutex
}

// NewService creates a new workflow service.
func NewService(config *Config, repo *Repository) (*WorkflowService, error) {
	if config == nil {
		config = DefaultConfig()
	}

	engine := NewEngine(config)

	s := &WorkflowService{
		config:   config,
		repo:     repo,
		engine:   engine,
		cron:     cron.New(cron.WithSeconds()),
		cronJobs: make(map[string]cron.EntryID),
		webhooks: make(map[string]string),
	}

	// Start cron scheduler
	s.cron.Start()

	// Load active workflows and register triggers
	if err := s.loadActiveTriggers(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to load triggers: %w", err)
	}

	return s, nil
}

// loadActiveTriggers loads and registers triggers for active workflows.
func (s *WorkflowService) loadActiveTriggers(ctx context.Context) error {
	// This would load all active workflows and register their triggers
	// For now, we'll skip this as it requires tenant context
	return nil
}

// CreateWorkflow creates a new workflow.
func (s *WorkflowService) CreateWorkflow(ctx context.Context, workflow *Workflow) (*Workflow, error) {
	if workflow.ID == "" {
		workflow.ID = uuid.New().String()
	}

	if workflow.Status == "" {
		workflow.Status = WorkflowStatusDraft
	}

	// Validate workflow
	if err := s.ValidateWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	// Save to repository
	if err := s.repo.CreateWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	// Register triggers if active
	if workflow.Status == WorkflowStatusActive {
		s.registerWorkflowTriggers(ctx, workflow)
	}

	return workflow, nil
}

// GetWorkflow retrieves a workflow by ID.
func (s *WorkflowService) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	return s.repo.GetWorkflow(ctx, id)
}

// UpdateWorkflow updates an existing workflow.
func (s *WorkflowService) UpdateWorkflow(ctx context.Context, workflow *Workflow) (*Workflow, error) {
	// Validate workflow
	if err := s.ValidateWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	// Get existing workflow to check status change
	existing, err := s.repo.GetWorkflow(ctx, workflow.ID)
	if err != nil {
		return nil, err
	}

	// Update in repository
	if err := s.repo.UpdateWorkflow(ctx, workflow); err != nil {
		return nil, err
	}

	// Handle status changes
	if existing.Status != workflow.Status {
		if workflow.Status == WorkflowStatusActive {
			s.registerWorkflowTriggers(ctx, workflow)
		} else {
			s.unregisterWorkflowTriggers(ctx, workflow.ID)
		}
	} else if workflow.Status == WorkflowStatusActive {
		// Re-register triggers if workflow is active (config might have changed)
		s.unregisterWorkflowTriggers(ctx, workflow.ID)
		s.registerWorkflowTriggers(ctx, workflow)
	}

	return workflow, nil
}

// DeleteWorkflow deletes a workflow.
func (s *WorkflowService) DeleteWorkflow(ctx context.Context, id string) error {
	// Unregister triggers
	s.unregisterWorkflowTriggers(ctx, id)

	// Delete from repository
	return s.repo.DeleteWorkflow(ctx, id)
}

// ListWorkflows lists workflows with pagination.
func (s *WorkflowService) ListWorkflows(ctx context.Context, tenantID string, opts *ListOptions) ([]*Workflow, int, error) {
	return s.repo.ListWorkflows(ctx, tenantID, opts)
}

// EnableWorkflow enables a workflow.
func (s *WorkflowService) EnableWorkflow(ctx context.Context, id string) error {
	workflow, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return err
	}

	workflow.Status = WorkflowStatusActive
	workflow.UpdatedAt = time.Now()

	if err := s.repo.UpdateWorkflow(ctx, workflow); err != nil {
		return err
	}

	s.registerWorkflowTriggers(ctx, workflow)
	return nil
}

// DisableWorkflow disables a workflow.
func (s *WorkflowService) DisableWorkflow(ctx context.Context, id string) error {
	workflow, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return err
	}

	workflow.Status = WorkflowStatusInactive
	workflow.UpdatedAt = time.Now()

	if err := s.repo.UpdateWorkflow(ctx, workflow); err != nil {
		return err
	}

	s.unregisterWorkflowTriggers(ctx, id)
	return nil
}

// ValidateWorkflow validates a workflow definition.
func (s *WorkflowService) ValidateWorkflow(ctx context.Context, workflow *Workflow) error {
	return s.engine.ValidateWorkflow(workflow)
}

// ExecuteWorkflow manually executes a workflow.
func (s *WorkflowService) ExecuteWorkflow(ctx context.Context, id string, triggerData map[string]interface{}) (*Execution, error) {
	workflow, err := s.repo.GetWorkflow(ctx, id)
	if err != nil {
		return nil, err
	}

	// Allow manual execution even if workflow is inactive
	originalStatus := workflow.Status
	workflow.Status = WorkflowStatusActive

	execution, err := s.engine.Execute(ctx, workflow, TriggerTypeManual, triggerData)

	workflow.Status = originalStatus

	if err != nil {
		return nil, err
	}

	// Save execution
	if err := s.repo.SaveExecution(ctx, execution); err != nil {
		// Log error but don't fail
		fmt.Printf("failed to save execution: %v\n", err)
	}

	return execution, nil
}

// GetExecution retrieves an execution by ID.
func (s *WorkflowService) GetExecution(ctx context.Context, id string) (*Execution, error) {
	// Try engine first (for running executions)
	execution, err := s.engine.GetExecution(id)
	if err == nil {
		return execution, nil
	}

	// Fall back to repository
	return s.repo.GetExecution(ctx, id)
}

// ListExecutions lists executions for a workflow.
func (s *WorkflowService) ListExecutions(ctx context.Context, workflowID string, opts *ListOptions) ([]*Execution, int, error) {
	return s.repo.ListExecutions(ctx, workflowID, opts)
}

// CancelExecution cancels a running execution.
func (s *WorkflowService) CancelExecution(ctx context.Context, id string) error {
	err := s.engine.CancelExecution(id)
	if err != nil {
		return err
	}

	// Update in repository
	execution, err := s.repo.GetExecution(ctx, id)
	if err != nil {
		return nil // Execution might not be persisted yet
	}

	execution.Status = ExecutionStatusCancelled
	now := time.Now()
	execution.CompletedAt = &now

	return s.repo.SaveExecution(ctx, execution)
}

// RetryExecution retries a failed execution.
func (s *WorkflowService) RetryExecution(ctx context.Context, id string) (*Execution, error) {
	execution, err := s.repo.GetExecution(ctx, id)
	if err != nil {
		return nil, err
	}

	if execution.Status != ExecutionStatusFailed {
		return nil, fmt.Errorf("can only retry failed executions")
	}

	// Execute workflow again with same trigger data
	return s.ExecuteWorkflow(ctx, execution.WorkflowID, execution.TriggerData)
}

// GetExecutionLogs retrieves logs for an execution.
func (s *WorkflowService) GetExecutionLogs(ctx context.Context, executionID string, opts *ListOptions) ([]*ExecutionLog, int, error) {
	return s.repo.GetExecutionLogs(ctx, executionID, opts)
}

// RegisterTrigger registers a trigger for a workflow.
func (s *WorkflowService) RegisterTrigger(ctx context.Context, workflowID string, trigger *TriggerConfig) error {
	workflow, err := s.repo.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	// Find or create trigger node
	found := false
	for i, node := range workflow.Nodes {
		if node.Type == NodeTypeTrigger {
			workflow.Nodes[i].Config = map[string]interface{}{
				"trigger": trigger,
			}
			found = true
			break
		}
	}

	if !found {
		workflow.Nodes = append(workflow.Nodes, Node{
			ID:   uuid.New().String(),
			Type: NodeTypeTrigger,
			Name: "Trigger",
			Config: map[string]interface{}{
				"trigger": trigger,
			},
		})
	}

	return s.repo.UpdateWorkflow(ctx, workflow)
}

// UnregisterTrigger unregisters a trigger for a workflow.
func (s *WorkflowService) UnregisterTrigger(ctx context.Context, workflowID string) error {
	s.unregisterWorkflowTriggers(ctx, workflowID)
	return nil
}

// HandleWebhook handles an incoming webhook request.
func (s *WorkflowService) HandleWebhook(ctx context.Context, path string, method string, headers map[string]string, body []byte) (*Execution, error) {
	// Look up workflow by webhook path
	s.webhookMu.RLock()
	workflowID, ok := s.webhooks[path]
	s.webhookMu.RUnlock()

	if !ok {
		// Try repository
		var err error
		workflowID, err = s.repo.GetWebhookByPath(ctx, path)
		if err != nil {
			return nil, ErrWorkflowNotFound
		}
	}

	// Execute workflow
	triggerData := map[string]interface{}{
		"method":  method,
		"headers": headers,
		"body":    string(body),
		"path":    path,
	}

	return s.ExecuteWorkflow(ctx, workflowID, triggerData)
}

// GetStats returns workflow statistics.
func (s *WorkflowService) GetStats(ctx context.Context, tenantID string) (*Stats, error) {
	return s.repo.GetStats(ctx, tenantID)
}

// registerWorkflowTriggers registers all triggers for a workflow.
func (s *WorkflowService) registerWorkflowTriggers(ctx context.Context, workflow *Workflow) {
	for _, node := range workflow.Nodes {
		if node.Type != NodeTypeTrigger {
			continue
		}

		triggerConfig, ok := node.Config["trigger"].(map[string]interface{})
		if !ok {
			continue
		}

		triggerType, _ := triggerConfig["type"].(string)

		switch TriggerType(triggerType) {
		case TriggerTypeSchedule:
			s.registerScheduleTrigger(ctx, workflow, triggerConfig)
		case TriggerTypeWebhook:
			s.registerWebhookTrigger(ctx, workflow, triggerConfig)
		}
	}
}

// unregisterWorkflowTriggers unregisters all triggers for a workflow.
func (s *WorkflowService) unregisterWorkflowTriggers(ctx context.Context, workflowID string) {
	// Unregister cron job
	s.cronMu.Lock()
	if entryID, ok := s.cronJobs[workflowID]; ok {
		s.cron.Remove(entryID)
		delete(s.cronJobs, workflowID)
	}
	s.cronMu.Unlock()

	// Unregister webhook
	s.webhookMu.Lock()
	for path, id := range s.webhooks {
		if id == workflowID {
			delete(s.webhooks, path)
			break
		}
	}
	s.webhookMu.Unlock()

	// Delete from repository
	s.repo.DeleteWebhook(ctx, workflowID)
}

// registerScheduleTrigger registers a schedule trigger.
func (s *WorkflowService) registerScheduleTrigger(ctx context.Context, workflow *Workflow, config map[string]interface{}) {
	scheduleConfig, ok := config["schedule"].(map[string]interface{})
	if !ok {
		return
	}

	cronExpr, _ := scheduleConfig["cron"].(string)
	if cronExpr == "" {
		return
	}

	s.cronMu.Lock()
	defer s.cronMu.Unlock()

	// Remove existing job if any
	if entryID, ok := s.cronJobs[workflow.ID]; ok {
		s.cron.Remove(entryID)
	}

	// Add new job
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		triggerData := map[string]interface{}{
			"scheduled_at": time.Now().Format(time.RFC3339),
			"cron":         cronExpr,
		}

		ctx := context.Background()
		execution, err := s.ExecuteWorkflow(ctx, workflow.ID, triggerData)
		if err != nil {
			fmt.Printf("scheduled execution failed for workflow %s: %v\n", workflow.ID, err)
			return
		}

		fmt.Printf("scheduled execution started for workflow %s: %s\n", workflow.ID, execution.ID)
	})

	if err != nil {
		fmt.Printf("failed to register cron job for workflow %s: %v\n", workflow.ID, err)
		return
	}

	s.cronJobs[workflow.ID] = entryID
}

// registerWebhookTrigger registers a webhook trigger.
func (s *WorkflowService) registerWebhookTrigger(ctx context.Context, workflow *Workflow, config map[string]interface{}) {
	webhookConfig, ok := config["webhook"].(map[string]interface{})
	if !ok {
		return
	}

	path, _ := webhookConfig["path"].(string)
	if path == "" {
		path = fmt.Sprintf("/workflow/%s", workflow.ID)
	}

	method, _ := webhookConfig["method"].(string)
	if method == "" {
		method = "POST"
	}

	authType, _ := webhookConfig["auth_type"].(string)
	authConfig, _ := webhookConfig["auth_config"].(map[string]string)

	s.webhookMu.Lock()
	s.webhooks[path] = workflow.ID
	s.webhookMu.Unlock()

	// Save to repository
	s.repo.SaveWebhook(ctx, workflow.ID, path, method, authType, authConfig)
}

// RegisterActionHandler registers a custom action handler.
func (s *WorkflowService) RegisterActionHandler(actionType ActionType, handler ActionHandler) {
	s.engine.RegisterHandler(actionType, handler)
}

// Close shuts down the service.
func (s *WorkflowService) Close() error {
	s.cron.Stop()
	return s.engine.Close()
}

// CleanupOldExecutions removes old execution data.
func (s *WorkflowService) CleanupOldExecutions(ctx context.Context) (int64, error) {
	return s.repo.CleanupOldExecutions(ctx, s.config.ExecutionDataRetention)
}

// StartCleanupJob starts a background job to clean up old executions.
func (s *WorkflowService) StartCleanupJob() {
	// Run cleanup daily at 3 AM
	s.cron.AddFunc("0 0 3 * * *", func() {
		ctx := context.Background()
		deleted, err := s.CleanupOldExecutions(ctx)
		if err != nil {
			fmt.Printf("cleanup job failed: %v\n", err)
			return
		}
		fmt.Printf("cleanup job deleted %d old executions\n", deleted)
	})
}
