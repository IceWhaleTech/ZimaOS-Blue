package preview

// PresetQuestionAttachment represents an attachment for a preset question.
type PresetQuestionAttachment struct {
	Type        string `json:"type"`        // "image" or "file"
	Name        string `json:"name"`        // filename
	MimeType    string `json:"mime_type"`   // MIME type
	Placeholder string `json:"placeholder"` // reserved for legacy preview payload compatibility
}

// PresetQuestion represents a curated preset question.
type PresetQuestion struct {
	ID             string                     `json:"id"`
	Title          string                     `json:"title,omitempty"`
	Description    string                     `json:"description,omitempty"`
	Prompt         string                     `json:"prompt,omitempty"`
	Text           string                     `json:"text"`
	TextKey        string                     `json:"text_key,omitempty"` // i18n key for frontend translation
	Category       string                     `json:"category"`
	Tags           []string                   `json:"tags,omitempty"`
	Icon           string                     `json:"icon,omitempty"`
	EditorialScore int                        `json:"editorial_score,omitempty"`
	Attachments    []PresetQuestionAttachment `json:"attachments,omitempty"`
}

// QuestionsService provides preset questions for preview-related UI.
type QuestionsService struct {
	questions map[string][]PresetQuestion
}

// NewQuestionsService creates a new QuestionsService.
func NewQuestionsService() *QuestionsService {
	return &QuestionsService{
		questions: defaultPresetQuestions(),
	}
}

// GetPresetQuestions returns the leading preset questions for a language.
func (s *QuestionsService) GetPresetQuestions(count int, lang string) []PresetQuestion {
	if count <= 0 {
		return []PresetQuestion{}
	}

	questions := s.questionsForLang(lang)
	if count > len(questions) {
		count = len(questions)
	}
	return clonePresetQuestions(questions[:count])
}

// GetPresetQuestionPage returns a deterministic slice of preset questions for incremental loading.
func (s *QuestionsService) GetPresetQuestionPage(offset, count int, lang string) ([]PresetQuestion, int) {
	questions := s.questionsForLang(lang)
	total := len(questions)
	if offset < 0 {
		offset = 0
	}
	if offset >= total || count <= 0 {
		return []PresetQuestion{}, total
	}

	end := offset + count
	if end > total {
		end = total
	}
	return clonePresetQuestions(questions[offset:end]), total
}

// GetAllQuestions returns all preset questions for a language.
func (s *QuestionsService) GetAllQuestions(lang string) []PresetQuestion {
	return clonePresetQuestions(s.questionsForLang(lang))
}

// GetQuestionsByCategory returns questions filtered by category.
func (s *QuestionsService) GetQuestionsByCategory(category string, lang string) []PresetQuestion {
	questions := s.questionsForLang(lang)
	filtered := make([]PresetQuestion, 0, len(questions))
	for _, question := range questions {
		if question.Category == category {
			filtered = append(filtered, clonePresetQuestion(question))
		}
	}
	return filtered
}

func (s *QuestionsService) questionsForLang(lang string) []PresetQuestion {
	if s == nil || len(s.questions) == 0 {
		return nil
	}
	if questions, ok := s.questions[lang]; ok {
		return questions
	}
	return s.questions["en"]
}

func clonePresetQuestions(questions []PresetQuestion) []PresetQuestion {
	cloned := make([]PresetQuestion, len(questions))
	for i, question := range questions {
		cloned[i] = clonePresetQuestion(question)
	}
	return cloned
}

func clonePresetQuestion(question PresetQuestion) PresetQuestion {
	cloned := question
	if question.Tags != nil {
		cloned.Tags = append([]string(nil), question.Tags...)
	}
	if question.Attachments != nil {
		cloned.Attachments = append([]PresetQuestionAttachment(nil), question.Attachments...)
	}
	return cloned
}

func defaultPresetQuestions() map[string][]PresetQuestion {
	return map[string][]PresetQuestion{
		"en": englishPresetQuestions(),
		"zh": chinesePresetQuestions(),
	}
}

func curatedPresetQuestion(
	id string,
	title string,
	description string,
	prompt string,
	category string,
	tags []string,
	icon string,
	editorialScore int,
) PresetQuestion {
	return PresetQuestion{
		ID:             id,
		Title:          title,
		Description:    description,
		Prompt:         prompt,
		Text:           prompt,
		Category:       category,
		Tags:           tags,
		Icon:           icon,
		EditorialScore: editorialScore,
	}
}

func englishPresetQuestions() []PresetQuestion {
	return []PresetQuestion{
		curatedPresetQuestion(
			"memory-bank",
			"Digital Memory Bank",
			"Turn chats, emails, photos, and documents into a searchable life timeline you can revisit anytime.",
			"Please organize my chat history, emails, photo metadata, and documents into a personal memory system. Extract the time, place, people, events, and keywords, use OCR when images contain text, build a traceable timeline and topic index, and answer future questions in a clear structure of what happened, supporting evidence, and chronology.",
			"personal-knowledge",
			[]string{"personal-knowledge", "learning-growth"},
			"🧠",
			120,
		),
		curatedPresetQuestion(
			"reading-companion",
			"Daily Reading Companion",
			"Pick the most worthwhile things for me to read each day, then help me digest and reflect on them.",
			"Track sources around the topics I care about and surface the three most worthwhile pieces for me each day. For every piece, generate a concise summary, the key ideas, the debatable points, and two discussion prompts, then archive everything by theme and keep track of my reading progress, feedback, and changing interests over time.",
			"learning-growth",
			[]string{"learning-growth"},
			"📚",
			118,
		),
		curatedPresetQuestion(
			"growth-map",
			"Personal Growth Map",
			"Turn courses, projects, notes, and review habits into a living skill map that tells me what to learn next.",
			"Please combine my course progress, project commits, notes, and knowledge cards into a personal growth map. Identify the skills I have already built, my weak spots, and where I am stalled, schedule spaced review, and recommend the next learning resources and practice tasks that are most worth my time.",
			"learning-growth",
			[]string{"learning-growth", "personal-knowledge"},
			"🗺️",
			116,
		),
		curatedPresetQuestion(
			"content-planner",
			"Cross-Platform Content Planner",
			"Watch X, Reddit, and YouTube trends, then turn them into a publishable content pipeline.",
			"Continuously track high-velocity AI software topics across X, Reddit, and YouTube. Distill the narratives, conflicts, and talking points that are gaining traction, then turn them into a week of content ideas with title angles, script outlines, posting-time suggestions, and follow-up metrics for reviewing performance.",
			"content-creation",
			[]string{"content-creation", "user-research"},
			"📣",
			114,
		),
		curatedPresetQuestion(
			"chip-market-briefing",
			"Chip Market Open Brief",
			"Read one page before the opening bell and know what changed across NVDA, AMD, and the chip chain.",
			"Generate a pre-market briefing focused on NVDA, AMD, and the broader semiconductor chain. Summarize overnight price moves, important news, analyst views, market sentiment, and possible catalysts, then present the result as what happened, why it matters, and what to watch today.",
			"market-investing",
			[]string{"market-investing"},
			"📈",
			112,
		),
		curatedPresetQuestion(
			"morning-briefing",
			"Personal Morning Brief",
			"Blend my schedule, email, tasks, and news into a morning brief with real priorities.",
			"Create a morning brief for me every day that pulls together my calendar, email activity, tasks, and news from the domains I follow. Rank the most important three priorities for today, call out likely risks, and suggest the best order of action so I can get oriented in three minutes.",
			"personal-knowledge",
			[]string{"personal-knowledge"},
			"🌅",
			110,
		),
		curatedPresetQuestion(
			"macro-tracker",
			"Economic Data Decoder",
			"Track macro releases and turn raw updates into a practical explanation of what they could mean.",
			"Continuously monitor public macro data from the Federal Reserve, statistics agencies, and similar sources, keeping historical comparisons and anomaly alerts. Whenever new data arrives, explain what changed, what it may imply for different asset classes, where the market might misread it, and which scenarios deserve attention next.",
			"market-investing",
			[]string{"market-investing", "learning-growth"},
			"🏛️",
			108,
		),
		curatedPresetQuestion(
			"design-adaptation",
			"One-Click Multi-Device Adaptation",
			"Take one design and quickly derive desktop, tablet, and mobile adaptation guidance plus a handoff checklist.",
			"Based on my design file or UI spec, produce a multi-device adaptation plan. Identify the core layout structure, reusable components, and key breakpoints, then output device-specific adaptation suggestions, spec changes, and a practical handoff checklist with the main risks for engineering.",
			"product-design",
			[]string{"product-design"},
			"📐",
			106,
		),
		curatedPresetQuestion(
			"inspiration-feed",
			"Infinite Design Inspiration",
			"Learn my taste and keep feeding me references that are not just similar, but actually reusable.",
			"Continuously collect and organize high-quality references around the visual styles I prefer. Classify inspiration by layout, color, texture, mood, and interaction language, recommend adjacent references, and package each round into a moodboard and style-direction brief that I can use immediately.",
			"product-design",
			[]string{"product-design"},
			"🎨",
			104,
		),
		curatedPresetQuestion(
			"sentiment-radar",
			"User Sentiment Radar",
			"Hear the emotional shifts across comments and communities, then spot what users care about that still is not being solved.",
			"Continuously monitor community posts, social discussions, and video comments to identify sentiment, repeated complaints, frequent expectations, and polarized viewpoints. Go beyond sentiment labels and summarize the real problems users care about, the likely user segments, the unmet needs, and the product opportunities worth validating.",
			"user-research",
			[]string{"user-research", "content-creation"},
			"🛰️",
			102,
		),
		curatedPresetQuestion(
			"dream-dialogue",
			"Dream Symbol Dialogue",
			"Do not explain my dream for me. Ask the questions that help me connect it back to real life.",
			"When I describe a dream, do not jump to a standardized interpretation. Ask a sequence of thoughtful questions that helps me connect the dream's scenes, people, and emotions back to real-life experiences, gradually build a personal dream-symbol lexicon, and keep track of recurring themes as they evolve.",
			"psychological-exploration",
			[]string{"psychological-exploration"},
			"🌙",
			100,
		),
		curatedPresetQuestion(
			"aristotle-dialogue",
			"Soul Dialogue: Aristotle",
			"Turn a philosopher's public writings into a long-lived container I can keep thinking with.",
			"Build a dialogue-ready knowledge container around Aristotle's public works, reliable historical sources, and core ideas. Distill his conceptual system, value judgments, argument style, and recurring question patterns, and when answering me, preserve his style of thought while clearly separating original doctrine, reasonable inference, and modern extension.",
			"philosophical-dialogue",
			[]string{"philosophical-dialogue", "learning-growth"},
			"🏛️",
			98,
		),
	}
}

func chinesePresetQuestions() []PresetQuestion {
	return []PresetQuestion{
		curatedPresetQuestion(
			"memory-bank",
			"数字记忆永生银行",
			"把聊天、邮件、照片和文档串成可追溯的人生时间线，随时找回某个人、某件事、某个阶段。",
			"请把我的聊天记录、邮件、照片元数据和文档整理成一套个人记忆库。自动抽取时间、地点、人物、事件与关键词，必要时用 OCR 识别图片文字，生成可追踪的时间线与主题索引，并在我提问时按“发生了什么、相关证据、时间脉络”的结构回答。",
			"personal-knowledge",
			[]string{"personal-knowledge", "learning-growth"},
			"🧠",
			120,
		),
		curatedPresetQuestion(
			"reading-companion",
			"每日阅读伴读",
			"每天替我挑出最值得读的内容，不只总结，还要帮我建立理解和复盘闭环。",
			"围绕我关注的主题持续追踪文章来源，筛选出每天最值得读的 3 篇。对每篇生成摘要、关键观点、可争议点和 2 个讨论问题，按主题归档，并记录我的阅读进度、反馈和长期兴趣变化。",
			"learning-growth",
			[]string{"learning-growth"},
			"📚",
			118,
		),
		curatedPresetQuestion(
			"growth-map",
			"个人学习成长地图",
			"把课程、项目、笔记和复习节奏拼成一张动态技能地图，知道下一步最该学什么。",
			"请把我的课程学习、项目提交、笔记和知识卡片整合成一张成长地图。识别我已经掌握的技能、薄弱环节和停滞点，安排间隔复习，并为每个阶段推荐下一步最值得投入的学习资源和练习任务。",
			"learning-growth",
			[]string{"learning-growth", "personal-knowledge"},
			"🗺️",
			116,
		),
		curatedPresetQuestion(
			"content-planner",
			"多平台内容策划",
			"盯住 X、Reddit、YouTube 的热点，把趋势变成一整套可发布的内容选题。",
			"持续跟踪 X、Reddit 和 YouTube 上与 AI 软件相关的高热度话题，提炼正在爆发的观点、争议和叙事。基于这些趋势，为我生成一周的内容选题、每条内容的标题角度、脚本提纲、发布时间建议和互动复盘指标。",
			"content-creation",
			[]string{"content-creation", "user-research"},
			"📣",
			114,
		),
		curatedPresetQuestion(
			"chip-market-briefing",
			"芯片行情开盘必读",
			"开盘前 30 分钟，用一页简报看完 NVDA、AMD 和芯片链的关键变化。",
			"围绕 NVDA、AMD 以及芯片产业链，生成一份开盘前必读简报。总结隔夜价格变动、相关新闻、分析师观点、市场情绪和潜在催化因素，并用“发生了什么、为什么重要、今天该关注什么”三段式输出。",
			"market-investing",
			[]string{"market-investing"},
			"📈",
			112,
		),
		curatedPresetQuestion(
			"morning-briefing",
			"个人数据晨间简报",
			"把日程、邮件、待办和新闻合成一份真正有优先级的晨报。",
			"每天早上为我生成一份晨间简报，整合我的日历安排、邮件动态、待办事项和关注领域新闻。按优先级给出今天最重要的 3 件事、需要预判的风险和建议的行动顺序，让我在 3 分钟内进入状态。",
			"personal-knowledge",
			[]string{"personal-knowledge"},
			"🌅",
			110,
		),
		curatedPresetQuestion(
			"macro-tracker",
			"经济数据追踪解读",
			"盯住关键宏观指标变化，帮我把“知道数据更新了”变成“理解它意味着什么”。",
			"持续追踪美联储、统计局等公开经济数据，建立历史对比和异常提醒。每次数据更新后，解释这组数据意味着什么、对不同资产可能有哪些影响、市场常见误读是什么，并给出几种可能的情景推演。",
			"market-investing",
			[]string{"market-investing", "learning-growth"},
			"🏛️",
			108,
		),
		curatedPresetQuestion(
			"design-adaptation",
			"设计稿一键多端适配",
			"从一份设计稿快速推演出桌面端、平板端和移动端的适配方案与交付清单。",
			"基于我的设计稿或页面规范，帮我产出多端适配方案。识别核心布局结构、组件复用关系和关键断点，输出不同设备的适配建议、设计规范变化点，以及给开发的交付清单和风险提示。",
			"product-design",
			[]string{"product-design"},
			"📐",
			106,
		),
		curatedPresetQuestion(
			"inspiration-feed",
			"设计灵感无限供应",
			"按我的审美偏好持续投喂灵感，不只找相似图，还要给出可复用的风格方向。",
			"围绕我偏好的视觉风格，持续收集和整理来自设计社区的优秀案例。把灵感按版式、色彩、材质、氛围和交互语言分类，推荐相似参考，并为每次创作生成一份可直接开工的情绪板和风格方向说明。",
			"product-design",
			[]string{"product-design"},
			"🎨",
			104,
		),
		curatedPresetQuestion(
			"sentiment-radar",
			"用户评论情感雷达",
			"实时听见评论区和社媒里的情绪变化，发现用户真正关心却没被满足的点。",
			"持续监控社区帖子、社媒讨论和视频评论，识别用户情绪、重复抱怨、高频期待和观点分化。不要只做情感分类，还要总结用户在意的核心问题、潜在画像、尚未被满足的需求，以及值得验证的产品机会。",
			"user-research",
			[]string{"user-research", "content-creation"},
			"🛰️",
			102,
		),
		curatedPresetQuestion(
			"dream-dialogue",
			"梦境符号深度对话",
			"不替我下结论，而是通过追问帮我从梦里看见现实里的线索。",
			"当我描述梦境时，请不要直接给出标准化解释。先通过连续追问，引导我把梦里的场景、人物和情绪与现实经历联系起来，逐步沉淀出属于我的梦境符号词典，并记录反复出现的主题与变化。",
			"psychological-exploration",
			[]string{"psychological-exploration"},
			"🌙",
			100,
		),
		curatedPresetQuestion(
			"aristotle-dialogue",
			"灵魂对话：亚里士多德",
			"把思想家的公开著作和方法论整理成一个可长期对话的“思想容器”。",
			"请围绕亚里士多德的公开著作、可信史料和核心思想，整理一套可对话的知识容器。提炼他的概念体系、价值判断、论证方式和常用追问框架，在回答我问题时尽量保持他的思考风格，同时明确区分原典观点、合理推断和现代延伸。",
			"philosophical-dialogue",
			[]string{"philosophical-dialogue", "learning-growth"},
			"🏛️",
			98,
		),
	}
}
