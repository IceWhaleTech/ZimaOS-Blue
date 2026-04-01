package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
	"github.com/google/uuid"
)

// Engine executes workflows.
type Engine struct {
	config            *Config
	executions        map[string]*executionState
	mu                sync.RWMutex
	handlers          map[ActionType]ActionHandler
	triggers          map[string]*triggerState
	triggerMu         sync.RWMutex
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup
	toolGateway       ToolRuntime
	checkpointEnabled func() bool
	executionObserver func(*Execution)
}

// executionState tracks the state of a running execution.
type executionState struct {
	execution *Execution
	workflow  *Workflow
	ctx       context.Context
	cancel    context.CancelFunc
	nodeQueue chan string
	resumeCh  chan ExecutionResumeInput
	completed map[string]bool
	mu        sync.Mutex
}

// triggerState tracks registered triggers.
type triggerState struct {
	workflowID string
	config     *TriggerConfig
	stopCh     chan struct{}
}

// ActionHandler handles execution of a specific action type.
type ActionHandler func(ctx context.Context, config map[string]interface{}, input map[string]interface{}) (map[string]interface{}, error)

// NewEngine creates a new workflow engine.
func NewEngine(config *Config) *Engine {
	if config == nil {
		config = DefaultConfig()
	}
	if config.MaxConcurrentExecutions < 0 {
		config.MaxConcurrentExecutions = resolveDefaultMaxConcurrentExecutions()
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		config:     config,
		executions: make(map[string]*executionState),
		handlers:   make(map[ActionType]ActionHandler),
		triggers:   make(map[string]*triggerState),
		ctx:        ctx,
		cancel:     cancel,
	}

	// Register default handlers
	e.registerDefaultHandlers()

	return e
}

// registerDefaultHandlers registers built-in action handlers.
func (e *Engine) registerDefaultHandlers() {
	e.handlers[ActionTypeHTTP] = e.handleHTTPAction
	e.handlers[ActionTypeSetVariable] = e.handleSetVariableAction
	e.handlers[ActionTypeJavaScript] = e.handleJavaScriptAction
	e.handlers[ActionTypeBrowser] = e.handleBrowserAction
	e.handlers[ActionTypeFileOps] = e.handleFileAction
	e.handlers[ActionTypeSkill] = e.handleSkillAction
}

// RegisterHandler registers a custom action handler.
func (e *Engine) RegisterHandler(actionType ActionType, handler ActionHandler) {
	e.handlers[actionType] = handler
}

// SetToolGateway wires the shared tool runtime into workflow action handlers.
func (e *Engine) SetToolGateway(gateway ToolRuntime) {
	e.toolGateway = gateway
}

// SetExecutionObserver wires a callback for execution state transitions.
func (e *Engine) SetExecutionObserver(observer func(*Execution)) {
	e.executionObserver = observer
}

// SetCheckpointEnabledFunc wires a runtime gate for checkpoint/pause semantics.
func (e *Engine) SetCheckpointEnabledFunc(enabled func() bool) {
	e.checkpointEnabled = enabled
}

// Execute starts a workflow execution.
func (e *Engine) Execute(ctx context.Context, workflow *Workflow, triggerType TriggerType, triggerData map[string]interface{}) (*Execution, error) {
	if workflow.Status != WorkflowStatusActive {
		return nil, ErrWorkflowDisabled
	}

	if err := e.waitForAvailableExecutionSlot(ctx, workflow); err != nil {
		return nil, err
	}

	// Create execution
	execution := &Execution{
		ID:           uuid.New().String(),
		WorkflowID:   workflow.ID,
		WorkflowName: workflow.Name,
		TenantID:     workflow.TenantID,
		Status:       ExecutionStatusPending,
		TriggerType:  triggerType,
		TriggerData:  triggerData,
		Variables:    make(map[string]interface{}),
		NodeResults:  make(map[string]*NodeResult),
		StartedAt:    timeutil.NowTime(),
	}

	// Copy workflow variables
	for k, v := range workflow.Variables {
		execution.Variables[k] = v
	}

	// Add trigger data to variables
	if triggerData != nil {
		execution.Variables["trigger"] = triggerData
	}

	// Create execution context with timeout
	timeout := time.Duration(e.config.DefaultTimeout) * time.Second
	if workflow.Settings != nil && workflow.Settings.Timeout > 0 {
		timeout = time.Duration(workflow.Settings.Timeout) * time.Second
	}

	execCtx, execCancel := context.WithTimeout(ctx, timeout)

	state := &executionState{
		execution: execution,
		workflow:  workflow,
		ctx:       execCtx,
		cancel:    execCancel,
		nodeQueue: make(chan string, len(workflow.Nodes)),
		resumeCh:  make(chan ExecutionResumeInput, 1),
		completed: make(map[string]bool),
	}

	e.mu.Lock()
	e.executions[execution.ID] = state
	e.mu.Unlock()

	// Start execution in background
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer execCancel()
		e.runExecution(state)
	}()

	return execution, nil
}

func (e *Engine) executionLimitForWorkflow(workflow *Workflow) int {
	if e == nil || e.config == nil {
		return 0
	}
	maxConcurrent := e.config.MaxConcurrentExecutions
	if workflow != nil && workflow.Settings != nil && workflow.Settings.MaxConcurrent > 0 {
		maxConcurrent = workflow.Settings.MaxConcurrent
	}
	return maxConcurrent
}

func (e *Engine) runningExecutionCount(workflowID string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	runningCount := 0
	for _, state := range e.executions {
		if state.execution.WorkflowID == workflowID && executionConsumesConcurrencySlot(state.execution.Status) {
			runningCount++
		}
	}
	return runningCount
}

func (e *Engine) waitForAvailableExecutionSlot(ctx context.Context, workflow *Workflow) error {
	if e == nil || workflow == nil {
		return fmt.Errorf("workflow engine is not configured")
	}
	maxConcurrent := e.executionLimitForWorkflow(workflow)
	if maxConcurrent <= 0 {
		return nil
	}

	for {
		if e.runningExecutionCount(workflow.ID) < maxConcurrent {
			return nil
		}
		if ctx == nil {
			time.Sleep(workflowConcurrentSlotPollInterval)
			continue
		}

		timer := time.NewTimer(workflowConcurrentSlotPollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("wait for workflow execution slot: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func executionConsumesConcurrencySlot(status ExecutionStatus) bool {
	switch status {
	case ExecutionStatusCompleted, ExecutionStatusFailed, ExecutionStatusCancelled:
		return false
	default:
		return true
	}
}

// runExecution runs the workflow execution.
func (e *Engine) runExecution(state *executionState) {
	state.execution.Status = ExecutionStatusRunning
	state.execution.StatusReason = ""
	e.notifyExecutionUpdate(state.execution)

	// Find trigger nodes (entry points)
	triggerNodes := e.findTriggerNodes(state.workflow)
	if len(triggerNodes) == 0 {
		state.execution.Status = ExecutionStatusFailed
		state.execution.Error = "no trigger nodes found"
		e.completeExecution(state)
		return
	}

	// Queue trigger nodes
	for _, node := range triggerNodes {
		state.nodeQueue <- node.ID
	}

	// Process nodes
	for {
		select {
		case <-state.ctx.Done():
			if state.ctx.Err() == context.DeadlineExceeded {
				state.execution.Status = ExecutionStatusFailed
				state.execution.Error = "execution timed out"
			} else {
				state.execution.Status = ExecutionStatusCancelled
			}
			e.completeExecution(state)
			return

		case nodeID, ok := <-state.nodeQueue:
			if !ok {
				// Queue closed, execution complete
				e.completeExecution(state)
				return
			}

			// Check if already completed
			state.mu.Lock()
			if state.completed[nodeID] {
				state.mu.Unlock()
				continue
			}
			state.mu.Unlock()

			// Execute node
			node := e.findNode(state.workflow, nodeID)
			if node == nil {
				continue
			}

			if node.Disabled {
				state.mu.Lock()
				state.completed[nodeID] = true
				state.mu.Unlock()
				e.queueNextNodes(state, nodeID, nil)
				continue
			}

			result := e.executeNode(state, node)
			state.execution.NodeResults[nodeID] = result

			state.mu.Lock()
			state.completed[nodeID] = true
			state.mu.Unlock()

			if result.Status != NodeStatusFailed {
				if checkpoint := e.buildExecutionCheckpoint(node, result); checkpoint != nil {
					if err := e.pauseForCheckpoint(state, checkpoint, result); err != nil {
						if state.ctx.Err() == context.DeadlineExceeded {
							state.execution.Status = ExecutionStatusFailed
							state.execution.StatusReason = "checkpoint_timeout"
							state.execution.Error = "execution timed out while waiting for checkpoint resume"
						} else if state.ctx.Err() != nil {
							state.execution.Status = ExecutionStatusCancelled
							state.execution.StatusReason = "checkpoint_cancelled"
						} else {
							state.execution.Status = ExecutionStatusFailed
							state.execution.StatusReason = "checkpoint_resume_failed"
							state.execution.Error = err.Error()
						}
						e.completeExecution(state)
						return
					}
				}
			}

			// Handle node result
			if result.Status == NodeStatusFailed {
				if state.ctx.Err() == context.Canceled {
					state.execution.Status = ExecutionStatusCancelled
					state.execution.StatusReason = "cancelled"
					state.execution.Error = ""
					e.completeExecution(state)
					return
				}
				if state.ctx.Err() == context.DeadlineExceeded {
					state.execution.Status = ExecutionStatusFailed
					state.execution.StatusReason = "timeout"
					state.execution.Error = "execution timed out"
					e.completeExecution(state)
					return
				}
				continueOnError := false
				if state.workflow.Settings != nil {
					continueOnError = state.workflow.Settings.ContinueOnError
				}

				if !continueOnError {
					state.execution.Status = ExecutionStatusFailed
					state.execution.StatusReason = "node_failed"
					state.execution.Error = fmt.Sprintf("node %s failed: %s", node.Name, result.Error)
					e.completeExecution(state)
					return
				}
			}

			// Queue next nodes
			e.queueNextNodes(state, nodeID, result)

			// Check if all nodes completed
			if e.isExecutionComplete(state) {
				state.execution.Status = ExecutionStatusCompleted
				state.execution.StatusReason = ""
				e.completeExecution(state)
				return
			}
		}
	}
}

// executeNode executes a single node.
func (e *Engine) executeNode(state *executionState, node *Node) *NodeResult {
	result := &NodeResult{
		NodeID:    node.ID,
		NodeName:  node.Name,
		Status:    NodeStatusRunning,
		StartedAt: timeutil.NowTime(),
	}

	// Get input from previous nodes
	input := e.getNodeInput(state, node)
	result.Input = input

	var output map[string]interface{}
	var err error

	switch node.Type {
	case NodeTypeTrigger:
		// Trigger nodes pass through trigger data
		output = map[string]interface{}{
			"trigger": state.execution.TriggerData,
		}

	case NodeTypeAction:
		output, err = e.executeActionNode(state.ctx, node, input, state.execution.Variables)

	case NodeTypeCondition:
		output, err = e.executeConditionNode(state.ctx, node, input, state.execution.Variables)

	case NodeTypeLoop:
		output, err = e.executeLoopNode(state, node, input)

	case NodeTypeSwitch:
		output, err = e.executeSwitchNode(state.ctx, node, input, state.execution.Variables)

	case NodeTypeDelay:
		output, err = e.executeDelayNode(state.ctx, node)

	case NodeTypeMerge:
		output = input // Pass through

	default:
		err = fmt.Errorf("unknown node type: %s", node.Type)
	}

	now := timeutil.NowTime()
	result.CompletedAt = &now
	result.Duration = now.Sub(result.StartedAt).Milliseconds()

	if err != nil {
		result.Status = NodeStatusFailed
		result.Error = err.Error()
	} else {
		result.Status = NodeStatusCompleted
		result.Output = output
	}

	return result
}

// executeActionNode executes an action node.
func (e *Engine) executeActionNode(ctx context.Context, node *Node, input map[string]interface{}, variables map[string]interface{}) (map[string]interface{}, error) {
	actionTypeStr, ok := node.Config["type"].(string)
	if !ok {
		return nil, fmt.Errorf("action type not specified")
	}

	actionType := ActionType(actionTypeStr)
	handler, exists := e.handlers[actionType]
	if !exists {
		return nil, fmt.Errorf("no handler for action type: %s", actionType)
	}

	// Interpolate variables in config
	config := e.interpolateConfig(node.Config, variables, input)

	return handler(ctx, config, input)
}

// executeConditionNode executes a condition node.
func (e *Engine) executeConditionNode(ctx context.Context, node *Node, input map[string]interface{}, variables map[string]interface{}) (map[string]interface{}, error) {
	expression, ok := node.Config["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("condition expression not specified")
	}

	// Interpolate and evaluate expression
	expression = e.interpolateString(expression, variables, input)
	result := e.evaluateCondition(expression, variables, input)

	return map[string]interface{}{
		"result": result,
		"port":   e.getConditionPort(node, result),
	}, nil
}

// executeLoopNode executes a loop node.
func (e *Engine) executeLoopNode(state *executionState, node *Node, input map[string]interface{}) (map[string]interface{}, error) {
	loopType, _ := node.Config["type"].(string)

	switch loopType {
	case "for_each":
		itemsExpr, _ := node.Config["items"].(string)
		items := e.evaluateExpression(itemsExpr, state.execution.Variables, input)

		itemsSlice, ok := items.([]interface{})
		if !ok {
			return nil, fmt.Errorf("items must be an array")
		}

		results := make([]interface{}, 0, len(itemsSlice))
		for i, item := range itemsSlice {
			results = append(results, map[string]interface{}{
				"index": i,
				"item":  item,
			})
		}

		return map[string]interface{}{
			"items": results,
			"count": len(results),
		}, nil

	case "count":
		count, _ := node.Config["count"].(float64)
		results := make([]interface{}, int(count))
		for i := 0; i < int(count); i++ {
			results[i] = map[string]interface{}{
				"index": i,
			}
		}

		return map[string]interface{}{
			"items": results,
			"count": int(count),
		}, nil

	default:
		return nil, fmt.Errorf("unknown loop type: %s", loopType)
	}
}

// executeSwitchNode executes a switch node.
func (e *Engine) executeSwitchNode(ctx context.Context, node *Node, input map[string]interface{}, variables map[string]interface{}) (map[string]interface{}, error) {
	expression, ok := node.Config["expression"].(string)
	if !ok {
		return nil, fmt.Errorf("switch expression not specified")
	}

	value := e.evaluateExpression(expression, variables, input)
	valueStr := fmt.Sprintf("%v", value)

	cases, _ := node.Config["cases"].(map[string]interface{})
	defaultPort, _ := node.Config["default"].(string)

	port := defaultPort
	for caseValue, casePort := range cases {
		if caseValue == valueStr {
			port = casePort.(string)
			break
		}
	}

	return map[string]interface{}{
		"value": value,
		"port":  port,
	}, nil
}

// executeDelayNode executes a delay node.
func (e *Engine) executeDelayNode(ctx context.Context, node *Node) (map[string]interface{}, error) {
	duration, _ := node.Config["duration"].(float64)
	if duration <= 0 {
		duration = 1
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(time.Duration(duration) * time.Second):
		return map[string]interface{}{
			"delayed": duration,
		}, nil
	}
}

// handleHTTPAction handles HTTP action execution.
func (e *Engine) handleHTTPAction(ctx context.Context, config map[string]interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	// This is a placeholder - actual HTTP implementation would use net/http
	url, _ := config["url"].(string)
	method, _ := config["method"].(string)
	if method == "" {
		method = "GET"
	}

	return map[string]interface{}{
		"url":        url,
		"method":     method,
		"status":     200,
		"statusText": "OK",
		"body":       map[string]interface{}{},
	}, nil
}

// handleSetVariableAction handles set variable action.
func (e *Engine) handleSetVariableAction(ctx context.Context, config map[string]interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	name, _ := config["name"].(string)
	value, _ := config["value"]

	return map[string]interface{}{
		"name":  name,
		"value": value,
	}, nil
}

// handleJavaScriptAction handles JavaScript action execution.
func (e *Engine) handleJavaScriptAction(ctx context.Context, config map[string]interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	// This is a placeholder - actual JS execution would use a JS engine
	code, _ := config["code"].(string)

	return map[string]interface{}{
		"code":     code,
		"executed": true,
	}, nil
}

func (e *Engine) handleBrowserAction(ctx context.Context, config map[string]interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	args := workflowToolArgs(config, "type")
	return e.executeToolAction(ctx, "browser", args)
}

func (e *Engine) handleFileAction(ctx context.Context, config map[string]interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	operation, _ := config["operation"].(string)
	switch strings.ToLower(strings.TrimSpace(operation)) {
	case "read":
		return e.executeToolAction(ctx, "read", map[string]interface{}{
			"path": nonEmptyString(config["source_path"], config["path"]),
		})
	case "write":
		return e.executeToolAction(ctx, "write", map[string]interface{}{
			"path":        nonEmptyString(config["dest_path"], config["source_path"], config["path"]),
			"content":     config["content"],
			"create_dirs": config["create_dirs"],
		})
	case "list", "ls":
		return e.executeToolAction(ctx, "ls", map[string]interface{}{
			"path": nonEmptyString(config["source_path"], config["path"]),
		})
	case "find":
		return e.executeToolAction(ctx, "find", map[string]interface{}{
			"path": nonEmptyString(config["source_path"], config["path"]),
			"glob": config["pattern"],
		})
	case "grep":
		return e.executeToolAction(ctx, "grep", map[string]interface{}{
			"path":  nonEmptyString(config["source_path"], config["path"]),
			"regex": config["pattern"],
		})
	default:
		return nil, fmt.Errorf("unsupported file operation: %s", operation)
	}
}

func (e *Engine) handleSkillAction(ctx context.Context, config map[string]interface{}, input map[string]interface{}) (map[string]interface{}, error) {
	skillID, _ := config["skill_id"].(string)
	if strings.TrimSpace(skillID) == "" {
		return nil, fmt.Errorf("skill_id is required")
	}
	args, _ := config["parameters"].(map[string]interface{})
	if len(args) == 0 {
		args = workflowToolArgs(config, "type", "skill_id")
	}
	return e.executeToolAction(ctx, skillID, args)
}

func (e *Engine) executeToolAction(ctx context.Context, toolName string, args map[string]interface{}) (map[string]interface{}, error) {
	if e == nil || e.toolGateway == nil {
		return nil, fmt.Errorf("workflow tool gateway is not configured")
	}
	result, err := e.toolGateway.Execute(ctx, ToolExecutionRequest{
		ToolName:  strings.TrimSpace(toolName),
		Arguments: workflowToolArgs(args),
	})
	if err != nil {
		return nil, err
	}
	return workflowToolOutputMap(result)
}

func workflowToolOutputMap(result *ToolExecutionResult) (map[string]interface{}, error) {
	if result == nil {
		return map[string]interface{}{}, nil
	}
	switch typed := result.ExecutionResult.(type) {
	case nil:
		return map[string]interface{}{}, nil
	case map[string]interface{}:
		return typed, nil
	case string:
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(typed), &decoded); err == nil {
			return decoded, nil
		}
		return map[string]interface{}{"result": typed}, nil
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return map[string]interface{}{"result": err.Error()}, nil
		}
		var decoded map[string]interface{}
		if err := json.Unmarshal(data, &decoded); err == nil {
			return decoded, nil
		}
		return map[string]interface{}{"result": string(data)}, nil
	}
}

func workflowToolArgs(config map[string]interface{}, excludeKeys ...string) map[string]interface{} {
	if len(config) == 0 {
		return nil
	}
	excluded := make(map[string]struct{}, len(excludeKeys))
	for _, key := range excludeKeys {
		excluded[key] = struct{}{}
	}
	out := make(map[string]interface{}, len(config))
	for key, value := range config {
		if _, skip := excluded[key]; skip {
			continue
		}
		out[key] = value
	}
	return out
}

func nonEmptyString(values ...interface{}) string {
	for _, value := range values {
		if s, ok := value.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// findTriggerNodes finds all trigger nodes in a workflow.
func (e *Engine) findTriggerNodes(workflow *Workflow) []Node {
	var triggers []Node
	for _, node := range workflow.Nodes {
		if node.Type == NodeTypeTrigger {
			triggers = append(triggers, node)
		}
	}
	return triggers
}

// findNode finds a node by ID.
func (e *Engine) findNode(workflow *Workflow, nodeID string) *Node {
	for i := range workflow.Nodes {
		if workflow.Nodes[i].ID == nodeID {
			return &workflow.Nodes[i]
		}
	}
	return nil
}

// getNodeInput gets input data from previous nodes.
func (e *Engine) getNodeInput(state *executionState, node *Node) map[string]interface{} {
	input := make(map[string]interface{})

	// Find incoming connections
	for _, conn := range state.workflow.Connections {
		if conn.TargetNode == node.ID {
			if result, ok := state.execution.NodeResults[conn.SourceNode]; ok && result.Output != nil {
				// Merge output from source node
				for k, v := range result.Output {
					input[k] = v
				}
			}
		}
	}

	return input
}

// queueNextNodes queues the next nodes to execute.
func (e *Engine) queueNextNodes(state *executionState, nodeID string, result *NodeResult) {
	// Find outgoing connections
	for _, conn := range state.workflow.Connections {
		if conn.SourceNode == nodeID {
			// Check if connection has a condition (for condition/switch nodes)
			if conn.Condition != "" && result != nil {
				port, _ := result.Output["port"].(string)
				if conn.SourcePort != port {
					continue
				}
			}

			// Check if target node's dependencies are met
			if e.areDependenciesMet(state, conn.TargetNode) {
				select {
				case state.nodeQueue <- conn.TargetNode:
				default:
					// Queue full, skip
				}
			}
		}
	}
}

// areDependenciesMet checks if all dependencies for a node are completed.
func (e *Engine) areDependenciesMet(state *executionState, nodeID string) bool {
	state.mu.Lock()
	defer state.mu.Unlock()

	for _, conn := range state.workflow.Connections {
		if conn.TargetNode == nodeID {
			if !state.completed[conn.SourceNode] {
				return false
			}
		}
	}
	return true
}

// isExecutionComplete checks if all reachable nodes are completed.
func (e *Engine) isExecutionComplete(state *executionState) bool {
	state.mu.Lock()
	defer state.mu.Unlock()

	// Check if there are any pending nodes that can be executed
	for _, node := range state.workflow.Nodes {
		if !state.completed[node.ID] && !node.Disabled {
			// Check if this node has any incoming connections
			hasIncoming := false
			allSourcesComplete := true

			for _, conn := range state.workflow.Connections {
				if conn.TargetNode == node.ID {
					hasIncoming = true
					if !state.completed[conn.SourceNode] {
						allSourcesComplete = false
						break
					}
				}
			}

			// If node has incoming connections and all sources are complete,
			// it should have been queued
			if hasIncoming && allSourcesComplete {
				return false
			}
		}
	}

	return true
}

// completeExecution marks an execution as complete.
func (e *Engine) completeExecution(state *executionState) {
	now := timeutil.NowTime()
	state.execution.CompletedAt = &now
	state.execution.Duration = now.Sub(state.execution.StartedAt).Milliseconds()
	e.notifyExecutionUpdate(state.execution)

	close(state.nodeQueue)
}

// interpolateConfig interpolates variables in a config map.
func (e *Engine) interpolateConfig(config map[string]interface{}, variables map[string]interface{}, input map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range config {
		switch val := v.(type) {
		case string:
			result[k] = e.interpolateString(val, variables, input)
		case map[string]interface{}:
			result[k] = e.interpolateConfig(val, variables, input)
		default:
			result[k] = v
		}
	}

	return result
}

// interpolateString interpolates variables in a string.
func (e *Engine) interpolateString(s string, variables map[string]interface{}, input map[string]interface{}) string {
	// Pattern: {{variable.path}} or {{input.field}}
	re := regexp.MustCompile(`\{\{([^}]+)\}\}`)

	return re.ReplaceAllStringFunc(s, func(match string) string {
		path := strings.TrimSpace(match[2 : len(match)-2])
		parts := strings.Split(path, ".")

		var value interface{}
		if parts[0] == "input" && len(parts) > 1 {
			value = e.getNestedValue(input, parts[1:])
		} else {
			value = e.getNestedValue(variables, parts)
		}

		if value == nil {
			return match
		}

		switch v := value.(type) {
		case string:
			return v
		default:
			data, _ := json.Marshal(v)
			return string(data)
		}
	})
}

// getNestedValue gets a nested value from a map.
func (e *Engine) getNestedValue(data map[string]interface{}, path []string) interface{} {
	if len(path) == 0 {
		return nil
	}

	value, ok := data[path[0]]
	if !ok {
		return nil
	}

	if len(path) == 1 {
		return value
	}

	nested, ok := value.(map[string]interface{})
	if !ok {
		return nil
	}

	return e.getNestedValue(nested, path[1:])
}

// evaluateCondition evaluates a condition expression.
func (e *Engine) evaluateCondition(expression string, variables map[string]interface{}, input map[string]interface{}) bool {
	// Simple evaluation - in production, use a proper expression evaluator
	expression = strings.TrimSpace(expression)

	// Handle simple comparisons
	if strings.Contains(expression, "==") {
		parts := strings.SplitN(expression, "==", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		return e.evaluateExpression(left, variables, input) == e.evaluateExpression(right, variables, input)
	}

	if strings.Contains(expression, "!=") {
		parts := strings.SplitN(expression, "!=", 2)
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		return e.evaluateExpression(left, variables, input) != e.evaluateExpression(right, variables, input)
	}

	// Handle boolean values
	if expression == "true" {
		return true
	}
	if expression == "false" {
		return false
	}

	// Evaluate as expression
	result := e.evaluateExpression(expression, variables, input)
	if b, ok := result.(bool); ok {
		return b
	}

	return result != nil && result != "" && result != 0
}

// evaluateExpression evaluates a simple expression.
func (e *Engine) evaluateExpression(expression string, variables map[string]interface{}, input map[string]interface{}) interface{} {
	expression = strings.TrimSpace(expression)

	// Handle string literals
	if strings.HasPrefix(expression, "\"") && strings.HasSuffix(expression, "\"") {
		return expression[1 : len(expression)-1]
	}
	if strings.HasPrefix(expression, "'") && strings.HasSuffix(expression, "'") {
		return expression[1 : len(expression)-1]
	}

	// Handle numbers
	if matched, _ := regexp.MatchString(`^-?\d+(\.\d+)?$`, expression); matched {
		var num float64
		fmt.Sscanf(expression, "%f", &num)
		return num
	}

	// Handle variable paths
	parts := strings.Split(expression, ".")
	if parts[0] == "input" && len(parts) > 1 {
		return e.getNestedValue(input, parts[1:])
	}

	return e.getNestedValue(variables, parts)
}

// getConditionPort returns the output port based on condition result.
func (e *Engine) getConditionPort(node *Node, result bool) string {
	if result {
		if port, ok := node.Config["true_port"].(string); ok {
			return port
		}
		return "true"
	}

	if port, ok := node.Config["false_port"].(string); ok {
		return port
	}
	return "false"
}

// GetExecution returns an execution by ID.
func (e *Engine) GetExecution(id string) (*Execution, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	state, ok := e.executions[id]
	if !ok {
		return nil, ErrExecutionNotFound
	}

	return state.execution, nil
}

// CancelExecution cancels a running execution.
func (e *Engine) CancelExecution(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	state, ok := e.executions[id]
	if !ok {
		return ErrExecutionNotFound
	}

	if state.execution.Status != ExecutionStatusRunning && state.execution.Status != ExecutionStatusPaused {
		return nil
	}

	state.cancel()
	state.execution.Status = ExecutionStatusCancelled
	state.execution.StatusReason = "cancelled"
	e.notifyExecutionUpdate(state.execution)

	return nil
}

// ResumeExecution resumes a paused execution with a decision payload.
func (e *Engine) ResumeExecution(id string, resume ExecutionResumeInput) (*Execution, error) {
	e.mu.RLock()
	state, ok := e.executions[id]
	e.mu.RUnlock()
	if !ok {
		return nil, ErrExecutionNotFound
	}
	if state.execution.Status != ExecutionStatusPaused {
		return nil, fmt.Errorf("execution is not paused")
	}
	if state.execution.Checkpoint == nil {
		return nil, fmt.Errorf("execution has no checkpoint to resume")
	}
	select {
	case state.resumeCh <- cloneExecutionResumeInput(resume):
	default:
		return nil, fmt.Errorf("execution resume is already pending")
	}
	state.execution.Status = ExecutionStatusRunning
	state.execution.StatusReason = string(ExecutionCheckpointResumeWithDecision)
	e.notifyExecutionUpdate(state.execution)
	return cloneExecution(state.execution), nil
}

// ListExecutions returns all executions.
func (e *Engine) ListExecutions() []*Execution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	executions := make([]*Execution, 0, len(e.executions))
	for _, state := range e.executions {
		executions = append(executions, state.execution)
	}

	return executions
}

// Close shuts down the engine.
func (e *Engine) Close() error {
	e.cancel()

	// Cancel all running executions
	e.mu.Lock()
	for _, state := range e.executions {
		state.cancel()
	}
	e.mu.Unlock()

	// Stop all triggers
	e.triggerMu.Lock()
	for _, trigger := range e.triggers {
		close(trigger.stopCh)
	}
	e.triggerMu.Unlock()

	e.wg.Wait()
	return nil
}

func (e *Engine) notifyExecutionUpdate(execution *Execution) {
	if e == nil || e.executionObserver == nil || execution == nil {
		return
	}
	e.executionObserver(cloneExecution(execution))
}

func (e *Engine) buildExecutionCheckpoint(node *Node, result *NodeResult) *ExecutionCheckpoint {
	if node == nil || result == nil {
		return nil
	}
	if e != nil && e.checkpointEnabled != nil && !e.checkpointEnabled() {
		return nil
	}
	rawCheckpoint, ok := node.Config["checkpoint"].(map[string]interface{})
	if !ok {
		kind, _ := node.Config["checkpoint_kind"].(string)
		if strings.TrimSpace(kind) == "" {
			return nil
		}
		rawCheckpoint = map[string]interface{}{
			"kind":    kind,
			"reason":  node.Config["checkpoint_reason"],
			"payload": node.Config["checkpoint_payload"],
		}
	}

	kind := normalizeExecutionCheckpointKind(valueAsString(rawCheckpoint["kind"]))
	if kind == "" {
		return nil
	}

	checkpoint := &ExecutionCheckpoint{
		ID:        uuid.New().String(),
		Kind:      kind,
		NodeID:    node.ID,
		NodeName:  node.Name,
		Reason:    firstNonEmptyString(valueAsString(rawCheckpoint["reason"]), string(kind)),
		CreatedAt: timeutil.NowTime(),
		Payload:   cloneMap(rawCheckpoint["payload"]),
	}
	if checkpoint.Payload == nil {
		checkpoint.Payload = make(map[string]interface{})
	}
	if len(result.Output) > 0 {
		checkpoint.Payload["node_output"] = cloneMap(result.Output)
	}
	return checkpoint
}

func (e *Engine) pauseForCheckpoint(state *executionState, checkpoint *ExecutionCheckpoint, result *NodeResult) error {
	if state == nil || checkpoint == nil {
		return nil
	}
	state.execution.Status = ExecutionStatusPaused
	state.execution.StatusReason = firstNonEmptyString(checkpoint.Reason, string(checkpoint.Kind))
	state.execution.Checkpoint = checkpoint
	e.notifyExecutionUpdate(state.execution)

	var resume ExecutionResumeInput
	select {
	case resume = <-state.resumeCh:
	case <-state.ctx.Done():
		return state.ctx.Err()
	}

	now := timeutil.NowTime()
	checkpoint.ResumedAt = &now
	checkpoint.Resume = &ExecutionResumeInput{
		Decision: strings.TrimSpace(resume.Decision),
		Payload:  cloneMap(resume.Payload),
	}
	state.execution.Status = ExecutionStatusRunning
	state.execution.StatusReason = string(ExecutionCheckpointResumeWithDecision)
	if state.execution.Variables == nil {
		state.execution.Variables = make(map[string]interface{})
	}
	state.execution.Variables["checkpoint"] = executionCheckpointToMap(checkpoint)
	state.execution.Variables["checkpoint_decision"] = strings.TrimSpace(resume.Decision)
	if checkpoint.Resume != nil && len(checkpoint.Resume.Payload) > 0 {
		state.execution.Variables["checkpoint_payload"] = cloneMap(checkpoint.Resume.Payload)
	}
	if result != nil {
		if result.Output == nil {
			result.Output = make(map[string]interface{})
		}
		result.Output["checkpoint"] = executionCheckpointToMap(checkpoint)
	}
	e.notifyExecutionUpdate(state.execution)
	return nil
}

func executionCheckpointToMap(checkpoint *ExecutionCheckpoint) map[string]interface{} {
	if checkpoint == nil {
		return nil
	}
	out := map[string]interface{}{
		"id":         checkpoint.ID,
		"kind":       checkpoint.Kind,
		"node_id":    checkpoint.NodeID,
		"node_name":  checkpoint.NodeName,
		"reason":     checkpoint.Reason,
		"created_at": checkpoint.CreatedAt,
	}
	if checkpoint.Payload != nil {
		out["payload"] = cloneMap(checkpoint.Payload)
	}
	if checkpoint.ResumedAt != nil {
		out["resumed_at"] = *checkpoint.ResumedAt
	}
	if checkpoint.Resume != nil {
		out["resume"] = map[string]interface{}{
			"decision": checkpoint.Resume.Decision,
			"payload":  cloneMap(checkpoint.Resume.Payload),
		}
	}
	return out
}

func normalizeExecutionCheckpointKind(raw string) ExecutionCheckpointKind {
	switch strings.TrimSpace(raw) {
	case string(ExecutionCheckpointPauseForApproval):
		return ExecutionCheckpointPauseForApproval
	case string(ExecutionCheckpointResumeWithDecision):
		return ExecutionCheckpointResumeWithDecision
	case string(ExecutionCheckpointAwaitToolResult):
		return ExecutionCheckpointAwaitToolResult
	case string(ExecutionCheckpointJSONTask):
		return ExecutionCheckpointJSONTask
	default:
		return ""
	}
}

func cloneExecution(execution *Execution) *Execution {
	if execution == nil {
		return nil
	}
	data, err := json.Marshal(execution)
	if err != nil {
		cloned := *execution
		return &cloned
	}
	var cloned Execution
	if err := json.Unmarshal(data, &cloned); err != nil {
		copied := *execution
		return &copied
	}
	return &cloned
}

func cloneExecutionResumeInput(in ExecutionResumeInput) ExecutionResumeInput {
	return ExecutionResumeInput{
		Decision: strings.TrimSpace(in.Decision),
		Payload:  cloneMap(in.Payload),
	}
}

func cloneMap(value interface{}) map[string]interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, item := range typed {
			out[key] = item
		}
		return out
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var out map[string]interface{}
		if err := json.Unmarshal(data, &out); err != nil {
			return nil
		}
		return out
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func valueAsString(value interface{}) string {
	if s, ok := value.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

// ValidateWorkflow validates a workflow definition.
func (e *Engine) ValidateWorkflow(workflow *Workflow) error {
	if workflow.ID == "" {
		return fmt.Errorf("workflow ID is required")
	}

	if workflow.Name == "" {
		return fmt.Errorf("workflow name is required")
	}

	if len(workflow.Nodes) == 0 {
		return fmt.Errorf("workflow must have at least one node")
	}

	// Check for trigger nodes
	hasTrigger := false
	for _, node := range workflow.Nodes {
		if node.Type == NodeTypeTrigger {
			hasTrigger = true
			break
		}
	}

	if !hasTrigger {
		return fmt.Errorf("workflow must have at least one trigger node")
	}

	// Validate node IDs are unique
	nodeIDs := make(map[string]bool)
	for _, node := range workflow.Nodes {
		if node.ID == "" {
			return fmt.Errorf("node ID is required")
		}
		if nodeIDs[node.ID] {
			return fmt.Errorf("duplicate node ID: %s", node.ID)
		}
		nodeIDs[node.ID] = true
	}

	// Validate connections reference valid nodes
	for _, conn := range workflow.Connections {
		if !nodeIDs[conn.SourceNode] {
			return fmt.Errorf("connection references unknown source node: %s", conn.SourceNode)
		}
		if !nodeIDs[conn.TargetNode] {
			return fmt.Errorf("connection references unknown target node: %s", conn.TargetNode)
		}
	}

	// Check for cycles
	if e.hasCycle(workflow) {
		return ErrCyclicDependency
	}

	return nil
}

// hasCycle checks if the workflow has cyclic dependencies.
func (e *Engine) hasCycle(workflow *Workflow) bool {
	// Build adjacency list
	adj := make(map[string][]string)
	for _, conn := range workflow.Connections {
		adj[conn.SourceNode] = append(adj[conn.SourceNode], conn.TargetNode)
	}

	// DFS to detect cycles
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var hasCycleDFS func(node string) bool
	hasCycleDFS = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				if hasCycleDFS(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, node := range workflow.Nodes {
		if !visited[node.ID] {
			if hasCycleDFS(node.ID) {
				return true
			}
		}
	}

	return false
}
