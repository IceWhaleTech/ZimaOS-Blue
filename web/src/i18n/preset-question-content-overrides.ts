import type { LocaleMessages } from './merge'

type PresetQuestionExampleMessages = Record<
  string,
  {
    title: string
    description: string
    prompt: string
  }
>

function example(title: string, description: string, prompt: string) {
  return { title, description, prompt }
}

function buildMessages(examples: PresetQuestionExampleMessages): LocaleMessages {
  return {
    chat: {
      presetQuestions: {
        examples,
      },
    },
  }
}

const presetQuestionContentOverrides: Record<string, LocaleMessages> = {
  'en-US': buildMessages({
    memoryBank: example(
      'Digital Memory Bank',
      'Turn chats, emails, photos, and documents into a searchable life timeline you can revisit anytime.',
      'Organize my chat history, emails, photo metadata, and documents into a personal memory system. Extract time, place, people, events, and keywords, use OCR when needed, build a searchable timeline and topic index, and answer later questions with what happened, evidence, and chronology.'
    ),
    readingCompanion: example(
      'Daily Reading Companion',
      'Pick the most worthwhile things for me to read each day, then help me digest and reflect on them.',
      'Track sources around the topics I care about and surface the three most worthwhile pieces each day. For each piece, provide a summary, key ideas, debatable points, and two discussion prompts, archive them by theme, and track my reading progress and changing interests.'
    ),
    growthMap: example(
      'Personal Growth Map',
      'Turn courses, projects, notes, and review habits into a living skill map that tells me what to learn next.',
      'Combine my courses, project commits, notes, and knowledge cards into a personal growth map. Identify the skills I have built, my weak spots, and stalled areas, schedule spaced review, and recommend the next resources and practice tasks worth my time.'
    ),
    contentPlanner: example(
      'Cross-Platform Content Planner',
      'Watch X, Reddit, and YouTube trends, then turn them into a publishable content pipeline.',
      'Track fast-moving AI software topics across X, Reddit, and YouTube. Extract the narratives, conflicts, and talking points gaining traction, then turn them into a week of content ideas with title angles, script outlines, posting-time suggestions, and review metrics.'
    ),
    chipMarketBriefing: example(
      'Chip Market Open Brief',
      'Read one page before the opening bell and know what changed across NVDA, AMD, and the chip chain.',
      'Generate a pre-market briefing focused on NVDA, AMD, and the semiconductor chain. Summarize overnight price moves, important news, analyst views, market sentiment, and possible catalysts, then present what happened, why it matters, and what to watch today.'
    ),
    morningBriefing: example(
      'Personal Morning Brief',
      'Blend my schedule, email, tasks, and news into a morning brief with real priorities.',
      'Create a morning brief that combines my calendar, email activity, tasks, and relevant news. Rank the three most important priorities for today, flag likely risks, and suggest the best order of action so I can get oriented in three minutes.'
    ),
    macroTracker: example(
      'Economic Data Decoder',
      'Track macro releases and turn raw updates into a practical explanation of what they could mean.',
      'Monitor public macro data from the Federal Reserve, statistics agencies, and similar sources with historical comparisons and anomaly alerts. When new data arrives, explain what changed, what it may mean for different asset classes, where the market may misread it, and which scenarios deserve attention.'
    ),
    designAdaptation: example(
      'One-Click Multi-Device Adaptation',
      'Take one design and quickly derive desktop, tablet, and mobile adaptation guidance plus a handoff checklist.',
      'Based on my design file or UI spec, produce a multi-device adaptation plan. Identify core layouts, reusable components, and key breakpoints, then output device-specific guidance, spec changes, and a handoff checklist with major engineering risks.'
    ),
    inspirationFeed: example(
      'Infinite Design Inspiration',
      'Learn my taste and keep feeding me references that are not just similar, but actually reusable.',
      'Collect and organize strong references around the visual styles I prefer. Classify them by layout, color, texture, mood, and interaction language, recommend adjacent references, and package each round into a moodboard and style brief I can use immediately.'
    ),
    sentimentRadar: example(
      'User Sentiment Radar',
      'Hear the emotional shifts across comments and communities, then spot what users care about that still is not being solved.',
      'Monitor community posts, social discussions, and video comments to identify sentiment, repeated complaints, expectations, and polarized viewpoints. Go beyond sentiment labels and summarize the real problems users care about, likely user segments, unmet needs, and product opportunities worth validating.'
    ),
    dreamDialogue: example(
      'Dream Symbol Dialogue',
      'Do not explain my dream for me. Ask the questions that help me connect it back to real life.',
      'When I describe a dream, do not jump to a standard interpretation. Ask thoughtful follow-up questions that connect the dream scenes, people, and emotions to real-life experiences, build a personal dream-symbol lexicon, and track recurring themes over time.'
    ),
    aristotleDialogue: example(
      'Soul Dialogue: Aristotle',
      "Turn Aristotle's public writings and methods into a long-lived container I can keep thinking with.",
      "Build a dialogue-ready knowledge container around Aristotle's works, reliable historical sources, and core ideas. Distill his concepts, values, argument style, and recurring question patterns, and answer me in his style while clearly separating original doctrine, reasonable inference, and modern extension."
    ),
  }),
  'en-GB': buildMessages({
    memoryBank: example(
      'Digital Memory Bank',
      'Turn chats, emails, photos, and documents into a searchable life timeline you can revisit any time.',
      'Organise my chat history, emails, photo metadata, and documents into a personal memory system. Extract time, place, people, events, and keywords, use OCR when needed, build a searchable timeline and topic index, and answer later questions with what happened, evidence, and chronology.'
    ),
    readingCompanion: example(
      'Daily Reading Companion',
      'Pick the most worthwhile things for me to read each day, then help me digest and reflect on them.',
      'Track sources around the topics I care about and surface the three most worthwhile pieces each day. For each piece, provide a summary, key ideas, debatable points, and two discussion prompts, archive them by theme, and track my reading progress and changing interests.'
    ),
    growthMap: example(
      'Personal Growth Map',
      'Turn courses, projects, notes, and review habits into a living skill map that tells me what to learn next.',
      'Combine my courses, project commits, notes, and knowledge cards into a personal growth map. Identify the skills I have built, my weak spots, and stalled areas, schedule spaced review, and recommend the next resources and practice tasks worth my time.'
    ),
    contentPlanner: example(
      'Cross-Platform Content Planner',
      'Watch X, Reddit, and YouTube trends, then turn them into a publishable content pipeline.',
      'Track fast-moving AI software topics across X, Reddit, and YouTube. Extract the narratives, conflicts, and talking points gaining traction, then turn them into a week of content ideas with title angles, script outlines, posting-time suggestions, and review metrics.'
    ),
    chipMarketBriefing: example(
      'Chip Market Open Brief',
      'Read one page before the opening bell and know what changed across NVDA, AMD, and the chip chain.',
      'Generate a pre-market briefing focused on NVDA, AMD, and the semiconductor chain. Summarise overnight price moves, important news, analyst views, market sentiment, and possible catalysts, then present what happened, why it matters, and what to watch today.'
    ),
    morningBriefing: example(
      'Personal Morning Brief',
      'Blend my schedule, email, tasks, and news into a morning brief with real priorities.',
      'Create a morning brief that combines my calendar, email activity, tasks, and relevant news. Rank the three most important priorities for today, flag likely risks, and suggest the best order of action so I can get oriented in three minutes.'
    ),
    macroTracker: example(
      'Economic Data Decoder',
      'Track macro releases and turn raw updates into a practical explanation of what they could mean.',
      'Monitor public macro data from the Federal Reserve, statistics agencies, and similar sources with historical comparisons and anomaly alerts. When new data arrives, explain what changed, what it may mean for different asset classes, where the market may misread it, and which scenarios deserve attention.'
    ),
    designAdaptation: example(
      'One-Click Multi-Device Adaptation',
      'Take one design and quickly derive desktop, tablet, and mobile adaptation guidance plus a hand-off checklist.',
      'Based on my design file or UI spec, produce a multi-device adaptation plan. Identify core layouts, reusable components, and key breakpoints, then output device-specific guidance, spec changes, and a hand-off checklist with major engineering risks.'
    ),
    inspirationFeed: example(
      'Infinite Design Inspiration',
      'Learn my taste and keep feeding me references that are not just similar, but actually reusable.',
      'Collect and organise strong references around the visual styles I prefer. Classify them by layout, colour, texture, mood, and interaction language, recommend adjacent references, and package each round into a moodboard and style brief I can use immediately.'
    ),
    sentimentRadar: example(
      'User Sentiment Radar',
      'Hear the emotional shifts across comments and communities, then spot what users care about that still is not being solved.',
      'Monitor community posts, social discussions, and video comments to identify sentiment, repeated complaints, expectations, and polarised viewpoints. Go beyond sentiment labels and summarise the real problems users care about, likely user segments, unmet needs, and product opportunities worth validating.'
    ),
    dreamDialogue: example(
      'Dream Symbol Dialogue',
      'Do not explain my dream for me. Ask the questions that help me connect it back to real life.',
      'When I describe a dream, do not jump to a standard interpretation. Ask thoughtful follow-up questions that connect the dream scenes, people, and emotions to real-life experiences, build a personal dream-symbol lexicon, and track recurring themes over time.'
    ),
    aristotleDialogue: example(
      'Soul Dialogue: Aristotle',
      "Turn Aristotle's public writings and methods into a long-lived container I can keep thinking with.",
      "Build a dialogue-ready knowledge container around Aristotle's works, reliable historical sources, and core ideas. Distil his concepts, values, argument style, and recurring question patterns, and answer me in his style while clearly separating original doctrine, reasonable inference, and modern extension."
    ),
  }),
  'zh-CN': buildMessages({
    memoryBank: example(
      '数字记忆永生银行',
      '把聊天、邮件、照片和文档串成可搜索的人生时间线，随时找回某个人、某件事、某个阶段。',
      '请把我的聊天记录、邮件、照片元数据和文档整理成个人记忆系统。抽取时间、地点、人物、事件和关键词，必要时用 OCR 识别图片文字，建立可搜索的时间线和主题索引，并在我追问时按发生了什么、证据和时间脉络回答。'
    ),
    readingCompanion: example(
      '每日阅读伴读',
      '每天替我挑出最值得读的内容，不只总结，还要帮我建立理解和复盘闭环。',
      '持续追踪我关注主题的文章来源，每天筛出最值得读的 3 篇。为每篇生成摘要、关键观点、可争议点和 2 个讨论问题，按主题归档，并记录我的阅读进度和兴趣变化。'
    ),
    growthMap: example(
      '个人学习成长地图',
      '把课程、项目、笔记和复习节奏拼成一张动态技能地图，知道下一步最该学什么。',
      '请把我的课程进度、项目提交、笔记和知识卡片整合成个人成长地图。识别我已经掌握的技能、薄弱点和停滞环节，安排间隔复习，并推荐下一步最值得投入的学习资源和练习任务。'
    ),
    contentPlanner: example(
      '多平台内容策划',
      '盯住 X、Reddit、YouTube 的热点，把趋势变成一整套可发布的内容选题。',
      '持续跟踪 X、Reddit 和 YouTube 上高速升温的 AI 软件话题。提炼正在扩散的叙事、争议和观点，再生成一周的内容选题、标题角度、脚本提纲、发布时间建议和复盘指标。'
    ),
    chipMarketBriefing: example(
      '芯片行情开盘必读',
      '开盘前 30 分钟，用一页简报看完 NVDA、AMD 和芯片链的关键变化。',
      '请围绕 NVDA、AMD 和半导体产业链生成一份盘前简报。总结隔夜价格变动、重要新闻、分析师观点、市场情绪和潜在催化因素，并按发生了什么、为什么重要、今天看什么来输出。'
    ),
    morningBriefing: example(
      '个人数据晨间简报',
      '把日程、邮件、待办和新闻合成一份真正有优先级的晨报。',
      '每天早上为我生成一份晨间简报，整合日历安排、邮件动态、待办事项和相关领域新闻。按优先级列出今天最重要的 3 件事，提示可能风险，并给出最合适的行动顺序，让我在 3 分钟内进入状态。'
    ),
    macroTracker: example(
      '经济数据追踪解读',
      '盯住关键宏观指标变化，帮我把知道数据更新了变成理解它意味着什么。',
      '持续监控美联储、统计机构等公开宏观数据，建立历史对比和异常提醒。每次数据更新后，解释发生了什么、对不同资产可能意味着什么、市场可能误读的地方，以及接下来值得关注的情景。'
    ),
    designAdaptation: example(
      '设计稿一键多端适配',
      '从一份设计稿快速推演出桌面端、平板端和移动端的适配方案与交付清单。',
      '基于我的设计稿或界面规范，产出多端适配方案。识别核心布局、可复用组件和关键断点，输出各设备的适配建议、规范变化，以及附带主要工程风险的开发交付清单。'
    ),
    inspirationFeed: example(
      '设计灵感无限供应',
      '按我的审美偏好持续投喂灵感，不只找相似图，还要给出可复用的风格方向。',
      '围绕我偏好的视觉风格，持续收集和整理优质参考。按版式、色彩、材质、氛围和交互语言分类，推荐相邻风格案例，并把每一轮整理成可直接开工的情绪板和风格说明。'
    ),
    sentimentRadar: example(
      '用户评论情感雷达',
      '实时听见评论区和社媒里的情绪变化，发现用户真正关心却还没被满足的点。',
      '持续监控社区帖子、社媒讨论和视频评论，识别情绪、重复抱怨、高频期待和观点分化。不要只停留在情感分类，还要总结用户真正关心的问题、可能的用户群体、未被满足的需求，以及值得验证的产品机会。'
    ),
    dreamDialogue: example(
      '梦境符号深度对话',
      '不替我下结论，而是通过追问帮我从梦里看见现实里的线索。',
      '当我描述梦境时，请不要直接给出标准答案。通过连续追问，把梦里的场景、人物和情绪与现实经历联系起来，逐步建立属于我的梦境符号词典，并记录反复出现的主题变化。'
    ),
    aristotleDialogue: example(
      '灵魂对话：亚里士多德',
      '把亚里士多德的公开著作和方法论整理成一个可长期对话的思想容器。',
      '请围绕亚里士多德的著作、可信史料和核心思想，搭建一套可对话的知识容器。提炼他的概念体系、价值判断、论证方式和常见追问框架，并在回答我时尽量保持他的思考风格，同时明确区分原典观点、合理推断和现代延伸。'
    ),
  }),
  'zh-TW': buildMessages({
    memoryBank: example(
      '數位記憶永生銀行',
      '把聊天、郵件、照片和文件串成可搜尋的人生時間線，隨時找回某個人、某件事、某個階段。',
      '請把我的聊天紀錄、郵件、照片中繼資料和文件整理成個人記憶系統。抽取時間、地點、人物、事件和關鍵字，必要時用 OCR 辨識圖片文字，建立可搜尋的時間線和主題索引，並在我追問時按發生了什麼、證據和時間脈絡回答。'
    ),
    readingCompanion: example(
      '每日閱讀伴讀',
      '每天替我挑出最值得讀的內容，不只總結，還要幫我建立理解和複盤閉環。',
      '持續追蹤我關注主題的文章來源，每天篩出最值得讀的 3 篇。為每篇生成摘要、關鍵觀點、可爭議點和 2 個討論問題，按主題歸檔，並記錄我的閱讀進度和興趣變化。'
    ),
    growthMap: example(
      '個人學習成長地圖',
      '把課程、專案、筆記和複習節奏拼成一張動態技能地圖，知道下一步最該學什麼。',
      '請把我的課程進度、專案提交、筆記和知識卡片整合成個人成長地圖。辨識我已掌握的技能、薄弱點和停滯環節，安排間隔複習，並推薦下一步最值得投入的學習資源和練習任務。'
    ),
    contentPlanner: example(
      '多平台內容策劃',
      '盯住 X、Reddit、YouTube 的熱點，把趨勢變成一整套可發布的內容選題。',
      '持續追蹤 X、Reddit 和 YouTube 上快速升溫的 AI 軟體話題。提煉正在擴散的敘事、爭議和觀點，再生成一週的內容選題、標題角度、腳本提綱、發布時間建議和復盤指標。'
    ),
    chipMarketBriefing: example(
      '晶片行情開盤必讀',
      '開盤前 30 分鐘，用一頁簡報看完 NVDA、AMD 和晶片鏈的關鍵變化。',
      '請圍繞 NVDA、AMD 和半導體產業鏈生成一份盤前簡報。總結隔夜價格變動、重要新聞、分析師觀點、市場情緒和潛在催化因素，並按發生了什麼、為什麼重要、今天看什麼來輸出。'
    ),
    morningBriefing: example(
      '個人資料晨間簡報',
      '把行程、郵件、待辦和新聞合成一份真正有優先順序的晨報。',
      '每天早上為我生成一份晨間簡報，整合行事曆安排、郵件動態、待辦事項和相關領域新聞。按優先順序列出今天最重要的 3 件事，提示可能風險，並給出最合適的行動順序，讓我在 3 分鐘內進入狀態。'
    ),
    macroTracker: example(
      '經濟數據追蹤解讀',
      '盯住關鍵宏觀指標變化，幫我把知道數據更新了變成理解它意味著什麼。',
      '持續監控美聯儲、統計機構等公開宏觀數據，建立歷史對比和異常提醒。每次數據更新後，解釋發生了什麼、對不同資產可能意味著什麼、市場可能誤讀的地方，以及接下來值得關注的情景。'
    ),
    designAdaptation: example(
      '設計稿一鍵多端適配',
      '從一份設計稿快速推演出桌面端、平板端和行動端的適配方案與交付清單。',
      '基於我的設計稿或介面規範，產出多端適配方案。辨識核心佈局、可重用元件和關鍵斷點，輸出各裝置的適配建議、規範變化，以及附帶主要工程風險的開發交付清單。'
    ),
    inspirationFeed: example(
      '設計靈感無限供應',
      '按我的審美偏好持續投餵靈感，不只找相似圖，還要給出可重用的風格方向。',
      '圍繞我偏好的視覺風格，持續收集和整理優質參考。按版式、色彩、材質、氛圍和互動語言分類，推薦相鄰風格案例，並把每一輪整理成可直接開工的情緒板和風格說明。'
    ),
    sentimentRadar: example(
      '使用者評論情感雷達',
      '即時聽見評論區和社群裡的情緒變化，發現使用者真正關心卻還沒被滿足的點。',
      '持續監控社群貼文、社媒討論和影片評論，辨識情緒、重複抱怨、高頻期待和觀點分化。不要只停留在情感分類，還要總結使用者真正關心的問題、可能的使用者群體、未被滿足的需求，以及值得驗證的產品機會。'
    ),
    dreamDialogue: example(
      '夢境符號深度對話',
      '不替我下結論，而是透過追問幫我從夢裡看見現實裡的線索。',
      '當我描述夢境時，請不要直接給出標準答案。透過連續追問，把夢裡的場景、人物和情緒與現實經歷連結起來，逐步建立屬於我的夢境符號詞典，並記錄反覆出現的主題變化。'
    ),
    aristotleDialogue: example(
      '靈魂對話：亞里士多德',
      '把亞里士多德的公開著作和方法論整理成一個可長期對話的思想容器。',
      '請圍繞亞里士多德的著作、可信史料和核心思想，搭建一套可對話的知識容器。提煉他的概念體系、價值判斷、論證方式和常見追問框架，並在回答我時盡量保持他的思考風格，同時明確區分原典觀點、合理推斷和現代延伸。'
    ),
  }),
  'ja-JP': buildMessages({
    memoryBank: example(
      'デジタル記憶バンク',
      'チャット、メール、写真、文書を検索できる人生のタイムラインにまとめ、必要な記憶をすぐ取り出せます。',
      '私のチャット履歴、メール、写真のメタデータ、文書を個人向けの記憶システムとして整理してください。日時、場所、人物、出来事、キーワードを抽出し、必要に応じて OCR で画像内の文字を読み取り、検索可能なタイムラインとトピック索引を作成し、後からの質問には何が起きたか、根拠、時系列で答えてください。'
    ),
    readingCompanion: example(
      '毎日の読書コンパニオン',
      '毎日読む価値の高い内容を選び、要約だけでなく理解と振り返りまで支えてくれます。',
      '私が関心を持つテーマの情報源を継続的に追跡し、毎日読む価値が最も高い 3 本を選んでください。各記事について要約、主要な論点、議論になりそうな点、2 つの対話用質問を作成し、テーマ別に保存しながら読書の進捗と興味の変化も追跡してください。'
    ),
    growthMap: example(
      '個人学習成長マップ',
      'コース、プロジェクト、ノート、復習習慣を動的なスキルマップにまとめ、次に何を学ぶべきかを示します。',
      '私のコース進捗、プロジェクトのコミット、ノート、知識カードを統合して個人の成長マップを作ってください。身についているスキル、弱点、停滞している領域を特定し、間隔反復を計画し、次に時間を使う価値が高い学習資源と練習課題を勧めてください。'
    ),
    contentPlanner: example(
      'マルチプラットフォームコンテンツ企画',
      'X、Reddit、YouTube のトレンドを見張り、公開できるコンテンツ企画に変えます。',
      'X、Reddit、YouTube で加熱している AI ソフトウェアの話題を継続的に追跡してください。広がりつつある物語、対立、論点を抽出し、それを 1 週間分の企画案、タイトル角度、台本アウトライン、投稿時間の提案、振り返り指標に変換してください。'
    ),
    chipMarketBriefing: example(
      '半導体相場の寄り前ブリーフ',
      '寄り付き 30 分前に 1 枚の要約で NVDA、AMD、半導体チェーンの変化を把握できます。',
      'NVDA、AMD、そして半導体サプライチェーンを中心に寄り前ブリーフィングを作成してください。夜間の価格変動、重要ニュース、アナリスト見解、市場センチメント、想定される材料をまとめ、何が起きたか、なぜ重要か、今日何を見るべきかの形で整理してください。'
    ),
    morningBriefing: example(
      '個人モーニングブリーフ',
      '予定、メール、タスク、ニュースを本当に優先順位のある朝の要約にまとめます。',
      '毎朝、カレンダー、メールの動き、タスク、関連ニュースをまとめたモーニングブリーフを作成してください。今日もっとも重要な 3 つの優先事項、想定されるリスク、最適な行動順序を示し、3 分で仕事に入れるようにしてください。'
    ),
    macroTracker: example(
      '経済データ解読',
      'マクロ指標の更新を追い、単なる数字の変化を実用的な意味に変えます。',
      'FRB、統計当局などの公開マクロデータを継続的に監視し、過去比較と異常アラートを維持してください。新しいデータが出たら、何が変わったのか、資産クラスごとに何を意味し得るのか、市場が誤読しやすい点、次に注目すべきシナリオを説明してください。'
    ),
    designAdaptation: example(
      'ワンクリック多端末適応',
      '1 つのデザインからデスクトップ、タブレット、モバイル向けの適応案と引き継ぎチェックリストを素早く作れます。',
      '私のデザインファイルや UI 仕様をもとに、多端末向けの適応計画を作成してください。中核レイアウト、再利用できるコンポーネント、主要ブレークポイントを特定し、端末別の調整案、仕様変更点、主要な実装リスクを含む引き継ぎチェックリストを出力してください。'
    ),
    inspirationFeed: example(
      '無限デザインインスピレーション',
      '私の好みを学び、似ているだけでなく再利用しやすい参考例を継続的に届けます。',
      '私が好むビジュアルスタイルに沿って、質の高い参考例を集めて整理してください。レイアウト、色、質感、ムード、インタラクション言語ごとに分類し、隣接する参考例も勧め、すぐ使えるムードボードとスタイル方針メモにまとめてください。'
    ),
    sentimentRadar: example(
      'ユーザー感情レーダー',
      'コメント欄やコミュニティの感情の揺れを捉え、まだ解決されていない本当の関心事を見つけます。',
      'コミュニティ投稿、SNS の議論、動画コメントを監視し、感情、繰り返される不満、頻出する期待、意見の分極を特定してください。感情ラベルだけで終わらせず、ユーザーが本当に気にしている問題、想定されるセグメント、満たされていないニーズ、検証すべきプロダクト機会を要約してください。'
    ),
    dreamDialogue: example(
      '夢の象徴対話',
      '夢を勝手に解釈せず、現実とのつながりを見つける問いを返してくれます。',
      '私が夢を説明したとき、定型的な解釈に飛びつかないでください。夢の場面、人物、感情を現実の経験につなげるような追質問を重ね、個人的な夢の象徴辞典を作り、繰り返し現れるテーマの変化も追跡してください。'
    ),
    aristotleDialogue: example(
      '魂の対話：アリストテレス',
      'アリストテレスの著作と方法を、長く思考を続けられる対話用コンテナにまとめます。',
      'アリストテレスの著作、信頼できる史料、中心思想をもとに、対話可能な知識コンテナを構築してください。彼の概念体系、価値判断、論証スタイル、繰り返し現れる問いの型を整理し、回答時には彼らしい思考様式を保ちつつ、原典の立場、妥当な推論、現代的な拡張を明確に区別してください。'
    ),
  }),
  'ko-KR': buildMessages({
    memoryBank: example(
      '디지털 기억 은행',
      '채팅, 이메일, 사진, 문서를 검색 가능한 인생 타임라인으로 정리해 필요할 때 바로 꺼내볼 수 있습니다.',
      '내 채팅 기록, 이메일, 사진 메타데이터, 문서를 개인 기억 시스템으로 정리해 주세요. 시간, 장소, 사람, 사건, 키워드를 추출하고 필요하면 OCR 로 이미지 속 글자를 읽어 검색 가능한 타임라인과 주제 인덱스를 만들고, 나중 질문에는 무엇이 있었는지, 근거, 시간 순서로 답해 주세요.'
    ),
    readingCompanion: example(
      '매일 읽기 동반자',
      '매일 가장 읽을 가치가 높은 내용을 골라 주고, 요약을 넘어 이해와 복기까지 도와줍니다.',
      '내가 관심 있는 주제의 출처를 계속 추적해서 매일 가장 읽을 가치가 높은 글 3개를 골라 주세요. 각 글마다 요약, 핵심 아이디어, 논쟁 지점, 토론 질문 2개를 만들고, 주제별로 보관하면서 내 읽기 진행도와 관심 변화도 추적해 주세요.'
    ),
    growthMap: example(
      '개인 학습 성장 지도',
      '강의, 프로젝트, 노트, 복습 습관을 살아 있는 스킬 맵으로 묶어 다음에 무엇을 배워야 하는지 알려줍니다.',
      '내 강의 진행도, 프로젝트 커밋, 노트, 지식 카드를 합쳐 개인 성장 지도를 만들어 주세요. 이미 갖춘 기술, 약한 부분, 정체 구간을 찾아 간격 복습을 계획하고, 다음에 시간을 투자할 가치가 높은 학습 자료와 연습 과제를 추천해 주세요.'
    ),
    contentPlanner: example(
      '멀티플랫폼 콘텐츠 기획',
      'X, Reddit, YouTube 의 흐름을 읽고 바로 발행 가능한 콘텐츠 파이프라인으로 바꿉니다.',
      'X, Reddit, YouTube 에서 빠르게 달아오르는 AI 소프트웨어 주제를 지속적으로 추적해 주세요. 확산 중인 서사, 갈등, 핵심 논점을 뽑아 일주일치 콘텐츠 아이디어, 제목 각도, 스크립트 개요, 게시 시간 제안, 회고 지표로 바꿔 주세요.'
    ),
    chipMarketBriefing: example(
      '반도체 장 시작 브리프',
      '개장 30분 전에 한 장 요약으로 NVDA, AMD, 반도체 체인의 변화를 파악합니다.',
      'NVDA, AMD, 그리고 반도체 공급망을 중심으로 장 시작 전 브리프를 만들어 주세요. 밤사이 가격 변동, 중요한 뉴스, 애널리스트 의견, 시장 심리, 가능한 촉매를 요약하고, 무엇이 일어났는지, 왜 중요한지, 오늘 무엇을 봐야 하는지로 정리해 주세요.'
    ),
    morningBriefing: example(
      '개인 아침 브리프',
      '일정, 이메일, 할 일, 뉴스를 실제 우선순위가 있는 아침 브리프로 합칩니다.',
      '매일 아침 내 캘린더, 이메일 활동, 할 일, 관련 뉴스를 묶은 아침 브리프를 만들어 주세요. 오늘 가장 중요한 우선순위 3개를 정리하고, 예상 위험을 알려 주며, 3분 안에 감을 잡을 수 있도록 최적의 실행 순서를 제안해 주세요.'
    ),
    macroTracker: example(
      '경제 데이터 해석기',
      '거시 지표 발표를 추적해 숫자 업데이트를 실전 의미로 바꿔 줍니다.',
      '연준, 통계 기관 등에서 공개하는 거시 데이터를 지속적으로 모니터링하고, 과거 비교와 이상 징후 알림을 유지해 주세요. 새 데이터가 나오면 무엇이 바뀌었는지, 자산군별로 어떤 의미가 있을 수 있는지, 시장이 어디서 오해할 수 있는지, 다음에 주목할 시나리오를 설명해 주세요.'
    ),
    designAdaptation: example(
      '원클릭 멀티디바이스 적응',
      '하나의 디자인에서 데스크톱, 태블릿, 모바일용 적응안과 핸드오프 체크리스트를 빠르게 만듭니다.',
      '내 디자인 파일이나 UI 명세를 바탕으로 멀티디바이스 적응 계획을 만들어 주세요. 핵심 레이아웃, 재사용 가능한 컴포넌트, 주요 브레이크포인트를 파악하고, 기기별 가이드, 명세 변경점, 주요 엔지니어링 위험이 담긴 핸드오프 체크리스트를 출력해 주세요.'
    ),
    inspirationFeed: example(
      '무한 디자인 영감 공급',
      '내 취향을 학습해 비슷할 뿐 아니라 실제로 재활용 가능한 레퍼런스를 계속 추천합니다.',
      '내가 선호하는 시각 스타일을 중심으로 좋은 레퍼런스를 수집하고 정리해 주세요. 레이아웃, 색, 질감, 무드, 인터랙션 언어별로 분류하고, 인접한 참고 사례도 추천하며, 바로 쓸 수 있는 무드보드와 스타일 브리프로 묶어 주세요.'
    ),
    sentimentRadar: example(
      '사용자 감정 레이더',
      '댓글과 커뮤니티 전반의 감정 변화를 듣고 아직 해결되지 않은 진짜 관심사를 찾아냅니다.',
      '커뮤니티 글, 소셜 대화, 영상 댓글을 모니터링해 감정, 반복 불만, 자주 나오는 기대, 의견 분화를 식별해 주세요. 감정 분류에만 머물지 말고 사용자가 실제로 신경 쓰는 문제, 예상 사용자군, 충족되지 않은 니즈, 검증할 만한 제품 기회를 요약해 주세요.'
    ),
    dreamDialogue: example(
      '꿈 상징 대화',
      '내 꿈을 대신 해석하지 않고 현실과 연결되는 질문을 던져 줍니다.',
      '내가 꿈을 설명하면 표준화된 해석으로 바로 넘어가지 말아 주세요. 꿈속 장면, 인물, 감정을 현실 경험과 연결하는 후속 질문을 이어 가고, 개인적인 꿈 상징 사전을 만들며, 반복되는 주제의 변화를 추적해 주세요.'
    ),
    aristotleDialogue: example(
      '영혼의 대화: 아리스토텔레스',
      '아리스토텔레스의 저작과 방법을 오래 두고 사고할 수 있는 대화형 지식 용기로 만듭니다.',
      '아리스토텔레스의 저작, 신뢰할 수 있는 사료, 핵심 사상을 바탕으로 대화 가능한 지식 컨테이너를 만들어 주세요. 그의 개념 체계, 가치 판단, 논증 방식, 반복되는 질문 패턴을 정리하고, 답변할 때는 그의 사고 스타일을 최대한 유지하되 원전의 입장, 타당한 추론, 현대적 확장을 분명히 구분해 주세요.'
    ),
  }),
  'de-DE': buildMessages({
    memoryBank: example(
      'Digitale Erinnerungsbank',
      'Verwandle Chats, E-Mails, Fotos und Dokumente in eine durchsuchbare Lebenszeitleiste, auf die ich jederzeit zurückgreifen kann.',
      'Ordne meinen Chatverlauf, meine E-Mails, Fotometadaten und Dokumente zu einem persönlichen Erinnerungssystem. Extrahiere Zeit, Ort, Personen, Ereignisse und Schlüsselwörter, nutze bei Bedarf OCR, erstelle eine durchsuchbare Zeitleiste und einen Themenindex und beantworte spätere Fragen mit Ereignis, Belegen und Chronologie.'
    ),
    readingCompanion: example(
      'Täglicher Lesebegleiter',
      'Wähle jeden Tag die lesenswertesten Inhalte für mich aus und hilf mir, sie wirklich zu verstehen und zu reflektieren.',
      'Verfolge Quellen zu den Themen, die mich interessieren, und zeige mir jeden Tag die drei lesenswertesten Beiträge. Liefere zu jedem Beitrag eine Zusammenfassung, die wichtigsten Gedanken, strittige Punkte und zwei Diskussionsfragen, archiviere alles nach Themen und verfolge meinen Lesefortschritt sowie meine Interessenentwicklung.'
    ),
    growthMap: example(
      'Persönliche Wachstumskarte',
      'Mache aus Kursen, Projekten, Notizen und Wiederholungsgewohnheiten eine lebendige Skill-Map, die zeigt, was ich als Nächstes lernen sollte.',
      'Kombiniere meine Kursfortschritte, Projekt-Commits, Notizen und Wissenskarten zu einer persönlichen Wachstumskarte. Erkenne vorhandene Fähigkeiten, Schwachstellen und Stillstände, plane verteilte Wiederholungen und empfehle mir als Nächstes die wertvollsten Lernressourcen und Übungsaufgaben.'
    ),
    contentPlanner: example(
      'Plattformübergreifende Content-Planung',
      'Beobachte Trends auf X, Reddit und YouTube und verwandle sie in eine veröffentlichungsreife Content-Pipeline.',
      'Verfolge schnell wachsende KI-Software-Themen auf X, Reddit und YouTube. Extrahiere die Narrative, Konflikte und Gesprächspunkte, die gerade Traktion bekommen, und forme daraus eine Woche voller Content-Ideen mit Titelwinkeln, Skriptgliederungen, Veröffentlichungsvorschlägen und Review-Metriken.'
    ),
    chipMarketBriefing: example(
      'Chip-Markt-Eröffnungsbriefing',
      'Lies eine Seite vor der Börseneröffnung und weiß sofort, was sich bei NVDA, AMD und der Halbleiterkette verändert hat.',
      'Erstelle ein Vorbörsen-Briefing zu NVDA, AMD und der Halbleiterkette. Fasse nächtliche Kursbewegungen, wichtige Nachrichten, Analystenmeinungen, Marktstimmung und mögliche Katalysatoren zusammen und gliedere das Ergebnis in was passiert ist, warum es wichtig ist und worauf heute zu achten ist.'
    ),
    morningBriefing: example(
      'Persönliches Morgenbriefing',
      'Verbinde Termine, E-Mails, Aufgaben und Nachrichten zu einem Morgenbriefing mit echten Prioritäten.',
      'Erstelle mir jeden Morgen ein Briefing, das Kalender, E-Mail-Aktivitäten, Aufgaben und relevante Nachrichten zusammenführt. Priorisiere die drei wichtigsten Punkte des Tages, markiere wahrscheinliche Risiken und schlage die beste Reihenfolge für meine nächsten Schritte vor.'
    ),
    macroTracker: example(
      'Konjunkturdaten-Decoder',
      'Verfolge Makroveröffentlichungen und verwandle rohe Updates in eine praktische Erklärung ihrer Bedeutung.',
      'Überwache öffentliche Makrodaten der Federal Reserve, Statistikämter und ähnlicher Quellen mit historischen Vergleichen und Anomaliehinweisen. Wenn neue Daten eintreffen, erkläre, was sich geändert hat, was das für verschiedene Anlageklassen bedeuten kann, wo der Markt sich irren könnte und welche Szenarien nun wichtig sind.'
    ),
    designAdaptation: example(
      'Ein-Klick-Mehrgeräte-Anpassung',
      'Leite aus einem Design schnell Anpassungen für Desktop, Tablet und Mobile samt Handoff-Checkliste ab.',
      'Erstelle auf Basis meiner Design-Datei oder UI-Spezifikation einen Anpassungsplan für mehrere Geräte. Identifiziere Kernlayouts, wiederverwendbare Komponenten und wichtige Breakpoints und gib gerätespezifische Hinweise, Spezifikationsänderungen und eine Handoff-Checkliste mit den größten Entwicklungsrisiken aus.'
    ),
    inspirationFeed: example(
      'Unendliche Design-Inspiration',
      'Lerne meinen Geschmack und liefere mir Referenzen, die nicht nur ähnlich sind, sondern wirklich weiterhelfen.',
      'Sammle und ordne starke Referenzen rund um die visuellen Stile, die ich bevorzuge. Sortiere sie nach Layout, Farbe, Materialität, Stimmung und Interaktionssprache, empfehle angrenzende Beispiele und fasse jede Runde in einem Moodboard und einer Stilrichtung zusammen, die ich sofort nutzen kann.'
    ),
    sentimentRadar: example(
      'Nutzerstimmungs-Radar',
      'Höre emotionale Verschiebungen in Kommentaren und Communities und erkenne, was Nutzer wirklich bewegt und noch ungelöst ist.',
      'Überwache Community-Posts, Social-Diskussionen und Videokommentare, um Stimmung, wiederkehrende Beschwerden, häufige Erwartungen und polarisierte Sichtweisen zu erkennen. Bleibe nicht bei Stimmungslabels stehen, sondern fasse die eigentlichen Probleme, wahrscheinliche Nutzersegmente, unerfüllte Bedürfnisse und validierbare Produktchancen zusammen.'
    ),
    dreamDialogue: example(
      'Traumsymbol-Dialog',
      'Erkläre mir meinen Traum nicht einfach, sondern stelle die Fragen, die ihn mit meinem echten Leben verbinden.',
      'Wenn ich einen Traum beschreibe, springe nicht sofort zu einer Standarddeutung. Stelle kluge Rückfragen, die Szenen, Personen und Emotionen des Traums mit realen Erfahrungen verbinden, baue ein persönliches Traumsymbol-Lexikon auf und verfolge wiederkehrende Themen im Zeitverlauf.'
    ),
    aristotleDialogue: example(
      'Seelendialog: Aristoteles',
      'Mache aus den Schriften und Methoden des Aristoteles ein langlebiges Denkgefäß, mit dem ich weiterdenken kann.',
      'Baue einen dialogfähigen Wissenscontainer rund um die Werke des Aristoteles, verlässliche historische Quellen und seine Kernideen. Verdichte seine Begriffe, Werturteile, Argumentationsweise und wiederkehrenden Fragemuster und antworte in seinem Stil, während du Ursprungstext, plausible Schlussfolgerung und moderne Erweiterung klar trennst.'
    ),
  }),
  'fr-FR': buildMessages({
    memoryBank: example(
      'Banque de mémoire numérique',
      'Transforme mes échanges, e-mails, photos et documents en une chronologie de vie consultable à tout moment.',
      'Organise mon historique de chat, mes e-mails, les métadonnées de mes photos et mes documents en un système de mémoire personnel. Extrais le temps, le lieu, les personnes, les événements et les mots-clés, utilise l’OCR si nécessaire, construis une chronologie et un index thématique consultables, puis réponds aux questions ultérieures avec ce qui s’est passé, les preuves et la chronologie.'
    ),
    readingCompanion: example(
      'Compagnon de lecture quotidien',
      'Choisis chaque jour ce qui mérite vraiment d’être lu, puis aide-moi à le comprendre et à y réfléchir.',
      'Suis les sources autour des thèmes qui m’intéressent et fais remonter chaque jour les trois lectures les plus utiles. Pour chacune, fournis un résumé, les idées clés, les points discutables et deux questions de discussion, archive le tout par thème et suis ma progression de lecture ainsi que l’évolution de mes centres d’intérêt.'
    ),
    growthMap: example(
      'Carte de progression personnelle',
      'Transforme cours, projets, notes et habitudes de révision en une carte de compétences vivante qui me dit quoi apprendre ensuite.',
      'Combine mes cours, mes commits de projet, mes notes et mes fiches de connaissance en une carte de progression personnelle. Identifie les compétences acquises, mes points faibles et les zones de stagnation, planifie la révision espacée et recommande les ressources et exercices les plus utiles pour la suite.'
    ),
    contentPlanner: example(
      'Planificateur de contenu multi-plateforme',
      'Observe les tendances sur X, Reddit et YouTube, puis transforme-les en un pipeline de contenu prêt à publier.',
      'Suis les sujets IA logiciels qui montent rapidement sur X, Reddit et YouTube. Extrais les récits, conflits et angles qui prennent de l’ampleur, puis transforme-les en une semaine d’idées de contenu avec angles de titre, plans de script, suggestions d’horaire de publication et indicateurs de revue.'
    ),
    chipMarketBriefing: example(
      'Brief d’ouverture des semi-conducteurs',
      'Lis une page avant l’ouverture et comprends ce qui a changé sur NVDA, AMD et la chaîne des puces.',
      'Génère un brief pré-ouverture centré sur NVDA, AMD et la chaîne des semi-conducteurs. Résume les mouvements de prix de la nuit, les actualités importantes, les avis d’analystes, le sentiment de marché et les catalyseurs possibles, puis présente ce qui s’est passé, pourquoi c’est important et ce qu’il faut surveiller aujourd’hui.'
    ),
    morningBriefing: example(
      'Brief matinal personnel',
      'Fusionne mon agenda, mes e-mails, mes tâches et l’actualité en un brief matinal avec de vraies priorités.',
      'Crée chaque matin un brief qui rassemble mon calendrier, l’activité e-mail, mes tâches et les actualités pertinentes. Classe les trois priorités du jour, signale les risques probables et propose le meilleur ordre d’action pour que je sois opérationnel en trois minutes.'
    ),
    macroTracker: example(
      'Décodeur de données économiques',
      'Suis les publications macro et transforme les mises à jour brutes en explication concrète de leur portée.',
      'Surveille les données macro publiques de la Réserve fédérale, des instituts statistiques et de sources similaires avec comparaisons historiques et alertes d’anomalie. À chaque nouvelle publication, explique ce qui a changé, ce que cela peut signifier pour les différentes classes d’actifs, où le marché peut se tromper et quels scénarios méritent de l’attention.'
    ),
    designAdaptation: example(
      'Adaptation multi-appareil en un clic',
      'À partir d’un seul design, déduis rapidement des adaptations desktop, tablette et mobile avec une checklist de handoff.',
      'À partir de mon fichier de design ou de ma spec UI, produis un plan d’adaptation multi-appareil. Identifie les structures de mise en page, les composants réutilisables et les points de rupture clés, puis fournis des recommandations par appareil, les changements de spec et une checklist de handoff avec les principaux risques techniques.'
    ),
    inspirationFeed: example(
      'Inspiration design infinie',
      'Apprends mes goûts et alimente-moi en références qui ne sont pas seulement proches, mais réellement réutilisables.',
      'Collecte et organise des références fortes autour des styles visuels que je préfère. Classe-les par mise en page, couleur, texture, ambiance et langage d’interaction, recommande des références voisines et transforme chaque sélection en moodboard et note de direction visuelle directement exploitables.'
    ),
    sentimentRadar: example(
      'Radar de sentiment utilisateur',
      'Écoute les variations émotionnelles dans les commentaires et les communautés, puis détecte ce qui préoccupe vraiment les utilisateurs sans encore être résolu.',
      'Surveille les posts de communauté, les discussions sociales et les commentaires vidéo pour identifier le sentiment, les plaintes répétées, les attentes fréquentes et les points de vue polarisés. Va au-delà des étiquettes de sentiment et résume les vrais problèmes, les segments probables, les besoins non satisfaits et les opportunités produit à valider.'
    ),
    dreamDialogue: example(
      'Dialogue des symboles du rêve',
      'N’explique pas mon rêve à ma place. Pose-moi les questions qui m’aident à le relier à ma vie réelle.',
      'Quand je décris un rêve, ne saute pas vers une interprétation standard. Pose des questions de suivi qui relient les scènes, les personnes et les émotions du rêve à mes expériences réelles, construis un lexique personnel des symboles de rêve et suis l’évolution des thèmes récurrents.'
    ),
    aristotleDialogue: example(
      'Dialogue de l’âme : Aristote',
      'Transforme les écrits et la méthode d’Aristote en un conteneur durable avec lequel je peux continuer à penser.',
      'Construis un conteneur de connaissance prêt pour le dialogue autour des œuvres d’Aristote, de sources historiques fiables et de ses idées centrales. Dégage ses concepts, ses jugements de valeur, son style argumentatif et ses schémas de questionnement, puis réponds dans son style en distinguant clairement doctrine originale, inférence raisonnable et prolongement moderne.'
    ),
  }),
  'es-ES': buildMessages({
    memoryBank: example(
      'Banco de memoria digital',
      'Convierte chats, correos, fotos y documentos en una línea de vida consultable cuando la necesite.',
      'Organiza mi historial de chat, correos, metadatos de fotos y documentos en un sistema personal de memoria. Extrae tiempo, lugar, personas, eventos y palabras clave, usa OCR cuando haga falta, crea una línea temporal y un índice temático consultables y responde después con qué pasó, qué pruebas hay y cuál fue la cronología.'
    ),
    readingCompanion: example(
      'Compañero diario de lectura',
      'Elige cada día lo más valioso para leer y ayúdame no solo a resumirlo, sino a entenderlo y repasarlo.',
      'Sigue las fuentes sobre los temas que me interesan y selecciona cada día las tres lecturas más valiosas. Para cada una, genera un resumen, ideas clave, puntos debatibles y dos preguntas de discusión, archívalas por tema y sigue mi progreso de lectura y la evolución de mis intereses.'
    ),
    growthMap: example(
      'Mapa de crecimiento personal',
      'Convierte cursos, proyectos, notas y hábitos de repaso en un mapa vivo de habilidades que me diga qué aprender después.',
      'Combina mis cursos, commits de proyectos, notas y tarjetas de conocimiento en un mapa de crecimiento personal. Identifica las habilidades ya adquiridas, mis puntos débiles y las áreas estancadas, programa repasos espaciados y recomienda los recursos y ejercicios más valiosos para el siguiente paso.'
    ),
    contentPlanner: example(
      'Planificador de contenido multiplataforma',
      'Observa las tendencias de X, Reddit y YouTube y conviértelas en un flujo de contenido listo para publicar.',
      'Sigue los temas de software de IA que están acelerando en X, Reddit y YouTube. Extrae las narrativas, conflictos y ángulos que están ganando tracción y conviértelos en una semana de ideas de contenido con enfoques de título, esquemas de guion, horarios sugeridos y métricas para revisar resultados.'
    ),
    chipMarketBriefing: example(
      'Brief de apertura del mercado de chips',
      'Lee una página antes de la apertura y entiende qué cambió en NVDA, AMD y la cadena de semiconductores.',
      'Genera un brief previo a la apertura centrado en NVDA, AMD y la cadena de semiconductores. Resume movimientos nocturnos de precio, noticias importantes, opiniones de analistas, sentimiento de mercado y posibles catalizadores, y preséntalo como qué pasó, por qué importa y qué vigilar hoy.'
    ),
    morningBriefing: example(
      'Brief matinal personal',
      'Une mi agenda, correo, tareas y noticias en un resumen matinal con prioridades reales.',
      'Crea cada mañana un brief que reúna mi calendario, actividad de correo, tareas y noticias relevantes. Ordena las tres prioridades más importantes del día, señala riesgos probables y sugiere el mejor orden de acción para situarme en tres minutos.'
    ),
    macroTracker: example(
      'Decodificador de datos económicos',
      'Sigue las publicaciones macro y convierte actualizaciones en bruto en una explicación práctica de lo que significan.',
      'Supervisa datos macro públicos de la Reserva Federal, oficinas estadísticas y fuentes similares con comparaciones históricas y alertas de anomalías. Cuando lleguen nuevos datos, explica qué cambió, qué puede significar para distintas clases de activos, dónde puede malinterpretarlo el mercado y qué escenarios merecen atención.'
    ),
    designAdaptation: example(
      'Adaptación multidispositivo en un clic',
      'A partir de un diseño, deriva rápidamente guías para escritorio, tableta y móvil junto con una checklist de entrega.',
      'A partir de mi archivo de diseño o especificación de UI, produce un plan de adaptación multidispositivo. Identifica estructuras de maquetación, componentes reutilizables y puntos de quiebre clave, y entrega guía por dispositivo, cambios de especificación y una checklist de handoff con los principales riesgos de ingeniería.'
    ),
    inspirationFeed: example(
      'Inspiración de diseño infinita',
      'Aprende mi gusto y sígueme dando referencias que no solo se parezcan, sino que realmente se puedan reutilizar.',
      'Recoge y organiza referencias sólidas en torno a los estilos visuales que prefiero. Clasifícalas por composición, color, textura, atmósfera y lenguaje de interacción, recomienda referencias vecinas y empaqueta cada ronda en un moodboard y una guía de estilo lista para usar.'
    ),
    sentimentRadar: example(
      'Radar de sentimiento del usuario',
      'Escucha los cambios emocionales en comentarios y comunidades y detecta qué preocupa de verdad a los usuarios y sigue sin resolverse.',
      'Monitorea publicaciones de comunidades, conversaciones sociales y comentarios de vídeo para identificar sentimiento, quejas repetidas, expectativas frecuentes y puntos de vista polarizados. No te quedes en etiquetas de sentimiento: resume los problemas reales, los segmentos probables, las necesidades no cubiertas y las oportunidades de producto que merece validar.'
    ),
    dreamDialogue: example(
      'Diálogo de símbolos del sueño',
      'No me expliques el sueño por mí. Hazme las preguntas que me ayuden a conectarlo con la vida real.',
      'Cuando describa un sueño, no saltes a una interpretación estándar. Haz preguntas de seguimiento que conecten las escenas, personas y emociones del sueño con experiencias reales, construye un léxico personal de símbolos oníricos y sigue los temas recurrentes con el tiempo.'
    ),
    aristotleDialogue: example(
      'Diálogo del alma: Aristóteles',
      'Convierte los escritos y el método de Aristóteles en un contenedor duradero con el que pueda seguir pensando.',
      'Construye un contenedor de conocimiento listo para dialogar a partir de las obras de Aristóteles, fuentes históricas fiables y sus ideas centrales. Destila sus conceptos, juicios de valor, estilo argumentativo y patrones de preguntas recurrentes, y respóndeme en su estilo separando con claridad doctrina original, inferencia razonable y extensión moderna.'
    ),
  }),
  'it-IT': buildMessages({
    memoryBank: example(
      'Banca della memoria digitale',
      'Trasforma chat, email, foto e documenti in una timeline della mia vita consultabile in qualsiasi momento.',
      'Organizza la mia cronologia chat, le email, i metadati delle foto e i documenti in un sistema personale della memoria. Estrai tempo, luogo, persone, eventi e parole chiave, usa OCR quando serve, costruisci una timeline e un indice tematico consultabili e rispondi in seguito con cosa è successo, prove e cronologia.'
    ),
    readingCompanion: example(
      'Compagno di lettura quotidiano',
      'Scegli ogni giorno i contenuti più utili da leggere e aiutami non solo a riassumerli, ma anche a capirli e rielaborarli.',
      'Segui le fonti sui temi che mi interessano e porta ogni giorno i tre contenuti più utili. Per ciascuno genera un riassunto, le idee chiave, i punti discutibili e due domande di confronto, archivia tutto per tema e tieni traccia dei miei progressi di lettura e dei miei interessi nel tempo.'
    ),
    growthMap: example(
      'Mappa di crescita personale',
      'Trasforma corsi, progetti, appunti e abitudini di ripasso in una mappa viva delle competenze che mi dica cosa imparare dopo.',
      'Combina i miei corsi, i commit dei progetti, gli appunti e le schede di conoscenza in una mappa di crescita personale. Individua le competenze già acquisite, i punti deboli e le aree ferme, pianifica il ripasso dilazionato e consigliami le prossime risorse e attività di pratica su cui vale davvero la pena investire tempo.'
    ),
    contentPlanner: example(
      'Pianificatore di contenuti multipiattaforma',
      'Osserva i trend di X, Reddit e YouTube e trasformali in un flusso di contenuti pronto per essere pubblicato.',
      'Segui i temi software AI che stanno accelerando su X, Reddit e YouTube. Estrai le narrazioni, i conflitti e i punti di discussione che stanno prendendo trazione e trasformali in una settimana di idee di contenuto con angoli di titolo, outline di script, suggerimenti sugli orari di pubblicazione e metriche di revisione.'
    ),
    chipMarketBriefing: example(
      'Brief di apertura del mercato chip',
      'Leggi una pagina prima dell’apertura e capisci subito cosa è cambiato su NVDA, AMD e la filiera dei semiconduttori.',
      'Genera un briefing pre-market focalizzato su NVDA, AMD e la filiera dei semiconduttori. Riassumi i movimenti di prezzo notturni, le notizie importanti, le opinioni degli analisti, il sentiment di mercato e i possibili catalizzatori, poi presentali come cosa è successo, perché conta e cosa osservare oggi.'
    ),
    morningBriefing: example(
      'Brief mattutino personale',
      'Unisci agenda, email, attività e notizie in un brief del mattino con priorità reali.',
      'Crea ogni mattina un brief che unisca calendario, attività email, task e notizie rilevanti. Ordina le tre priorità più importanti della giornata, segnala i rischi probabili e suggerisci il miglior ordine di azione per orientarmi in tre minuti.'
    ),
    macroTracker: example(
      'Decodificatore di dati economici',
      'Segui i rilasci macro e trasforma aggiornamenti grezzi in una spiegazione pratica del loro significato.',
      'Monitora i dati macro pubblici della Federal Reserve, degli istituti statistici e di fonti simili con confronti storici e avvisi di anomalia. Quando arrivano nuovi dati, spiega cosa è cambiato, cosa può significare per le diverse classi di attivi, dove il mercato potrebbe sbagliare lettura e quali scenari meritano attenzione.'
    ),
    designAdaptation: example(
      'Adattamento multi-dispositivo con un clic',
      'Da un solo design ricava rapidamente indicazioni per desktop, tablet e mobile insieme a una checklist di handoff.',
      'Sulla base del mio file di design o della specifica UI, produci un piano di adattamento multi-dispositivo. Individua layout principali, componenti riutilizzabili e breakpoint chiave, quindi fornisci indicazioni per dispositivo, modifiche di specifica e una checklist di handoff con i principali rischi di implementazione.'
    ),
    inspirationFeed: example(
      'Ispirazione di design infinita',
      'Impara il mio gusto e continua a propormi riferimenti che non siano solo simili, ma davvero riutilizzabili.',
      'Raccogli e organizza riferimenti di qualità intorno agli stili visivi che preferisco. Classificali per layout, colore, texture, atmosfera e linguaggio d’interazione, consiglia riferimenti vicini e trasforma ogni raccolta in un moodboard e in una nota di direzione stilistica pronta all’uso.'
    ),
    sentimentRadar: example(
      'Radar del sentiment utente',
      'Ascolta i cambiamenti emotivi tra commenti e community e individua ciò che agli utenti importa davvero e che ancora non viene risolto.',
      'Monitora post delle community, discussioni social e commenti ai video per identificare sentiment, lamentele ricorrenti, aspettative frequenti e punti di vista polarizzati. Vai oltre le etichette di sentiment e riassumi i problemi reali, i segmenti probabili, i bisogni insoddisfatti e le opportunità di prodotto da validare.'
    ),
    dreamDialogue: example(
      'Dialogo con i simboli del sogno',
      'Non interpretare il mio sogno al posto mio. Fammi le domande che mi aiutano a collegarlo alla vita reale.',
      'Quando descrivo un sogno, non passare subito a un’interpretazione standard. Fai domande di approfondimento che colleghino scene, persone ed emozioni del sogno alle esperienze reali, costruisci un lessico personale dei simboli onirici e traccia i temi ricorrenti nel tempo.'
    ),
    aristotleDialogue: example(
      'Dialogo dell’anima: Aristotele',
      'Trasforma gli scritti e il metodo di Aristotele in un contenitore durevole con cui posso continuare a pensare.',
      'Costruisci un contenitore di conoscenza pronto al dialogo intorno alle opere di Aristotele, alle fonti storiche affidabili e alle sue idee centrali. Distilla concetti, giudizi di valore, stile argomentativo e schemi di domande ricorrenti, e rispondimi nel suo stile distinguendo chiaramente dottrina originaria, inferenza ragionevole ed estensione moderna.'
    ),
  }),

  'ca-ES': buildMessages({
    memoryBank: example(
      'Banc de memòria digital',
      'Converteix els xats, els correus electrònics, les fotos i els documents en una línia de temps de recerca que pots tornar a visitar en qualsevol moment.',
      "Organitzeu el meu historial de xat, correus electrònics, metadades de fotos i documents en un sistema de memòria personal. Extraieu el temps, el lloc, les persones, els esdeveniments i les paraules clau, feu servir l'OCR quan sigui necessari, creeu una línia de temps i un índex de temes que es puguin cercar i responeu a preguntes posteriors amb què va passar, proves i cronologia."
    ),
    readingCompanion: example(
      'Acompanyant de lectura diària',
      "Tria les coses que més val la pena per llegir-me cada dia i després ajuda'm a digerir-les i reflexionar-hi.",
      "Feu un seguiment de les fonts sobre els temes que m'importen i afloreu les tres peces que més valen la pena cada dia. Per a cada peça, proporcioneu un resum, idees clau, punts discutibles i dos missatges de discussió, arxiveu-los per tema i feu un seguiment del meu progrés de lectura i els meus interessos canviants."
    ),
    growthMap: example(
      'Mapa de creixement personal',
      "Converteix els cursos, els projectes, les notes i els hàbits de revisió en un mapa d'habilitats de vida que em digui què he d'aprendre després.",
      'Combina els meus cursos, compromisos de projectes, notes i targetes de coneixement en un mapa de creixement personal. Identifiqueu les habilitats que he desenvolupat, els meus punts febles i les àrees estancades, programeu una revisió espaiada i recomaneu els propers recursos i practiqueu tasques que valguin la pena.'
    ),
    contentPlanner: example(
      'Planificador de contingut multiplataforma',
      'Mireu les tendències de X, Reddit i YouTube i, a continuació, convertiu-les en una canalització de contingut publicable.',
      "Feu un seguiment dels temes de programari d'IA que es mouen ràpidament a X, Reddit i YouTube. Extraieu les narracions, els conflictes i els punts de discussió que guanyen força i, a continuació, convertiu-los en una setmana d'idees de contingut amb angles de títol, contorns de guió, suggeriments de temps de publicació i mètriques de revisió."
    ),
    chipMarketBriefing: example(
      'Breu obert de Chip Market',
      "Llegiu una pàgina abans de la campana d'obertura i sabeu què ha canviat a NVDA, AMD i la cadena de xips.",
      'Genereu una sessió informativa prèvia al mercat centrada en NVDA, AMD i la cadena de semiconductors. Resumeix els moviments de preus durant la nit, les notícies importants, les opinions dels analistes, el sentiment del mercat i els possibles catalitzadors i, a continuació, presenteu què ha passat, per què és important i què cal veure avui.'
    ),
    morningBriefing: example(
      'Breu personal del matí',
      'Combina la meva agenda, correu electrònic, tasques i notícies en un resum del matí amb prioritats reals.',
      "Creeu un resum del matí que combini el meu calendari, l'activitat de correu electrònic, les tasques i les notícies rellevants. Classifica les tres prioritats més importants d'avui, marca els riscos probables i proposa el millor ordre d'acció perquè pugui orientar-me en tres minuts."
    ),
    macroTracker: example(
      'Descodificador de dades econòmiques',
      'Feu un seguiment dels llançaments de macros i converteix les actualitzacions en brut en una explicació pràctica del que podrien significar.',
      "Superviseu les dades macro públiques de la Reserva Federal, agències d'estadístiques i fonts similars amb comparacions històriques i alertes d'anomalies. Quan arribin dades noves, expliqueu què ha canviat, què pot significar per a diferents classes d'actius, on el mercat pot interpretar-les malament i quins escenaris mereixen atenció."
    ),
    designAdaptation: example(
      'Adaptació multidispositiu amb un sol clic',
      "Agafeu un disseny i obteniu ràpidament una guia d'adaptació per a ordinadors, tauletes i mòbils a més d'una llista de comprovació d'entrega.",
      "En funció del meu fitxer de disseny o de les especificacions de la interfície d'usuari, produir un pla d'adaptació per a diversos dispositius. Identifiqueu dissenys bàsics, components reutilitzables i punts d'interrupció clau i, a continuació, escriviu orientacions específiques del dispositiu, canvis d'especificacions i una llista de verificació de lliurament amb riscos d'enginyeria importants."
    ),
    inspirationFeed: example(
      'Inspiració de disseny infinita',
      'Apreneu el meu gust i seguiu donant-me referències que no només són similars, sinó que en realitat són reutilitzables.',
      "Recull i organitza referències sòlides al voltant dels estils visuals que prefereixo. Classifica'ls per disseny, color, textura, estat d'ànim i llenguatge d'interacció, recomana referències adjacents i empaqueta cada ronda en un moodboard i un resum d'estil que pugui utilitzar immediatament."
    ),
    sentimentRadar: example(
      "Radar del sentiment de l'usuari",
      "Escolteu els canvis emocionals entre els comentaris i les comunitats i, a continuació, detecteu què els importa als usuaris i que encara no s'estan resolent.",
      "Superviseu les publicacions de la comunitat, les discussions socials i els comentaris de vídeo per identificar sentiments, queixes repetides, expectatives i punts de vista polaritzats. Va més enllà de les etiquetes de sentiments i resumeix els problemes reals que els preocupen els usuaris, els segments d'usuaris probables, les necessitats no satisfetes i les oportunitats de producte que val la pena validar."
    ),
    dreamDialogue: example(
      'Diàleg Símbol dels somnis',
      "No m'expliqueu el meu somni. Fes les preguntes que m'ajudin a connectar-lo a la vida real.",
      'Quan descriu un somni, no salteu a una interpretació estàndard. Feu preguntes de seguiment reflexives que connectin les escenes dels somnis, les persones i les emocions amb experiències de la vida real, creï un lèxic personal de símbols de somnis i faci un seguiment de temes recurrents al llarg del temps.'
    ),
    aristotleDialogue: example(
      "Diàleg de l'ànima: Aristòtil",
      "Converteix els escrits i els mètodes públics d'Aristòtil en un contenidor de llarga vida amb què puc seguir pensant.",
      "Construeix un contenidor de coneixement preparat per a diàlegs al voltant de les obres d'Aristòtil, fonts històriques fiables i idees bàsiques. Destil·leu els seus conceptes, valors, estil d'argumentació i patrons de preguntes recurrents, i responeu-me amb el seu estil alhora que separa clarament la doctrina original, la inferència raonable i l'extensió moderna."
    ),
  }),

  'cs-CZ': buildMessages({
    memoryBank: example(
      'Digitální paměťová banka',
      'Proměňte chaty, e-maily, fotografie a dokumenty na prohledávatelnou časovou osu, kterou můžete kdykoli znovu navštívit.',
      'Uspořádejte moji historii chatu, e-maily, metadata fotografií a dokumenty do osobního paměťového systému. Extrahujte čas, místo, lidi, události a klíčová slova, v případě potřeby použijte OCR, vytvořte si prohledávatelnou časovou osu a rejstřík témat a odpovězte na pozdější otázky pomocí toho, co se stalo, důkazů a chronologie.'
    ),
    readingCompanion: example(
      'Společník denního čtení',
      'Vyberte si ty nejcennější věci, které pro mě každý den přečtu, a pak mi je pomozte strávit a přemýšlet o nich.',
      'Sledujte zdroje týkající se témat, která mě zajímají, a každý den vystavte tři nejcennější kousky. U každého dílu uveďte shrnutí, klíčové myšlenky, diskutabilní body a dvě diskusní výzvy, archivujte je podle tématu a sledujte můj pokrok ve čtení a měnící se zájmy.'
    ),
    growthMap: example(
      'Mapa osobního růstu',
      'Udělejte z kurzů, projektů, poznámek a kontrolních návyků živou mapu dovedností, která mi řekne, co se mám dále učit.',
      'Spojte mé kurzy, projekty, poznámky a karty znalostí do mapy osobního růstu. Identifikujte dovednosti, které jsem si vybudoval, moje slabá místa a pozastavené oblasti, naplánujte si rozloženou kontrolu a doporučte další zdroje a procvičujte úkoly, které stojí za můj čas.'
    ),
    contentPlanner: example(
      'Plánovač obsahu pro více platforem',
      'Sledujte trendy X, Reddit a YouTube a pak je přeměňte na publikovatelný obsah.',
      'Sledujte rychle se měnící témata softwaru AI na X, Reddit a YouTube. Extrahujte příběhy, konflikty a mluvené body, které získávají na tahu, a pak je proměňte v týden nápadů na obsah s úhly názvů, osnovami scénáře, návrhy na čas zveřejnění a metrikami recenzí.'
    ),
    chipMarketBriefing: example(
      'Otevřený přehled chip Market',
      'Přečtěte si jednu stránku před úvodním zvonkem a zjistěte, co se změnilo napříč NVDA, AMD a řetězcem čipů.',
      'Vytvořte briefing před uvedením na trh zaměřený na NVDA, AMD a polovodičový řetězec. Shrňte pohyby cen přes noc, důležité zprávy, názory analytiků, sentiment trhu a možné katalyzátory a poté prezentujte, co se stalo, proč na tom záleží a na co se dnes dívat.'
    ),
    morningBriefing: example(
      'Osobní ranní stručná zpráva',
      'Smíchejte můj plán, e-mail, úkoly a zprávy do ranní zprávy se skutečnými prioritami.',
      'Vytvořit ranní brief, který kombinuje můj kalendář, e-mailovou aktivitu, úkoly a relevantní zprávy. Seřaďte tři nejdůležitější priority dneška, označte pravděpodobná rizika a navrhněte nejlepší pořadí akcí, abych se mohl zorientovat za tři minuty.'
    ),
    macroTracker: example(
      'Dekodér ekonomických dat',
      'Sledujte vydání maker a proměňte nezpracované aktualizace v praktické vysvětlení toho, co by mohly znamenat.',
      'Sledujte veřejná makrodata z Federálního rezervního systému, statistických agentur a podobných zdrojů pomocí historických srovnání a upozornění na anomálie. Když dorazí nová data, vysvětlete, co se změnilo, co to může znamenat pro různé třídy aktiv, kde je trh může špatně číst a které scénáře si zaslouží pozornost.'
    ),
    designAdaptation: example(
      'Adaptace na více zařízení jedním kliknutím',
      'Vezměte si jeden návrh a rychle z něj vyvodte pokyny pro přizpůsobení pro stolní počítače, tablety a mobilní zařízení a kontrolní seznam pro předání.',
      'Na základě mého souboru návrhu nebo specifikace uživatelského rozhraní vytvořte plán přizpůsobení pro více zařízení. Identifikujte rozvržení jádra, opakovaně použitelné součásti a klíčové body přerušení a poté vytiskněte pokyny pro konkrétní zařízení, změny specifikací a kontrolní seznam pro předání s hlavními technickými riziky.'
    ),
    inspirationFeed: example(
      'Nekonečná designová inspirace',
      'Naučte se můj vkus a nepřestávejte mě krmit referencemi, které jsou nejen podobné, ale ve skutečnosti znovupoužitelné.',
      'Sbírejte a organizujte silné reference kolem vizuálních stylů, které preferuji. Klasifikujte je podle rozvržení, barvy, textury, nálady a jazyka interakce, doporučte sousední reference a zabalte každé kolo do moodboardu a stylu, který mohu okamžitě použít.'
    ),
    sentimentRadar: example(
      'Radar uživatelského sentimentu',
      'Poslechněte si emocionální posuny napříč komentáři a komunitami a pak zjistěte, co uživatele zajímá a stále není vyřešeno.',
      'Sledujte příspěvky komunity, sociální diskuse a komentáře k videím, abyste identifikovali sentiment, opakované stížnosti, očekávání a polarizované názory. Jděte za hranice sentimentálních štítků a shrňte skutečné problémy, které uživatele zajímají, pravděpodobné segmenty uživatelů, nenaplněné potřeby a produktové příležitosti, které stojí za ověření.'
    ),
    dreamDialogue: example(
      'Dialog symbolů snů',
      'Nevysvětlujte za mě můj sen. Zeptejte se na otázky, které mi pomohou propojit to zpět se skutečným životem.',
      'Když popisuji sen, nepřeskakujte na standardní výklad. Pokládejte promyšlené doplňující otázky, které spojují snové scény, lidi a emoce se zážitky ze skutečného života, vytvářejte osobní lexikon snových symbolů a sledujte opakující se témata v průběhu času.'
    ),
    aristotleDialogue: example(
      'Dialog duše: Aristoteles',
      'Proměňte Aristotelovy veřejné spisy a metody v dlouhověkou nádobu, se kterou mohu dál přemýšlet.',
      'Vytvořte kontejner znalostí připravený pro dialog kolem Aristotelových děl, spolehlivých historických zdrojů a základních myšlenek. Destilujte jeho koncepty, hodnoty, styl argumentace a vzorce opakujících se otázek a odpovězte mi jeho stylem, přičemž jasně oddělujte původní doktrínu, rozumné závěry a moderní rozšíření.'
    ),
  }),

  'da-DK': buildMessages({
    memoryBank: example(
      'Digital hukommelsesbank',
      'Gør chats, e-mails, billeder og dokumenter til en søgbar livstidslinje, som du kan se igen når som helst.',
      'Organiser min chathistorik, e-mails, fotometadata og dokumenter i et personligt hukommelsessystem. Uddrag tid, sted, personer, begivenheder og nøgleord, brug OCR, når det er nødvendigt, opbyg en søgbar tidslinje og emneindeks, og besvar senere spørgsmål med hvad der skete, beviser og kronologi.'
    ),
    readingCompanion: example(
      'Daglig læseledsager',
      'Vælg de mest værdifulde ting for mig at læse hver dag, og hjælp mig derefter med at fordøje og reflektere over dem.',
      'Spor kilder omkring de emner, jeg interesserer mig for, og kom med de tre mest værdifulde stykker hver dag. For hvert stykke skal du give et resumé, centrale ideer, diskutable punkter og to diskussionsopfordringer, arkivere dem efter tema og spore mine læsefremskridt og skiftende interesser.'
    ),
    growthMap: example(
      'Personligt vækstkort',
      'Gør kurser, projekter, noter og anmeldelsesvaner til et levende færdighedskort, der fortæller mig, hvad jeg skal lære næste gang.',
      'Kombiner mine kurser, projektforpligtelser, noter og videnskort til et personligt vækstkort. Identificer de færdigheder, jeg har opbygget, mine svage punkter og områder, der er gået i stå, planlæg en gennemgang med afstand, og anbefal de næste ressourcer og øve opgaver, der er min tid værd.'
    ),
    contentPlanner: example(
      'Indholdsplanlægger på tværs af platforme',
      'Se X-, Reddit- og YouTube-trends, og forvandl dem derefter til en publicerbar indholdspipeline.',
      'Spor hurtige AI-softwareemner på tværs af X, Reddit og YouTube. Uddrag de fortællinger, konflikter og diskussionspunkter, der vinder indpas, og forvandl dem derefter til en uge med indholdsideer med titelvinkler, manuskriptkonturer, forslag til udsendelsestidspunkt og gennemgangsmålinger.'
    ),
    chipMarketBriefing: example(
      'Chip Market Open Brief',
      'Læs en side før åbningsklokken og ved, hvad der ændrede sig på tværs af NVDA, AMD og chipkæden.',
      'Generer en pre-market briefing med fokus på NVDA, AMD og halvlederkæden. Opsummer kursbevægelser fra den ene dag til den anden, vigtige nyheder, analytikersyn, markedsstemning og mulige katalysatorer, og præsentere derefter, hvad der skete, hvorfor det betyder noget, og hvad du skal se i dag.'
    ),
    morningBriefing: example(
      'Personlig morgenbrief',
      'Bland min tidsplan, e-mail, opgaver og nyheder til en morgenbrief med rigtige prioriteter.',
      'Opret en morgenbrief, der kombinerer min kalender, e-mail-aktivitet, opgaver og relevante nyheder. Ranger de tre vigtigste prioriteter for i dag, marker sandsynlige risici, og foreslå den bedste rækkefølge, så jeg kan blive orienteret på tre minutter.'
    ),
    macroTracker: example(
      'Økonomisk datadekoder',
      'Spor makroudgivelser og gør rå opdateringer til en praktisk forklaring på, hvad de kan betyde.',
      'Overvåg offentlige makrodata fra Federal Reserve, statistikbureauer og lignende kilder med historiske sammenligninger og advarsler om uregelmæssigheder. Når der kommer nye data, så forklar, hvad der er ændret, hvad det kan betyde for forskellige aktivklasser, hvor markedet kan misforstå det, og hvilke scenarier der fortjener opmærksomhed.'
    ),
    designAdaptation: example(
      'Tilpasning af flere enheder med ét klik',
      'Tag ét design, og få hurtigt en vejledning om tilpasning til desktop, tablet og mobil plus en tjekliste for overdragelse.',
      'Baseret på min designfil eller UI-specifikation, lav en tilpasningsplan for flere enheder. Identificer kernelayouts, genanvendelige komponenter og vigtige brudpunkter, og output derefter enhedsspecifik vejledning, specifikationer og en overdragelsestjekliste med store tekniske risici.'
    ),
    inspirationFeed: example(
      'Uendelig designinspiration',
      'Lær min smag og fortsæt med at give mig referencer, der ikke bare ligner, men faktisk kan genbruges.',
      'Saml og organiser stærke referencer omkring de visuelle stilarter, jeg foretrækker. Klassificer dem efter layout, farve, tekstur, stemning og interaktionssprog, anbefal tilstødende referencer, og pak hver runde ind i et moodboard og stilkort, jeg kan bruge med det samme.'
    ),
    sentimentRadar: example(
      'Brugerstemningsradar',
      'Hør de følelsesmæssige skift på tværs af kommentarer og fællesskaber, og find derefter, hvad brugerne interesserer sig for, som stadig ikke bliver løst.',
      'Overvåg fællesskabsindlæg, sociale diskussioner og videokommentarer for at identificere følelser, gentagne klager, forventninger og polariserede synspunkter. Gå ud over følelsesetiketter og opsummer de reelle problemer, brugerne bekymrer sig om, sandsynlige brugersegmenter, udækkede behov og produktmuligheder, der er værd at validere.'
    ),
    dreamDialogue: example(
      'Drømmesymbol dialog',
      'Forklar ikke min drøm for mig. Stil de spørgsmål, der hjælper mig med at forbinde det tilbage til det virkelige liv.',
      'Når jeg beskriver en drøm, skal du ikke springe til en standardfortolkning. Stil tankevækkende opfølgende spørgsmål, der forbinder drømmescenerne, -personerne og -følelserne til virkelige oplevelser, opbyg et personligt drømmesymbol-leksikon og spor tilbagevendende temaer over tid.'
    ),
    aristotleDialogue: example(
      'Sjæledialog: Aristoteles',
      "Gør Aristoteles' offentlige skrifter og metoder til en langlivet beholder, jeg kan blive ved med at tænke med.",
      "Byg en dialogklar videnbeholder omkring Aristoteles' værker, pålidelige historiske kilder og kerneideer. Destiller hans begreber, værdier, argumentstil og tilbagevendende spørgsmålsmønstre, og svar mig i hans stil, mens han klart adskiller original doktrin, rimelig slutning og moderne forlængelse."
    ),
  }),

  'el-GR': buildMessages({
    memoryBank: example(
      'Τράπεζα Ψηφιακής Μνήμης',
      'Μετατρέψτε τις συνομιλίες, τα email, τις φωτογραφίες και τα έγγραφα σε ένα χρονοδιάγραμμα ζωής με δυνατότητα αναζήτησης που μπορείτε να επισκεφτείτε ξανά ανά πάσα στιγμή.',
      'Οργανώστε το ιστορικό συνομιλιών, τα email, τα μεταδεδομένα φωτογραφιών και τα έγγραφά μου σε ένα σύστημα προσωπικής μνήμης. Εξάγετε χρόνο, τόπο, άτομα, συμβάντα και λέξεις-κλειδιά, χρησιμοποιήστε το OCR όταν χρειάζεται, δημιουργήστε ένα χρονοδιάγραμμα και ευρετήριο θεμάτων με δυνατότητα αναζήτησης και απαντήστε σε μεταγενέστερες ερωτήσεις με το τι συνέβη, τα στοιχεία και τη χρονολογία.'
    ),
    readingCompanion: example(
      'Καθημερινός σύντροφος ανάγνωσης',
      'Διάλεξε τα πιο αξιόλογα πράγματα για μένα να διαβάζω κάθε μέρα και μετά βοήθησέ με να τα αφομοιώσω και να τα σκεφτώ.',
      'Παρακολουθήστε πηγές γύρω από τα θέματα που με ενδιαφέρουν και εμφανίστε τα τρία πιο αξιόλογα κομμάτια κάθε μέρα. Για κάθε κομμάτι, παρέχετε μια περίληψη, βασικές ιδέες, συζητήσιμα σημεία και δύο προτροπές συζήτησης, αρχειοθετήστε τα ανά θέμα και παρακολουθήστε την πρόοδό μου στην ανάγνωση και την αλλαγή των ενδιαφερόντων μου.'
    ),
    growthMap: example(
      'Προσωπικός Χάρτης Ανάπτυξης',
      'Μετατρέψτε τα μαθήματα, τα έργα, τις σημειώσεις και τις συνήθειες αναθεώρησης σε έναν ζωντανό χάρτη δεξιοτήτων που μου λέει τι να μάθω στη συνέχεια.',
      'Συνδυάστε τα μαθήματα, τις δεσμεύσεις του έργου, τις σημειώσεις και τις κάρτες γνώσεων σε έναν προσωπικό χάρτη ανάπτυξης. Προσδιορίστε τις δεξιότητες που έχω δημιουργήσει, τα αδύναμα σημεία μου και τις περιοχές που έχουν σταματήσει, προγραμματίστε ανασκόπηση σε απόσταση μεταξύ τους και προτείνετε τους επόμενους πόρους και ασκήστε τις εργασίες που αξίζουν τον χρόνο μου.'
    ),
    contentPlanner: example(
      'Εργαλείο σχεδιασμού περιεχομένου μεταξύ πλατφορμών',
      'Παρακολουθήστε τις τάσεις των X, Reddit και YouTube και, στη συνέχεια, μετατρέψτε τις σε μια σειρά περιεχομένου με δυνατότητα δημοσίευσης.',
      'Παρακολουθήστε τα γρήγορα κινούμενα θέματα λογισμικού τεχνητής νοημοσύνης στο X, το Reddit και το YouTube. Εξάγετε τις αφηγήσεις, τις συγκρούσεις και τα σημεία συζήτησης που αποκτούν έλξη και, στη συνέχεια, μετατρέψτε τα σε μια εβδομάδα ιδεών περιεχομένου με γωνίες τίτλου, περιγράμματα σεναρίων, προτάσεις χρόνου ανάρτησης και μετρήσεις αναθεώρησης.'
    ),
    chipMarketBriefing: example(
      'Chip Market Open Brief',
      'Διαβάστε μια σελίδα πριν από το κουδούνι έναρξης και μάθετε τι άλλαξε στο NVDA, την AMD και την αλυσίδα των chip.',
      'Δημιουργήστε μια ενημέρωση πριν από τη διάθεση στην αγορά με επίκεντρο το NVDA, την AMD και την αλυσίδα ημιαγωγών. Συνοψίστε τις κινήσεις των τιμών κατά τη διάρκεια της νύχτας, τις σημαντικές ειδήσεις, τις απόψεις των αναλυτών, το συναίσθημα της αγοράς και τους πιθανούς καταλύτες και, στη συνέχεια, παρουσιάστε τι συνέβη, γιατί έχει σημασία και τι πρέπει να παρακολουθήσετε σήμερα.'
    ),
    morningBriefing: example(
      'Προσωπική Πρωινή Σύντομη',
      'Συνδυάστε το πρόγραμμα, το email, τις εργασίες και τα νέα μου σε μια πρωινή σύντομη ενημέρωση με πραγματικές προτεραιότητες.',
      'Δημιουργήστε μια πρωινή σύντομη ενημέρωση που συνδυάζει το ημερολόγιό μου, τη δραστηριότητα ηλεκτρονικού ταχυδρομείου, τις εργασίες και τις σχετικές ειδήσεις μου. Κατατάξτε τις τρεις πιο σημαντικές προτεραιότητες για σήμερα, επισημάνετε τους πιθανούς κινδύνους και προτείνετε την καλύτερη σειρά δράσης, ώστε να μπορέσω να προσανατολιστώ σε τρία λεπτά.'
    ),
    macroTracker: example(
      'Οικονομικός αποκωδικοποιητής δεδομένων',
      'Παρακολουθήστε εκδόσεις μακροεντολών και μετατρέψτε τις ακατέργαστες ενημερώσεις σε μια πρακτική εξήγηση του τι θα μπορούσαν να σημαίνουν.',
      'Παρακολουθήστε δημόσια μακροοικονομικά δεδομένα από την Federal Reserve, στατιστικές υπηρεσίες και παρόμοιες πηγές με ιστορικές συγκρίσεις και ειδοποιήσεις ανωμαλιών. Όταν φτάνουν νέα δεδομένα, εξηγήστε τι άλλαξε, τι μπορεί να σημαίνει για διαφορετικές κατηγορίες περιουσιακών στοιχείων, πού μπορεί να τα διαβάσει εσφαλμένα η αγορά και ποια σενάρια αξίζουν προσοχή.'
    ),
    designAdaptation: example(
      'Προσαρμογή πολλαπλών συσκευών με ένα κλικ',
      'Πάρτε ένα σχέδιο και αποκτήστε γρήγορα οδηγίες προσαρμογής για επιτραπέζιους υπολογιστές, tablet και κινητά καθώς και μια λίστα ελέγχου μεταβίβασης.',
      'Με βάση το αρχείο σχεδίασης ή τις προδιαγραφές διεπαφής χρήστη, δημιουργήστε ένα σχέδιο προσαρμογής πολλών συσκευών. Προσδιορίστε τις βασικές διατάξεις, τα επαναχρησιμοποιήσιμα στοιχεία και τα βασικά σημεία διακοπής και, στη συνέχεια, εξάγετε καθοδήγηση για τη συσκευή, αλλαγές προδιαγραφών και μια λίστα ελέγχου μεταβίβασης με σημαντικούς μηχανικούς κινδύνους.'
    ),
    inspirationFeed: example(
      'Infinite Design Inspiration',
      'Μάθετε τα γούστα μου και συνεχίστε να μου δίνετε αναφορές που δεν είναι απλώς παρόμοιες, αλλά στην πραγματικότητα επαναχρησιμοποιήσιμες.',
      'Συλλέξτε και οργανώστε έντονες αναφορές γύρω από τα οπτικά στυλ που προτιμώ. Ταξινομήστε τα κατά διάταξη, χρώμα, υφή, διάθεση και γλώσσα αλληλεπίδρασης, προτείνετε παρακείμενες αναφορές και συσκευάστε κάθε γύρο σε ένα moodboard και στιλ που μπορώ να χρησιμοποιήσω αμέσως.'
    ),
    sentimentRadar: example(
      'Ραντάρ συναισθήματος χρήστη',
      'Ακούστε τις συναισθηματικές αλλαγές στα σχόλια και τις κοινότητες και, στη συνέχεια, εντοπίστε τι ενδιαφέρει τους χρήστες που ακόμα δεν έχουν λυθεί.',
      'Παρακολουθήστε τις αναρτήσεις της κοινότητας, τις κοινωνικές συζητήσεις και τα σχόλια βίντεο για να εντοπίσετε συναισθήματα, επαναλαμβανόμενα παράπονα, προσδοκίες και πολωμένες απόψεις. Πηγαίνετε πέρα ​​από τις ετικέτες συναισθημάτων και συνοψίστε τα πραγματικά προβλήματα για τα οποία ενδιαφέρονται οι χρήστες, τα πιθανά τμήματα χρηστών, τις ανεκπλήρωτες ανάγκες και τις ευκαιρίες προϊόντων που αξίζει να επικυρωθούν.'
    ),
    dreamDialogue: example(
      'Διάλογος συμβόλων ονείρου',
      'Μην μου εξηγείς το όνειρό μου. Κάντε τις ερωτήσεις που με βοηθούν να το συνδέσω με την πραγματική ζωή.',
      'Όταν περιγράφω ένα όνειρο, μην μεταπηδήσετε σε μια τυπική ερμηνεία. Κάντε στοχαστικές επακόλουθες ερωτήσεις που συνδέουν τις σκηνές των ονείρων, τους ανθρώπους και τα συναισθήματα με εμπειρίες της πραγματικής ζωής, δημιουργήστε ένα προσωπικό λεξιλόγιο συμβόλων ονείρων και παρακολουθήστε τα επαναλαμβανόμενα θέματα με την πάροδο του χρόνου.'
    ),
    aristotleDialogue: example(
      'Διάλογος ψυχής: Αριστοτέλης',
      'Μετατρέψτε τα δημόσια γραπτά και τις μεθόδους του Αριστοτέλη σε ένα μακρόβιο δοχείο με το οποίο μπορώ να συνεχίσω να σκέφτομαι.',
      'Δημιουργήστε ένα δοχείο γνώσης έτοιμο για διάλογο γύρω από τα έργα του Αριστοτέλη, αξιόπιστες ιστορικές πηγές και βασικές ιδέες. Αποστάξτε τις έννοιες, τις αξίες, το στυλ επιχειρημάτων και τα επαναλαμβανόμενα μοτίβα ερωτήσεων και απαντήστε μου με το ύφος του, διαχωρίζοντας ξεκάθαρα το αρχικό δόγμα, το εύλογο συμπέρασμα και τη σύγχρονη επέκταση.'
    ),
  }),

  'ga-IE': buildMessages({
    memoryBank: example(
      'Banc Cuimhne Digiteach',
      'Déan comhráite, ríomhphoist, grianghraif agus doiciméid a chur in amlíne saoil inchuardaithe ar féidir leat cuairt a thabhairt uirthi arís am ar bith.',
      'Eagraigh mo stair chomhrá, ríomhphoist, meiteashonraí grianghraf, agus doiciméid i gcóras cuimhne pearsanta. Sliocht am, áit, daoine, imeachtaí, agus eochairfhocail, úsáid OCR nuair is gá, tóg amlíne inchuardaithe agus innéacs topaicí, agus freagair ceisteanna níos déanaí leis an méid a tharla, fianaise, agus croineolaíocht.'
    ),
    readingCompanion: example(
      'Compánach Léitheoireachta Laethúil',
      'Roghnaigh na rudaí is fiúntaí dom a léamh gach lá, ansin cabhraigh liom iad a chíoradh agus machnamh a dhéanamh orthu.',
      'Rianaigh foinsí thart ar na topaicí is cúram liom agus cuir dromchla ar na trí phíosa is fiúntaí gach lá. Do gach píosa, tabhair achoimre, príomhsmaointe, pointí díospóireachta, agus dhá leid díospóireachta, cuir i gcartlann iad de réir téama, agus rianaigh mo dhul chun cinn léitheoireachta agus mo spéiseanna athraitheacha.'
    ),
    growthMap: example(
      'Léarscáil Fás Pearsanta',
      'Déan cúrsaí, tionscadail, nótaí, agus nósanna athbhreithnithe a thiontú ina léarscáil de scileanna maireachtála a insíonn dom cad atá le foghlaim ina dhiaidh sin.',
      'Comhcheangail mo chuid cúrsaí, gealltanais tionscadail, nótaí, agus cártaí eolais isteach i léarscáil fáis phearsanta. Sainaithin na scileanna atá tógtha agam, mo chuid spotaí laga, agus limistéir stoptha, déan athbhreithniú spásála ar sceideal, agus mol na chéad acmhainní eile agus na tascanna cleachtaidh is fiú mo chuid ama.'
    ),
    contentPlanner: example(
      'Pleanálaí Ábhar Tras-Ardán',
      'Féach ar threochtaí X, Reddit agus YouTube, ansin déan píblíne ábhair infhoilsithe orthu.',
      'Rianaigh topaicí bogearraí AI atá ag gluaiseacht go tapa ar fud X, Reddit, agus YouTube. Sliocht na scéalta, na coinbhleachtaí, agus na pointí cainte chun tarraingt a fháil, ansin déan seachtain de smaointe ábhair iad le huillinneacha teidil, imlínte scripte, moltaí maidir le ham postála, agus méadracht athbhreithnithe.'
    ),
    chipMarketBriefing: example(
      'Achoimre Oscailte ar an Margadh Sliseanna',
      "Léigh leathanach amháin roimh an clog tosaigh agus bíodh a fhios agat cad a d'athraigh thar NVDA, AMD, agus an slabhra sliseanna.",
      'Gin faisnéisiú réamh-mhargaidh dírithe ar NVDA, AMD, agus an slabhra leathsheoltóra. Déan achoimre ar ghluaiseachtaí praghais thar oíche, nuacht thábhachtach, tuairimí anailísí, meon an mhargaidh, agus catalaíochí féideartha, ansin cuir i láthair cad a tharla, cad chuige a bhfuil tábhacht leis, agus cad ba cheart féachaint air inniu.'
    ),
    morningBriefing: example(
      'Achoimre Pearsanta Maidin',
      'Cumaisc mo sceideal, ríomhphost, tascanna, agus nuacht isteach i gearr maidin le tosaíochtaí fíor.',
      'Cruthaigh mionteagasc maidine a chomhcheanglaíonn mo fhéilire, gníomhaíocht ríomhphoist, tascanna agus nuacht ábhartha. Déan rangú ar na trí thosaíocht is tábhachtaí don lá atá inniu ann, cuir in iúl do rioscaí dóchúla, agus mol an t-ord gníomhaíochta is fearr ionas gur féidir liom a bheith dírithe i dtrí nóiméad.'
    ),
    macroTracker: example(
      'Díchódóir Sonraí Eacnamaíochta',
      "Rianaigh eisiúintí macra agus cas nuashonruithe amh ina míniú praiticiúil ar cad a d'fhéadfadh a bheith i gceist leo.",
      "Monatóireacht a dhéanamh ar shonraí macra poiblí ón gCúlchiste Feidearálach, gníomhaireachtaí staitisticí, agus foinsí comhchosúla le comparáidí stairiúla agus foláirimh aimhrialtacht. Nuair a thagann sonraí nua isteach, mínigh cad a d'athraigh, cad a d'fhéadfadh a bheith i gceist le haicmí sócmhainní éagsúla, nuair a d'fhéadfadh an margadh míléamh a dhéanamh orthu, agus cad iad na cásanna ar gá aird a thabhairt orthu."
    ),
    designAdaptation: example(
      'Oiriúnú Il-Gléas One-Cliceáil',
      'Glac dearadh amháin agus díorthaigh go tapa treoir maidir le hoiriúnú deisce, táibléad agus soghluaiste chomh maith le seicliosta láimhe.',
      'Bunaithe ar mo chomhad dearaidh nó sonraíocht Chomhéadain, déan plean oiriúnaithe ilfheiste. Sainaithin leagan amach lárnach, comhpháirteanna ath-inúsáidte, agus príomhbhriseadh, ansin treoir aschuir a bhaineann go sonrach le feiste, athruithe sonracha, agus seicliosta aistrithe le rioscaí móra innealtóireachta.'
    ),
    inspirationFeed: example(
      'Inspioráid Dearadh Infinite',
      'Foghlaim mo bhlas agus coinnigh mé ag beathú tagairtí dom nach bhfuil díreach cosúil leo, ach atá in-athúsáidte i ndáiríre.',
      'Bailigh agus eagraigh tagairtí láidre timpeall ar na stíleanna amhairc is fearr liom. Déan iad a rangú de réir leagan amach, datha, uigeachta, giúmar, agus teanga idirghníomhaíochta, mol tagairtí cóngaracha, agus pacáiste gach babhta isteach i gclár giúmar agus mionteagasc stíl is féidir liom a úsáid láithreach.'
    ),
    sentimentRadar: example(
      'Radar Meon Úsáideora',
      'Éist leis na hathruithe mothúchánacha trasna tuairimí agus pobail, ansin tabhair faoi deara cad is cúram d’úsáideoirí nach bhfuil á réiteach go fóill.',
      'Monatóireacht a dhéanamh ar phoist phobail, plé sóisialta, agus tuairimí físeáin chun meon, gearáin arís agus arís eile, ionchais, agus tuairimí polaraithe a aithint. Téigh níos faide ná lipéid mothúcháin agus déan achoimre ar na fíorfhadhbanna a dtugann úsáideoirí aire dóibh, na codanna úsáideoirí dóchúla, riachtanais nach bhfuiltear ag freastal orthu, agus deiseanna táirgí ar fiú iad a bhailíochtú.'
    ),
    dreamDialogue: example(
      'Comhphlé Siombail Aisling',
      'Ná mínigh mo bhrionglóid dom. Cuir na ceisteanna a chuidíonn liom é a nascadh ar ais leis an saol fíor.',
      'Nuair a dhéanaim cur síos ar aisling, ná léim chuig léirmhíniú caighdeánach. Cuir ceisteanna leantacha meabhracha a nascann radhairc aisling, daoine agus mothúcháin le heispéiris fhíorshaolacha, tóg foclóir pearsanta siombail aisling, agus rianaíonn téamaí athfhillteacha le himeacht ama.'
    ),
    aristotleDialogue: example(
      'Comhphlé Soul: Arastatail',
      'Déan scríbhinní poiblí agus modhanna Arastatail a iompú isteach i gcoimeádán a bhfuil cónaí air le fada ar féidir liom coinneáil ag smaoineamh air.',
      'Tóg coimeádán eolais réidh le haghaidh comhphlé timpeall ar shaothair Arastatail, foinsí iontaofa stairiúla, agus bunsmaointe. Déan a choincheapa, a luachanna, a stíl argóinte, agus a phatrúin cheisteacha athfhillteacha a dhriogadh, agus freagair mé ina stíl agus é ag scaradh go soiléir idir fhoirceadal bunaidh, tátal réasúnta, agus síneadh nua-aimseartha.'
    ),
  }),

  'hr-HR': buildMessages({
    memoryBank: example(
      'Banka digitalne memorije',
      'Pretvorite razgovore, e-mailove, fotografije i dokumente u vremenski okvir koji se može pretraživati u bilo kojem trenutku.',
      'Organiziraj moju povijest razgovora, e-poštu, metapodatke o fotografijama i dokumente u osobni memorijski sustav. Izdvojite vrijeme, mjesto, ljude, događaje i ključne riječi, koristite OCR kada je to potrebno, izradite vremensku traku i indeks tema koji se mogu pretraživati i odgovorite na kasnija pitanja s onim što se dogodilo, dokazima i kronologijom.'
    ),
    readingCompanion: example(
      'Svakodnevni vodič za čitanje',
      'Odaberite najvrednije stvari koje ću čitati svaki dan, a zatim mi pomozite da ih svarim i razmislim o njima.',
      'Pratite izvore oko tema do kojih mi je stalo i svaki dan iznesite tri najvrjednija djela. Za svako djelo navedite sažetak, ključne ideje, diskutabilne točke i dvije upute za raspravu, arhivirajte ih po temama i pratite moj napredak u čitanju i promjene interesa.'
    ),
    growthMap: example(
      'OSOBNI RAST',
      'Pretvorite tečajeve, projekte, bilješke i navike pregleda u kartu životnih vještina koja mi govori što trebam naučiti sljedeće.',
      'Kombinirajte moje tečajeve, projektne poruke, bilješke i kartice znanja u kartu osobnog rasta. Identificirajte vještine koje sam izgradio, moje slabe točke i područja u zastoju, zakažite pregled s razmakom i preporučite sljedeće resurse i zadatke vrijedne mog vremena.'
    ),
    contentPlanner: example(
      'Planer sadržaja na više platformi',
      'Pogledajte X, Reddit i YouTube trendove, a zatim ih pretvorite u sadržaj koji se može objaviti.',
      'Pratite brzorastuće teme softvera za umjetnu inteligenciju na servisima X, Reddit i YouTube. Izdvojite priče, sukobe i točke razgovora, a zatim ih pretvorite u tjedan sadržajnih ideja s naslovnim kutovima, obrisima skripti, prijedlozima vremena objavljivanja i pregledajte mjerne podatke.'
    ),
    chipMarketBriefing: example(
      'Chip Market Open Brief',
      'Pročitajte jednu stranicu prije otvaranja i saznajte što se promijenilo u NVDA, AMD i lancu čipova.',
      'Izradite brifing prije stavljanja na tržište usredotočen na NVDA, AMD i poluvodički lanac. Sažmite kretanja cijena preko noći, važne vijesti, stavove analitičara, tržišni sentiment i moguće katalizatore, a zatim predstavite što se dogodilo, zašto je to važno i što danas gledati.'
    ),
    morningBriefing: example(
      'Osobni jutarnji izvještaj',
      'Spoji moj raspored, e-poštu, zadatke i vijesti u jutarnji sažetak sa stvarnim prioritetima.',
      'Kreirajte jutarnji sažetak koji objedinjuje moj kalendar, aktivnost e-pošte, zadatke i relevantne vijesti. Rangirajte tri najvažnija prioriteta za danas, označite vjerojatne rizike i predložite najbolji redoslijed djelovanja kako bih se mogao orijentirati u tri minute.'
    ),
    macroTracker: example(
      'Dekoder ekonomskih podataka',
      'Pratite izdanja makronaredbi i pretvorite RAW ažuriranja u praktično objašnjenje onoga što bi mogla značiti.',
      'Praćenje javnih makro podataka iz Federalnih rezervi, statističkih agencija i sličnih izvora s povijesnim usporedbama i upozorenjima o anomalijama. Kada stignu novi podaci, objasnite što se promijenilo, što to može značiti za različite klase imovine, gdje ih tržište može pogrešno protumačiti i koji scenariji zaslužuju pozornost.'
    ),
    designAdaptation: example(
      'Prilagodba više uređaja jednim klikom',
      'Uzmite jedan dizajn i brzo izradite smjernice za prilagodbu za stolna računala, tablete i mobilne uređaje te kontrolni popis za primopredaju.',
      'Na temelju moje dizajnerske datoteke ili specifikacije korisničkog sučelja, izradite plan prilagodbe na više uređaja. Identificirajte osnovne rasporede, komponente za višekratnu uporabu i ključne prijelomne točke, a zatim izradite smjernice specifične za uređaj, promjene specifikacija i kontrolni popis za primopredaju s glavnim inženjerskim rizicima.'
    ),
    inspirationFeed: example(
      'Beskrajna inspiracija za dizajn',
      'Naučite moj ukus i nastavite mi davati preporuke koje nisu samo slične, već se zapravo mogu ponovno upotrijebiti.',
      'Prikupljam i organiziram snažne reference oko vizualnih stilova koje preferiram. Razvrstajte ih prema izgledu, boji, teksturi, raspoloženju i jeziku interakcije, preporučite susjedne reference i spakirajte svaku rundu u moodboard i style brief koji mogu odmah upotrijebiti.'
    ),
    sentimentRadar: example(
      'Radar za korisničko mišljenje',
      'Poslušajte emocionalne pomake u komentarima i zajednicama, a zatim uočite što je korisnicima stalo do toga da se još uvijek ne rješava.',
      'Pratite objave u zajednici, društvene rasprave i video komentare kako biste utvrdili osjećaje, ponovljene pritužbe, očekivanja i polarizirana stajališta. Izađite iz oznaka sentimenta i sažmite stvarne probleme do kojih je korisnicima stalo, vjerojatno korisničke segmente, nezadovoljene potrebe i mogućnosti proizvoda koje vrijedi potvrditi.'
    ),
    dreamDialogue: example(
      'Dijalog simbola sna',
      'Ne objašnjavaj mi moj san. Postavljajte pitanja koja mi pomažu da ga povežem sa stvarnim životom.',
      'Kada opisujem san, nemojte skakati na standardno tumačenje. Postavite promišljena naknadna pitanja koja povezuju scene iz snova, ljude i emocije sa stvarnim životnim iskustvima, izradite osobni leksikon simbola sna i pratite ponavljajuće teme tijekom vremena.'
    ),
    aristotleDialogue: example(
      'Dijalog duše: Aristotel',
      'Pretvorite Aristotelove javne spise i metode u dugovječni spremnik s kojim mogu nastaviti razmišljati.',
      'Izgradite spremnik znanja spreman za dijalog oko Aristotelovih djela, pouzdanih povijesnih izvora i ključnih ideja. Destilirajte njegove koncepte, vrijednosti, stil argumenata i ponavljajuće obrasce pitanja i odgovorite mi u njegovom stilu, jasno odvajajući izvornu doktrinu, razumno zaključivanje i moderno proširenje.'
    ),
  }),

  'hu-HU': buildMessages({
    memoryBank: example(
      'Digitális memóriabank',
      'Változtassa a csevegéseket, e-maileket, fényképeket és dokumentumokat kereshető életidővonalmá, amelyet bármikor újra megtekinthet.',
      'Csevegési előzményeimet, e-mailjeimet, fotók metaadatait és dokumentumaimat személyes memóriarendszerbe rendezheti. Kivonja az időt, helyet, személyeket, eseményeket és kulcsszavakat, szükség esetén használja az OCR-t, készítsen kereshető idővonalat és témaindexet, és válaszoljon a későbbi kérdésekre a történtekkel, bizonyítékokkal és kronológiával.'
    ),
    readingCompanion: example(
      'Napi olvasótárs',
      'Minden nap válaszd ki a számomra legérdemesebb olvasmányokat, majd segíts megemészteni és elgondolkodni rajtuk.',
      'Kövesse nyomon a forrásokat az általam érdekelt témák körül, és minden nap hozza nyilvánosságra a három legérdemesebb darabot. Minden egyes darabhoz adjon meg egy összefoglalót, kulcsfontosságú ötleteket, vitatható pontokat és két vitakérdést, archiválja ezeket témák szerint, és kövesse nyomon olvasási folyamatomat és a változó érdeklődési körömet.'
    ),
    growthMap: example(
      'Személyes növekedési térkép',
      'A tanfolyamokat, projekteket, jegyzeteket és a szokások áttekintését élő készségtérképpé alakítsa, amely megmondja, mit kell tanulnom ezután.',
      'Kombinálja kurzusaimat, projektkötelezettségeimet, jegyzeteimet és tudáskártyáimat egy személyes növekedési térképté. Határozza meg az általam felépített készségeket, gyenge pontjaimat és elakadt területeimet, ütemezze be az időközönkénti felülvizsgálatot, és javasolja a következő erőforrásokat és gyakorlati feladatokat, amelyek megérik az időmet.'
    ),
    contentPlanner: example(
      'Platformok közötti tartalomtervező',
      'Nézze meg az X, Reddit és YouTube trendeket, majd alakítsa őket közzétehető tartalommá.',
      'Kövesse nyomon a gyorsan változó AI-szoftverekkel kapcsolatos témákat az X-en, a Reddit-en és a YouTube-on. Vonja ki a narratívákat, konfliktusokat és beszédpontokat, amelyek egyre nagyobb teret nyernek, majd alakítsa őket egy hét tartalmi ötletté címszögekkel, forgatókönyv-vázlatokkal, közzétételi időre vonatkozó javaslatokkal és áttekintési mutatókkal.'
    ),
    chipMarketBriefing: example(
      'Chip Market Open Brief',
      'Olvasson el egy oldalt a nyitó csengő előtt, és tudja meg, mi változott az NVDA, az AMD és a chiplánc között.',
      'Készítsen egy forgalomba hozatal előtti tájékoztatót, amely az NVDA-ra, az AMD-re és a félvezetőláncra összpontosít. Foglalja össze az éjszakai ármozgásokat, a fontos híreket, az elemzői véleményeket, a piaci hangulatot és a lehetséges katalizátorokat, majd mutassa be, mi történt, miért számít, és mit érdemes ma megnézni.'
    ),
    morningBriefing: example(
      'Személyes reggeli tájékoztató',
      'Keverje össze az ütemtervemet, az e-mailjeimet, a feladataimat és a híreimet egy reggeli összefoglalóba, valódi prioritásokkal.',
      'Készítsen egy reggeli összefoglalót, amely egyesíti a naptáram, az e-mail tevékenységem, a feladataim és a releváns hírek. Sorolja fel a mai három legfontosabb prioritást, jelölje meg a valószínű kockázatokat, és javasolja a legjobb cselekvési sorrendet, hogy három perc alatt tájékozódhassak.'
    ),
    macroTracker: example(
      'Gazdasági adatok dekódolója',
      'Kövesse nyomon a makrókiadásokat, és alakítsa át a nyers frissítéseket gyakorlati magyarázatokká, hogy mit jelenthetnek.',
      'Kövesse nyomon a Federal Reserve-től, a statisztikai ügynökségektől és hasonló forrásokból származó nyilvános makróadatokat előzmény-összehasonlításokkal és anomáliákkal kapcsolatos riasztásokkal. Amikor új adatok érkeznek, magyarázza el, mi változott, mit jelenthet a különböző eszközosztályok számára, hol értelmezheti félre a piac, és mely forgatókönyvek érdemelnek figyelmet.'
    ),
    designAdaptation: example(
      'Egykattintásos többeszköz-adaptáció',
      'Vegyünk egy tervezést, és gyorsan levezetjük az asztali, táblagépes és mobil adaptációs útmutatót, valamint egy átadási ellenőrzőlistát.',
      'A tervfájlom vagy a felhasználói felület specifikációja alapján készítsen több eszközre vonatkozó adaptációs tervet. Azonosítsa az alapvető elrendezéseket, az újrafelhasználható összetevőket és a kulcsfontosságú töréspontokat, majd adjon ki eszközspecifikus útmutatást, specifikációs változtatásokat és egy átadási ellenőrzőlistát a főbb mérnöki kockázatokkal.'
    ),
    inspirationFeed: example(
      'Végtelen tervezési inspiráció',
      'Tanuld meg az ízlésemet, és folytasd azokkal a referenciákkal, amelyek nem csak hasonlóak, hanem valóban újrafelhasználhatók.',
      'Gyűjtsön össze és rendszerezzen erős referenciákat az általam preferált vizuális stílusok köré. Osztályozza őket elrendezés, szín, textúra, hangulat és interakciós nyelv alapján, ajánljon szomszédos hivatkozásokat, és csomagolja be minden kört egy hangulattáblázatba és stílusjegyzetbe, amelyet azonnal használhatok.'
    ),
    sentimentRadar: example(
      'Felhasználói hangulatradar',
      'Hallgassa meg a megjegyzések és közösségek érzelmi változásait, majd fedezze fel, mi érdekli a felhasználókat, amelyek még mindig nem oldódnak meg.',
      'Kövesse nyomon a közösségi bejegyzéseket, a társadalmi beszélgetéseket és a videokommentárokat, hogy azonosítsa az érzelmeket, az ismétlődő panaszokat, elvárásokat és polarizált nézőpontokat. Lépjen túl a hangulatcímkéken, és foglalja össze a felhasználókat érdeklő valós problémákat, a valószínű felhasználói szegmenseket, a kielégítetlen igényeket és a terméklehetőségeket, amelyeket érdemes ellenőrizni.'
    ),
    dreamDialogue: example(
      'Álom Szimbólum Párbeszéd',
      'Ne magyarázd meg nekem az álmomat. Tedd fel azokat a kérdéseket, amelyek segítenek visszakapcsolni a való élethez.',
      'Amikor leírok egy álmot, ne ugorjon a standard értelmezésre. Tegyen fel átgondolt utólagos kérdéseket, amelyek összekapcsolják az álomjeleneteket, az embereket és az érzelmeket a valós élményekkel, építsenek fel személyes álomszimbólum-lexikont, és nyomon kövessék az idő múlásával visszatérő témákat.'
    ),
    aristotleDialogue: example(
      'Lélekpárbeszéd: Arisztotelész',
      'Tedd Arisztotelész nyilvános írásait és módszereit egy hosszú életű konténerré, amelyben tovább tudok gondolkodni.',
      'Építsen párbeszédre kész tudástárolót Arisztotelész művei, megbízható történelmi források és alapvető ötletek köré. Desztillálja fogalmait, értékeit, érvelési stílusát és visszatérő kérdésmintáit, és válaszoljon nekem az ő stílusában, miközben világosan elválasztja az eredeti doktrínát, az ésszerű következtetést és a modern kiterjesztést.'
    ),
  }),

  'ml-IN': buildMessages({
    memoryBank: example(
      'ഡിജിറ്റൽ മെമ്മറി ബാങ്ക്',
      'ചാറ്റുകൾ, ഇമെയിലുകൾ, ഫോട്ടോകൾ, ഡോക്യുമെൻ്റുകൾ എന്നിവ നിങ്ങൾക്ക് എപ്പോൾ വേണമെങ്കിലും വീണ്ടും സന്ദർശിക്കാൻ കഴിയുന്ന തിരയാനാകുന്ന ലൈഫ് ടൈംലൈനാക്കി മാറ്റുക.',
      'എൻ്റെ ചാറ്റ് ചരിത്രം, ഇമെയിലുകൾ, ഫോട്ടോ മെറ്റാഡാറ്റ, ഡോക്യുമെൻ്റുകൾ എന്നിവ ഒരു വ്യക്തിഗത മെമ്മറി സിസ്റ്റത്തിലേക്ക് ഓർഗനൈസ് ചെയ്യുക. സമയം, സ്ഥലം, ആളുകൾ, ഇവൻ്റുകൾ, കീവേഡുകൾ എന്നിവ എക്‌സ്‌ട്രാക്റ്റുചെയ്യുക, ആവശ്യമുള്ളപ്പോൾ OCR ഉപയോഗിക്കുക, തിരയാനാകുന്ന ടൈംലൈനും വിഷയ സൂചികയും നിർമ്മിക്കുക, എന്താണ് സംഭവിച്ചത്, തെളിവുകൾ, കാലഗണന എന്നിവ ഉപയോഗിച്ച് പിന്നീടുള്ള ചോദ്യങ്ങൾക്ക് ഉത്തരം നൽകുക.'
    ),
    readingCompanion: example(
      'പ്രതിദിന വായന കൂട്ടാളി',
      'ഓരോ ദിവസവും വായിക്കാൻ എനിക്ക് ഏറ്റവും മൂല്യവത്തായ കാര്യങ്ങൾ തിരഞ്ഞെടുക്കുക, എന്നിട്ട് അവയെ ദഹിപ്പിക്കാനും പ്രതിഫലിപ്പിക്കാനും എന്നെ സഹായിക്കൂ.',
      'ഞാൻ ശ്രദ്ധിക്കുന്ന വിഷയങ്ങളെ ചുറ്റിപ്പറ്റിയുള്ള ഉറവിടങ്ങൾ ട്രാക്ക് ചെയ്യുകയും ഓരോ ദിവസവും ഏറ്റവും മൂല്യവത്തായ മൂന്ന് ഭാഗങ്ങൾ അവതരിപ്പിക്കുകയും ചെയ്യുക. ഓരോ ഭാഗത്തിനും, ഒരു സംഗ്രഹം, പ്രധാന ആശയങ്ങൾ, സംവാദാത്മക പോയിൻ്റുകൾ, രണ്ട് ചർച്ചാ നിർദ്ദേശങ്ങൾ എന്നിവ നൽകുക, തീം അനുസരിച്ച് അവ ആർക്കൈവ് ചെയ്യുക, ഒപ്പം എൻ്റെ വായനാ പുരോഗതിയും മാറുന്ന താൽപ്പര്യങ്ങളും ട്രാക്കുചെയ്യുക.'
    ),
    growthMap: example(
      'വ്യക്തിഗത വളർച്ചയുടെ മാപ്പ്',
      'കോഴ്സുകൾ, പ്രോജക്റ്റുകൾ, കുറിപ്പുകൾ, അവലോകന ശീലങ്ങൾ എന്നിവ അടുത്തതായി എന്താണ് പഠിക്കേണ്ടതെന്ന് എന്നോട് പറയുന്ന ഒരു ലിവിംഗ് സ്‌കിൽ മാപ്പാക്കി മാറ്റുക.',
      'എൻ്റെ കോഴ്സുകൾ, പ്രോജക്റ്റ് കമ്മിറ്റുകൾ, കുറിപ്പുകൾ, വിജ്ഞാന കാർഡുകൾ എന്നിവ ഒരു വ്യക്തിഗത വളർച്ചാ മാപ്പിലേക്ക് സംയോജിപ്പിക്കുക. ഞാൻ ഉണ്ടാക്കിയ കഴിവുകൾ, എൻ്റെ ദുർബലമായ സ്ഥലങ്ങൾ, സ്തംഭിച്ച പ്രദേശങ്ങൾ എന്നിവ തിരിച്ചറിയുക, സ്‌പെയ്‌സ്ഡ് റിവ്യൂ ഷെഡ്യൂൾ ചെയ്യുക, അടുത്ത റിസോഴ്‌സുകളും പരിശീലന ജോലികളും ശുപാർശ ചെയ്യുക.'
    ),
    contentPlanner: example(
      'ക്രോസ്-പ്ലാറ്റ്ഫോം ഉള്ളടക്ക പ്ലാനർ',
      'X, Reddit, YouTube ട്രെൻഡുകൾ കാണുക, തുടർന്ന് അവയെ പ്രസിദ്ധീകരിക്കാവുന്ന ഉള്ളടക്ക പൈപ്പ്ലൈനാക്കി മാറ്റുക.',
      'X, Reddit, YouTube എന്നിവയിലുടനീളം അതിവേഗം ചലിക്കുന്ന AI സോഫ്റ്റ്‌വെയർ വിഷയങ്ങൾ ട്രാക്ക് ചെയ്യുക. ആഖ്യാനങ്ങൾ, വൈരുദ്ധ്യങ്ങൾ, സംസാരിക്കുന്ന പോയിൻ്റുകൾ എന്നിവ എക്‌സ്‌ട്രാക്‌റ്റ് ചെയ്യുക, തുടർന്ന് അവയെ ശീർഷക കോണുകൾ, സ്‌ക്രിപ്റ്റ് ഔട്ട്‌ലൈനുകൾ, പോസ്‌റ്റിംഗ് സമയ നിർദ്ദേശങ്ങൾ, അവലോകന അളവുകൾ എന്നിവ ഉപയോഗിച്ച് ഉള്ളടക്ക ആശയങ്ങളുടെ ആഴ്‌ചയിലേക്ക് മാറ്റുക.'
    ),
    chipMarketBriefing: example(
      'ചിപ്പ് മാർക്കറ്റ് ഓപ്പൺ ബ്രീഫ്',
      'ഓപ്പണിംഗ് ബെല്ലിന് മുമ്പ് ഒരു പേജ് വായിച്ച് എൻവിഡിഎ, എഎംഡി, ചിപ്പ് ചെയിൻ എന്നിവയിലുടനീളം എന്താണ് മാറിയതെന്ന് അറിയുക.',
      'എൻവിഡിഎ, എഎംഡി, അർദ്ധചാലക ശൃംഖല എന്നിവയെ കേന്ദ്രീകരിച്ച് ഒരു പ്രീ-മാർക്കറ്റ് ബ്രീഫിംഗ് സൃഷ്ടിക്കുക. ഒറ്റരാത്രികൊണ്ട് വില നീക്കങ്ങൾ, പ്രധാന വാർത്തകൾ, വിശകലന വീക്ഷണങ്ങൾ, വിപണി വികാരം, സാധ്യമായ ഉത്തേജകങ്ങൾ എന്നിവ സംഗ്രഹിക്കുക, തുടർന്ന് എന്താണ് സംഭവിച്ചത്, എന്തുകൊണ്ട് ഇത് പ്രധാനമാണ്, ഇന്ന് എന്താണ് കാണേണ്ടത് എന്നിവ അവതരിപ്പിക്കുക.'
    ),
    morningBriefing: example(
      'സ്വകാര്യ പ്രഭാത സംക്ഷിപ്തം',
      'എൻ്റെ ഷെഡ്യൂൾ, ഇമെയിൽ, ടാസ്‌ക്കുകൾ, വാർത്തകൾ എന്നിവ യഥാർത്ഥ മുൻഗണനകളോടെ ഒരു പ്രഭാത സംക്ഷിപ്‌തമായി സംയോജിപ്പിക്കുക.',
      'എൻ്റെ കലണ്ടർ, ഇമെയിൽ ആക്‌റ്റിവിറ്റി, ടാസ്‌ക്കുകൾ, പ്രസക്തമായ വാർത്തകൾ എന്നിവ സംയോജിപ്പിച്ച് ഒരു പ്രഭാത ലഘുലേഖ സൃഷ്‌ടിക്കുക. ഇന്നത്തെ ഏറ്റവും പ്രധാനപ്പെട്ട മൂന്ന് മുൻഗണനകൾ റാങ്ക് ചെയ്യുക, അപകടസാധ്യതകൾ ഫ്ലാഗ് ചെയ്യുക, മികച്ച പ്രവർത്തന ക്രമം നിർദ്ദേശിക്കുക, അതുവഴി എനിക്ക് മൂന്ന് മിനിറ്റിനുള്ളിൽ ഓറിയൻ്റേറ്റ് ചെയ്യാൻ കഴിയും.'
    ),
    macroTracker: example(
      'സാമ്പത്തിക ഡാറ്റ ഡീകോഡർ',
      'മാക്രോ റിലീസുകൾ ട്രാക്ക് ചെയ്യുക, അസംസ്‌കൃത അപ്‌ഡേറ്റുകൾ അവ എന്താണ് അർത്ഥമാക്കുന്നത് എന്നതിൻ്റെ പ്രായോഗിക വിശദീകരണമാക്കി മാറ്റുക.',
      'ഫെഡറൽ റിസർവ്, സ്റ്റാറ്റിസ്റ്റിക്സ് ഏജൻസികൾ, സമാന ഉറവിടങ്ങൾ എന്നിവയിൽ നിന്നുള്ള പൊതു മാക്രോ ഡാറ്റ ചരിത്രപരമായ താരതമ്യങ്ങളും അനോമലി അലേർട്ടുകളും ഉപയോഗിച്ച് നിരീക്ഷിക്കുക. പുതിയ ഡാറ്റ വരുമ്പോൾ, എന്താണ് മാറിയതെന്ന് വിശദീകരിക്കുക, വ്യത്യസ്ത അസറ്റ് ക്ലാസുകൾക്ക് ഇത് എന്താണ് അർത്ഥമാക്കുന്നത്, മാർക്കറ്റ് എവിടെ അത് തെറ്റായി വായിച്ചേക്കാം, ഏതൊക്കെ സാഹചര്യങ്ങളാണ് ശ്രദ്ധ അർഹിക്കുന്നത്.'
    ),
    designAdaptation: example(
      'ഒറ്റ-ക്ലിക്ക് മൾട്ടി-ഡിവൈസ് അഡാപ്റ്റേഷൻ',
      'ഒരു ഡിസൈൻ എടുത്ത് ഡെസ്‌ക്‌ടോപ്പ്, ടാബ്‌ലെറ്റ്, മൊബൈൽ അഡാപ്റ്റേഷൻ മാർഗ്ഗനിർദ്ദേശവും ഒരു ഹാൻഡ്ഓഫ് ചെക്ക്‌ലിസ്റ്റും വേഗത്തിൽ നേടുക.',
      'എൻ്റെ ഡിസൈൻ ഫയൽ അല്ലെങ്കിൽ UI സ്പെസിഫിക്കേഷൻ അടിസ്ഥാനമാക്കി, ഒരു മൾട്ടി-ഡിവൈസ് അഡാപ്റ്റേഷൻ പ്ലാൻ നിർമ്മിക്കുക. കോർ ലേഔട്ടുകൾ, പുനരുപയോഗിക്കാവുന്ന ഘടകങ്ങൾ, പ്രധാന ബ്രേക്ക്‌പോയിൻ്റുകൾ എന്നിവ തിരിച്ചറിയുക, തുടർന്ന് ഉപകരണ-നിർദ്ദിഷ്‌ട മാർഗ്ഗനിർദ്ദേശം, പ്രത്യേക മാറ്റങ്ങൾ, പ്രധാന എഞ്ചിനീയറിംഗ് അപകടസാധ്യതകളുള്ള ഒരു ഹാൻഡ്ഓഫ് ചെക്ക്‌ലിസ്റ്റ് എന്നിവ ഔട്ട്‌പുട്ട് ചെയ്യുക.'
    ),
    inspirationFeed: example(
      'അനന്തമായ ഡിസൈൻ പ്രചോദനം',
      'എൻ്റെ അഭിരുചി മനസിലാക്കുക, സമാനമല്ലാത്ത, യഥാർത്ഥത്തിൽ പുനരുപയോഗിക്കാവുന്ന റഫറൻസുകൾ എനിക്ക് നൽകുന്നത് തുടരുക.',
      'ഞാൻ ഇഷ്ടപ്പെടുന്ന വിഷ്വൽ ശൈലികളെ ചുറ്റിപ്പറ്റിയുള്ള ശക്തമായ റഫറൻസുകൾ ശേഖരിക്കുകയും സംഘടിപ്പിക്കുകയും ചെയ്യുക. ലേഔട്ട്, നിറം, ടെക്‌സ്‌ചർ, മൂഡ്, ഇൻ്ററാക്ഷൻ ഭാഷ എന്നിവ പ്രകാരം അവയെ തരംതിരിക്കുക, അടുത്തുള്ള റഫറൻസുകൾ ശുപാർശ ചെയ്യുക, ഓരോ റൗണ്ടും എനിക്ക് ഉടനടി ഉപയോഗിക്കാൻ കഴിയുന്ന ഒരു മൂഡ്‌ബോർഡിലേക്കും ശൈലിയിലേക്കും പാക്കേജുചെയ്യുക.'
    ),
    sentimentRadar: example(
      'ഉപയോക്തൃ വികാരം റഡാർ',
      'അഭിപ്രായങ്ങളിലും കമ്മ്യൂണിറ്റികളിലും ഉടനീളമുള്ള വൈകാരിക വ്യതിയാനങ്ങൾ കേൾക്കുക, തുടർന്ന് ഉപയോക്താക്കൾ ശ്രദ്ധിക്കുന്നതെന്താണെന്ന് കണ്ടെത്തുക.',
      'വികാരം, ആവർത്തിച്ചുള്ള പരാതികൾ, പ്രതീക്ഷകൾ, ധ്രുവീകരിക്കപ്പെട്ട വീക്ഷണങ്ങൾ എന്നിവ തിരിച്ചറിയാൻ കമ്മ്യൂണിറ്റി പോസ്റ്റുകൾ, സോഷ്യൽ ചർച്ചകൾ, വീഡിയോ കമൻ്റുകൾ എന്നിവ നിരീക്ഷിക്കുക. സെൻ്റിമെൻ്റ് ലേബലുകൾക്കപ്പുറം പോയി, ഉപയോക്താക്കൾ ശ്രദ്ധിക്കുന്ന യഥാർത്ഥ പ്രശ്നങ്ങൾ, സാധ്യതയുള്ള ഉപയോക്തൃ വിഭാഗങ്ങൾ, പാലിക്കാത്ത ആവശ്യങ്ങൾ, സാധൂകരിക്കേണ്ട ഉൽപ്പന്ന അവസരങ്ങൾ എന്നിവ സംഗ്രഹിക്കുക.'
    ),
    dreamDialogue: example(
      'ഡ്രീം സിംബൽ ഡയലോഗ്',
      'എൻ്റെ സ്വപ്നം എനിക്കായി വിശദീകരിക്കരുത്. അതിനെ യഥാർത്ഥ ജീവിതവുമായി ബന്ധിപ്പിക്കാൻ എന്നെ സഹായിക്കുന്ന ചോദ്യങ്ങൾ ചോദിക്കുക.',
      'ഞാൻ ഒരു സ്വപ്നം വിവരിക്കുമ്പോൾ, ഒരു സാധാരണ വ്യാഖ്യാനത്തിലേക്ക് പോകരുത്. സ്വപ്ന രംഗങ്ങൾ, ആളുകൾ, വികാരങ്ങൾ എന്നിവയെ യഥാർത്ഥ ജീവിതാനുഭവങ്ങളുമായി ബന്ധിപ്പിക്കുന്ന, വ്യക്തിഗത സ്വപ്ന-ചിഹ്ന നിഘണ്ടു നിർമ്മിക്കുന്ന, കാലക്രമേണ ആവർത്തിച്ചുള്ള തീമുകൾ ട്രാക്ക് ചെയ്യുന്ന ചിന്താപൂർവ്വമായ ഫോളോ-അപ്പ് ചോദ്യങ്ങൾ ചോദിക്കുക.'
    ),
    aristotleDialogue: example(
      'സോൾ ഡയലോഗ്: അരിസ്റ്റോട്ടിൽ',
      'അരിസ്റ്റോട്ടിലിൻ്റെ പൊതു രചനകളും രീതികളും എനിക്ക് ചിന്തിക്കാൻ കഴിയുന്ന ഒരു ദീർഘകാല പാത്രമാക്കി മാറ്റുക.',
      'അരിസ്റ്റോട്ടിലിൻ്റെ കൃതികൾ, വിശ്വസനീയമായ ചരിത്ര സ്രോതസ്സുകൾ, പ്രധാന ആശയങ്ങൾ എന്നിവയെ ചുറ്റിപ്പറ്റിയുള്ള ഒരു സംഭാഷണ-തയ്യാറായ വിജ്ഞാന കണ്ടെയ്നർ നിർമ്മിക്കുക. അദ്ദേഹത്തിൻ്റെ ആശയങ്ങൾ, മൂല്യങ്ങൾ, തർക്ക ശൈലി, ആവർത്തിച്ചുള്ള ചോദ്യ പാറ്റേണുകൾ എന്നിവ ഡിസ്റ്റിൽ ചെയ്യുക, യഥാർത്ഥ സിദ്ധാന്തം, ന്യായമായ അനുമാനം, ആധുനിക വിപുലീകരണം എന്നിവ വ്യക്തമായി വേർതിരിക്കുമ്പോൾ അദ്ദേഹത്തിൻ്റെ ശൈലിയിൽ എനിക്ക് ഉത്തരം നൽകുക.'
    ),
  }),
}

const fallbackExampleLocales = [
  'nb-NO',
  'nl-NL',
  'pl-PL',
  'pt-BR',
  'pt-PT',
  'ro-RO',
  'ru-RU',
  'sk-SK',
  'sv-SE',
] as const

const englishExamples = (
  presetQuestionContentOverrides as Record<
    string,
    { chat?: { presetQuestions?: { examples?: PresetQuestionExampleMessages } } }
  >
)['en-US']?.chat?.presetQuestions?.examples

if (englishExamples) {
  for (const locale of fallbackExampleLocales) {
    presetQuestionContentOverrides[locale] = buildMessages(englishExamples)
  }
}

export default presetQuestionContentOverrides
