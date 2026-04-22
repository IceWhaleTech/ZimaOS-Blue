package tools

import (
	"fmt"
	"net/url"
	"path/filepath"
	"sort"
	"strings"
)

// ApprovalPresentation contains human-readable approval metadata for dialogs.
type ApprovalPresentation struct {
	Purpose         string   `json:"purpose,omitempty"`
	RiskSummary     string   `json:"risk_summary,omitempty"`
	ScopeSummary    string   `json:"scope_summary,omitempty"`
	ExpectedEffects string   `json:"expected_effects,omitempty"`
	AffectedTargets []string `json:"affected_targets,omitempty"`
}

func buildToolApprovalPresentation(req ToolApprovalRequest, lang string) ApprovalPresentation {
	return BuildToolApprovalPresentation(req, lang)
}

func BuildToolApprovalPresentation(req ToolApprovalRequest, lang string) ApprovalPresentation {
	zh := approvalUsesChinese(lang)
	targets := approvalTargetsFromArgs(req.Arguments)
	toolName := strings.TrimSpace(req.ToolName)
	lowerTool := strings.ToLower(toolName)

	purpose := ifApprovalText(zh,
		fmt.Sprintf("允许工具 %s 继续当前任务", toolName),
		fmt.Sprintf("Allow tool %s to continue the current task", toolName),
	)
	expected := ifApprovalText(zh, "可能会继续读取或处理当前任务所需的数据。", "May continue reading or processing task data.")
	switch {
	case strings.Contains(lowerTool, "write"):
		purpose = ifApprovalText(zh, "允许写入本地文件", "Allow tool to write a local file")
		expected = ifApprovalText(zh, "可能会创建新文件或修改现有文件内容。", "May create new files or modify existing file content.")
	case strings.Contains(lowerTool, "read"):
		purpose = ifApprovalText(zh, "允许读取本地文件", "Allow reading local files")
		expected = ifApprovalText(zh, "会读取目标文件内容，但不应写入文件。", "Reads target files without writing to them.")
	case strings.Contains(lowerTool, "browser"), strings.Contains(lowerTool, "web"):
		purpose = ifApprovalText(zh, "允许访问网页内容", "Allow accessing web content")
		expected = ifApprovalText(zh, "可能会联网访问页面并读取网页内容。", "May access external pages over the network and read web content.")
	case strings.Contains(lowerTool, "search"):
		purpose = ifApprovalText(zh, "允许执行搜索查询", "Allow running a search query")
		expected = ifApprovalText(zh, "可能会向外部搜索服务发送查询请求。", "May send queries to external search services.")
	}

	scope := ifApprovalText(zh, "影响范围：当前任务参数。", "Scope: current task arguments.")
	if len(targets) > 0 {
		scope = ifApprovalText(
			zh,
			"影响目标："+strings.Join(targets, "、"),
			"Targets: "+strings.Join(targets, ", "),
		)
	}

	return ApprovalPresentation{
		Purpose:         purpose,
		RiskSummary:     approvalRiskSummary(req.RiskLevel, zh, false, ""),
		ScopeSummary:    scope,
		ExpectedEffects: expected,
		AffectedTargets: targets,
	}
}

func buildExecApprovalPresentation(req ApprovalRequest, lang string) ApprovalPresentation {
	return BuildExecApprovalPresentation(req, lang)
}

func BuildExecApprovalPresentation(req ApprovalRequest, lang string) ApprovalPresentation {
	zh := approvalUsesChinese(lang)
	targets := approvalExecTargets(req)
	purpose := ifApprovalText(zh, "允许继续本次执行操作", "Allow this execution step to continue")
	scope := ifApprovalText(zh, "影响范围：当前执行上下文。", "Scope: current execution context.")
	expected := ifApprovalText(zh, "可能会启动进程、访问目录或读取上下文文件。", "May start processes, access directories, or read context files.")

	switch strings.ToLower(strings.TrimSpace(req.Type)) {
	case "command":
		purpose = ifApprovalText(zh, "允许运行当前命令", "Allow running the requested command")
		if strings.TrimSpace(req.Workdir) != "" {
			scope = ifApprovalText(zh, "工作目录："+strings.TrimSpace(req.Workdir), "Working directory: "+strings.TrimSpace(req.Workdir))
		}
	case "directory":
		purpose = ifApprovalText(zh, "允许访问额外目录", "Allow accessing an additional directory")
		if strings.TrimSpace(req.Directory) != "" {
			scope = ifApprovalText(zh, "目标目录："+strings.TrimSpace(req.Directory), "Directory: "+strings.TrimSpace(req.Directory))
		}
	}

	if security := strings.ToLower(strings.TrimSpace(req.Security)); security != "" {
		switch {
		case strings.Contains(security, "filesystem-write") && strings.Contains(security, "network"):
			expected = ifApprovalText(zh, "可能会写入文件并访问网络资源。", "May write files and access network resources.")
		case strings.Contains(security, "filesystem-write"):
			expected = ifApprovalText(zh, "可能会创建、修改或覆盖文件。", "May create, modify, or overwrite files.")
		case strings.Contains(security, "network"):
			expected = ifApprovalText(zh, "可能会访问外部网络或远程服务。", "May access external networks or remote services.")
		}
	}

	if len(targets) > 0 {
		scope = ifApprovalText(
			zh,
			"影响目标："+strings.Join(targets, "、"),
			"Targets: "+strings.Join(targets, ", "),
		)
	}

	return ApprovalPresentation{
		Purpose:         purpose,
		RiskSummary:     approvalRiskSummary(req.RiskLevel, zh, true, req.Security),
		ScopeSummary:    scope,
		ExpectedEffects: expected,
		AffectedTargets: targets,
	}
}

func approvalUsesChinese(lang string) bool {
	lang = strings.ToLower(strings.TrimSpace(lang))
	return strings.HasPrefix(lang, "zh")
}

func ifApprovalText(zh bool, zhText, enText string) string {
	if zh {
		return zhText
	}
	return enText
}

func approvalRiskSummary(riskLevel string, zh bool, exec bool, security string) string {
	switch strings.ToLower(strings.TrimSpace(riskLevel)) {
	case "high":
		if exec {
			return ifApprovalText(zh, "高风险：这次执行可能写入文件、联网或启动子进程。", "High risk: this execution may write files, access the network, or launch subprocesses.")
		}
		return ifApprovalText(zh, "高风险：这次工具调用可能修改数据或访问敏感资源。", "High risk: this tool call may modify data or access sensitive resources.")
	case "medium":
		if exec {
			return ifApprovalText(zh, "中风险：请确认命令的目录和副作用符合预期。", "Medium risk: confirm the command scope and side effects before proceeding.")
		}
		return ifApprovalText(zh, "中风险：请确认这次工具调用的参数和目标。", "Medium risk: confirm the tool arguments and targets before proceeding.")
	default:
		if exec && strings.TrimSpace(security) != "" {
			return ifApprovalText(zh, "已标记执行能力，请确认影响范围后继续。", "Execution capabilities were detected; confirm the scope before continuing.")
		}
		return ifApprovalText(zh, "请确认本次操作确实是你希望继续的步骤。", "Confirm that this is the step you want to continue.")
	}
}

func approvalTargetsFromArgs(args map[string]interface{}) []string {
	if len(args) == 0 {
		return nil
	}
	keys := []string{
		"path", "paths", "file", "files", "filepath", "directory", "dir",
		"url", "urls", "uri", "host", "origin", "target",
	}
	targets := make([]string, 0, 4)
	for _, key := range keys {
		value, ok := args[key]
		if !ok {
			continue
		}
		targets = append(targets, approvalTargetsFromValue(value)...)
	}
	return normalizeApprovalPresentationTargets(targets)
}

func approvalTargetsFromValue(value interface{}) []string {
	switch v := value.(type) {
	case string:
		return approvalTargetsFromString(v)
	case []string:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, approvalTargetsFromString(item)...)
		}
		return out
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			out = append(out, approvalTargetsFromValue(item)...)
		}
		return out
	default:
		return nil
	}
}

func approvalTargetsFromString(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		return []string{parsed.String()}
	}
	if strings.Contains(trimmed, "/") || strings.HasPrefix(trimmed, ".") {
		if abs, err := filepath.Abs(trimmed); err == nil {
			return []string{filepath.Clean(abs)}
		}
		return []string{filepath.Clean(trimmed)}
	}
	return []string{trimmed}
}

func approvalExecTargets(req ApprovalRequest) []string {
	targets := make([]string, 0, len(req.ReferencedPaths)+3)
	if strings.TrimSpace(req.Directory) != "" {
		targets = append(targets, strings.TrimSpace(req.Directory))
	}
	if strings.TrimSpace(req.Workdir) != "" {
		targets = append(targets, strings.TrimSpace(req.Workdir))
	}
	if strings.TrimSpace(req.Host) != "" {
		targets = append(targets, strings.TrimSpace(req.Host))
	}
	targets = append(targets, req.ReferencedPaths...)
	return normalizeApprovalPresentationTargets(targets)
}

func normalizeApprovalPresentationTargets(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
			trimmed = parsed.String()
		} else if strings.Contains(trimmed, "/") || strings.HasPrefix(trimmed, ".") {
			if abs, err := filepath.Abs(trimmed); err == nil {
				trimmed = filepath.Clean(abs)
			} else {
				trimmed = filepath.Clean(trimmed)
			}
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}
