package tools

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
)

type toolSelectorBundle struct {
	profile selector.SelectorProfile
	docText string
}

func buildToolSelectorBundle(def ToolDefinition) toolSelectorBundle {
	profile := buildToolSelectorProfile(def)
	return toolSelectorBundle{
		profile: profile,
		docText: buildToolSelectorDoc(def, profile),
	}
}

func buildToolSelectorProfile(def ToolDefinition) selector.SelectorProfile {
	name := strings.ToLower(strings.TrimSpace(def.Name))
	human := selector.HumanizeName(name)
	base := selector.SelectorProfile{
		Name:         def.Name,
		ExactAliases: selector.CompactTerms(name, human),
		ContextCues:  selector.CompactTerms(human),
	}
	base, sharedPresetApplied := selector.ApplyCommonProfilePreset(base, name)

	switch name {
	case "bash", "exec":
		base.ExactAliases = selector.CompactTerms(append(base.ExactAliases, "bash", "exec")...)
		base.Actions = selector.CompactTerms("run", "execute", "shell", "command", "bash", "terminal", "script", "执行", "运行", "命令", "终端")
		base.Objects = selector.CompactTerms("command", "shell", "terminal", "script", "process", "session", "命令", "终端", "脚本", "进程")
	case "ask":
		base.Objects = selector.CompactTerms(append(base.Objects, "preference", "偏好")...)
	case "web", "web_query", "web_search", "search":
		base.Actions = selector.CompactTerms("search", "look up", "lookup", "find", "check", "latest", "news", "搜索", "检索", "查找", "最新", "新闻")
		base.Objects = selector.CompactTerms("web", "url", "page", "site", "news", "source", "sources", "citation", "citations", "reference", "references", "网页", "网站", "网址", "新闻", "来源", "引用", "参考")
		base.PreferredDomains = []string{selector.DomainLiveWeb}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "deep_research", "research_run", "research_status":
		base.Actions = selector.CompactTerms("research", "investigate", "compare", "analyze", "study", "调研", "研究", "查阅", "梳理", "比较", "分析")
		base.Objects = selector.CompactTerms("sources", "citations", "evidence", "references", "views", "timeline", "status", "progress", "job", "来源", "引用", "证据", "观点", "时期", "状态", "进度", "任务")
		base.PreferredDomains = []string{selector.DomainLiveWeb}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "sessions":
		base.Actions = selector.CompactTerms("session", "sessions", "conversation", "history", "send", "spawn", "会话", "对话", "历史", "发送", "创建")
		base.Objects = selector.CompactTerms("session", "conversation", "message", "messages", "history", "thread", "会话", "对话", "消息", "历史", "线程")
	case "read", "file_read", "files":
		base.Actions = selector.CompactTerms("read", "open", "inspect", "review", "查看", "读取", "打开", "检查")
		base.Objects = selector.CompactTerms("file", "files", "content", "contents", "document", "folder", "directory", "path", "repo", "repository", "文件", "内容", "文档", "目录", "路径", "仓库")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "ls":
		base.Actions = selector.CompactTerms("list", "show", "browse", "discover", "列出", "浏览", "查看")
		base.Objects = selector.CompactTerms("files", "folders", "directories", "workspace", "repo", "文件", "文件夹", "目录", "工作区", "仓库")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "find":
		base.Actions = selector.CompactTerms("find", "search", "grep", "locate", "查找", "搜索", "定位", "检索")
		base.Objects = selector.CompactTerms("files", "text", "pattern", "keyword", "code", "workspace", "文件", "文本", "关键词", "代码", "工作区")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "grep", "rg":
		base.ExactAliases = selector.CompactTerms(append(base.ExactAliases, "grep", "rg", "ripgrep")...)
		base.Actions = selector.CompactTerms("grep", "rg", "ripgrep", "search", "scan", "match", "查找", "搜索", "检索", "匹配")
		base.Objects = selector.CompactTerms("text", "content", "pattern", "regex", "keyword", "code", "workspace", "文本", "内容", "模式", "正则", "关键词", "代码", "工作区")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "write", "file_write":
		base.Actions = selector.CompactTerms("write", "save", "create", "draft", "export", "append", "写", "写入", "保存", "创建", "导出")
		base.Objects = selector.CompactTerms("file", "report", "summary", "markdown", "document", "文件", "报告", "摘要", "文档")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "delete", "remove", "rm", "unlink", "file_delete":
		base.Actions = selector.CompactTerms("delete", "remove", "clean", "cleanup", "trash", "删", "删除", "移除", "清理")
		base.Objects = selector.CompactTerms("file", "files", "directory", "folder", "artifact", "workspace", "文件", "目录", "文件夹", "产物", "工作区")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "convert":
		base.Actions = selector.CompactTerms("convert", "parse", "extract", "import", "export", "转换", "解析", "提取", "导入", "导出")
		base.Objects = selector.CompactTerms("csv", "xlsx", "xls", "spreadsheet", "table", "sheet", "表格", "工作表", "电子表格")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "office":
		base.Actions = selector.CompactTerms("create", "generate", "export", "format", "layout", "style", "render", "写", "生成", "导出", "排版", "美化", "格式化")
		base.Objects = selector.CompactTerms("xlsx", "xls", "docx", "doc", "excel", "word", "spreadsheet", "workbook", "worksheet", "report", "document", "table", "sheet", "表格", "工作簿", "工作表", "报告", "文档", "Excel", "Word")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "pdf":
		base.Actions = selector.CompactTerms("read", "extract", "parse", "summarize", "answer", "review", "读取", "提取", "解析", "总结", "回答", "查看")
		base.Objects = selector.CompactTerms("pdf", "document", "paper", "report", "scan", "pages", "table", "pdf文档", "文档", "报告", "扫描件", "页面")
		base.PreferredDomains = []string{selector.DomainLocalWorkspace}
		base.ConflictDomains = []string{selector.DomainLiveWeb}
	case "image", "image_generation", "generate_image", "generateimage":
		base.Actions = selector.CompactTerms("generate", "create", "draw", "edit", "render", "review", "analyze", "compare", "生成", "创建", "绘制", "编辑", "渲染", "看图", "分析图片", "对比图片")
		base.Objects = selector.CompactTerms("image", "images", "picture", "photo", "art", "scene", "illustration", "logo", "screenshot", "png", "jpg", "jpeg", "webp", "图片", "图像", "照片", "插画", "场景", "logo", "截图")
	case "analyze":
		base.Actions = selector.CompactTerms("analyze", "summarize", "compare", "synthesize", "inspect", "research", "分析", "总结", "比较", "提炼", "评估", "研究", "梳理")
		base.Objects = selector.CompactTerms("file", "report", "text", "data", "content", "document", "url", "urls", "link", "links", "page", "pages", "website", "site", "webpage", "topic", "article", "articles", "source", "sources", "文件", "报告", "文本", "数据", "内容", "文档", "网址", "链接", "页面", "网站", "主题", "文章", "来源")
		base.PreferredDomains = []string{selector.DomainLiveWeb, selector.DomainLocalWorkspace}
	case "email":
		base.Actions = selector.CompactTerms("email", "emails", "reply", "archive", "triage", "send", "邮件", "回复", "归档", "整理", "发送")
		base.Objects = selector.CompactTerms("email", "emails", "mail", "mails", "inbox", "message", "messages", "sender", "unread", "收件箱", "邮箱", "邮件", "消息", "发件人", "未读")
		base.PreferredDomains = []string{selector.DomainProductivity}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "calendar":
		base.Actions = selector.CompactTerms("calendar", "schedule", "book", "plan", "安排", "日程", "计划", "预定")
		base.Objects = selector.CompactTerms("calendar", "meeting", "meetings", "event", "events", "agenda", "appointment", "appointments", "日历", "会议", "事件", "议程", "预约")
		base.PreferredDomains = []string{selector.DomainProductivity}
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "reminder":
		base.Actions = selector.CompactTerms(append(base.Actions, "reminder")...)
		base.ConflictDomains = []string{selector.DomainLocalWorkspace}
	case "weather":
		base.Actions = selector.CompactTerms("weather", "forecast", "temperature", "rain", "天气", "气温", "温度", "预报")
		base.Objects = selector.CompactTerms("weather", "forecast", "temperature", "conditions", "天气", "气温", "温度", "预报")
	case "calculator":
		base.Actions = selector.CompactTerms("calculate", "compute", "math", "算", "计算", "换算")
		base.Objects = selector.CompactTerms("sum", "tip", "percentage", "percent", "equation", "arithmetic", "加减乘除", "百分比", "算式", "数学")
	case "network":
		base.Actions = selector.CompactTerms("network", "ping", "trace", "diagnose", "check", "网络", "诊断", "检查", "连通性")
		base.Objects = selector.CompactTerms("network", "dns", "connectivity", "latency", "网络", "dns", "连接", "延迟", "解析")
	case "docker":
		base.Actions = selector.CompactTerms("docker", "container", "build", "run", "list", "容器", "镜像")
		base.Objects = selector.CompactTerms("docker", "container", "containers", "image", "images", "volume", "容器", "镜像", "卷")
	case "translate":
		base.Actions = selector.CompactTerms("translate", "translation", "翻译")
		base.Objects = selector.CompactTerms("translate", "language", "text", "translation", "语言", "文本", "翻译")
	case "system_info":
		base.Actions = selector.CompactTerms("show", "inspect", "check", "system", "查看", "检查", "展示", "系统")
		base.Objects = selector.CompactTerms("system", "hardware", "cpu", "memory", "gpu", "os", "机器", "硬件", "cpu", "内存", "gpu", "系统")
	case "notes":
		base.Actions = selector.CompactTerms("note", "notes", "write", "create", "search", "笔记", "记录", "创建", "搜索")
		base.Objects = selector.CompactTerms("note", "notes", "document", "tag", "笔记", "文档", "标签")
	case "scheduler":
		base.Objects = selector.CompactTerms(append(base.Objects, "cron", "定时任务")...)
	case "notifications":
		base.Actions = selector.CompactTerms("notify", "notification", "alert", "提醒", "通知", "告警")
		base.Objects = selector.CompactTerms("notification", "alert", "message", "通知", "消息", "告警")
		base.PreferredDomains = []string{selector.DomainProductivity}
	case "unit_converter":
		base.Actions = selector.CompactTerms("convert", "convert units", "换算", "转换")
		base.Objects = selector.CompactTerms("unit", "temperature", "length", "weight", "单位", "温度", "长度", "重量")
	case "workflows":
		base.Actions = selector.CompactTerms("workflow", "automation", "automate", "automated", "orchestrate", "工作流", "自动化", "编排")
		base.Objects = selector.CompactTerms("workflow", "automation", "automated", "trigger", "job", "工作流", "自动化", "触发器", "任务")
	case "memory":
		base.Actions = selector.CompactTerms("memory", "recall", "remember", "search", "记忆", "回忆", "检索")
		base.Objects = selector.CompactTerms("memory", "history", "context", "记忆", "历史", "上下文")
	default:
		if !sharedPresetApplied {
			base.Actions = selector.CompactTerms(def.Name)
			base.Objects = selector.CompactTerms(human, selector.HumanizeName(def.Description))
		}
	}

	if len(base.ContextCues) == 0 {
		base.ContextCues = selector.CompactTerms(human)
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

func toolSelectorBundleCacheKey(def ToolDefinition) string {
	var b strings.Builder
	b.WriteString(strings.ToLower(strings.TrimSpace(def.Name)))
	b.WriteByte('\x1f')
	b.WriteString(strings.ToLower(strings.TrimSpace(def.Description)))
	return b.String()
}
