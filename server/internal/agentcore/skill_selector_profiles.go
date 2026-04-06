package agentcore

import (
	"strings"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/routingcue"
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
		ExactAliases: sel.CompactTerms(name, human),
		Objects:      sel.CompactTerms(doc.Tags...),
		ContextCues:  sel.CompactTerms(doc.Category, doc.Example),
	}
	profile, sharedPresetApplied := sel.ApplyCommonProfilePreset(profile, name)

	for _, route := range doc.TaskRoutes {
		profile.ContextCues = sel.CompactTerms(append(profile.ContextCues, route.Intent, route.Action)...)
	}

	switch name {
	case "web_query", "web_search":
		profile.Actions = sel.CompactTerms("search", "look up", "lookup", "find", "check", "latest", "news", "搜索", "检索", "查找", "最新", "新闻")
		profile.Objects = sel.CompactTerms(append(profile.Objects, "web", "news", "sources", "citations", "references", "docs", "documentation", "manual", "文档", "官方文档", "网页", "新闻", "来源", "引用")...)
		profile.PreferredDomains = []string{sel.DomainLiveWeb}
		profile.ConflictDomains = []string{sel.DomainLocalWorkspace}
	case "deep_research":
		profile.Actions = sel.CompactTerms("research", "investigate", "compare", "study", "benchmark", "timeline", "调研", "研究", "查阅", "梳理", "比较", "基准", "时间线")
		profile.Objects = sel.CompactTerms(append(profile.Objects, "sources", "citations", "evidence", "references", "multi-source", "comparison", "tradeoff", "views", "timeline", "benchmark", "来源", "引用", "证据", "多来源", "对比", "权衡", "观点", "时期", "基准")...)
		profile.PreferredDomains = []string{sel.DomainLiveWeb}
		profile.ConflictDomains = []string{sel.DomainLocalWorkspace}
	case "analyze":
		profile.Actions = sel.CompactTerms("analyze", "summarize", "compare", "synthesize", "inspect", "review", "report", "分析", "总结", "比较", "提炼", "查看", "归纳", "报告")
		profile.Objects = sel.CompactTerms(append(profile.Objects, "report", "text", "data", "content", "url", "urls", "link", "links", "page", "pages", "website", "site", "webpage", "topic", "article", "articles", "document", "documents", "http://", "https://", "www.", "报告", "文本", "数据", "内容", "网址", "链接", "页面", "网站", "主题", "文章", "文档")...)
		profile.PreferredDomains = []string{sel.DomainLiveWeb}
		profile.ConflictDomains = []string{sel.DomainLocalWorkspace}
	case "himalaya":
		profile.Actions = sel.CompactTerms("email", "mail", "imap", "smtp", "reply", "forward", "compose", "send", "archive", "search", "triage", "download attachment", "邮件", "邮箱", "回复", "转发", "发送", "归档", "检索", "整理")
		profile.Objects = sel.CompactTerms(append(profile.Objects, "email", "mail", "inbox", "folder", "message", "attachment", "account", "imap", "smtp", "notmuch", "maildir", "收件箱", "邮件", "附件", "账户")...)
		profile.PreferredDomains = []string{sel.DomainProductivity}
		profile.ConflictDomains = []string{sel.DomainLocalWorkspace}
	case "config":
		profile.Actions = sel.CompactTerms("manage", "configure", "set", "update", "enable", "disable", "管理", "配置", "设置", "更新", "启用", "关闭")
		profile.Objects = sel.CompactTerms(append(profile.Objects, "settings", "providers", "config", "runtime", "设置", "提供商", "配置", "运行时")...)
	case "mediagen":
		profile.Actions = sel.CompactTerms("generate", "create", "draw", "edit", "animate", "生成", "创建", "绘制", "编辑", "动画")
		profile.Objects = sel.CompactTerms(append(profile.Objects, "image", "video", "picture", "illustration", "图片", "图像", "视频", "插画")...)
	case "reminder":
		profile.Actions = sel.CompactTerms(append(profile.Actions, "schedule", "安排")...)
	case "scheduler":
		profile.Objects = sel.CompactTerms(append(profile.Objects, "任务")...)
	case "plan_create":
		profile.ExactAliases = sel.CompactTerms(append(profile.ExactAliases, "plan_create", "plan update", "plan_update", "plan append", "plan_append")...)
		profile.Actions = sel.CompactTerms("plan", "update", "append", "checklist", "规划", "计划", "更新", "清单")
		profile.Objects = sel.CompactTerms(append(profile.Objects, "plan", "task", "checklist", "计划", "任务", "清单")...)
	default:
		if !sharedPresetApplied {
			// No shared preset or package-specific override; keep the base profile.
		}
	}

	if terms := routingcue.SkillTerms(name); len(terms.Actions) > 0 || len(terms.Objects) > 0 || len(terms.Context) > 0 || len(terms.Examples) > 0 {
		profile.ExactAliases = sel.CompactTerms(append(profile.ExactAliases, terms.Examples...)...)
		profile.Actions = sel.CompactTerms(append(profile.Actions, terms.Actions...)...)
		profile.Objects = sel.CompactTerms(append(profile.Objects, terms.Objects...)...)
		profile.ContextCues = sel.CompactTerms(append(profile.ContextCues, terms.Context...)...)
	}
	if terms := routingcue.URLBypassTermsForSkill(name); len(terms) > 0 {
		profile.ExactAliases = sel.CompactTerms(append(profile.ExactAliases, terms...)...)
		profile.Actions = sel.CompactTerms(append(profile.Actions, terms...)...)
		profile.Objects = sel.CompactTerms(append(profile.Objects, terms...)...)
		profile.ContextCues = sel.CompactTerms(append(profile.ContextCues, terms...)...)
	}

	return profile
}
