package workflow

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	z "github.com/IceWhaleTech/zorm"
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

func (r *Repository) migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY, tenant_id TEXT NOT NULL, name TEXT NOT NULL,
			description TEXT, status TEXT NOT NULL DEFAULT 'draft',
			nodes TEXT NOT NULL, connections TEXT NOT NULL, variables TEXT,
			settings TEXT, tags TEXT, version INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL, updated_at DATETIME NOT NULL,
			created_by TEXT, updated_by TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_workflows_tenant_id ON workflows(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status)`,
		`CREATE TABLE IF NOT EXISTS workflow_executions (
			id TEXT PRIMARY KEY, workflow_id TEXT NOT NULL, workflow_name TEXT NOT NULL,
			tenant_id TEXT NOT NULL, status TEXT NOT NULL, trigger_type TEXT NOT NULL,
			trigger_data TEXT, variables TEXT, node_results TEXT, error TEXT,
			started_at DATETIME NOT NULL, completed_at DATETIME, duration INTEGER,
			FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_workflow_id ON workflow_executions(workflow_id)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_tenant_id ON workflow_executions(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_status ON workflow_executions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_executions_started_at ON workflow_executions(started_at)`,
		`CREATE TABLE IF NOT EXISTS workflow_execution_logs (
			id TEXT PRIMARY KEY, execution_id TEXT NOT NULL, node_id TEXT,
			level TEXT NOT NULL, message TEXT NOT NULL, data TEXT, timestamp DATETIME NOT NULL,
			FOREIGN KEY (execution_id) REFERENCES workflow_executions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_execution_logs_execution_id ON workflow_execution_logs(execution_id)`,
		`CREATE TABLE IF NOT EXISTS workflow_webhooks (
			id TEXT PRIMARY KEY, workflow_id TEXT NOT NULL UNIQUE, path TEXT NOT NULL UNIQUE,
			method TEXT NOT NULL DEFAULT 'POST', auth_type TEXT, auth_config TEXT,
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

func (r *Repository) wfTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "workflows")
}
func (r *Repository) execTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "workflow_executions")
}
func (r *Repository) logTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "workflow_execution_logs")
}
func (r *Repository) webhookTable(ctx context.Context) *z.ZormTable {
	return z.TableContext(ctx, r.db, "workflow_webhooks")
}

func marshalJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// workflowRow for zorm scanning
type workflowRow struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Nodes       string `json:"nodes"`
	Connections string `json:"connections"`
	Variables   string `json:"variables"`
	Settings    string `json:"settings"`
	Tags        string `json:"tags"`
	Version     int    `json:"version"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	CreatedBy   string `json:"created_by"`
	UpdatedBy   string `json:"updated_by"`
}

func parseWfTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	if t.IsZero() {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}

func rowToWorkflow(r workflowRow) *Workflow {
	wf := &Workflow{
		ID: r.ID, TenantID: r.TenantID, Name: r.Name, Description: r.Description,
		Status: WorkflowStatus(r.Status), Version: r.Version,
		CreatedAt: parseWfTime(r.CreatedAt), UpdatedAt: parseWfTime(r.UpdatedAt),
		CreatedBy: r.CreatedBy, UpdatedBy: r.UpdatedBy,
	}
	json.Unmarshal([]byte(r.Nodes), &wf.Nodes)
	json.Unmarshal([]byte(r.Connections), &wf.Connections)
	json.Unmarshal([]byte(r.Variables), &wf.Variables)
	json.Unmarshal([]byte(r.Settings), &wf.Settings)
	json.Unmarshal([]byte(r.Tags), &wf.Tags)
	return wf
}

func (r *Repository) CreateWorkflow(ctx context.Context, workflow *Workflow) error {
	if workflow.ID == "" {
		workflow.ID = uuid.New().String()
	}
	now := time.Now()
	workflow.CreatedAt = now
	workflow.UpdatedAt = now
	workflow.Version = 1

	_, err := r.wfTable(ctx).Insert(map[string]interface{}{
		"id": workflow.ID, "tenant_id": workflow.TenantID, "name": workflow.Name,
		"description": workflow.Description, "status": workflow.Status,
		"nodes": marshalJSON(workflow.Nodes), "connections": marshalJSON(workflow.Connections),
		"variables": marshalJSON(workflow.Variables), "settings": marshalJSON(workflow.Settings),
		"tags": marshalJSON(workflow.Tags), "version": workflow.Version,
		"created_at": workflow.CreatedAt, "updated_at": workflow.UpdatedAt,
		"created_by": workflow.CreatedBy, "updated_by": workflow.UpdatedBy,
	})
	return err
}

func (r *Repository) GetWorkflow(ctx context.Context, id string) (*Workflow, error) {
	var rows []workflowRow
	_, err := r.wfTable(ctx).Select(&rows, z.Where(z.Eq("id", id)), z.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrWorkflowNotFound
	}
	return rowToWorkflow(rows[0]), nil
}

func (r *Repository) UpdateWorkflow(ctx context.Context, workflow *Workflow) error {
	workflow.UpdatedAt = time.Now()
	workflow.Version++
	n, err := r.wfTable(ctx).Update(map[string]interface{}{
		"name": workflow.Name, "description": workflow.Description, "status": workflow.Status,
		"nodes": marshalJSON(workflow.Nodes), "connections": marshalJSON(workflow.Connections),
		"variables": marshalJSON(workflow.Variables), "settings": marshalJSON(workflow.Settings),
		"tags": marshalJSON(workflow.Tags), "version": workflow.Version,
		"updated_at": workflow.UpdatedAt, "updated_by": workflow.UpdatedBy,
	}, z.Where(z.Eq("id", workflow.ID)))
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrWorkflowNotFound
	}
	return nil
}

func (r *Repository) DeleteWorkflow(ctx context.Context, id string) error {
	n, err := r.wfTable(ctx).Delete(z.Where(z.Eq("id", id)))
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrWorkflowNotFound
	}
	return nil
}

func (r *Repository) ListWorkflows(ctx context.Context, tenantID string, opts *ListOptions) ([]*Workflow, int, error) {
	if opts == nil {
		opts = &ListOptions{Limit: 20}
	}

	conds := []interface{}{z.Eq("tenant_id", tenantID)}
	if status, ok := opts.Filters["status"]; ok {
		conds = append(conds, z.Eq("status", status))
	}
	if name, ok := opts.Filters["name"]; ok {
		conds = append(conds, z.Like("name", "%"+name+"%"))
	}

	var total int64
	_, err := r.wfTable(ctx).Select(&total, z.Fields("count(1)"), z.Where(conds...))
	if err != nil {
		return nil, 0, err
	}

	orderClause := "created_at DESC"
	if opts.Sort != "" {
		order := "ASC"
		if opts.Order == "desc" {
			order = "DESC"
		}
		orderClause = fmt.Sprintf("%s %s", opts.Sort, order)
	}

	var rows []workflowRow
	_, err = r.wfTable(ctx).Select(&rows, z.Where(conds...), z.OrderBy(orderClause), z.Limit(opts.Limit, opts.Offset))
	if err != nil {
		return nil, 0, err
	}

	workflows := make([]*Workflow, len(rows))
	for i, row := range rows {
		workflows[i] = rowToWorkflow(row)
	}
	return workflows, int(total), nil
}

// execRow for zorm scanning
type execRow struct {
	ID           string  `json:"id"`
	WorkflowID   string  `json:"workflow_id"`
	WorkflowName string  `json:"workflow_name"`
	TenantID     string  `json:"tenant_id"`
	Status       string  `json:"status"`
	TriggerType  string  `json:"trigger_type"`
	TriggerData  string  `json:"trigger_data"`
	Variables    string  `json:"variables"`
	NodeResults  string  `json:"node_results"`
	Error        string  `json:"error"`
	StartedAt    string  `json:"started_at"`
	CompletedAt  *string `json:"completed_at"`
	Duration     *int64  `json:"duration"`
}

func rowToExecution(r execRow) *Execution {
	e := &Execution{
		ID: r.ID, WorkflowID: r.WorkflowID, WorkflowName: r.WorkflowName,
		TenantID: r.TenantID, Status: ExecutionStatus(r.Status),
		TriggerType: TriggerType(r.TriggerType), Error: r.Error,
		StartedAt: parseWfTime(r.StartedAt),
	}
	if r.CompletedAt != nil {
		t := parseWfTime(*r.CompletedAt)
		if !t.IsZero() {
			e.CompletedAt = &t
		}
	}
	if r.Duration != nil {
		e.Duration = *r.Duration
	}
	json.Unmarshal([]byte(r.TriggerData), &e.TriggerData)
	json.Unmarshal([]byte(r.Variables), &e.Variables)
	json.Unmarshal([]byte(r.NodeResults), &e.NodeResults)
	return e
}

func (r *Repository) SaveExecution(ctx context.Context, execution *Execution) error {
	_, err := r.execTable(ctx).ReplaceInto(map[string]interface{}{
		"id": execution.ID, "workflow_id": execution.WorkflowID,
		"workflow_name": execution.WorkflowName, "tenant_id": execution.TenantID,
		"status": execution.Status, "trigger_type": execution.TriggerType,
		"trigger_data": marshalJSON(execution.TriggerData), "variables": marshalJSON(execution.Variables),
		"node_results": marshalJSON(execution.NodeResults), "error": execution.Error,
		"started_at": execution.StartedAt, "completed_at": execution.CompletedAt,
		"duration": execution.Duration,
	})
	return err
}

func (r *Repository) GetExecution(ctx context.Context, id string) (*Execution, error) {
	var rows []execRow
	_, err := r.execTable(ctx).Select(&rows, z.Where(z.Eq("id", id)), z.Limit(1))
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, ErrExecutionNotFound
	}
	return rowToExecution(rows[0]), nil
}

func (r *Repository) ListExecutions(ctx context.Context, workflowID string, opts *ListOptions) ([]*Execution, int, error) {
	if opts == nil {
		opts = &ListOptions{Limit: 20}
	}
	var total int64
	_, err := r.execTable(ctx).Select(&total, z.Fields("count(1)"), z.Where(z.Eq("workflow_id", workflowID)))
	if err != nil {
		return nil, 0, err
	}

	var rows []execRow
	_, err = r.execTable(ctx).Select(&rows,
		z.Where(z.Eq("workflow_id", workflowID)),
		z.OrderBy("started_at DESC"), z.Limit(opts.Limit, opts.Offset),
	)
	if err != nil {
		return nil, 0, err
	}
	executions := make([]*Execution, len(rows))
	for i, row := range rows {
		executions[i] = rowToExecution(row)
	}
	return executions, int(total), nil
}

// logRow for zorm scanning
type logRow struct {
	ID          string `json:"id"`
	ExecutionID string `json:"execution_id"`
	NodeID      string `json:"node_id"`
	Level       string `json:"level"`
	Message     string `json:"message"`
	Data        string `json:"data"`
	Timestamp   string `json:"timestamp"`
}

func (r *Repository) SaveExecutionLog(ctx context.Context, log *ExecutionLog) error {
	if log.ID == "" {
		log.ID = uuid.New().String()
	}
	_, err := r.logTable(ctx).Insert(map[string]interface{}{
		"id": log.ID, "execution_id": log.ExecutionID, "node_id": log.NodeID,
		"level": log.Level, "message": log.Message, "data": log.Data, "timestamp": log.Timestamp,
	})
	return err
}

func (r *Repository) GetExecutionLogs(ctx context.Context, executionID string, opts *ListOptions) ([]*ExecutionLog, int, error) {
	if opts == nil {
		opts = &ListOptions{Limit: 100}
	}
	var total int64
	_, err := r.logTable(ctx).Select(&total, z.Fields("count(1)"), z.Where(z.Eq("execution_id", executionID)))
	if err != nil {
		return nil, 0, err
	}

	var rows []logRow
	_, err = r.logTable(ctx).Select(&rows,
		z.Where(z.Eq("execution_id", executionID)),
		z.OrderBy("timestamp ASC"), z.Limit(opts.Limit, opts.Offset),
	)
	if err != nil {
		return nil, 0, err
	}
	logs := make([]*ExecutionLog, len(rows))
	for i, row := range rows {
		logs[i] = &ExecutionLog{
			ID: row.ID, ExecutionID: row.ExecutionID, NodeID: row.NodeID,
			Level: row.Level, Message: row.Message, Data: row.Data,
			Timestamp: parseWfTime(row.Timestamp),
		}
	}
	return logs, int(total), nil
}

func (r *Repository) SaveWebhook(ctx context.Context, workflowID, path, method, authType string, authConfig map[string]string) error {
	_, err := r.webhookTable(ctx).ReplaceInto(map[string]interface{}{
		"id": uuid.New().String(), "workflow_id": workflowID, "path": path,
		"method": method, "auth_type": authType, "auth_config": marshalJSON(authConfig),
		"created_at": time.Now(),
	})
	return err
}

func (r *Repository) GetWebhookByPath(ctx context.Context, path string) (string, error) {
	var ids []string
	_, err := r.webhookTable(ctx).Select(&ids, z.Fields("workflow_id"), z.Where(z.Eq("path", path)), z.Limit(1))
	if err != nil {
		return "", err
	}
	if len(ids) == 0 {
		return "", ErrWorkflowNotFound
	}
	return ids[0], nil
}

func (r *Repository) DeleteWebhook(ctx context.Context, workflowID string) error {
	_, err := r.webhookTable(ctx).Delete(z.Where(z.Eq("workflow_id", workflowID)))
	return err
}

func (r *Repository) GetStats(ctx context.Context, tenantID string) (*Stats, error) {
	stats := &Stats{}
	var v int64

	r.wfTable(ctx).Select(&v, z.Fields("count(1)"), z.Where(z.Eq("tenant_id", tenantID)))
	stats.TotalWorkflows = int(v)

	v = 0
	r.wfTable(ctx).Select(&v, z.Fields("count(1)"), z.Where(z.Eq("tenant_id", tenantID), z.Eq("status", "active")))
	stats.ActiveWorkflows = int(v)

	v = 0
	r.execTable(ctx).Select(&v, z.Fields("count(1)"), z.Where(z.Eq("tenant_id", tenantID)))
	stats.TotalExecutions = int(v)

	v = 0
	r.execTable(ctx).Select(&v, z.Fields("count(1)"), z.Where(z.Eq("tenant_id", tenantID), z.Eq("status", "running")))
	stats.RunningExecutions = int(v)

	v = 0
	r.execTable(ctx).Select(&v, z.Fields("count(1)"), z.Where(z.Eq("tenant_id", tenantID), z.Eq("status", "completed")))
	stats.SuccessfulExecutions = int(v)

	v = 0
	r.execTable(ctx).Select(&v, z.Fields("count(1)"), z.Where(z.Eq("tenant_id", tenantID), z.Eq("status", "failed")))
	stats.FailedExecutions = int(v)

	return stats, nil
}

func (r *Repository) CleanupOldExecutions(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Delete old logs first (subquery not supported in zorm Delete, use raw)
	r.db.ExecContext(ctx,
		`DELETE FROM workflow_execution_logs WHERE execution_id IN (
			SELECT id FROM workflow_executions WHERE completed_at < ?
		)`, cutoff)

	n, err := r.execTable(ctx).Delete(z.Where(z.Lt("completed_at", cutoff)))
	return int64(n), err
}
