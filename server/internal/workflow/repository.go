package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Repository handles workflow persistence.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new workflow repository.
func NewRepository(db *sql.DB) (*Repository, error) {
	r := &Repository{db: db}

	if err := r.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return r, nil
}

// migrate creates the necessary database tables.
func (r *Repository) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'draft',
			nodes TEXT NOT NULL,
			connections TEXT NOT NULL,
			variables TEXT,
			settings TEXT,
			tags TEXT,
			version INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			created_by TEXT,
			updated_by TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workflows_tenant_id ON workflows(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status)`,

		`CREATE TABLE IF NOT EXISTS workflow_executions (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			workflow_name TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			status TEXT NOT NULL,
			trigger_type TEXT NOT NULL,
			trigger_data TEXT,
			variables TEXT,
			node_results TEXT,
			error TEXT,
			started_at DATETIME NOT NULL,
			completed_at DATETIME,
			duration INTEGER,
			FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_workflow_id ON workflow_executions(workflow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_tenant_id ON workflow_executions(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_status ON workflow_executions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_started_at ON workflow_executions(started_at)`,

		`CREATE TABLE IF NOT EXISTS workflow_execution_logs (
			id TEXT PRIMARY KEY,
			execution_id TEXT NOT NULL,
			node_id TEXT,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			data TEXT,
			timestamp DATETIME NOT NULL,
			FOREIGN KEY (execution_id) REFERENCES workflow_executions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_logs_execution_id ON workflow_execution_logs(execution_id)`,

		`CREATE TABLE IF NOT EXISTS workflow_webhooks (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL UNIQUE,
			path TEXT NOT NULL UNIQUE,
			method TEXT NOT NULL DEFAULT 'POST',
			auth_type TEXT,
			auth_config TEXT,
			created_at DATETIME NOT NULL,
			FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_webhooks_path ON workflow_webhooks(path)`,
	}

	for _, query := range queries {
		if _, err := r.db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute migration: %w", err)
		}
	}

	return nil
}

// CreateWorkflow creates a new workflow.
func (r *Repository) CreateWorkflow(ctx context.Context, workflow *Workflow) error {
	if workflow.ID == "" {
		workflow.ID = uuid.New().String()
	}

	now := time.Now()
	workflow.CreatedAt = now
	workflow.UpdatedAt = now
	workflow.Version = 1

	nodesJSON, err := json.Marshal(workflow.Nodes)
	if err != nil {
		return fmt.Errorf("failed to marshal nodes: %w", err)
	}

	connectionsJSON, err := json.Marshal(workflow.Connections)
	if err != nil {
		return fmt.Errorf("failed to marshal connections: %w", err)
	}

	variablesJSON, err := json.Marshal(workflow.Variables)
	if err != nil {
		return fmt.Errorf("failed to marshal variables: %w", err)
	}

	settingsJSON, err := json.Marshal(workflow.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	tagsJSON, err := json.Marshal(workflow.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `INSERT INTO workflows (
		id, tenant_id, name, description, status, nodes, connections,
		variables, settings, tags, version, created_at, updated_at, created_by, updated_by
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = r.db.ExecContext(ctx, query,
		workflow.ID, workflow.TenantID, workflow.Name, workflow.Description,
		workflow.Status, nodesJSON, connectionsJSON, variablesJSON, settingsJSON,
		tagsJSON, workflow.Version, workflow.CreatedAt, workflow.UpdatedAt,
		workflow.CreatedBy, workflow.UpdatedBy,
	)

	return err
}

// GetWorkflow retrieves a workflow by ID.
func (r *Repository) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	query := `SELECT id, tenant_id, name, description, status, nodes, connections,
		variables, settings, tags, version, created_at, updated_at, created_by, updated_by
		FROM workflows WHERE id = ?`

	var workflow Workflow
	var nodesJSON, connectionsJSON, variablesJSON, settingsJSON, tagsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&workflow.ID, &workflow.TenantID, &workflow.Name, &workflow.Description,
		&workflow.Status, &nodesJSON, &connectionsJSON, &variablesJSON, &settingsJSON,
		&tagsJSON, &workflow.Version, &workflow.CreatedAt, &workflow.UpdatedAt,
		&workflow.CreatedBy, &workflow.UpdatedBy,
	)

	if err == sql.ErrNoRows {
		return nil, ErrWorkflowNotFound
	}
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(nodesJSON, &workflow.Nodes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal nodes: %w", err)
	}

	if err := json.Unmarshal(connectionsJSON, &workflow.Connections); err != nil {
		return nil, fmt.Errorf("failed to unmarshal connections: %w", err)
	}

	if len(variablesJSON) > 0 {
		if err := json.Unmarshal(variablesJSON, &workflow.Variables); err != nil {
			return nil, fmt.Errorf("failed to unmarshal variables: %w", err)
		}
	}

	if len(settingsJSON) > 0 {
		if err := json.Unmarshal(settingsJSON, &workflow.Settings); err != nil {
			return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
		}
	}

	if len(tagsJSON) > 0 {
		if err := json.Unmarshal(tagsJSON, &workflow.Tags); err != nil {
			return nil, fmt.Errorf("failed to unmarshal tags: %w", err)
		}
	}

	return &workflow, nil
}

// UpdateWorkflow updates an existing workflow.
func (r *Repository) UpdateWorkflow(ctx context.Context, workflow *Workflow) error {
	workflow.UpdatedAt = time.Now()
	workflow.Version++

	nodesJSON, err := json.Marshal(workflow.Nodes)
	if err != nil {
		return fmt.Errorf("failed to marshal nodes: %w", err)
	}

	connectionsJSON, err := json.Marshal(workflow.Connections)
	if err != nil {
		return fmt.Errorf("failed to marshal connections: %w", err)
	}

	variablesJSON, err := json.Marshal(workflow.Variables)
	if err != nil {
		return fmt.Errorf("failed to marshal variables: %w", err)
	}

	settingsJSON, err := json.Marshal(workflow.Settings)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	tagsJSON, err := json.Marshal(workflow.Tags)
	if err != nil {
		return fmt.Errorf("failed to marshal tags: %w", err)
	}

	query := `UPDATE workflows SET
		name = ?, description = ?, status = ?, nodes = ?, connections = ?,
		variables = ?, settings = ?, tags = ?, version = ?, updated_at = ?, updated_by = ?
		WHERE id = ?`

	result, err := r.db.ExecContext(ctx, query,
		workflow.Name, workflow.Description, workflow.Status, nodesJSON, connectionsJSON,
		variablesJSON, settingsJSON, tagsJSON, workflow.Version, workflow.UpdatedAt,
		workflow.UpdatedBy, workflow.ID,
	)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrWorkflowNotFound
	}

	return nil
}

// DeleteWorkflow deletes a workflow.
func (r *Repository) DeleteWorkflow(ctx context.Context, id string) error {
	result, err := r.db.ExecContext(ctx, "DELETE FROM workflows WHERE id = ?", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rows == 0 {
		return ErrWorkflowNotFound
	}

	return nil
}

// ListWorkflows lists workflows with pagination and filtering.
func (r *Repository) ListWorkflows(ctx context.Context, tenantID string, opts *ListOptions) ([]*Workflow, int, error) {
	if opts == nil {
		opts = &ListOptions{Limit: 20}
	}

	// Build query
	var conditions []string
	var args []interface{}

	conditions = append(conditions, "tenant_id = ?")
	args = append(args, tenantID)

	if status, ok := opts.Filters["status"]; ok {
		conditions = append(conditions, "status = ?")
		args = append(args, status)
	}

	if name, ok := opts.Filters["name"]; ok {
		conditions = append(conditions, "name LIKE ?")
		args = append(args, "%"+name+"%")
	}

	whereClause := strings.Join(conditions, " AND ")

	// Count total
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM workflows WHERE %s", whereClause)
	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Build order clause
	orderClause := "created_at DESC"
	if opts.Sort != "" {
		order := "ASC"
		if opts.Order == "desc" {
			order = "DESC"
		}
		orderClause = fmt.Sprintf("%s %s", opts.Sort, order)
	}

	// Query workflows
	query := fmt.Sprintf(`SELECT id, tenant_id, name, description, status, nodes, connections,
		variables, settings, tags, version, created_at, updated_at, created_by, updated_by
		FROM workflows WHERE %s ORDER BY %s LIMIT ? OFFSET ?`,
		whereClause, orderClause)

	args = append(args, opts.Limit, opts.Offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var workflows []*Workflow
	for rows.Next() {
		var workflow Workflow
		var nodesJSON, connectionsJSON, variablesJSON, settingsJSON, tagsJSON []byte

		err := rows.Scan(
			&workflow.ID, &workflow.TenantID, &workflow.Name, &workflow.Description,
			&workflow.Status, &nodesJSON, &connectionsJSON, &variablesJSON, &settingsJSON,
			&tagsJSON, &workflow.Version, &workflow.CreatedAt, &workflow.UpdatedAt,
			&workflow.CreatedBy, &workflow.UpdatedBy,
		)
		if err != nil {
			return nil, 0, err
		}

		json.Unmarshal(nodesJSON, &workflow.Nodes)
		json.Unmarshal(connectionsJSON, &workflow.Connections)
		json.Unmarshal(variablesJSON, &workflow.Variables)
		json.Unmarshal(settingsJSON, &workflow.Settings)
		json.Unmarshal(tagsJSON, &workflow.Tags)

		workflows = append(workflows, &workflow)
	}

	return workflows, total, nil
}

// SaveExecution saves a workflow execution.
func (r *Repository) SaveExecution(ctx context.Context, execution *Execution) error {
	triggerDataJSON, _ := json.Marshal(execution.TriggerData)
	variablesJSON, _ := json.Marshal(execution.Variables)
	nodeResultsJSON, _ := json.Marshal(execution.NodeResults)

	query := `INSERT OR REPLACE INTO workflow_executions (
		id, workflow_id, workflow_name, tenant_id, status, trigger_type,
		trigger_data, variables, node_results, error, started_at, completed_at, duration
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		execution.ID, execution.WorkflowID, execution.WorkflowName, execution.TenantID,
		execution.Status, execution.TriggerType, triggerDataJSON, variablesJSON,
		nodeResultsJSON, execution.Error, execution.StartedAt, execution.CompletedAt,
		execution.Duration,
	)

	return err
}

// GetExecution retrieves an execution by ID.
func (r *Repository) GetExecution(ctx context.Context, id string) (*Execution, error) {
	query := `SELECT id, workflow_id, workflow_name, tenant_id, status, trigger_type,
		trigger_data, variables, node_results, error, started_at, completed_at, duration
		FROM workflow_executions WHERE id = ?`

	var execution Execution
	var triggerDataJSON, variablesJSON, nodeResultsJSON []byte

	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&execution.ID, &execution.WorkflowID, &execution.WorkflowName, &execution.TenantID,
		&execution.Status, &execution.TriggerType, &triggerDataJSON, &variablesJSON,
		&nodeResultsJSON, &execution.Error, &execution.StartedAt, &execution.CompletedAt,
		&execution.Duration,
	)

	if err == sql.ErrNoRows {
		return nil, ErrExecutionNotFound
	}
	if err != nil {
		return nil, err
	}

	json.Unmarshal(triggerDataJSON, &execution.TriggerData)
	json.Unmarshal(variablesJSON, &execution.Variables)
	json.Unmarshal(nodeResultsJSON, &execution.NodeResults)

	return &execution, nil
}

// ListExecutions lists executions for a workflow.
func (r *Repository) ListExecutions(ctx context.Context, workflowID string, opts *ListOptions) ([]*Execution, int, error) {
	if opts == nil {
		opts = &ListOptions{Limit: 20}
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM workflow_executions WHERE workflow_id = ?"
	if err := r.db.QueryRowContext(ctx, countQuery, workflowID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Query executions
	query := `SELECT id, workflow_id, workflow_name, tenant_id, status, trigger_type,
		trigger_data, variables, node_results, error, started_at, completed_at, duration
		FROM workflow_executions WHERE workflow_id = ?
		ORDER BY started_at DESC LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, workflowID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var executions []*Execution
	for rows.Next() {
		var execution Execution
		var triggerDataJSON, variablesJSON, nodeResultsJSON []byte

		err := rows.Scan(
			&execution.ID, &execution.WorkflowID, &execution.WorkflowName, &execution.TenantID,
			&execution.Status, &execution.TriggerType, &triggerDataJSON, &variablesJSON,
			&nodeResultsJSON, &execution.Error, &execution.StartedAt, &execution.CompletedAt,
			&execution.Duration,
		)
		if err != nil {
			return nil, 0, err
		}

		json.Unmarshal(triggerDataJSON, &execution.TriggerData)
		json.Unmarshal(variablesJSON, &execution.Variables)
		json.Unmarshal(nodeResultsJSON, &execution.NodeResults)

		executions = append(executions, &execution)
	}

	return executions, total, nil
}

// SaveExecutionLog saves an execution log entry.
func (r *Repository) SaveExecutionLog(ctx context.Context, log *ExecutionLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}

	query := `INSERT INTO workflow_execution_logs (
		id, execution_id, node_id, level, message, data, timestamp
	) VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		log.ID, log.ExecutionID, log.NodeID, log.Level, log.Message, log.Data, log.Timestamp,
	)

	return err
}

// GetExecutionLogs retrieves logs for an execution.
func (r *Repository) GetExecutionLogs(ctx context.Context, executionID string, opts *ListOptions) ([]*ExecutionLog, int, error) {
	if opts == nil {
		opts = &ListOptions{Limit: 100}
	}

	// Count total
	var total int
	countQuery := "SELECT COUNT(*) FROM workflow_execution_logs WHERE execution_id = ?"
	if err := r.db.QueryRowContext(ctx, countQuery, executionID).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Query logs
	query := `SELECT id, execution_id, node_id, level, message, data, timestamp
		FROM workflow_execution_logs WHERE execution_id = ?
		ORDER BY timestamp ASC LIMIT ? OFFSET ?`

	rows, err := r.db.QueryContext(ctx, query, executionID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*ExecutionLog
	for rows.Next() {
		var log ExecutionLog
		err := rows.Scan(
			&log.ID, &log.ExecutionID, &log.NodeID, &log.Level, &log.Message, &log.Data, &log.Timestamp,
		)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, &log)
	}

	return logs, total, nil
}

// SaveWebhook saves a webhook configuration.
func (r *Repository) SaveWebhook(ctx context.Context, workflowID, path, method, authType string, authConfig map[string]string) error {
	authConfigJSON, _ := json.Marshal(authConfig)

	query := `INSERT OR REPLACE INTO workflow_webhooks (
		id, workflow_id, path, method, auth_type, auth_config, created_at
	) VALUES (?, ?, ?, ?, ?, ?, ?)`

	_, err := r.db.ExecContext(ctx, query,
		uuid.New().String(), workflowID, path, method, authType, authConfigJSON, time.Now(),
	)

	return err
}

// GetWebhookByPath retrieves a webhook by path.
func (r *Repository) GetWebhookByPath(ctx context.Context, path string) (string, error) {
	var workflowID string
	query := "SELECT workflow_id FROM workflow_webhooks WHERE path = ?"

	err := r.db.QueryRowContext(ctx, query, path).Scan(&workflowID)
	if err == sql.ErrNoRows {
		return "", ErrWorkflowNotFound
	}

	return workflowID, err
}

// DeleteWebhook deletes a webhook.
func (r *Repository) DeleteWebhook(ctx context.Context, workflowID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM workflow_webhooks WHERE workflow_id = ?", workflowID)
	return err
}

// GetStats returns workflow statistics for a tenant.
func (r *Repository) GetStats(ctx context.Context, tenantID string) (*Stats, error) {
	stats := &Stats{}

	// Total workflows
	r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM workflows WHERE tenant_id = ?", tenantID,
	).Scan(&stats.TotalWorkflows)

	// Active workflows
	r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM workflows WHERE tenant_id = ? AND status = 'active'", tenantID,
	).Scan(&stats.ActiveWorkflows)

	// Total executions
	r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM workflow_executions WHERE tenant_id = ?", tenantID,
	).Scan(&stats.TotalExecutions)

	// Running executions
	r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM workflow_executions WHERE tenant_id = ? AND status = 'running'", tenantID,
	).Scan(&stats.RunningExecutions)

	// Successful executions
	r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM workflow_executions WHERE tenant_id = ? AND status = 'completed'", tenantID,
	).Scan(&stats.SuccessfulExecutions)

	// Failed executions
	r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM workflow_executions WHERE tenant_id = ? AND status = 'failed'", tenantID,
	).Scan(&stats.FailedExecutions)

	return stats, nil
}

// CleanupOldExecutions removes old execution data.
func (r *Repository) CleanupOldExecutions(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Delete old logs first
	r.db.ExecContext(ctx,
		`DELETE FROM workflow_execution_logs WHERE execution_id IN (
			SELECT id FROM workflow_executions WHERE completed_at < ?
		)`, cutoff,
	)

	// Delete old executions
	result, err := r.db.ExecContext(ctx,
		"DELETE FROM workflow_executions WHERE completed_at < ?", cutoff,
	)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}
