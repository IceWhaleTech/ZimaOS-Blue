package tools

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
)

func buildToolSelectorProfile(def ToolDefinition) selector.SelectorProfile {
	name := strings.ToLower(strings.TrimSpace(def.Name))
	human := selector.HumanizeName(name)
	base := selector.SelectorProfile{
		Name:         def.Name,
		ExactAliases: compactTerms(name, human),
		ContextCues:  compactTerms(human),
	}

	switch name {
	case "exec":
		base.Actions = compactTerms("run", "execute", "shell", "command", "bash", "terminal", "script", "执行", "运行", "命令", "终端")
		base.Objects = compactTerms("command", "shell", "terminal", "script", "process", "session", "命令", "终端", "脚本", "进程")
	case "ask":
		base.Actions = compactTerms("ask", "clarify", "confirm", "question", "询问", "澄清", "确认", "提问")
		base.Objects = compactTerms("question", "choice", "preference", "clarification", "问题", "选项", "偏好", "澄清")
	case "browser":
		base.Actions = compactTerms("open", "visit", "navigate", "browse", "打开", "访问", "跳转", "浏览")
		base.Objects = compactTerms("url", "web", "website", "site", "web page", "webpage", "网页", "网站", "网址")
		base.ContextCues = compactTerms(append(base.ContextCues, "http://", "https://", "www.")...)
		base.PreferredDomains = []string{selector.DomainLiveWeb, selector.DomainURLPresent}
	case "web_search", "search":
		base.Actions = compactTerms("search", "look up", "lookup", "find", "check", "latest", "news", "搜索", "检索", "查找", "最新", "新闻")
		base.Objects = compactTerms("web", "news", "source", "sources", "citation", "citations", "reference", "references", "网页", "新闻", "来源", "引用", "参考")
		base.PreferredDomains = []string{selector.DomainLiveWeb}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "research_run":
		base.Actions = compactTerms("research", "investigate", "compare", "analyze", "study", "调研", "研究", "查阅", "梳理", "比较", "分析")
		base.Objects = compactTerms("sources", "citations", "evidence", "references", "views", "timeline", "来源", "引用", "证据", "观点", "时期")
		base.PreferredDomains = []string{selector.DomainLiveWeb}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "research_status":
		base.Actions = compactTerms("research status", "status", "job status", "状态", "进度")
		base.Objects = compactTerms("research", "job", "task", "调研", "任务")
	case "read", "files":
		base.Actions = compactTerms("read", "open", "inspect", "review", "查看", "读取", "打开", "检查")
		base.Objects = compactTerms("file", "files", "content", "contents", "document", "folder", "directory", "path", "repo", "repository", "文件", "内容", "文档", "目录", "路径", "仓库")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "ls":
		base.Actions = compactTerms("list", "show", "browse", "discover", "列出", "浏览", "查看")
		base.Objects = compactTerms("files", "folders", "directories", "workspace", "repo", "文件", "文件夹", "目录", "工作区", "仓库")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "find":
		base.Actions = compactTerms("find", "search", "grep", "locate", "查找", "搜索", "定位", "检索")
		base.Objects = compactTerms("files", "text", "pattern", "keyword", "code", "workspace", "文件", "文本", "关键词", "代码", "工作区")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "write":
		base.Actions = compactTerms("write", "save", "create", "draft", "export", "append", "写", "写入", "保存", "创建", "导出")
		base.Objects = compactTerms("file", "report", "summary", "markdown", "document", "文件", "报告", "摘要", "文档")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "convert":
		base.Actions = compactTerms("convert", "parse", "extract", "import", "export", "转换", "解析", "提取", "导入", "导出")
		base.Objects = compactTerms("csv", "xlsx", "xls", "spreadsheet", "table", "sheet", "表格", "工作表", "电子表格")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "analyze":
		base.Actions = compactTerms("analyze", "summarize", "compare", "review", "inspect", "分析", "总结", "比较", "评估", "查看")
		base.Objects = compactTerms("file", "report", "text", "data", "content", "document", "文件", "报告", "文本", "数据", "内容", "文档")
	case "ui_reviewer":
		base.Actions = compactTerms("review", "audit", "inspect", "evaluate", "评审", "审查", "检查")
		base.Objects = compactTerms("ui", "ux", "screen", "screenshot", "design", "mockup", "layout", "component", "界面", "截图", "设计稿", "布局", "组件")
		base.RequireAnyDomains = []string{selector.DomainUIArtifact}
		base.PreferredDomains = []string{selector.DomainUIArtifact}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace, selector.DomainProductivity}
	case "email":
		base.Actions = compactTerms("email", "emails", "reply", "archive", "triage", "send", "邮件", "回复", "归档", "整理", "发送")
		base.Objects = compactTerms("email", "emails", "mail", "mails", "inbox", "message", "messages", "sender", "unread", "收件箱", "邮箱", "邮件", "消息", "发件人", "未读")
		base.PreferredDomains = []string{selector.DomainProductivity}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "calendar":
		base.Actions = compactTerms("calendar", "schedule", "book", "plan", "安排", "日程", "计划", "预定")
		base.Objects = compactTerms("calendar", "meeting", "meetings", "event", "events", "agenda", "appointment", "appointments", "日历", "会议", "事件", "议程", "预约")
		base.PreferredDomains = []string{selector.DomainProductivity}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "reminder":
		base.Actions = compactTerms("remind", "reminder", "notify", "提醒", "通知")
		base.Objects = compactTerms("reminder", "notification", "task", "提醒", "通知", "任务")
		base.PreferredDomains = []string{selector.DomainProductivity}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "weather":
		base.Actions = compactTerms("weather", "forecast", "temperature", "rain", "天气", "气温", "温度", "预报")
		base.Objects = compactTerms("weather", "forecast", "temperature", "conditions", "天气", "气温", "温度", "预报")
	case "datetime":
		base.Actions = compactTerms("time", "date", "today", "tomorrow", "timezone", "时间", "日期", "今天", "明天", "时区")
		base.Objects = compactTerms("time", "date", "timezone", "clock", "时间", "日期", "时区")
	case "calculator":
		base.Actions = compactTerms("calculate", "compute", "math", "算", "计算", "换算")
		base.Objects = compactTerms("sum", "tip", "percentage", "percent", "equation", "arithmetic", "加减乘除", "百分比", "算式", "数学")
	case "network":
		base.Actions = compactTerms("network", "ping", "trace", "diagnose", "check", "网络", "诊断", "检查", "连通性")
		base.Objects = compactTerms("network", "dns", "connectivity", "latency", "网络", "dns", "连接", "延迟", "解析")
	case "docker":
		base.Actions = compactTerms("docker", "container", "build", "run", "list", "容器", "镜像")
		base.Objects = compactTerms("docker", "container", "containers", "image", "images", "volume", "容器", "镜像", "卷")
	case "translate":
		base.Actions = compactTerms("translate", "translation", "翻译")
		base.Objects = compactTerms("translate", "language", "text", "translation", "语言", "文本", "翻译")
	case "system_info":
		base.Actions = compactTerms("show", "inspect", "check", "system", "查看", "检查", "展示", "系统")
		base.Objects = compactTerms("system", "hardware", "cpu", "memory", "gpu", "os", "机器", "硬件", "cpu", "内存", "gpu", "系统")
	case "notes":
		base.Actions = compactTerms("note", "notes", "write", "create", "search", "笔记", "记录", "创建", "搜索")
		base.Objects = compactTerms("note", "notes", "document", "tag", "笔记", "文档", "标签")
	case "tasks":
		base.Actions = compactTerms("task", "todo", "track", "manage", "任务", "待办", "跟踪", "管理")
		base.Objects = compactTerms("task", "todo", "priority", "status", "任务", "待办", "优先级", "状态")
		base.PreferredDomains = []string{selector.DomainProductivity}
	case "scheduler":
		base.Actions = compactTerms("schedule", "cron", "run every", "every hour", "定时", "调度", "计划任务")
		base.Objects = compactTerms("schedule", "cron", "job", "workflow", "日程", "定时任务", "工作流")
		base.PreferredDomains = []string{selector.DomainProductivity}
	case "notifications":
		base.Actions = compactTerms("notify", "notification", "alert", "提醒", "通知", "告警")
		base.Objects = compactTerms("notification", "alert", "message", "通知", "消息", "告警")
		base.PreferredDomains = []string{selector.DomainProductivity}
	case "unit_converter":
		base.Actions = compactTerms("convert", "convert units", "换算", "转换")
		base.Objects = compactTerms("unit", "temperature", "length", "weight", "单位", "温度", "长度", "重量")
	case "workflows":
		base.Actions = compactTerms("workflow", "automation", "automate", "automated", "orchestrate", "工作流", "自动化", "编排")
		base.Objects = compactTerms("workflow", "automation", "automated", "trigger", "job", "工作流", "自动化", "触发器", "任务")
	case "memory":
		base.Actions = compactTerms("memory", "recall", "remember", "search", "记忆", "回忆", "检索")
		base.Objects = compactTerms("memory", "history", "context", "记忆", "历史", "上下文")
	default:
		base.Actions = compactTerms(def.Name)
		base.Objects = compactTerms(human, selector.HumanizeName(def.Description))
	}

	if len(base.ContextCues) == 0 {
		base.ContextCues = compactTerms(human)
	}
	return base
}

func buildToolSelectorDoc(def ToolDefinition, profile selector.SelectorProfile) string {
	parts := []string{def.Name, def.Description}
	parts = append(parts, profile.ExactAliases...)
	parts = append(parts, profile.Actions...)
	parts = append(parts, profile.Objects...)
	parts = append(parts, profile.ContextCues...)
	return strings.Join(parts, " ")
}

func compactTerms(terms ...string) []string {
	seen := make(map[string]struct{}, len(terms))
	out := make([]string, 0, len(terms))
	for _, term := range terms {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			continue
		}
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}
	return out
}
