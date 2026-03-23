package preview

import (
	"math/rand"
	"sync"

	"github.com/IceWhaleTech/ZimaOS-Blue/server/internal/timeutil"
)

// PresetQuestionAttachment represents an attachment for a preset question.
type PresetQuestionAttachment struct {
	Type        string `json:"type"`        // "image" or "file"
	Name        string `json:"name"`        // filename
	MimeType    string `json:"mime_type"`   // MIME type
	Placeholder string `json:"placeholder"` // "sample-image" or "sample-document"
}

// PresetQuestion represents a preset demo question.
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

// QuestionsService provides preset questions for the preview mode.
type QuestionsService struct {
	questions    map[string][]PresetQuestion // keyed by language code
	rng          *rand.Rand
	lastSelected []string // Track last selected question IDs
	mu           sync.Mutex
}

// NewQuestionsService creates a new QuestionsService with default questions.
func NewQuestionsService() *QuestionsService {
	return &QuestionsService{
		questions:    defaultPresetQuestions(),
		rng:          rand.New(rand.NewSource(timeutil.NowNano())),
		lastSelected: make([]string, 0),
	}
}

// GetPresetQuestions returns a random selection of preset questions.
// It tries to avoid returning the same questions as the last request.
func (s *QuestionsService) GetPresetQuestions(count int, lang string) []PresetQuestion {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get questions for the specified language, fallback to English
	questions, ok := s.questions[lang]
	if !ok {
		questions = s.questions["en"]
	}

	if count <= 0 {
		return []PresetQuestion{}
	}

	if count >= len(questions) {
		// Return all questions in random order
		result := make([]PresetQuestion, len(questions))
		copy(result, questions)
		s.rng.Shuffle(len(result), func(i, j int) {
			result[i], result[j] = result[j], result[i]
		})
		s.updateLastSelected(result)
		return result
	}

	// Build a list of available questions, prioritizing those not in lastSelected
	available := make([]PresetQuestion, 0, len(questions))
	lastSelectedSet := make(map[string]bool)
	for _, id := range s.lastSelected {
		lastSelectedSet[id] = true
	}

	// First, add questions that were NOT in the last selection
	for _, q := range questions {
		if !lastSelectedSet[q.ID] {
			available = append(available, q)
		}
	}

	// If we don't have enough new questions, add some from last selection
	if len(available) < count {
		for _, q := range questions {
			if lastSelectedSet[q.ID] {
				available = append(available, q)
			}
		}
	}

	// Shuffle available questions
	s.rng.Shuffle(len(available), func(i, j int) {
		available[i], available[j] = available[j], available[i]
	})

	// Take the first 'count' questions
	result := available[:count]
	s.updateLastSelected(result)
	return result
}

// updateLastSelected updates the last selected question IDs
func (s *QuestionsService) updateLastSelected(questions []PresetQuestion) {
	s.lastSelected = make([]string, len(questions))
	for i, q := range questions {
		s.lastSelected[i] = q.ID
	}
}

// GetAllQuestions returns all preset questions for a language.
func (s *QuestionsService) GetAllQuestions(lang string) []PresetQuestion {
	questions, ok := s.questions[lang]
	if !ok {
		questions = s.questions["en"]
	}
	result := make([]PresetQuestion, len(questions))
	copy(result, questions)
	return result
}

// GetQuestionsByCategory returns questions filtered by category.
func (s *QuestionsService) GetQuestionsByCategory(category string, lang string) []PresetQuestion {
	questions, ok := s.questions[lang]
	if !ok {
		questions = s.questions["en"]
	}
	var result []PresetQuestion
	for _, q := range questions {
		if q.Category == category {
			result = append(result, q)
		}
	}
	return result
}

// defaultPresetQuestions returns the default set of preset questions for all languages.
func defaultPresetQuestions() map[string][]PresetQuestion {
	return map[string][]PresetQuestion{
		"zh":    chinesePresetQuestions(),
		"en":    englishPresetQuestions(),
		"ja":    japanesePresetQuestions(),
		"ko":    koreanPresetQuestions(),
		"zh-TW": traditionalChinesePresetQuestions(),
		"de":    germanPresetQuestions(),
		"fr":    frenchPresetQuestions(),
		"es":    spanishPresetQuestions(),
		"it":    italianPresetQuestions(),
		"pt-BR": brazilianPortuguesePresetQuestions(),
		"ru":    russianPresetQuestions(),
		"nl":    dutchPresetQuestions(),
		"pl":    polishPresetQuestions(),
		"sv":    swedishPresetQuestions(),
		"da":    danishPresetQuestions(),
		"nb":    norwegianPresetQuestions(),
		"cs":    czechPresetQuestions(),
		"sk":    slovakPresetQuestions(),
		"hu":    hungarianPresetQuestions(),
		"ro":    romanianPresetQuestions(),
		"hr":    croatianPresetQuestions(),
		"el":    greekPresetQuestions(),
		"ca":    catalanPresetQuestions(),
		"ga":    irishPresetQuestions(),
		"ml":    malayalamPresetQuestions(),
		"pt-PT": europeanPortuguesePresetQuestions(),
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

// chinesePresetQuestions returns Chinese preset questions.
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

// englishPresetQuestions returns English preset questions.
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

// japanesePresetQuestions returns Japanese preset questions.
func japanesePresetQuestions() []PresetQuestion {
	return []PresetQuestion{
		{ID: "q1", Text: "休暇申請メールを書いてください", Category: "writing", Icon: "✉️"},
		{ID: "q2", Text: "機械学習について説明してください", Category: "learning", Icon: "🎓"},
		{ID: "q3", Text: "SF映画をいくつかおすすめしてください", Category: "entertainment", Icon: "🎬"},
		{ID: "q4", Text: "フィットネスプランを作成してください", Category: "lifestyle", Icon: "💪"},
		{ID: "q6", Text: "Pythonでクイックソートを書いてください", Category: "coding", Icon: "💻"},
		{ID: "q7", Text: "REST APIの設計原則を説明してください", Category: "coding", Icon: "🔧"},
		{ID: "q8", Text: "Dockerと仮想マシンの違いは？", Category: "tech", Icon: "🐳"},
		{ID: "q9", Text: "春についての詩を書いてください", Category: "creative", Icon: "🌸"},
		{ID: "q10", Text: "新製品の名前を考えてください", Category: "creative", Icon: "💡"},
		{ID: "q11", Text: "NASの自動バックアップの設定方法は？", Category: "nas", Icon: "💾"},
		{ID: "q12", Text: "家庭向けNASアプリをおすすめしてください", Category: "nas", Icon: "🏠"},
		{ID: "q13", Text: "今夜の夕食は何がいい？", Category: "lifestyle", Icon: "🍽️"},
		{ID: "q14", Text: "週末の旅行を計画してください", Category: "travel", Icon: "✈️"},
		{ID: "q15", Text: "早起きの習慣をつけるには？", Category: "lifestyle", Icon: "🌅"},
		{ID: "q16", Text: "英語学習の良い方法は？", Category: "learning", Icon: "📚"},
		{ID: "q17", Text: "ブロックチェーンの仕組みを説明してください", Category: "learning", Icon: "🔗"},
		{ID: "q18", Text: "量子コンピュータとは？", Category: "learning", Icon: "⚛️"},
		{ID: "q19", Text: "技術面接の準備方法は？", Category: "career", Icon: "👔"},
		{ID: "q20", Text: "仕事の効率を上げるには？", Category: "productivity", Icon: "⚡"},
		{ID: "q22", Text: "この写真の構図と色彩を分析してください", Category: "vision", Icon: "🎨", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "cityscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-scene"},
		}},
		{ID: "q23", Text: "画像内の文字を認識してください", Category: "vision", Icon: "📷", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "invoice.jpg", MimeType: "image/jpeg", Placeholder: "sample-text-image"},
		}},
		{ID: "q42", Text: "このグラフのデータを解説してください", Category: "agent2-ui", Icon: "📈", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "chart.png", MimeType: "image/png", Placeholder: "sample-chart"},
		}},
		{ID: "q43", Text: "このコードの構造とロジックを分析してください", Category: "agent2-ui", Icon: "💻", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},
		{ID: "q44", Text: "このコードを最適化してください", Category: "agent2-ui", Icon: "🔧", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},
		{ID: "q45", Text: "この販売レポートを分析して提案してください", Category: "agent2-ui", Icon: "📋", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "report.txt", MimeType: "text/plain", Placeholder: "sample-document"},
		}},
		{ID: "q47", Text: "この風景画像の内容を説明してください", Category: "agent2-ui", Icon: "🖼️", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "landscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-image"},
		}},
		{ID: "q49", Text: "このCSV販売データを分析して傾向を教えてください", Category: "agent2-ui", Icon: "📊", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "sales_data.csv", MimeType: "text/csv", Placeholder: "sample-csv"},
		}},
		{ID: "q50", Text: "この設定ファイルに問題がないか確認してください", Category: "agent2-ui", Icon: "⚙️", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "config.json", MimeType: "application/json", Placeholder: "sample-json"},
		}},
		{ID: "q51", Text: "このJavaScriptコードのバグを特定してください", Category: "agent2-ui", Icon: "🐛", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "buggy_calculator.js", MimeType: "text/javascript", Placeholder: "sample-js"},
		}},

		// Flowchart
		{ID: "q52", Text: "ユーザー登録・ログインのフローチャートを描いてください", Category: "creative", Icon: "📐"},
	}
}

// koreanPresetQuestions returns Korean preset questions.
func koreanPresetQuestions() []PresetQuestion {
	return []PresetQuestion{
		{ID: "q1", Text: "휴가 신청 이메일을 작성해 주세요", Category: "writing", Icon: "✉️"},
		{ID: "q2", Text: "머신러닝이 무엇인지 설명해 주세요", Category: "learning", Icon: "🎓"},
		{ID: "q3", Text: "SF 영화를 추천해 주세요", Category: "entertainment", Icon: "🎬"},
		{ID: "q4", Text: "운동 계획을 세워 주세요", Category: "lifestyle", Icon: "💪"},
		{ID: "q6", Text: "Python으로 퀵 정렬을 작성해 주세요", Category: "coding", Icon: "💻"},
		{ID: "q7", Text: "REST API 설계 원칙을 설명해 주세요", Category: "coding", Icon: "🔧"},
		{ID: "q8", Text: "Docker와 가상 머신의 차이는?", Category: "tech", Icon: "🐳"},
		{ID: "q9", Text: "봄에 대한 시를 써 주세요", Category: "creative", Icon: "🌸"},
		{ID: "q10", Text: "새 제품 이름을 지어 주세요", Category: "creative", Icon: "💡"},
		{ID: "q11", Text: "NAS 자동 백업 설정 방법은?", Category: "nas", Icon: "💾"},
		{ID: "q12", Text: "가정용 NAS 앱을 추천해 주세요", Category: "nas", Icon: "🏠"},
		{ID: "q13", Text: "오늘 저녁 뭐 먹을까요?", Category: "lifestyle", Icon: "🍽️"},
		{ID: "q14", Text: "주말 여행을 계획해 주세요", Category: "travel", Icon: "✈️"},
		{ID: "q15", Text: "일찍 일어나는 습관 들이는 법?", Category: "lifestyle", Icon: "🌅"},
		{ID: "q16", Text: "영어 공부 좋은 방법은?", Category: "learning", Icon: "📚"},
		{ID: "q17", Text: "블록체인 원리를 설명해 주세요", Category: "learning", Icon: "🔗"},
		{ID: "q18", Text: "양자 컴퓨팅이란?", Category: "learning", Icon: "⚛️"},
		{ID: "q19", Text: "기술 면접 준비 방법은?", Category: "career", Icon: "👔"},
		{ID: "q20", Text: "업무 효율을 높이는 방법은?", Category: "productivity", Icon: "⚡"},
		{ID: "q22", Text: "이 사진의 구도와 색감을 분석해 주세요", Category: "vision", Icon: "🎨", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "cityscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-scene"},
		}},
		{ID: "q23", Text: "이미지의 글자를 인식해 주세요", Category: "vision", Icon: "📷", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "invoice.jpg", MimeType: "image/jpeg", Placeholder: "sample-text-image"},
		}},
		{ID: "q42", Text: "이 차트 데이터를 해석해 주세요", Category: "agent2-ui", Icon: "📈", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "chart.png", MimeType: "image/png", Placeholder: "sample-chart"},
		}},
		{ID: "q43", Text: "이 코드의 구조와 로직을 분석해 주세요", Category: "agent2-ui", Icon: "💻", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},
		{ID: "q44", Text: "이 코드를 최적화해 주세요", Category: "agent2-ui", Icon: "🔧", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},
		{ID: "q45", Text: "이 판매 보고서를 분석하고 제안해 주세요", Category: "agent2-ui", Icon: "📋", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "report.txt", MimeType: "text/plain", Placeholder: "sample-document"},
		}},
		{ID: "q47", Text: "이 풍경 사진의 내용을 설명해 주세요", Category: "agent2-ui", Icon: "🖼️", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "landscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-image"},
		}},
		{ID: "q49", Text: "이 CSV 판매 데이터를 분석하고 추세를 찾아 주세요", Category: "agent2-ui", Icon: "📊", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "sales_data.csv", MimeType: "text/csv", Placeholder: "sample-csv"},
		}},
		{ID: "q50", Text: "이 설정 파일에 문제가 없는지 확인해 주세요", Category: "agent2-ui", Icon: "⚙️", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "config.json", MimeType: "application/json", Placeholder: "sample-json"},
		}},
		{ID: "q51", Text: "이 JavaScript 코드의 버그를 찾아 주세요", Category: "agent2-ui", Icon: "🐛", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "buggy_calculator.js", MimeType: "text/javascript", Placeholder: "sample-js"},
		}},

		// Flowchart
		{ID: "q52", Text: "사용자 등록 및 로그인 흐름도를 그려 주세요", Category: "creative", Icon: "📐"},
	}
}
