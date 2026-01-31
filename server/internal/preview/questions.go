package preview

import (
	"math/rand"
	"sync"
	"time"
)

// PresetQuestion represents a preset demo question.
type PresetQuestion struct {
	ID       string `json:"id"`
	Text     string `json:"text"`
	TextKey  string `json:"text_key,omitempty"` // i18n key for frontend translation
	Category string `json:"category"`
	Icon     string `json:"icon,omitempty"`
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
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
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
		"zh": chinesePresetQuestions(),
		"en": englishPresetQuestions(),
	}
}

// chinesePresetQuestions returns Chinese preset questions.
func chinesePresetQuestions() []PresetQuestion {
	return []PresetQuestion{
		// General AI Assistant
		{ID: "q1", Text: "帮我写一封请假邮件", Category: "writing", Icon: "✉️"},
		{ID: "q2", Text: "解释一下什么是机器学习", Category: "learning", Icon: "🎓"},
		{ID: "q3", Text: "给我推荐几部科幻电影", Category: "entertainment", Icon: "🎬"},
		{ID: "q4", Text: "帮我制定一个健身计划", Category: "lifestyle", Icon: "💪"},
		{ID: "q5", Text: "如何提高工作效率？", Category: "productivity", Icon: "⚡"},

		// Coding & Tech
		{ID: "q6", Text: "用 Python 写一个快速排序", Category: "coding", Icon: "💻"},
		{ID: "q7", Text: "解释 REST API 的设计原则", Category: "coding", Icon: "🔧"},
		{ID: "q8", Text: "Docker 和虚拟机有什么区别？", Category: "tech", Icon: "🐳"},

		// Creative
		{ID: "q9", Text: "帮我写一首关于春天的诗", Category: "creative", Icon: "🌸"},
		{ID: "q10", Text: "给我的新产品起个名字", Category: "creative", Icon: "💡"},

		// NAS & Home Server
		{ID: "q11", Text: "如何设置 NAS 的自动备份？", Category: "nas", Icon: "💾"},
		{ID: "q12", Text: "推荐一些适合家庭使用的 NAS 应用", Category: "nas", Icon: "🏠"},

		// Daily Life
		{ID: "q13", Text: "今天晚餐吃什么好？", Category: "lifestyle", Icon: "🍽️"},
		{ID: "q14", Text: "帮我规划一次周末旅行", Category: "travel", Icon: "✈️"},
		{ID: "q15", Text: "如何养成早起的习惯？", Category: "lifestyle", Icon: "🌅"},

		// Learning
		{ID: "q16", Text: "学习英语有什么好方法？", Category: "learning", Icon: "📚"},
		{ID: "q17", Text: "解释一下区块链的工作原理", Category: "learning", Icon: "🔗"},
		{ID: "q18", Text: "什么是量子计算？", Category: "learning", Icon: "⚛️"},

		// Work
		{ID: "q19", Text: "如何准备一场技术面试？", Category: "career", Icon: "👔"},
		{ID: "q20", Text: "帮我写一份项目总结报告", Category: "writing", Icon: "📝"},

		// Multimodal - Image Analysis
		{ID: "q21", Text: "识别这张图片中的物体并标注位置", Category: "vision", Icon: "🔍"},
		{ID: "q22", Text: "分析这张照片的构图和色彩", Category: "vision", Icon: "🎨"},
		{ID: "q23", Text: "帮我识别图片中的文字内容", Category: "vision", Icon: "📷"},

		// File & Document Analysis
		{ID: "q24", Text: "帮我分析这个 Excel 表格的数据", Category: "document", Icon: "📊"},
		{ID: "q25", Text: "总结这份 PDF 文档的主要内容", Category: "document", Icon: "📄"},
		{ID: "q26", Text: "帮我整理这个 CSV 文件的数据", Category: "document", Icon: "📋"},

		// Data Visualization
		{ID: "q27", Text: "根据数据生成一个简单的表格", Category: "data", Icon: "📈"},
		{ID: "q28", Text: "帮我分析这组销售数据的趋势", Category: "data", Icon: "📉"},

		// Smart Home & IoT
		{ID: "q29", Text: "如何用 Home Assistant 自动化家居？", Category: "smarthome", Icon: "🏡"},
		{ID: "q30", Text: "推荐一些智能家居入门设备", Category: "smarthome", Icon: "💡"},

		// Additional Chinese questions
		{ID: "q31", Text: "帮我翻译这段英文", Category: "writing", Icon: "🌐"},
		{ID: "q32", Text: "写一个简单的 JavaScript 函数", Category: "coding", Icon: "📜"},
		{ID: "q33", Text: "如何学习一门新的编程语言？", Category: "learning", Icon: "🎯"},
		{ID: "q34", Text: "帮我写一个会议纪要模板", Category: "writing", Icon: "📋"},
		{ID: "q35", Text: "解释一下人工智能和深度学习的区别", Category: "learning", Icon: "🤖"},
		{ID: "q36", Text: "推荐一些提高专注力的方法", Category: "productivity", Icon: "🧘"},
		{ID: "q37", Text: "帮我设计一个简单的数据库结构", Category: "coding", Icon: "🗄️"},
		{ID: "q38", Text: "如何在 Linux 上配置 SSH？", Category: "tech", Icon: "🔐"},
		{ID: "q39", Text: "帮我写一个产品介绍文案", Category: "creative", Icon: "✍️"},
		{ID: "q40", Text: "解释一下 Git 的工作流程", Category: "coding", Icon: "🔀"},
	}
}

// englishPresetQuestions returns English preset questions.
func englishPresetQuestions() []PresetQuestion {
	return []PresetQuestion{
		// General AI Assistant
		{ID: "q1", Text: "Help me write a leave request email", Category: "writing", Icon: "✉️"},
		{ID: "q2", Text: "Explain what machine learning is", Category: "learning", Icon: "🎓"},
		{ID: "q3", Text: "Recommend some sci-fi movies", Category: "entertainment", Icon: "🎬"},
		{ID: "q4", Text: "Help me create a fitness plan", Category: "lifestyle", Icon: "💪"},
		{ID: "q5", Text: "How can I improve work efficiency?", Category: "productivity", Icon: "⚡"},

		// Coding & Tech
		{ID: "q6", Text: "Write a quicksort in Python", Category: "coding", Icon: "💻"},
		{ID: "q7", Text: "Explain REST API design principles", Category: "coding", Icon: "🔧"},
		{ID: "q8", Text: "What's the difference between Docker and VMs?", Category: "tech", Icon: "🐳"},

		// Creative
		{ID: "q9", Text: "Write a poem about spring", Category: "creative", Icon: "🌸"},
		{ID: "q10", Text: "Help me name my new product", Category: "creative", Icon: "💡"},

		// NAS & Home Server
		{ID: "q11", Text: "How to set up automatic NAS backup?", Category: "nas", Icon: "💾"},
		{ID: "q12", Text: "Recommend NAS apps for home use", Category: "nas", Icon: "🏠"},

		// Daily Life
		{ID: "q13", Text: "What should I have for dinner?", Category: "lifestyle", Icon: "🍽️"},
		{ID: "q14", Text: "Help me plan a weekend trip", Category: "travel", Icon: "✈️"},
		{ID: "q15", Text: "How to develop an early rising habit?", Category: "lifestyle", Icon: "🌅"},

		// Learning
		{ID: "q16", Text: "What are good methods to learn English?", Category: "learning", Icon: "📚"},
		{ID: "q17", Text: "Explain how blockchain works", Category: "learning", Icon: "🔗"},
		{ID: "q18", Text: "What is quantum computing?", Category: "learning", Icon: "⚛️"},

		// Work
		{ID: "q19", Text: "How to prepare for a tech interview?", Category: "career", Icon: "👔"},
		{ID: "q20", Text: "Help me write a project summary report", Category: "writing", Icon: "📝"},

		// Multimodal - Image Analysis
		{ID: "q21", Text: "Identify objects in this image and mark positions", Category: "vision", Icon: "🔍"},
		{ID: "q22", Text: "Analyze the composition and colors of this photo", Category: "vision", Icon: "🎨"},
		{ID: "q23", Text: "Help me recognize text in this image", Category: "vision", Icon: "📷"},

		// File & Document Analysis
		{ID: "q24", Text: "Help me analyze this Excel spreadsheet", Category: "document", Icon: "📊"},
		{ID: "q25", Text: "Summarize the main content of this PDF", Category: "document", Icon: "📄"},
		{ID: "q26", Text: "Help me organize this CSV file data", Category: "document", Icon: "📋"},

		// Data Visualization
		{ID: "q27", Text: "Generate a simple table from data", Category: "data", Icon: "📈"},
		{ID: "q28", Text: "Help me analyze sales data trends", Category: "data", Icon: "📉"},

		// Smart Home & IoT
		{ID: "q29", Text: "How to automate home with Home Assistant?", Category: "smarthome", Icon: "🏡"},
		{ID: "q30", Text: "Recommend smart home starter devices", Category: "smarthome", Icon: "💡"},

		// Additional English questions
		{ID: "q31", Text: "Help me translate this text", Category: "writing", Icon: "🌐"},
		{ID: "q32", Text: "Write a simple JavaScript function", Category: "coding", Icon: "📜"},
		{ID: "q33", Text: "How to learn a new programming language?", Category: "learning", Icon: "🎯"},
		{ID: "q34", Text: "Help me create a meeting notes template", Category: "writing", Icon: "📋"},
		{ID: "q35", Text: "Explain the difference between AI and deep learning", Category: "learning", Icon: "🤖"},
		{ID: "q36", Text: "Recommend ways to improve focus", Category: "productivity", Icon: "🧘"},
		{ID: "q37", Text: "Help me design a simple database schema", Category: "coding", Icon: "🗄️"},
		{ID: "q38", Text: "How to configure SSH on Linux?", Category: "tech", Icon: "🔐"},
		{ID: "q39", Text: "Help me write product introduction copy", Category: "creative", Icon: "✍️"},
		{ID: "q40", Text: "Explain the Git workflow", Category: "coding", Icon: "🔀"},
	}
}
