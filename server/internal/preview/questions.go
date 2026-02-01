package preview

import (
	"math/rand"
	"sync"
	"time"
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
	ID          string                     `json:"id"`
	Text        string                     `json:"text"`
	TextKey     string                     `json:"text_key,omitempty"` // i18n key for frontend translation
	Category    string                     `json:"category"`
	Icon        string                     `json:"icon,omitempty"`
	Attachments []PresetQuestionAttachment `json:"attachments,omitempty"`
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
		"ja": japanesePresetQuestions(),
		"ko": koreanPresetQuestions(),
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
		{ID: "q20", Text: "如何提高工作效率？", Category: "productivity", Icon: "⚡"},

		// Multimodal - Image Analysis
		{ID: "q22", Text: "分析这张照片的构图和色彩", Category: "vision", Icon: "🎨", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "cityscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-scene"},
		}},
		{ID: "q23", Text: "帮我识别图片中的文字内容", Category: "vision", Icon: "📷", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "invoice.jpg", MimeType: "image/jpeg", Placeholder: "sample-text-image"},
		}},

		// Agent 2 UI Demo - Chart Analysis
		{ID: "q42", Text: "帮我解读这个图表的数据", Category: "agent2-ui", Icon: "📈", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "chart.png", MimeType: "image/png", Placeholder: "sample-chart"},
		}},

		// Agent 2 UI Demo - Code Analysis
		{ID: "q43", Text: "分析这段代码的结构和逻辑", Category: "agent2-ui", Icon: "💻", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},
		{ID: "q44", Text: "帮我优化这段代码", Category: "agent2-ui", Icon: "🔧", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},

		// Agent 2 UI Demo - Document Analysis
		{ID: "q45", Text: "分析这份销售报告并给出建议", Category: "agent2-ui", Icon: "📋", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "report.txt", MimeType: "text/plain", Placeholder: "sample-document"},
		}},

		// Agent 2 UI Demo - Image Analysis
		{ID: "q47", Text: "描述这张风景图片的内容", Category: "agent2-ui", Icon: "🖼️", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "landscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-image"},
		}},

		// Agent 2 UI Demo - Data Analysis
		{ID: "q49", Text: "分析这个CSV销售数据并找出趋势", Category: "agent2-ui", Icon: "📊", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "sales_data.csv", MimeType: "text/csv", Placeholder: "sample-csv"},
		}},
		{ID: "q50", Text: "帮我检查这个配置文件是否有问题", Category: "agent2-ui", Icon: "⚙️", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "config.json", MimeType: "application/json", Placeholder: "sample-json"},
		}},
		{ID: "q51", Text: "找出这段JavaScript代码中的bug", Category: "agent2-ui", Icon: "🐛", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "buggy_calculator.js", MimeType: "text/javascript", Placeholder: "sample-js"},
		}},
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
		{ID: "q21", Text: "Identify objects in this image and mark positions", Category: "vision", Icon: "🔍", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "room.jpg", MimeType: "image/jpeg", Placeholder: "sample-photo"},
		}},
		{ID: "q22", Text: "Analyze the composition and colors of this photo", Category: "vision", Icon: "🎨", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "cityscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-scene"},
		}},
		{ID: "q23", Text: "Help me recognize text in this image", Category: "vision", Icon: "📷", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "invoice.jpg", MimeType: "image/jpeg", Placeholder: "sample-text-image"},
		}},

		// File & Document Analysis
		{ID: "q24", Text: "Help me analyze this Excel spreadsheet", Category: "document", Icon: "📊", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "data.txt", MimeType: "text/plain", Placeholder: "sample-document"},
		}},
		{ID: "q25", Text: "Summarize the main content of this PDF", Category: "document", Icon: "📄", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "document.txt", MimeType: "text/plain", Placeholder: "sample-document"},
		}},
		{ID: "q26", Text: "Help me organize this CSV file data", Category: "document", Icon: "📋", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "sales_data.csv", MimeType: "text/csv", Placeholder: "sample-csv"},
		}},

		// Data Visualization
		{ID: "q27", Text: "Generate a simple table from data", Category: "data", Icon: "📈"},
		{ID: "q28", Text: "Help me analyze sales data trends", Category: "data", Icon: "📉"},

		// Smart Home & IoT
		{ID: "q29", Text: "How to automate home with Home Assistant?", Category: "smarthome", Icon: "🏡"},
		{ID: "q30", Text: "Recommend smart home starter devices", Category: "smarthome", Icon: "💡"},

		// Agent 2 UI Demo - Chart Analysis
		{ID: "q41", Text: "Analyze the data trends in this chart", Category: "agent2-ui", Icon: "📊", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "chart.png", MimeType: "image/png", Placeholder: "sample-chart"},
		}},
		{ID: "q42", Text: "Help me interpret this sales data chart", Category: "agent2-ui", Icon: "📈", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "chart.png", MimeType: "image/png", Placeholder: "sample-chart"},
		}},

		// Agent 2 UI Demo - Code Analysis
		{ID: "q43", Text: "Analyze the structure and logic of this code", Category: "agent2-ui", Icon: "💻", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},
		{ID: "q44", Text: "Help me optimize this code", Category: "agent2-ui", Icon: "🔧", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "hello.py", MimeType: "text/x-python", Placeholder: "sample-code"},
		}},

		// Agent 2 UI Demo - Document Analysis
		{ID: "q45", Text: "Summarize the key data in this report", Category: "agent2-ui", Icon: "📋", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "report.txt", MimeType: "text/plain", Placeholder: "sample-document"},
		}},
		{ID: "q46", Text: "Analyze this sales report and provide recommendations", Category: "agent2-ui", Icon: "📄", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "sales-report.txt", MimeType: "text/plain", Placeholder: "sample-document"},
		}},

		// Agent 2 UI Demo - Image Analysis
		{ID: "q47", Text: "Describe the content of this landscape image", Category: "agent2-ui", Icon: "🖼️", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "landscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-image"},
		}},
		{ID: "q48", Text: "Analyze the composition and color scheme of this image", Category: "agent2-ui", Icon: "🎨", Attachments: []PresetQuestionAttachment{
			{Type: "image", Name: "cityscape.jpg", MimeType: "image/jpeg", Placeholder: "sample-scene"},
		}},

		// Agent 2 UI Demo - Data Analysis
		{ID: "q49", Text: "Analyze this CSV sales data and find trends", Category: "agent2-ui", Icon: "📊", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "sales_data.csv", MimeType: "text/csv", Placeholder: "sample-csv"},
		}},
		{ID: "q50", Text: "Check this config file for any issues", Category: "agent2-ui", Icon: "⚙️", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "config.json", MimeType: "application/json", Placeholder: "sample-json"},
		}},
		{ID: "q51", Text: "Find the bugs in this JavaScript code", Category: "agent2-ui", Icon: "🐛", Attachments: []PresetQuestionAttachment{
			{Type: "file", Name: "buggy_calculator.js", MimeType: "text/javascript", Placeholder: "sample-js"},
		}},

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
	}
}
