package agentcore

import (
	"strings"

	sel "github.com/IceWhaleTech/ZimaOS-Blue/server/internal/selector"
)

type skillSelectorBundle struct {
	profile sel.SelectorProfile
	docText string
}

func buildSkillSelectorBundle(doc SkillDoc) skillSelectorBundle {
	profile := buildSkillSelectorProfile(doc)
	return skillSelectorBundle{
		profile: profile,
		docText: skillDocTextForIR(doc),
	}
}

func buildSkillSelectorProfile(doc SkillDoc) sel.SelectorProfile {
	name := strings.ToLower(strings.TrimSpace(doc.Name))
	human := sel.HumanizeName(name)
	profile := sel.SelectorProfile{
		Name:         doc.Name,
		ExactAliases: compactSelectorTerms(name, human),
		Objects:      compactSelectorTerms(doc.Tags...),
		ContextCues:  compactSelectorTerms(doc.Category, doc.Example),
	}

	for _, route := range doc.TaskRoutes {
		profile.ContextCues = compactSelectorTerms(append(profile.ContextCues, route.Intent, route.Action)...)
	}

	switch name {
	case "ask":
		profile.Actions = compactSelectorTerms("ask", "clarify", "confirm", "question", "询问", "澄清", "确认", "提问")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "question", "choice", "clarification", "问题", "选项", "澄清")...)
	case "browser":
		profile.Actions = compactSelectorTerms("open", "visit", "navigate", "browse", "打开", "访问", "跳转", "浏览")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "url", "web", "website", "site", "web page", "webpage", "网页", "网站", "网址")...)
		profile.ContextCues = compactSelectorTerms(append(profile.ContextCues, "http://", "https://", "www.")...)
		profile.PreferredDomains = []string{sel.DomainLiveWeb, sel.DomainURLPresent}
	case "web_search":
		profile.Actions = compactSelectorTerms("search", "look up", "lookup", "find", "check", "latest", "news", "搜索", "检索", "查找", "最新", "新闻")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "web", "news", "sources", "citations", "references", "网页", "新闻", "来源", "引用")...)
		profile.PreferredDomains = []string{sel.DomainLiveWeb}
		profile.ConflictDomains = []string{sel.DomainLocalWorkspace}
	case "deep_research":
		profile.Actions = compactSelectorTerms("research", "investigate", "compare", "analyze", "study", "timeline", "调研", "研究", "查阅", "梳理", "比较", "分析")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "sources", "citations", "evidence", "references", "views", "timeline", "来源", "引用", "证据", "观点", "时期")...)
		profile.PreferredDomains = []string{sel.DomainLiveWeb}
		profile.ConflictDomains = []string{sel.DomainLocalWorkspace}
	case "ui_reviewer":
		profile.Actions = compactSelectorTerms("review", "audit", "inspect", "evaluate", "critique", "score", "rate", "assess", "accessibility check", "评审", "审查", "检查", "点评", "打分", "评分", "无障碍检查")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "ui", "ux", "screen", "screenshot", "design", "mockup", "layout", "component", "website", "site", "webpage", "landing page", "app", "accessibility", "a11y", "visual", "界面", "截图", "设计稿", "布局", "组件", "网站", "网页", "落地页", "应用", "无障碍", "可访问性", "视觉")...)
		profile.RequireAnyDomains = []string{sel.DomainUIArtifact}
		profile.PreferredDomains = []string{sel.DomainUIArtifact}
		profile.ConflictDomains = []string{sel.DomainLocalWorkspace, sel.DomainProductivity}
	case "analyze":
		profile.Actions = compactSelectorTerms("analyze", "summarize", "compare", "synthesize", "inspect", "research", "分析", "总结", "比较", "提炼", "查看", "研究", "梳理")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "file", "report", "text", "data", "content", "url", "urls", "link", "links", "page", "pages", "website", "site", "webpage", "topic", "article", "articles", "source", "sources", "文件", "报告", "文本", "数据", "内容", "网址", "链接", "页面", "网站", "主题", "文章", "来源")...)
		profile.PreferredDomains = []string{sel.DomainLiveWeb, sel.DomainLocalWorkspace}
	case "himalaya":
		profile.Actions = compactSelectorTerms("email", "mail", "imap", "smtp", "reply", "forward", "compose", "send", "archive", "search", "triage", "download attachment", "邮件", "邮箱", "回复", "转发", "发送", "归档", "检索", "整理")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "email", "mail", "inbox", "folder", "message", "attachment", "account", "imap", "smtp", "notmuch", "maildir", "收件箱", "邮件", "附件", "账户")...)
		profile.PreferredDomains = []string{sel.DomainProductivity}
		profile.ConflictDomains = []string{sel.DomainLocalWorkspace}
	case "mgmt":
		profile.Actions = compactSelectorTerms("manage", "configure", "set", "update", "enable", "disable", "管理", "配置", "设置", "更新", "启用", "关闭")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "settings", "providers", "config", "runtime", "设置", "提供商", "配置", "运行时")...)
	case "mediagen":
		profile.Actions = compactSelectorTerms("generate", "create", "draw", "edit", "animate", "生成", "创建", "绘制", "编辑", "动画")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "image", "video", "picture", "illustration", "图片", "图像", "视频", "插画")...)
	case "reminder":
		profile.Actions = compactSelectorTerms("remind", "notify", "schedule", "提醒", "通知", "安排")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "reminder", "task", "notification", "提醒", "任务", "通知")...)
		profile.PreferredDomains = []string{sel.DomainProductivity}
	case "scheduler":
		profile.Actions = compactSelectorTerms("schedule", "cron", "run every", "every hour", "定时", "调度", "计划任务")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "schedule", "job", "workflow", "日程", "任务", "工作流")...)
		profile.PreferredDomains = []string{sel.DomainProductivity}
	case "datetime":
		profile.Actions = compactSelectorTerms("time", "date", "today", "tomorrow", "timezone", "时间", "日期", "今天", "明天", "时区")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "time", "date", "timezone", "clock", "时间", "日期", "时区")...)
	case "tasks":
		profile.Actions = compactSelectorTerms("task", "todo", "track", "manage", "任务", "待办", "跟踪", "管理")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "task", "todo", "priority", "status", "任务", "待办", "优先级", "状态")...)
		profile.PreferredDomains = []string{sel.DomainProductivity}
	case "plan_create":
		profile.ExactAliases = compactSelectorTerms(append(profile.ExactAliases, "plan_create", "plan update", "plan_update", "plan append", "plan_append")...)
		profile.Actions = compactSelectorTerms("plan", "update", "append", "checklist", "规划", "计划", "更新", "清单")
		profile.Objects = compactSelectorTerms(append(profile.Objects, "plan", "task", "checklist", "计划", "任务", "清单")...)
	}

	return profile
}

func compactSelectorTerms(terms ...string) []string {
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
