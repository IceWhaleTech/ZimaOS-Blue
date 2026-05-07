package tools

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	maxToolGatewayAuditStringBytes   = 64 * 1024
	maxToolGatewayLLMStringBytes     = 8 * 1024
	maxToolGatewayPDFPreCompactBytes = 256 * 1024
	maxToolGatewaySanitizeDepth      = 64
)

var ansiEscapeRE = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type toolPayloadVisitKey struct {
	kind reflect.Kind
	ptr  uintptr
}

// ToolGatewayError is a structured runtime error for tool-call normalization/execution.
type ToolGatewayError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *ToolGatewayError) Error() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Message)
}

func (e *ToolGatewayError) ToolRuntimeCode() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Code)
}

func (e *ToolGatewayError) ToolRuntimeDetails() map[string]interface{} {
	if e == nil {
		return nil
	}
	return cloneJSONInterfaceMap(e.Details)
}

// ToolGatewayRequest describes a tool execution request flowing through the shared gateway.
type ToolGatewayRequest struct {
	ToolCallID   string
	ToolName     string
	Arguments    string
	Provider     string
	ProviderID   string
	Model        string
	AgentID      string
	SessionID    string
	RouteKind    ToolRouteKind
	UserID       string
	PolicySource string
	RiskLevel    string
}

// ToolGatewayCall is the normalized tool call shape after parsing/validation.
type ToolGatewayCall struct {
	ToolCallID    string                 `json:"tool_call_id,omitempty"`
	ToolName      string                 `json:"tool_name"`
	Arguments     map[string]interface{} `json:"arguments,omitempty"`
	ArgumentsJSON string                 `json:"arguments_json"`
}

// ToolGatewayResult contains normalized tool execution outputs for audit and LLM use.
type ToolGatewayResult struct {
	NormalizedCall        ToolGatewayCall       `json:"normalized_call"`
	ExecutionResult       interface{}           `json:"execution_result,omitempty"`
	CompactLLMPayload     interface{}           `json:"compact_llm_payload,omitempty"`
	CompactLLMContent     string                `json:"compact_llm_content,omitempty"`
	AuditPayload          interface{}           `json:"audit_payload,omitempty"`
	AuditContent          string                `json:"audit_content,omitempty"`
	Approval              *ToolApprovalEnvelope `json:"approval,omitempty"`
	GroundingEvidenceRefs []string              `json:"grounding_evidence_refs,omitempty"`
	ToolDurationMs        float64               `json:"tool_duration_ms,omitempty"`
}

// ToolGateway centralizes tool validation, approval, execution, and output shaping.
type ToolGateway struct {
	registry         *Registry
	executor         *Executor
	approver         ToolApprover
	observer         RuntimeEventObserver
	metrics          toolGatewayMetricsRecorder
	toolSurfaceAudit *ToolSurfaceAuditState
}

type toolGatewayMetricsRecorder interface {
	RecordCounter(name string, value int64, tags map[string]string)
}

type toolGatewayNameRewrite struct {
	From string
	To   string
}

// NewToolGateway creates a new shared tool execution gateway.
func NewToolGateway(registry *Registry, executor *Executor) *ToolGateway {
	return &ToolGateway{
		registry: registry,
		executor: executor,
	}
}

// SetApprover wires an approval backend used for non-exec runtime enforcement.
func (g *ToolGateway) SetApprover(approver ToolApprover) {
	if g == nil {
		return
	}
	g.approver = approver
}

// SetEventObserver wires runtime lifecycle observation into the gateway.
func (g *ToolGateway) SetEventObserver(observer RuntimeEventObserver) {
	if g == nil {
		return
	}
	g.observer = observer
}

// SetMetricsRecorder wires a lightweight counter recorder for runtime events.
func (g *ToolGateway) SetMetricsRecorder(recorder toolGatewayMetricsRecorder) {
	if g == nil {
		return
	}
	g.metrics = recorder
}

// SetToolSurfaceAuditState wires a shared tool-surface audit counter sink into the gateway.
func (g *ToolGateway) SetToolSurfaceAuditState(state *ToolSurfaceAuditState) {
	if g == nil {
		return
	}
	g.toolSurfaceAudit = state
}

// Execute validates and executes a tool call through the shared runtime gateway.
func (g *ToolGateway) Execute(ctx context.Context, req ToolGatewayRequest) (*ToolGatewayResult, error) {
	if g == nil || g.executor == nil || g.registry == nil {
		return nil, &ToolGatewayError{Code: "tool_gateway_unavailable", Message: "tool gateway is not configured"}
	}

	ctx, req = applyToolGatewayContext(ctx, req)
	result := &ToolGatewayResult{}
	requestedName := strings.TrimSpace(req.ToolName)

	def, resolvedName, rewrite, err := g.lookupDefinition(requestedName, req.RouteKind)
	if err != nil {
		result.populateError(req.ToolCallID, requestedName, nil, err)
		g.recordMetric("tool_call_rejected_total", req, map[string]string{
			"reason": classifyToolGatewayErrorCode(err),
		})
		return result, err
	}
	if rewrite.From != "" && rewrite.To != "" {
		if g.toolSurfaceAudit != nil {
			g.toolSurfaceAudit.RecordAliasRewrite()
		}
		g.recordMetric("tool_surface_alias_rewrite_total", req, map[string]string{
			"from": rewrite.From,
			"to":   rewrite.To,
		})
	}

	args, argsJSON, err := normalizeToolArguments(g.executor, resolvedName, req.Arguments)
	if err != nil {
		result.populateError(req.ToolCallID, resolvedName, nil, err)
		g.recordMetric("tool_call_rejected_total", req, map[string]string{
			"tool":   resolvedName,
			"reason": classifyToolGatewayErrorCode(err),
		})
		return result, err
	}
	args = normalizeCompatArgs(requestedName, resolvedName, args)
	if normalizedJSON, marshalErr := json.Marshal(args); marshalErr == nil {
		argsJSON = string(normalizedJSON)
	}
	result.NormalizedCall = ToolGatewayCall{
		ToolCallID:    strings.TrimSpace(req.ToolCallID),
		ToolName:      resolvedName,
		Arguments:     args,
		ArgumentsJSON: argsJSON,
	}

	if err := ValidateToolSchema(def.Parameters); err != nil {
		schemaErr := &ToolGatewayError{
			Code:    "invalid_tool_schema",
			Message: fmt.Sprintf("tool %q has an invalid schema", resolvedName),
			Details: map[string]interface{}{"tool": resolvedName, "cause": err.Error()},
		}
		result.populateError(req.ToolCallID, resolvedName, args, schemaErr)
		g.recordMetric("tool_call_rejected_total", req, map[string]string{
			"tool":   resolvedName,
			"reason": schemaErr.Code,
		})
		return result, schemaErr
	}
	if err := ValidateToolArguments(def.Parameters, args); err != nil {
		argErr := &ToolGatewayError{
			Code:    "invalid_tool_arguments",
			Message: fmt.Sprintf("tool %q arguments did not match schema", resolvedName),
			Details: map[string]interface{}{"tool": resolvedName, "cause": err.Error()},
		}
		result.populateError(req.ToolCallID, resolvedName, args, argErr)
		g.recordMetric("tool_call_rejected_total", req, map[string]string{
			"tool":   resolvedName,
			"reason": argErr.Code,
		})
		return result, argErr
	}

	req.ToolName = resolvedName
	req.RiskLevel = normalizeToolRiskLevel(nonEmpty(strings.TrimSpace(req.RiskLevel), strings.TrimSpace(def.RiskLevel), inferToolRiskLevel(resolvedName)))
	bindingHash := buildToolBindingHash(req, argsJSON)
	capabilityKind := inferToolTraceCapabilityKind(resolvedName, args)
	toolEvent := ToolRuntimeEvent{
		RunID:          GetRunID(ctx),
		TurnID:         GetTurnID(ctx),
		StepIndex:      GetRunStep(ctx),
		ToolCallID:     strings.TrimSpace(req.ToolCallID),
		ToolName:       resolvedName,
		CapabilityKind: capabilityKind,
		SessionID:      strings.TrimSpace(req.SessionID),
		RouteKind:      req.RouteKind,
		UserID:         strings.TrimSpace(req.UserID),
		Provider:       strings.TrimSpace(req.Provider),
		ProviderID:     strings.TrimSpace(req.ProviderID),
		Model:          strings.TrimSpace(req.Model),
		AgentID:        strings.TrimSpace(req.AgentID),
		Arguments:      cloneJSONInterfaceMap(args),
	}
	if g.observer != nil {
		g.observer.OnToolRequested(toolEvent)
	}
	emitToolFinished := func(resultPayload interface{}, runErr error) {
		if g.observer == nil {
			return
		}
		finished := toolEvent
		if resultPayload != nil {
			finished.Result = resultPayload
		}
		if runErr != nil {
			finished.Error = runErr.Error()
		}
		g.observer.OnToolFinished(finished)
	}
	if g.approver != nil && !strings.EqualFold(resolvedName, "exec") && !strings.EqualFold(resolvedName, "bash") {
		approvalDecision, approvalErr := g.approver.AuthorizeToolCall(ctx, ToolApprovalRequest{
			ToolName:     resolvedName,
			ToolCallID:   strings.TrimSpace(req.ToolCallID),
			Arguments:    cloneJSONInterfaceMap(args),
			Provider:     strings.TrimSpace(req.Provider),
			ProviderID:   strings.TrimSpace(req.ProviderID),
			Model:        strings.TrimSpace(req.Model),
			AgentID:      strings.TrimSpace(req.AgentID),
			SessionID:    strings.TrimSpace(req.SessionID),
			RouteKind:    req.RouteKind,
			UserID:       strings.TrimSpace(req.UserID),
			RiskLevel:    req.RiskLevel,
			PolicySource: nonEmpty(strings.TrimSpace(req.PolicySource), "approval_config"),
			BindingHash:  bindingHash,
		})
		if approvalErr != nil {
			result.populateError(req.ToolCallID, resolvedName, args, approvalErr)
			emitToolFinished(result.AuditPayload, approvalErr)
			g.recordMetric("tool_call_rejected_total", req, map[string]string{
				"tool":   resolvedName,
				"reason": classifyToolGatewayErrorCode(approvalErr),
			})
			return result, approvalErr
		}
		result.Approval = &approvalDecision.Approval
		result.Approval.BindingHash = nonEmpty(strings.TrimSpace(result.Approval.BindingHash), bindingHash)
		result.Approval.RiskLevel = nonEmpty(strings.TrimSpace(result.Approval.RiskLevel), req.RiskLevel)
		if result.Approval.Required {
			g.recordMetric("tool_approval_requested_total", req, map[string]string{
				"tool":          resolvedName,
				"policy_mode":   strings.TrimSpace(result.Approval.Mode),
				"policy_source": strings.TrimSpace(result.Approval.PolicySource),
				"risk_level":    strings.TrimSpace(result.Approval.RiskLevel),
			})
		}
		if !approvalDecision.Allowed {
			denyErr := &ToolGatewayError{
				Code:    "tool_approval_denied",
				Message: fmt.Sprintf("tool %q was denied by approval policy", resolvedName),
				Details: map[string]interface{}{
					"tool":         resolvedName,
					"approval_id":  result.Approval.ID,
					"binding_hash": result.Approval.BindingHash,
					"policy_mode":  result.Approval.Mode,
				},
			}
			result.populateError(req.ToolCallID, resolvedName, args, denyErr)
			result.Approval = &approvalDecision.Approval
			emitToolFinished(result.AuditPayload, denyErr)
			g.recordMetric("tool_call_rejected_total", req, map[string]string{
				"tool":   resolvedName,
				"reason": denyErr.Code,
			})
			return result, denyErr
		}
	}

	toolStart := time.Now()
	rawResult, execErr := g.executor.Execute(ctx, resolvedName, args)
	toolDuration := time.Since(toolStart)
	result.ToolDurationMs = float64(toolDuration.Milliseconds())
	normalizedResult := normalizeGatewayToolResult(rawResult)
	if execErr != nil {
		execToolErr := normalizeToolExecutionError(resolvedName, execErr)
		result.ExecutionResult = normalizedResult
		result.populateError(req.ToolCallID, resolvedName, args, execToolErr)
		if result.Approval != nil {
			result.Approval.BindingHash = bindingHash
		}
		emitToolFinished(normalizedResult, execToolErr)
		return result, execToolErr
	}

	result.ExecutionResult = normalizedResult
	result.AuditPayload = sanitizeToolPayload(normalizedResult, maxToolGatewayAuditStringBytes)
	result.AuditContent = serializeToolPayload(result.AuditPayload)
	result.CompactLLMPayload = compactToolPayloadForLLM(ctx, req, resolvedName, normalizedResult, result.AuditContent)
	result.CompactLLMContent = serializeToolPayload(result.CompactLLMPayload)
	if strings.Contains(result.AuditContent, "[circular payload omitted]") || strings.Contains(result.CompactLLMContent, "[circular payload omitted]") {
		g.recordMetric("tool_payload_circular_total", req, map[string]string{
			"tool": resolvedName,
		})
	}
	emitToolFinished(result.AuditPayload, nil)
	if id := strings.TrimSpace(req.ToolCallID); id != "" {
		result.GroundingEvidenceRefs = []string{id}
	}
	if result.Approval != nil {
		result.Approval.BindingHash = bindingHash
	}

	return result, nil
}

// ToolErrorPayload converts a runtime error into a structured tool payload.
func ToolErrorPayload(err error) map[string]interface{} {
	if err == nil {
		return map[string]interface{}{}
	}
	if code, details, ok := toolRuntimeErrorContract(err); ok {
		payload := map[string]interface{}{
			"error": err.Error(),
			"code":  code,
		}
		if len(details) > 0 {
			payload["details"] = details
		}
		return payload
	}
	return map[string]interface{}{"error": err.Error()}
}

func (r *ToolGatewayResult) populateError(toolCallID, toolName string, args map[string]interface{}, err error) {
	if r == nil {
		return
	}
	if r.NormalizedCall.ToolName == "" {
		r.NormalizedCall = ToolGatewayCall{
			ToolCallID:    strings.TrimSpace(toolCallID),
			ToolName:      strings.TrimSpace(toolName),
			Arguments:     cloneJSONInterfaceMap(args),
			ArgumentsJSON: serializeToolPayload(args),
		}
	}
	payload := ToolErrorPayload(err)
	r.AuditPayload = payload
	r.AuditContent = serializeToolPayload(payload)
	r.CompactLLMPayload = payload
	r.CompactLLMContent = serializeToolPayload(payload)
}

func (g *ToolGateway) lookupDefinition(name string, routeKind ToolRouteKind) (ToolDefinition, string, toolGatewayNameRewrite, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ToolDefinition{}, "", toolGatewayNameRewrite{}, &ToolGatewayError{Code: "tool_not_found", Message: "tool name is required"}
	}
	if def, ok := g.registry.LookupDefinitionForRoute(trimmed, routeKind); ok {
		return def, trimmed, toolGatewayNameRewrite{}, nil
	}
	normalized := normalizeCompatToolName(trimmed)
	if def, ok := g.registry.LookupDefinitionForRoute(normalized, routeKind); ok {
		return def, normalized, toolGatewayNameRewrite{}, nil
	}
	if def, fallbackName, ok := g.lookupPublicRouteFallback(trimmed, routeKind); ok {
		return def, fallbackName, toolGatewayNameRewrite{From: trimmed, To: fallbackName}, nil
	}
	if routeKind != ToolRouteKindUnknown {
		if _, ok := g.registry.LookupDefinition(trimmed); ok {
			return ToolDefinition{}, "", toolGatewayNameRewrite{}, &ToolGatewayError{
				Code:    "tool_not_visible_for_route",
				Message: fmt.Sprintf("tool %q is not visible for route %q", trimmed, routeKind),
				Details: map[string]interface{}{"tool": trimmed, "route_kind": routeKind},
			}
		}
		if normalized != trimmed {
			if _, ok := g.registry.LookupDefinition(normalized); ok {
				return ToolDefinition{}, "", toolGatewayNameRewrite{}, &ToolGatewayError{
					Code:    "tool_not_visible_for_route",
					Message: fmt.Sprintf("tool %q is not visible for route %q", normalized, routeKind),
					Details: map[string]interface{}{"tool": normalized, "route_kind": routeKind},
				}
			}
		}
	}
	return ToolDefinition{}, "", toolGatewayNameRewrite{}, &ToolGatewayError{
		Code:    "tool_not_found",
		Message: fmt.Sprintf("tool %q is not registered or visible", trimmed),
		Details: map[string]interface{}{"tool": trimmed},
	}
}

func (g *ToolGateway) lookupPublicRouteFallback(name string, routeKind ToolRouteKind) (ToolDefinition, string, bool) {
	if g == nil || g.registry == nil || routeKind == ToolRouteKindUnknown {
		return ToolDefinition{}, "", false
	}
	if !strings.EqualFold(strings.TrimSpace(name), "exec") {
		return ToolDefinition{}, "", false
	}
	if _, ok := g.registry.LookupDefinition("exec"); !ok {
		return ToolDefinition{}, "", false
	}
	if def, ok := g.registry.LookupDefinitionForRoute("bash", routeKind); ok {
		return def, "bash", true
	}
	return ToolDefinition{}, "", false
}

func normalizeToolArguments(executor *Executor, name, raw string) (map[string]interface{}, string, error) {
	normalizedJSON, ok := CanonicalizeToolArgumentsJSON(raw)
	if ok {
		var args map[string]interface{}
		if err := json.Unmarshal([]byte(normalizedJSON), &args); err == nil {
			return args, normalizedJSON, nil
		}
	}
	if executor != nil {
		for _, candidate := range buildArgsCandidates(strings.TrimSpace(raw)) {
			if args, ok := executor.parseLooseArgs(name, candidate); ok {
				return args, serializeToolPayload(args), nil
			}
		}
	}
	trimmed := strings.TrimSpace(raw)
	return nil, "", &ToolGatewayError{
		Code:    "invalid_tool_arguments_json",
		Message: "tool arguments must be a JSON object",
		Details: map[string]interface{}{"raw_arguments": truncateToolString(trimmed, 512)},
	}
}

// CanonicalizeToolArgumentsJSON normalizes tool-call arguments into a JSON object
// string when they are empty or recoverably object-shaped.
func CanonicalizeToolArgumentsJSON(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "{}", true
	}
	args, ok := parseJSONObjectArgs(trimmed)
	if !ok {
		return "", false
	}
	return serializeToolPayload(args), true
}

func applyToolGatewayContext(ctx context.Context, req ToolGatewayRequest) (context.Context, ToolGatewayRequest) {
	if ctx == nil {
		ctx = context.Background()
	}
	req.Provider = nonEmpty(strings.TrimSpace(req.Provider), GetProvider(ctx))
	req.ProviderID = nonEmpty(strings.TrimSpace(req.ProviderID), GetProviderID(ctx))
	req.Model = nonEmpty(strings.TrimSpace(req.Model), GetModel(ctx))
	req.AgentID = nonEmpty(strings.TrimSpace(req.AgentID), GetAgentID(ctx))
	req.SessionID = nonEmpty(strings.TrimSpace(req.SessionID), GetSessionID(ctx))
	if req.RouteKind == ToolRouteKindUnknown {
		req.RouteKind = GetRouteKind(ctx)
	}
	req.UserID = nonEmpty(strings.TrimSpace(req.UserID), GetUserID(ctx))

	if req.Provider != "" {
		ctx = WithProvider(ctx, req.Provider)
	}
	if req.ProviderID != "" {
		ctx = WithProviderID(ctx, req.ProviderID)
	}
	if req.Model != "" {
		ctx = WithModel(ctx, req.Model)
	}
	if req.AgentID != "" {
		ctx = WithAgentID(ctx, req.AgentID)
	}
	if req.SessionID != "" {
		ctx = WithSessionID(ctx, req.SessionID)
	}
	if req.RouteKind != ToolRouteKindUnknown {
		ctx = WithRouteKind(ctx, req.RouteKind)
	}
	if req.UserID != "" {
		ctx = WithUserID(ctx, req.UserID)
	}
	return ctx, req
}

func buildToolBindingHash(req ToolGatewayRequest, argsJSON string) string {
	payload := map[string]interface{}{
		"tool":        strings.TrimSpace(req.ToolName),
		"tool_call":   strings.TrimSpace(req.ToolCallID),
		"args_json":   strings.TrimSpace(argsJSON),
		"provider":    strings.TrimSpace(req.Provider),
		"provider_id": strings.TrimSpace(req.ProviderID),
		"model":       strings.TrimSpace(req.Model),
		"agent_id":    strings.TrimSpace(req.AgentID),
		"session_id":  strings.TrimSpace(req.SessionID),
		"route_kind":  strings.TrimSpace(string(req.RouteKind)),
		"risk_level":  strings.TrimSpace(req.RiskLevel),
	}
	data, _ := json.Marshal(payload)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func normalizeGatewayToolResult(raw interface{}) interface{} {
	switch typed := raw.(type) {
	case *ForwardedResult:
		return normalizeGatewayToolResult(typed.Result)
	default:
		return typed
	}
}

func compactToolPayloadForLLM(ctx context.Context, req ToolGatewayRequest, toolName string, raw interface{}, auditContent string) interface{} {
	if strings.EqualFold(strings.TrimSpace(toolName), "pdf") {
		preCompacted := sanitizeToolPayload(raw, maxToolGatewayPDFPreCompactBytes)
		return sanitizeToolPayload(compactPDFPayloadForLLM(preCompacted), maxToolGatewayLLMStringBytes)
	}
	if strings.EqualFold(strings.TrimSpace(toolName), "web_query") {
		if compacted, ok := BuildCompactWebQueryPayloadForLLM(ctx, raw, auditContent, WebQueryLLMCompactionOptions{
			ByteBudget:     maxToolGatewayLLMStringBytes,
			Mode:           WebQueryLLMCompactionDefault,
			ResultLimit:    3,
			FactLimit:      10,
			WarningLimit:   3,
			Materialize:    true,
			ToolCallID:     strings.TrimSpace(req.ToolCallID),
			ConversationID: strings.TrimSpace(req.SessionID),
			ToolName:       strings.TrimSpace(toolName),
		}); ok {
			return compacted
		}
	}
	sanitized := sanitizeToolPayload(raw, maxToolGatewayLLMStringBytes)
	if isExternalContentTool(toolName) {
		return map[string]interface{}{
			"trust": "untrusted_external_content",
			"tool":  strings.TrimSpace(toolName),
			"data":  sanitized,
		}
	}
	return sanitized
}

func compactPDFPayloadForLLM(raw interface{}) interface{} {
	payload, ok := raw.(map[string]interface{})
	if !ok || len(payload) == 0 {
		return raw
	}

	out := make(map[string]interface{}, 8)
	for _, key := range []string{
		"document",
		"selected_pages",
		"char_count",
		"truncated",
		"ocr_used",
		"ocr_pages",
		"ocr_models",
		"vision_used",
		"vision_pages",
		"vision_models",
		"warnings",
		"mode",
		"count",
		"documents",
		"results",
	} {
		if value, exists := payload[key]; exists {
			out[key] = value
		}
	}

	if pages, exists := payload["pages"].([]interface{}); exists && len(pages) > 0 {
		compactedPages := make([]interface{}, 0, len(pages))
		for _, item := range pages {
			page, ok := item.(map[string]interface{})
			if !ok || len(page) == 0 {
				continue
			}
			compacted := make(map[string]interface{}, 5)
			for _, key := range []string{"number", "text", "char_count", "source", "empty"} {
				if value, exists := page[key]; exists {
					compacted[key] = value
				}
			}
			if len(compacted) > 0 {
				compactedPages = append(compactedPages, compacted)
			}
		}
		if len(compactedPages) > 0 {
			out["pages"] = compactedPages
		}
	} else if text, exists := payload["text"]; exists {
		if derived := derivePDFPagesForLLM(anyToStringForLLM(text)); len(derived) > 0 {
			out["pages"] = derived
		}
	}

	if _, hasPages := out["pages"]; !hasPages {
		if rawText, exists := payload["raw_text"]; exists {
			if derived := derivePDFPagesForLLM(anyToStringForLLM(rawText)); len(derived) > 0 {
				out["pages"] = derived
			}
		}
	}

	if _, hasPages := out["pages"]; !hasPages {
		if text, exists := payload["text"]; exists {
			out["text"] = text
		}
	}

	if len(out) == 0 {
		return raw
	}
	return out
}

func derivePDFPagesForLLM(text string) []interface{} {
	sections := splitPDFSectionsForToolLLM(text)
	if len(sections) == 0 {
		return nil
	}
	pages := make([]interface{}, 0, len(sections))
	for _, section := range sections {
		number, body := parsePDFSectionForToolLLM(section)
		if strings.TrimSpace(body) == "" {
			continue
		}
		entry := map[string]interface{}{
			"text": body,
		}
		if number > 0 {
			entry["number"] = number
		}
		pages = append(pages, entry)
	}
	if len(pages) <= 1 {
		return nil
	}
	return pages
}

func splitPDFSectionsForToolLLM(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	sections := make([]string, 0, 8)
	var current strings.Builder
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[Page ") && strings.HasSuffix(trimmed, "]") {
			if strings.TrimSpace(current.String()) != "" {
				sections = append(sections, strings.TrimSpace(current.String()))
				current.Reset()
			}
		}
		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}
	if strings.TrimSpace(current.String()) != "" {
		sections = append(sections, strings.TrimSpace(current.String()))
	}
	if len(sections) <= 1 {
		return nil
	}
	return sections
}

func parsePDFSectionForToolLLM(section string) (int, string) {
	section = strings.TrimSpace(section)
	if section == "" {
		return 0, ""
	}
	lines := strings.Split(section, "\n")
	if len(lines) == 0 {
		return 0, section
	}
	first := strings.TrimSpace(lines[0])
	if strings.HasPrefix(first, "[Page ") && strings.HasSuffix(first, "]") {
		pageToken := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(first, "[Page "), "]"))
		pageNumber, _ := strconv.Atoi(pageToken)
		body := strings.TrimSpace(strings.Join(lines[1:], "\n"))
		return pageNumber, body
	}
	return 0, section
}

// SafeToolPayloadValue normalizes arbitrary tool payloads into a JSON-safe,
// cycle-aware value that can be logged or re-serialized safely.
func SafeToolPayloadValue(raw interface{}, maxStringBytes int) interface{} {
	return sanitizeToolPayload(raw, maxStringBytes)
}

// SafeToolPayloadString renders arbitrary tool payloads as a stable string
// without recursing on self-referential values.
func SafeToolPayloadString(raw interface{}, maxStringBytes int) string {
	sanitized := SafeToolPayloadValue(raw, maxStringBytes)
	switch typed := sanitized.(type) {
	case nil:
		return "{}"
	case string:
		return typed
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return sanitizeToolString("[payload omitted]", maxStringBytes)
		}
		return string(data)
	}
}

func sanitizeToolPayload(raw interface{}, maxStringBytes int) interface{} {
	return sanitizeToolPayloadValue(raw, maxStringBytes, make(map[toolPayloadVisitKey]struct{}), 0)
}

func sanitizeToolPayloadValue(raw interface{}, maxStringBytes int, seen map[toolPayloadVisitKey]struct{}, depth int) interface{} {
	if depth > maxToolGatewaySanitizeDepth {
		return sanitizeToolString("[truncated nested payload]", maxStringBytes)
	}
	if token, ok := toolPayloadVisitToken(raw); ok {
		if _, exists := seen[token]; exists {
			return sanitizeToolString("[circular payload omitted]", maxStringBytes)
		}
		seen[token] = struct{}{}
		defer delete(seen, token)
	}
	switch typed := raw.(type) {
	case nil:
		return map[string]interface{}{}
	case string:
		return sanitizeToolString(typed, maxStringBytes)
	case []byte:
		return sanitizeToolString(string(typed), maxStringBytes)
	case bool:
		return typed
	case int, int8, int16, int32, int64:
		return typed
	case uint, uint8, uint16, uint32, uint64:
		return typed
	case float32, float64:
		return typed
	case json.Number:
		return typed
	case map[string]string:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = sanitizeToolPayloadValue(value, maxStringBytes, seen, depth+1)
		}
		return out
	case map[string]interface{}:
		out := make(map[string]interface{}, len(typed))
		for key, value := range typed {
			out[key] = sanitizeToolPayloadValue(value, maxStringBytes, seen, depth+1)
		}
		return out
	case []string:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = sanitizeToolPayloadValue(value, maxStringBytes, seen, depth+1)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(typed))
		for i, value := range typed {
			out[i] = sanitizeToolPayloadValue(value, maxStringBytes, seen, depth+1)
		}
		return out
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return sanitizeToolPayloadFallbackValue(reflect.ValueOf(raw), maxStringBytes, seen, depth+1, true)
		}
		var decoded interface{}
		if err := json.Unmarshal(data, &decoded); err != nil {
			return sanitizeToolString(string(data), maxStringBytes)
		}
		return sanitizeToolPayloadValue(decoded, maxStringBytes, seen, depth+1)
	}
}

func sanitizeToolPayloadFallbackValue(value reflect.Value, maxStringBytes int, seen map[toolPayloadVisitKey]struct{}, depth int, skipToken bool) interface{} {
	if depth > maxToolGatewaySanitizeDepth {
		return sanitizeToolString("[truncated nested payload]", maxStringBytes)
	}
	if !value.IsValid() {
		return map[string]interface{}{}
	}
	if !skipToken {
		if token, ok := toolPayloadReflectToken(value); ok {
			if _, exists := seen[token]; exists {
				return sanitizeToolString("[circular payload omitted]", maxStringBytes)
			}
			seen[token] = struct{}{}
			defer delete(seen, token)
		}
	}
	if value.CanInterface() {
		if errValue, ok := value.Interface().(error); ok && errValue != nil {
			return sanitizeToolString(errValue.Error(), maxStringBytes)
		}
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return map[string]interface{}{}
		}
		return sanitizeToolPayloadFallbackValue(value.Elem(), maxStringBytes, seen, depth+1, false)
	case reflect.Pointer:
		if value.IsNil() {
			return map[string]interface{}{}
		}
		return sanitizeToolPayloadFallbackValue(value.Elem(), maxStringBytes, seen, depth+1, false)
	case reflect.Bool:
		return value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint()
	case reflect.Float32, reflect.Float64:
		return value.Float()
	case reflect.String:
		return sanitizeToolString(value.String(), maxStringBytes)
	case reflect.Slice:
		if value.IsNil() {
			return map[string]interface{}{}
		}
		if value.Type().Elem().Kind() == reflect.Uint8 {
			return sanitizeToolString(string(value.Bytes()), maxStringBytes)
		}
		fallthrough
	case reflect.Array:
		out := make([]interface{}, value.Len())
		for i := 0; i < value.Len(); i++ {
			out[i] = sanitizeToolPayloadFallbackValue(value.Index(i), maxStringBytes, seen, depth+1, false)
		}
		return out
	case reflect.Map:
		if value.IsNil() {
			return map[string]interface{}{}
		}
		out := make(map[string]interface{}, value.Len())
		iter := value.MapRange()
		for iter.Next() {
			out[sanitizeToolPayloadMapKey(iter.Key(), maxStringBytes)] = sanitizeToolPayloadFallbackValue(iter.Value(), maxStringBytes, seen, depth+1, false)
		}
		return out
	case reflect.Struct:
		out := make(map[string]interface{}, value.NumField())
		valueType := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := valueType.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name, ok := sanitizeToolPayloadStructFieldName(field)
			if !ok {
				continue
			}
			out[name] = sanitizeToolPayloadFallbackValue(value.Field(i), maxStringBytes, seen, depth+1, false)
		}
		return out
	case reflect.Func:
		return sanitizeToolString("[func omitted]", maxStringBytes)
	case reflect.Chan:
		return sanitizeToolString("[chan omitted]", maxStringBytes)
	case reflect.UnsafePointer:
		return sanitizeToolString("[unsafe pointer omitted]", maxStringBytes)
	default:
		return sanitizeToolString(value.Type().String(), maxStringBytes)
	}
}

func sanitizeToolPayloadMapKey(value reflect.Value, maxStringBytes int) string {
	if !value.IsValid() {
		return ""
	}
	switch value.Kind() {
	case reflect.String:
		return sanitizeToolString(value.String(), maxStringBytes)
	case reflect.Bool:
		return strconv.FormatBool(value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(value.Uint(), 10)
	case reflect.Float32:
		return strconv.FormatFloat(value.Float(), 'f', -1, 32)
	case reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'f', -1, 64)
	default:
		if token, ok := toolPayloadReflectToken(value); ok {
			return sanitizeToolString(value.Type().String()+"@0x"+strconv.FormatUint(uint64(token.ptr), 16), maxStringBytes)
		}
		return sanitizeToolString(value.Type().String(), maxStringBytes)
	}
}

func sanitizeToolPayloadStructFieldName(field reflect.StructField) (string, bool) {
	tag := strings.TrimSpace(field.Tag.Get("json"))
	if tag == "-" {
		return "", false
	}
	if tag != "" {
		name := strings.TrimSpace(strings.Split(tag, ",")[0])
		if name != "" {
			return name, true
		}
	}
	return field.Name, true
}

func toolPayloadVisitToken(raw interface{}) (toolPayloadVisitKey, bool) {
	if raw == nil {
		return toolPayloadVisitKey{}, false
	}
	return toolPayloadReflectToken(reflect.ValueOf(raw))
}

func toolPayloadReflectToken(value reflect.Value) (toolPayloadVisitKey, bool) {
	if !value.IsValid() {
		return toolPayloadVisitKey{}, false
	}
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return toolPayloadVisitKey{}, false
		}
		return toolPayloadReflectToken(value.Elem())
	case reflect.Map, reflect.Slice, reflect.Pointer:
		if value.IsNil() {
			return toolPayloadVisitKey{}, false
		}
		return toolPayloadVisitKey{kind: value.Kind(), ptr: value.Pointer()}, true
	default:
		return toolPayloadVisitKey{}, false
	}
}

func serializeToolPayload(payload interface{}) string {
	return SafeToolPayloadString(payload, maxToolGatewayAuditStringBytes)
}

func sanitizeToolString(raw string, maxLen int) string {
	return truncateToolString(strings.ToValidUTF8(ansiEscapeRE.ReplaceAllString(raw, ""), "\uFFFD"), maxLen)
}

func truncateToolString(raw string, maxLen int) string {
	if maxLen <= 0 || len(raw) <= maxLen {
		return raw
	}
	return raw[:maxLen] + "\n...[output truncated]"
}

func normalizeToolRiskLevel(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(RiskLevelCritical):
		return string(RiskLevelCritical)
	case string(RiskLevelHigh):
		return string(RiskLevelHigh)
	case string(RiskLevelMedium):
		return string(RiskLevelMedium)
	default:
		return string(RiskLevelLow)
	}
}

func inferToolRiskLevel(toolName string) string {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "bash", "exec":
		return string(RiskLevelHigh)
	case "browser", "web_query", "web_fetch", "web_extract", "web_crawl":
		return string(RiskLevelMedium)
	default:
		return string(RiskLevelLow)
	}
}

func isExternalContentTool(toolName string) bool {
	switch strings.ToLower(strings.TrimSpace(toolName)) {
	case "browser", "web_query", "web_fetch", "web_read", "web_extract", "web_crawl", "web_search":
		return true
	default:
		return false
	}
}

func cloneJSONInterfaceMap(in map[string]interface{}) map[string]interface{} {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for key, value := range in {
		out[key] = cloneJSONCompatibleValue(value)
	}
	return out
}

func (g *ToolGateway) recordMetric(name string, req ToolGatewayRequest, tags map[string]string) {
	if g == nil || g.metrics == nil || strings.TrimSpace(name) == "" {
		return
	}
	merged := map[string]string{
		"tool":        strings.TrimSpace(req.ToolName),
		"route_kind":  strings.TrimSpace(string(req.RouteKind)),
		"provider":    strings.TrimSpace(req.Provider),
		"provider_id": strings.TrimSpace(req.ProviderID),
		"model":       strings.TrimSpace(req.Model),
		"risk_level":  strings.TrimSpace(req.RiskLevel),
	}
	for key, value := range tags {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			merged[key] = trimmed
		}
	}
	g.metrics.RecordCounter(name, 1, merged)
}

func classifyToolGatewayErrorCode(err error) string {
	if code, _, ok := toolRuntimeErrorContract(err); ok {
		return code
	}
	return "tool_gateway_error"
}

func normalizeToolExecutionError(toolName string, err error) *ToolGatewayError {
	if err == nil {
		return nil
	}
	if gatewayErr, ok := err.(*ToolGatewayError); ok {
		normalized := &ToolGatewayError{
			Code:    strings.TrimSpace(gatewayErr.Code),
			Message: strings.TrimSpace(gatewayErr.Message),
			Details: cloneJSONInterfaceMap(gatewayErr.Details),
		}
		if normalized.Message == "" {
			normalized.Message = fmt.Sprintf("tool %q execution failed", strings.TrimSpace(toolName))
		}
		if strings.TrimSpace(toolName) != "" {
			if normalized.Details == nil {
				normalized.Details = map[string]interface{}{}
			}
			if _, exists := normalized.Details["tool"]; !exists {
				normalized.Details["tool"] = strings.TrimSpace(toolName)
			}
		}
		return normalized
	}
	if code, details, ok := toolRuntimeErrorContract(err); ok {
		if details == nil {
			details = map[string]interface{}{}
		}
		if strings.TrimSpace(toolName) != "" {
			if _, exists := details["tool"]; !exists {
				details["tool"] = strings.TrimSpace(toolName)
			}
		}
		message := strings.TrimSpace(err.Error())
		if message == "" {
			message = fmt.Sprintf("tool %q execution failed", strings.TrimSpace(toolName))
		}
		return &ToolGatewayError{
			Code:    nonEmpty(strings.TrimSpace(code), "tool_execution_failed"),
			Message: message,
			Details: details,
		}
	}
	return &ToolGatewayError{
		Code:    "tool_execution_failed",
		Message: fmt.Sprintf("tool %q execution failed", strings.TrimSpace(toolName)),
		Details: map[string]interface{}{"tool": strings.TrimSpace(toolName), "cause": err.Error()},
	}
}

func toolRuntimeErrorContract(err error) (string, map[string]interface{}, bool) {
	if err == nil {
		return "", nil, false
	}
	var runtimeErr ToolRuntimeError
	if errors.As(err, &runtimeErr) {
		return strings.TrimSpace(runtimeErr.ToolRuntimeCode()), cloneJSONInterfaceMap(runtimeErr.ToolRuntimeDetails()), true
	}
	switch {
	case errors.Is(err, context.Canceled):
		return "tool_execution_cancelled", nil, true
	case errors.Is(err, context.DeadlineExceeded):
		return "tool_execution_timeout", nil, true
	default:
		return "", nil, false
	}
}
