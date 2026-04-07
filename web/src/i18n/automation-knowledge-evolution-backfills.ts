import type { LocaleKey } from './locale-catalog'

const automationKnowledgeEvolutionBackfills: Partial<Record<LocaleKey, object>> = {
  'en-US': {
    automation: {
      tabs: {
        knowledge: 'Knowledge',
        knowledgeDesc: 'Browse compiled pages, lint health, and grounded answers.',
        evolution: 'Evolution',
        evolutionDesc: 'Review skill, runner, and instruction evidence.',
      },
    },
    knowledge: {
      eyebrow: 'Knowledge Space',
      title: 'Incremental ingest, durable wiki pages, and query-to-knowledge promotion',
      description:
        'Turn curated local sources into a durable knowledge wiki with schema, activity log, conflict surfacing, and promotion-ready query results.',
      controlTitle: 'Ingest and maintain Blue knowledge',
      controlDescription:
        'Run ingest and lint jobs, then jump into the schema and activity panels.',
      ingest: 'Ingest',
      ingestStarted: 'Knowledge ingest started.',
      ingestComplete: 'Knowledge ingest completed.',
      mapTitle: 'Knowledge map',
      queryTitle: 'Query',
      activityTitle: 'Recent activity',
      schemaTitle: 'Schema',
      openSchema: 'Open Schema',
      openLog: 'Open Log',
      saveSchema: 'Save Schema',
      saveAnswer: 'Save answer history',
      promoteToWiki: 'Promote to Wiki',
      query: 'Query',
      latestIngestDelta: 'Latest ingest delta',
      changedPages: 'Changed pages',
      conflictsLabel: 'Conflicts',
      gapsLabel: 'Gaps',
      recentIngests: 'Recent ingests',
      unresolvedConflicts: 'Unresolved conflicts',
      openGaps: 'Open gaps',
      conflictCount: '{count} unresolved conflicts',
      gapCount: '{count} open gaps',
      schemaSaved: 'Knowledge schema saved.',
      promoted: 'Query promoted to the wiki.',
    },
    evolution: {
      title: 'Self-Repair And Evolution Console',
      subtitle:
        'Review skills, runner changes, and instruction proposals with evidence and safe approvals.',
      tabs: {
        knowledge: 'Knowledge',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Review Queue',
      },
      lanes: {
        knowledgeDescription:
          'Ground the workspace in compiled pages, source-backed queries, and conflict or gap signals.',
        skillsDescription:
          'Review canonical skills, accepted revisions, and rollback readiness.',
        runnerDescription:
          'Inspect runner candidates, execution evidence, and switch boundaries.',
        instructionsDescription:
          'Review instruction proposals and keep AGENTS.md approvals explicit.',
      },
    },
  },
  'en-GB': {
    automation: {
      tabs: {
        knowledge: 'Knowledge',
        knowledgeDesc: 'Browse compiled pages, lint health, and grounded answers.',
        evolution: 'Evolution',
        evolutionDesc: 'Review skill, runner, and instruction evidence.',
      },
    },
    knowledge: {
      eyebrow: 'Knowledge Space',
      title: 'Compiled pages, lint health, and ask-and-archive',
      description:
        'Inspect compiled knowledge pages, run maintenance, and ask grounded questions.',
      pageListHint:
        'Select a page to inspect sources, related pages, and archived answers.',
      askTitle: 'Ask the compiled knowledge space',
      askHint:
        'Answer jobs prefer compiled pages first and can archive strong answers back into the workspace.',
      archiveAnswer: 'Archive this answer into the knowledge workspace',
    },
    evolution: {
      title: 'Self-Repair And Evolution Console',
      subtitle:
        'Review skills, runner changes, and instruction proposals with evidence and safe approvals.',
      tabs: {
        knowledge: 'Knowledge',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Review Queue',
      },
      lanes: {
        knowledgeDescription:
          'Ground the workspace in compiled pages, source-backed queries, and conflict or gap signals.',
        skillsDescription:
          'Review canonical skills, accepted revisions, and rollback readiness.',
        runnerDescription:
          'Inspect runner candidates, execution evidence, and switch boundaries.',
        instructionsDescription:
          'Review instruction proposals and keep AGENTS.md approvals explicit.',
      },
    },
  },
  'zh-CN': {
	    automation: {
	      tabs: {
	        knowledge: '知识',
	        knowledgeDesc: '查看编译页面、检查健康状态与有依据的答案。',
	        evolution: '进化',
	        evolutionDesc: '审查技能、运行器与指令演进证据。',
	      },
	    },
	    knowledge: {
	      controlEyebrow: '知识控制',
	      controlTitle: '摄入并维护 Blue 知识',
	      controlDescription: '运行摄入与检查任务，并进入结构定义与活动面板继续查看。',
	      eyebrow: '知识空间',
	      title: '增量摄入、持久 wiki 页面与查询沉淀',
	      description: '将本地来源整理成持久知识 wiki，并在结构定义、活动日志、冲突与查询沉淀之间持续演化。',
	      pageList: '编译页面',
	      pageCount: '{count} 个编译页面',
	      searchResultCount: '{visible} / {total} 个页面',
	      allStatuses: '全部状态',
	      emptyGroup: '该分组暂无页面。',
	      latestLint: '最近检查',
	      lintHealth: '检查健康度',
	      evidenceState: '证据与状态',
	      evidence: {
	        status: '状态',
	        confidence: '可信度',
	        sourceRefs: '来源引用',
	        updatedAt: '更新时间',
	        conflictsWith: '冲突于',
	        supersededBy: '被替代于',
	      },
	      pageTypes: {
	        all: '全部',
	        source_summary: '来源摘要',
	        entity: '实体',
	        concept: '概念',
	        comparison: '对比',
	        synthesis: '综合',
	        decision: '决策',
	      },
	      status: {
	        active: '有效',
	        superseded: '已替代',
	        conflicted: '冲突',
	      },
	      confidence: {
	        low: '低',
	        medium: '中',
	        high: '高',
	      },
	      loadingPages: '正在加载页面...',
	      sources: '来源',
	      relatedPages: '关联页面',
	      archivedAnswers: '归档答案',
	      pageListHint: '选择一个页面以查看来源、关联页面和归档答案。',
	      searchPlaceholder: '按标题、摘要、来源或关键词搜索编译页面',
      clearSearch: '清空',
      searchEmpty: '当前搜索没有匹配的编译页面。请尝试更宽泛的标题、来源或关键词。',
      emptyPages: '暂时还没有编译后的知识页面。请先运行编译任务构建工作区。',
      selectPage: '选择一个编译页面以查看详情。',
      noRelatedPages: '尚未关联页面。',
      noArchivedAnswers: '该页面尚未关联归档答案。',
      noSummary: '暂无摘要。',
	      askTitle: '向编译后的知识空间提问',
      askHint: '回答任务会优先使用编译页面，并可将高质量答案归档回工作区。',
      askQuestion: '问题',
      askPlaceholder: 'Blue 的知识编译是如何工作的？我下一步应该查看哪些页面？',
      ask: '提问',
      answerResult: '回答结果',
      answerReady: '知识回答已生成。',
      answerFailed: '知识回答失败。',
	      answerStarted: '知识回答任务已开始。',
	      archiveAnswer: '将此答案归档到知识工作区',
	      archived: '已归档',
	      pageBody: '编译后的 Markdown',
	      activeJob: '当前任务',
	      awaitingAction: '等待处理',
	      idle: '空闲',
	      none: '无',
	      noActiveJob: '暂无活动任务',
	      compile: '编译',
	      ingest: '摄入',
	      lint: '检查',
	      openWorkspace: '打开知识空间',
	      compileComplete: '知识编译已完成。',
	      ingestComplete: '知识摄入已完成。',
	      lintComplete: '知识检查已完成。',
	      jobComplete: '知识任务已完成。',
	      jobFailed: '知识任务失败。',
	      lintUnavailable: '暂无检查报告。',
	      lintHealthy: '最近一次检查未发现未处理问题。',
	      lintIssues: '有 {count} 个检查问题需要处理。',
	      compileStarted: '知识编译已开始。',
	      ingestStarted: '知识摄入已开始。',
	      lintStarted: '知识检查已开始。',
	      never: '从未',
	      mapTitle: '知识地图',
	      mapHint: '按类型浏览持久页面，跨来源与摘要搜索，并检查证据状态。',
	      queryTitle: '查询',
	      queryHint: '查询 wiki、保存高质量答案，并将持久综合内容沉淀回知识空间。',
	      queryPlaceholder: '这个知识空间最近发生了什么变化？哪些结论可信？我接下来该重点查什么？',
	      activityTitle: '最近活动',
	      activityHint: '查看最新的摄入、查询、检查与结构定义事件。',
	      schemaTitle: '结构定义',
	      schemaHint: '定义 wiki 应如何组织与分组。',
	      openSchema: '打开结构定义',
	      openLog: '打开日志',
	      saveSchema: '保存结构定义',
	      saveAnswer: '保存答案历史',
      promoteToWiki: '提升为 Wiki 页面',
      query: '查询',
	      latestIngestDelta: '最近一次摄入变化',
	      changedPages: '变化页面',
	      deltaSummary: '{newPages} 个新增，{updatedPages} 个更新',
	      conflictsLabel: '冲突',
	      gapsLabel: '空洞',
	      recentIngests: '最近摄入',
	      noRecentIngests: '暂无最近摄入。',
	      unresolvedConflicts: '未解决冲突',
	      openGaps: '待补空洞',
	      conflictCount: '{count} 个未解决冲突',
	      gapCount: '{count} 个待补空洞',
      active: '当前',
      schemaSaved: '知识结构定义已保存。',
	      promoted: '查询结果已沉淀到 wiki。',
	      scopeAll: '全部',
	      scopeCurrentPage: '当前页面',
	      scopeSelectedSources: '已选来源',
	      unknown: '未知',
	    },
	    evolution: {
	      title: '自修复与演进控制台',
	      subtitle: '基于证据与安全审批，审查技能、运行器变更和指令提案。',
      tabs: {
        knowledge: '知识',
        skills: '技能',
        runner: '运行器',
        instructions: '审查队列',
      },
      lanes: {
        knowledgeDescription: '以编译页面、有依据的查询与冲突或空洞信号作为整个工作台的知识底座。',
        skillsDescription: '审查规范技能、已接受修订与回滚准备情况。',
        runnerDescription: '检查运行器候选、执行证据与切换边界。',
        instructionsDescription: '审查指令提案，并保持 AGENTS.md 审批显式可控。',
      },
      health: {
        title: '面板健康度',
        skillsActive: '技能轨道状态',
        runnerActive: '运行器轨道状态',
        instructionsActive: '指令轨道状态',
        filters: '筛选健康度',
        clearFilters: '清除筛选',
        filtersClear: '已清空',
        visibility: '可见性',
        revealSelection: '显示当前选择',
        visibilityClear: '全部可见',
        betterVersion: '更优版本',
        betterVersionSelected: '已选修订可用',
        betterVersionEmpty: '暂无已接受修订',
        selectAcceptedRevision: '选择已接受修订',
        reviewLatestCandidate: '查看最新候选',
        rollback: '回滚安全',
        rollbackSelected: '已选备份可用',
        rollbackEmpty: '暂无备份',
        openLatestBackup: '打开最新备份',
        reviewQueue: '审查队列',
        reviewQueueLoading: '正在刷新队列',
        reviewQueueNotLoaded: '尚未加载',
        reviewQueueClear: '队列已清空',
        loadProposals: '加载提案',
        openPendingProposal: '打开待处理提案',
        agents: 'AGENTS.md 审批',
        agentsLoading: '正在加载 AGENTS.md',
        agentsNotLoaded: '尚未加载',
        loadAgentsQueue: '加载 AGENTS.md 队列',
        agentsEmpty: '暂无 AGENTS.md 提案',
        reviewAgentsProposal: '审查 AGENTS.md',
        agentsReviewed: '已审查',
        skillReadiness: '技能就绪度',
        skillReadinessEmpty: '暂无已接受技能修订',
        openInSkills: '在 Skills 中打开',
        openSkills: '打开 Skills',
        openBackupInSkills: '在 Skills 中打开备份',
        switchBoundary: '切换边界',
        switchBoundaryValue: 'Skills 负责提升',
        instructionBoundary: '指令边界',
        instructionBoundaryLoadedValue: '指令已审查',
        instructionBoundaryOpenValue: '打开 Instructions',
        openInstructions: '打开 Instructions',
        skillCatalog: '技能目录',
        revisionLineage: '修订谱系',
        caseQueue: '案例队列',
        hiddenSkill: '技能',
        hiddenRevision: '修订',
        hiddenCase: '案例',
        instructionSearch: '提案搜索',
        instructionStatus: '状态筛选',
        hiddenProposal: '提案',
      },
      runner: {
        title: '运行器演进',
        followupSkipped: '已跳过后续评估',
        errorBadge: '已记录运行器错误',
        candidate: {
          title: '运行器候选补丁',
        },
        summary: {
          candidate: '关联修订',
          followup: '后续关卡',
          runtime: '运行器运行时',
          parts: '优化部分',
          partsEmpty: '没有部分变更',
          primaryPart: '主要部分',
        },
        links: {
          title: '关联证据与输出',
          sourceEval: '源评估运行',
          followupEval: '后续评估运行',
          linkedRevision: '关联技能修订',
          openSourceEval: '打开源评估',
          openFollowupEval: '打开后续评估',
          openLinkedRevision: '在 Skills 中打开',
        },
        metrics: {
          loading: '正在加载评估快照...',
          empty: '尚未附带结构化的后续评估指标。',
          sourceEmpty: '尚未附带结构化的源评估指标。',
          sourceTitle: '源评估快照',
          followupTitle: '后续评估快照',
        },
        panelTitle: '运行器设置与构建状态',
        panelSubtitle:
          '保留构建与准备控制，方便在 Evolution 控制台里直接检查候选并继续管理底层运行器二进制。',
        transcript: {
          title: '运行器记录',
          empty: '此次优化运行没有捕获到记录条目。',
          event: '事件',
        },
      },
      instructions: {
        title: '指令审查队列',
        empty: '当前没有排队的指令提案。',
        emptyActionTitle: '暂时没有提案',
        emptyAction: '打开运行器演进',
        searchPlaceholder: '按文件、经验或证据搜索提案',
        statusAll: '全部状态',
        searchEmpty: '当前筛选下没有匹配的指令提案。',
        hiddenSelectionTitle: '当前筛选隐藏了所选提案',
        hiddenSelectionAction: '显示所选提案',
        review: '审查提案',
        openSourceGroup: '打开源 Harness 分组',
        openSourceEvalRun: '打开源评估运行',
        evidence: '提案证据',
        whenToApply: '何时应用',
        evidenceText: '证据',
        patch: '补丁预览',
        approving: '批准中...',
        approve: '批准',
        rejecting: '拒绝中...',
        reject: '拒绝',
        noPatch: '暂无补丁预览。',
        approveConfirm: '确认批准这条指令提案吗？',
        rejectConfirm: '确认拒绝这条指令提案吗？',
        confirmTarget: '目标文件',
        approved: '指令提案已批准。',
        rejected: '指令提案已拒绝。',
        meta: {
          sourceKind: '来源类型',
          sourceId: '来源 ID',
          evidenceIds: '证据 ID',
          reviewNote: '审查备注',
        },
      },
    },
  },
	  'zh-TW': {
	    automation: {
	      tabs: {
	        knowledge: '知識',
	        knowledgeDesc: '查看編譯頁面、檢查健康狀態與有依據的答案。',
	        evolution: '進化',
	        evolutionDesc: '審查技能、運行器與指令演進證據。',
	      },
	    },
	    knowledge: {
	      controlEyebrow: '知識控制',
	      controlTitle: '編譯並維護 Blue 知識',
	      controlDescription: '執行編譯與檢查任務、追蹤最新維護狀態，並進入知識空間繼續查看。',
	      eyebrow: '知識空間',
	      title: '編譯頁面、檢查健康與提問歸檔',
	      description: '查看編譯後的知識頁面、執行維護操作，並提出有依據的問題。',
	      pageList: '編譯頁面',
	      pageCount: '{count} 個編譯頁面',
	      searchResultCount: '{visible} / {total} 個頁面',
	      allStatuses: '全部狀態',
	      emptyGroup: '這個分組目前沒有頁面。',
	      latestLint: '最新檢查',
	      lintHealth: '檢查健康度',
	      evidenceState: '證據與狀態',
	      evidence: {
	        status: '狀態',
	        confidence: '可信度',
	        sourceRefs: '來源引用',
	        updatedAt: '更新時間',
	        conflictsWith: '衝突於',
	        supersededBy: '被取代於',
	      },
	      pageTypes: {
	        all: '全部',
	        source_summary: '來源摘要',
	        entity: '實體',
	        concept: '概念',
	        comparison: '對比',
	        synthesis: '綜合',
	        decision: '決策',
	      },
	      status: {
	        active: '有效',
	        superseded: '已取代',
	        conflicted: '衝突',
	      },
	      confidence: {
	        low: '低',
	        medium: '中',
	        high: '高',
	      },
	      loadingPages: '正在載入頁面...',
	      sources: '來源',
	      relatedPages: '關聯頁面',
	      archivedAnswers: '歸檔答案',
	      pageListHint: '選擇一個頁面以查看來源、關聯頁面和歸檔答案。',
	      searchPlaceholder: '按標題、摘要、來源或關鍵字搜尋編譯頁面',
      clearSearch: '清除',
      searchEmpty: '目前搜尋沒有匹配的編譯頁面。請嘗試更寬鬆的標題、來源或關鍵字。',
      emptyPages: '目前還沒有編譯後的知識頁面。請先執行編譯任務建立工作區。',
      selectPage: '選擇一個編譯頁面以查看詳情。',
      noRelatedPages: '尚未關聯頁面。',
      noArchivedAnswers: '這個頁面尚未關聯歸檔答案。',
      noSummary: '暫無摘要。',
      askTitle: '向編譯後的知識空間提問',
      askHint: '回答任務會優先使用編譯頁面，並可將高品質答案歸檔回工作區。',
      askQuestion: '問題',
      askPlaceholder: 'Blue 的知識編譯是如何運作的？我下一步應該查看哪些頁面？',
      ask: '提問',
      answerResult: '回答結果',
      answerReady: '知識回答已產生。',
      answerFailed: '知識回答失敗。',
	      answerStarted: '知識回答任務已開始。',
	      archiveAnswer: '將此答案歸檔到知識工作區',
	      archived: '已歸檔',
	      pageBody: '編譯後的 Markdown',
	      activeJob: '目前任務',
	      awaitingAction: '等待處理',
	      idle: '閒置',
	      none: '無',
	      noActiveJob: '暫無活動任務',
	      compile: '編譯',
	      ingest: '匯入',
	      lint: '檢查',
	      openWorkspace: '打開知識空間',
	      compileComplete: '知識編譯已完成。',
	      ingestComplete: '知識匯入已完成。',
	      lintComplete: '知識檢查已完成。',
	      jobComplete: '知識任務已完成。',
	      jobFailed: '知識任務失敗。',
	      lintUnavailable: '暫無檢查報告。',
	      lintHealthy: '最近一次檢查未發現待處理問題。',
	      lintIssues: '有 {count} 個檢查問題需要處理。',
	      compileStarted: '知識編譯已開始。',
	      ingestStarted: '知識匯入已開始。',
	      lintStarted: '知識檢查已開始。',
	      never: '從未',
	      mapTitle: '知識地圖',
	      mapHint: '按類型瀏覽持久頁面，跨來源與摘要搜尋，並檢查證據狀態。',
	      queryTitle: '查詢',
	      queryHint: '查詢 wiki、保存高品質答案，並將持久綜合內容沉澱回知識空間。',
	      queryPlaceholder: '這個知識空間最近發生了什麼變化？哪些結論可信？我接下來該重點查什麼？',
	      activityTitle: '最近活動',
	      activityHint: '查看最新的匯入、查詢、檢查與結構定義事件。',
	      schemaTitle: '結構定義',
	      schemaHint: '定義 wiki 應如何組織與分組。',
	      openSchema: '打開結構定義',
	      openLog: '打開日誌',
	      saveSchema: '保存結構定義',
	      saveAnswer: '保存答案歷史',
	      promoteToWiki: '提升為 Wiki 頁面',
	      query: '查詢',
	      latestIngestDelta: '最近一次匯入變化',
	      changedPages: '變更頁面',
	      deltaSummary: '{newPages} 個新增，{updatedPages} 個更新',
	      conflictsLabel: '衝突',
	      gapsLabel: '空洞',
	      recentIngests: '最近匯入',
	      noRecentIngests: '暫無最近匯入。',
	      unresolvedConflicts: '未解決衝突',
	      openGaps: '待補空洞',
	      conflictCount: '{count} 個未解決衝突',
	      gapCount: '{count} 個待補空洞',
	      active: '當前',
	      schemaSaved: '知識結構定義已保存。',
	      promoted: '查詢結果已沉澱到 wiki。',
	      scopeAll: '全部',
	      scopeCurrentPage: '當前頁面',
	      scopeSelectedSources: '已選來源',
	      unknown: '未知',
	    },
	    evolution: {
	      title: '自我修復與演進主控台',
	      subtitle: '基於證據與安全審批，審查技能、運行器變更與指令提案。',
      tabs: {
        knowledge: '知識',
        skills: '技能',
        runner: '運行器',
        instructions: '審查佇列',
      },
      lanes: {
        knowledgeDescription: '以編譯頁面、有依據的查詢與衝突或空洞訊號作為整個工作台的知識底座。',
        skillsDescription: '審查規範技能、已接受修訂與回滾準備情況。',
        runnerDescription: '檢查運行器候選、執行證據與切換邊界。',
        instructionsDescription: '審查指令提案，並保持 AGENTS.md 審批顯式可控。',
      },
      health: {
        title: '面板健康度',
        skillsActive: '技能軌道狀態',
        runnerActive: '運行器軌道狀態',
        instructionsActive: '指令軌道狀態',
        filters: '篩選健康度',
        clearFilters: '清除篩選',
        filtersClear: '已清空',
        visibility: '可見性',
        revealSelection: '顯示目前選擇',
        visibilityClear: '全部可見',
        betterVersion: '更佳版本',
        betterVersionSelected: '已選修訂可用',
        betterVersionEmpty: '暫無已接受修訂',
        selectAcceptedRevision: '選擇已接受修訂',
        reviewLatestCandidate: '查看最新候選',
        rollback: '回滾安全',
        rollbackSelected: '已選備份可用',
        rollbackEmpty: '暫無備份',
        openLatestBackup: '打開最新備份',
        reviewQueue: '審查佇列',
        reviewQueueLoading: '正在刷新佇列',
        reviewQueueNotLoaded: '尚未載入',
        reviewQueueClear: '佇列已清空',
        loadProposals: '載入提案',
        openPendingProposal: '打開待處理提案',
        agents: 'AGENTS.md 審批',
        agentsLoading: '正在載入 AGENTS.md',
        agentsNotLoaded: '尚未載入',
        loadAgentsQueue: '載入 AGENTS.md 佇列',
        agentsEmpty: '暫無 AGENTS.md 提案',
        reviewAgentsProposal: '審查 AGENTS.md',
        agentsReviewed: '已審查',
        skillReadiness: '技能就緒度',
        skillReadinessEmpty: '暫無已接受技能修訂',
        openInSkills: '在 Skills 中打開',
        openSkills: '打開 Skills',
        openBackupInSkills: '在 Skills 中打開備份',
        switchBoundary: '切換邊界',
        switchBoundaryValue: 'Skills 負責提升',
        instructionBoundary: '指令邊界',
        instructionBoundaryLoadedValue: '指令已審查',
        instructionBoundaryOpenValue: '打開 Instructions',
        openInstructions: '打開 Instructions',
        skillCatalog: '技能目錄',
        revisionLineage: '修訂譜系',
        caseQueue: '案例佇列',
        hiddenSkill: '技能',
        hiddenRevision: '修訂',
        hiddenCase: '案例',
        instructionSearch: '提案搜尋',
        instructionStatus: '狀態篩選',
        hiddenProposal: '提案',
      },
      runner: {
        title: '運行器演進',
        followupSkipped: '已跳過後續評估',
        errorBadge: '已記錄運行器錯誤',
        candidate: {
          title: '運行器候選補丁',
        },
        summary: {
          candidate: '關聯修訂',
          followup: '後續關卡',
          runtime: '運行器運行時',
          parts: '優化部分',
          partsEmpty: '沒有部分變更',
          primaryPart: '主要部分',
        },
        links: {
          title: '關聯證據與輸出',
          sourceEval: '來源評估運行',
          followupEval: '後續評估運行',
          linkedRevision: '關聯技能修訂',
          openSourceEval: '打開來源評估',
          openFollowupEval: '打開後續評估',
          openLinkedRevision: '在 Skills 中打開',
        },
        metrics: {
          loading: '正在載入評估快照...',
          empty: '尚未附帶結構化的後續評估指標。',
          sourceEmpty: '尚未附帶結構化的來源評估指標。',
          sourceTitle: '來源評估快照',
          followupTitle: '後續評估快照',
        },
        panelTitle: '運行器設定與構建狀態',
        panelSubtitle:
          '保留構建與準備控制，方便在 Evolution 主控台內直接檢查候選並繼續管理底層運行器二進位。',
        transcript: {
          title: '運行器記錄',
          empty: '這次優化運行沒有捕捉到記錄條目。',
          event: '事件',
        },
      },
      instructions: {
        title: '指令審查佇列',
        empty: '目前沒有排隊中的指令提案。',
        emptyActionTitle: '暫時沒有提案',
        emptyAction: '打開運行器演進',
        searchPlaceholder: '按檔案、經驗或證據搜尋提案',
        statusAll: '全部狀態',
        searchEmpty: '目前篩選下沒有匹配的指令提案。',
        hiddenSelectionTitle: '目前篩選隱藏了所選提案',
        hiddenSelectionAction: '顯示所選提案',
        review: '審查提案',
        openSourceGroup: '打開來源 Harness 分組',
        openSourceEvalRun: '打開來源評估運行',
        evidence: '提案證據',
        whenToApply: '何時套用',
        evidenceText: '證據',
        patch: '補丁預覽',
        approving: '批准中...',
        approve: '批准',
        rejecting: '拒絕中...',
        reject: '拒絕',
        noPatch: '暫無補丁預覽。',
        approveConfirm: '確認批准這條指令提案嗎？',
        rejectConfirm: '確認拒絕這條指令提案嗎？',
        confirmTarget: '目標檔案',
        approved: '指令提案已批准。',
        rejected: '指令提案已拒絕。',
        meta: {
          sourceKind: '來源類型',
          sourceId: '來源 ID',
          evidenceIds: '證據 ID',
          reviewNote: '審查備註',
        },
      },
    },
  },
  'ja-JP': {
    automation: {
      tabs: {
        knowledge: 'ナレッジ',
        knowledgeDesc: 'コンパイル済みページ、lint 状態、根拠付き回答を確認します。',
        evolution: '進化',
        evolutionDesc: 'スキル、Runner、指示変更の根拠を確認します。',
      },
    },
    knowledge: {
      eyebrow: 'ナレッジ空間',
      title: 'コンパイル済みページ、lint 状態、質問とアーカイブ',
      description: 'コンパイル済みの知識ページを確認し、保守を実行し、根拠付きの質問を行います。',
      pageListHint: 'ページを選択して、ソース、関連ページ、アーカイブ済み回答を確認します。',
      askTitle: 'コンパイル済みナレッジ空間に質問する',
      askHint:
        '回答ジョブはまずコンパイル済みページを優先し、良い回答をワークスペースにアーカイブできます。',
      archiveAnswer: 'この回答をナレッジワークスペースにアーカイブする',
    },
    evolution: {
      title: '自己修復と進化のコンソール',
      subtitle:
        '証拠と安全な承認にもとづいて、スキル、Runner 変更、指示提案を確認します。',
      tabs: {
        knowledge: 'ナレッジ',
        skills: 'スキル',
        runner: 'Runner',
        instructions: 'レビューキュー',
      },
      lanes: {
        knowledgeDescription:
          'コンパイル済みページ、ソースに基づく質問、競合や欠落シグナルをこのワークスペースの土台として扱います。',
        skillsDescription: '正規スキル、承認済みリビジョン、ロールバック準備を確認します。',
        runnerDescription: 'Runner 候補、実行証拠、切り替え境界を確認します。',
        instructionsDescription: '指示提案を確認し、AGENTS.md 承認を明示的に保ちます。',
      },
    },
  },
  'ko-KR': {
    automation: {
      tabs: {
        knowledge: '지식',
        knowledgeDesc: '컴파일된 페이지, lint 상태, 근거 있는 답변을 확인합니다.',
        evolution: '진화',
        evolutionDesc: '스킬, Runner, 지침 변경 증거를 검토합니다.',
      },
    },
    knowledge: {
      eyebrow: '지식 공간',
      title: '컴파일된 페이지, lint 상태, 질문 및 아카이브',
      description: '컴파일된 지식 페이지를 검토하고 유지 관리를 실행하며 근거 있는 질문을 합니다.',
      pageListHint: '페이지를 선택해 소스, 관련 페이지, 아카이브된 답변을 확인합니다.',
      askTitle: '컴파일된 지식 공간에 질문하기',
      askHint:
        '답변 작업은 먼저 컴파일된 페이지를 우선 사용하고, 좋은 답변을 작업공간에 다시 보관할 수 있습니다.',
      archiveAnswer: '이 답변을 지식 작업공간에 아카이브하기',
    },
    evolution: {
      title: '자가 복구 및 진화 콘솔',
      subtitle: '증거와 안전한 승인에 따라 스킬, Runner 변경, 지침 제안을 검토합니다.',
      tabs: {
        knowledge: '지식',
        skills: '스킬',
        runner: 'Runner',
        instructions: '검토 대기열',
      },
      lanes: {
        knowledgeDescription:
          '컴파일된 페이지, 근거 있는 질의, 충돌 또는 공백 신호를 이 작업공간의 기반으로 삼습니다.',
        skillsDescription: '정식 스킬, 승인된 리비전, 롤백 준비 상태를 검토합니다.',
        runnerDescription: 'Runner 후보, 실행 증거, 전환 경계를 점검합니다.',
        instructionsDescription: '지침 제안을 검토하고 AGENTS.md 승인을 명시적으로 유지합니다.',
      },
    },
  },
  'fr-FR': {
    automation: {
      tabs: {
        knowledge: 'Connaissance',
        knowledgeDesc: 'Parcourir les pages compilées, la santé lint et les réponses fondées.',
        evolution: 'Évolution',
        evolutionDesc: 'Examiner les preuves des skills, du runner et des instructions.',
      },
    },
    knowledge: {
      eyebrow: 'Espace de connaissance',
      title: 'Pages compilées, santé lint et question/réarchivage',
      description:
        'Inspectez les pages de connaissance compilées, lancez la maintenance et posez des questions fondées.',
      pageListHint:
        'Sélectionnez une page pour examiner les sources, les pages liées et les réponses archivées.',
      askTitle: 'Interroger l’espace de connaissance compilé',
      askHint:
        'Les réponses privilégient les pages compilées et peuvent réarchiver une bonne réponse dans l’espace de travail.',
      archiveAnswer: 'Archiver cette réponse dans l’espace de connaissance',
    },
    evolution: {
      title: 'Console d’auto-réparation et d’évolution',
      subtitle:
        'Examinez les skills, les changements du runner et les propositions d’instructions avec preuves et validations sûres.',
      tabs: {
        knowledge: 'Connaissance',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'File de revue',
      },
      lanes: {
        knowledgeDescription:
          'Ancrez l’espace de travail dans les pages compilées, les requêtes appuyées sur des sources et les signaux de conflit ou de manque.',
        skillsDescription:
          'Examinez les skills canoniques, les révisions acceptées et la préparation au rollback.',
        runnerDescription:
          'Inspectez les candidats runner, les preuves d’exécution et les frontières de bascule.',
        instructionsDescription:
          'Examinez les propositions d’instructions et gardez les validations AGENTS.md explicites.',
      },
    },
  },
  'de-DE': {
    automation: {
      tabs: {
        knowledge: 'Wissen',
        knowledgeDesc: 'Kompilierte Seiten, Lint-Zustand und fundierte Antworten durchsuchen.',
        evolution: 'Evolution',
        evolutionDesc: 'Belege für Skills, Runner und Anweisungen prüfen.',
      },
    },
    knowledge: {
      eyebrow: 'Wissensraum',
      title: 'Kompilierte Seiten, Lint-Zustand und Fragen mit Archivierung',
      description:
        'Prüfen Sie kompilierte Wissensseiten, führen Sie Wartung aus und stellen Sie fundierte Fragen.',
      pageListHint:
        'Wählen Sie eine Seite aus, um Quellen, verknüpfte Seiten und archivierte Antworten zu prüfen.',
      askTitle: 'Den kompilierten Wissensraum befragen',
      askHint:
        'Antwortjobs bevorzugen zuerst kompilierte Seiten und können gute Antworten zurück ins Workspace archivieren.',
      archiveAnswer: 'Diese Antwort im Wissens-Workspace archivieren',
    },
    evolution: {
      title: 'Konsole für Selbstreparatur und Evolution',
      subtitle:
        'Prüfen Sie Skills, Runner-Änderungen und Instruktionsvorschläge mit Belegen und sicheren Freigaben.',
      tabs: {
        knowledge: 'Wissen',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Prüfwarteschlange',
      },
      lanes: {
        knowledgeDescription:
          'Verankern Sie den Arbeitsbereich in kompilierten Seiten, quellengestützten Abfragen sowie Konflikt- und Lückensignalen.',
        skillsDescription:
          'Prüfen Sie kanonische Skills, akzeptierte Revisionen und die Rollback-Bereitschaft.',
        runnerDescription:
          'Prüfen Sie Runner-Kandidaten, Ausführungsbelege und Umschaltgrenzen.',
        instructionsDescription:
          'Prüfen Sie Instruktionsvorschläge und halten Sie AGENTS.md-Freigaben ausdrücklich fest.',
      },
    },
  },
  'es-ES': {
    automation: {
      tabs: {
        knowledge: 'Conocimiento',
        knowledgeDesc: 'Explora páginas compiladas, estado de lint y respuestas fundamentadas.',
        evolution: 'Evolución',
        evolutionDesc: 'Revisa la evidencia de skills, runner e instrucciones.',
      },
    },
    knowledge: {
      eyebrow: 'Espacio de conocimiento',
      title: 'Páginas compiladas, estado de lint y preguntar/archivar',
      description:
        'Inspecciona páginas de conocimiento compiladas, ejecuta mantenimiento y haz preguntas fundamentadas.',
      pageListHint:
        'Selecciona una página para revisar fuentes, páginas relacionadas y respuestas archivadas.',
      askTitle: 'Preguntar al espacio de conocimiento compilado',
      askHint:
        'Los trabajos de respuesta priorizan las páginas compiladas y pueden archivar una buena respuesta en el workspace.',
      archiveAnswer: 'Archivar esta respuesta en el workspace de conocimiento',
    },
    evolution: {
      title: 'Consola de autorreparación y evolución',
      subtitle:
        'Revisa skills, cambios del runner y propuestas de instrucciones con evidencia y aprobaciones seguras.',
      tabs: {
        knowledge: 'Conocimiento',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Cola de revisión',
      },
      lanes: {
        knowledgeDescription:
          'Sustenta el espacio de trabajo con páginas compiladas, consultas respaldadas por fuentes y señales de conflictos o vacíos.',
        skillsDescription:
          'Revisa skills canónicas, revisiones aceptadas y preparación para rollback.',
        runnerDescription:
          'Inspecciona candidatos de runner, evidencia de ejecución y límites de cambio.',
        instructionsDescription:
          'Revisa propuestas de instrucciones y mantén explícitas las aprobaciones de AGENTS.md.',
      },
    },
  },
  'pt-BR': {
    automation: {
      tabs: {
        knowledge: 'Conhecimento',
        knowledgeDesc: 'Navegue por páginas compiladas, saúde do lint e respostas fundamentadas.',
        evolution: 'Evolução',
        evolutionDesc: 'Revise evidências de skills, runner e instruções.',
      },
    },
    knowledge: {
      eyebrow: 'Espaço de conhecimento',
      title: 'Páginas compiladas, saúde do lint e perguntar/arquivar',
      description:
        'Inspecione páginas de conhecimento compiladas, execute manutenção e faça perguntas fundamentadas.',
      pageListHint:
        'Selecione uma página para revisar fontes, páginas relacionadas e respostas arquivadas.',
      askTitle: 'Perguntar ao espaço de conhecimento compilado',
      askHint:
        'Os trabalhos de resposta priorizam páginas compiladas e podem arquivar uma boa resposta de volta no workspace.',
      archiveAnswer: 'Arquivar esta resposta no workspace de conhecimento',
    },
    evolution: {
      title: 'Console de autoreparo e evolução',
      subtitle:
        'Revise skills, mudanças do runner e propostas de instruções com evidências e aprovações seguras.',
      tabs: {
        knowledge: 'Conhecimento',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Fila de revisão',
      },
      lanes: {
        knowledgeDescription:
          'Baseie o workspace em páginas compiladas, consultas com respaldo em fontes e sinais de conflitos ou lacunas.',
        skillsDescription:
          'Revise skills canônicas, revisões aceitas e prontidão para rollback.',
        runnerDescription:
          'Inspecione candidatos de runner, evidências de execução e limites de troca.',
        instructionsDescription:
          'Revise propostas de instruções e mantenha as aprovações de AGENTS.md explícitas.',
      },
    },
  },
  'pt-PT': {
    automation: {
      tabs: {
        knowledge: 'Conhecimento',
        knowledgeDesc: 'Navegue por páginas compiladas, estado do lint e respostas fundamentadas.',
        evolution: 'Evolução',
        evolutionDesc: 'Reveja evidências de skills, runner e instruções.',
      },
    },
    knowledge: {
      eyebrow: 'Espaço de conhecimento',
      title: 'Páginas compiladas, estado do lint e perguntar/arquivar',
      description:
        'Inspecione páginas de conhecimento compiladas, execute manutenção e faça perguntas fundamentadas.',
      pageListHint:
        'Selecione uma página para rever fontes, páginas relacionadas e respostas arquivadas.',
      askTitle: 'Perguntar ao espaço de conhecimento compilado',
      askHint:
        'Os trabalhos de resposta priorizam páginas compiladas e podem arquivar uma boa resposta de volta no workspace.',
      archiveAnswer: 'Arquivar esta resposta no workspace de conhecimento',
    },
    evolution: {
      title: 'Consola de autorreparação e evolução',
      subtitle:
        'Reveja skills, alterações do runner e propostas de instruções com evidência e aprovações seguras.',
      tabs: {
        knowledge: 'Conhecimento',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Fila de revisão',
      },
      lanes: {
        knowledgeDescription:
          'Baseie o workspace em páginas compiladas, consultas suportadas por fontes e sinais de conflitos ou lacunas.',
        skillsDescription:
          'Reveja skills canónicas, revisões aceites e preparação para rollback.',
        runnerDescription:
          'Inspecione candidatos de runner, evidência de execução e limites de mudança.',
        instructionsDescription:
          'Reveja propostas de instruções e mantenha explícitas as aprovações de AGENTS.md.',
      },
    },
  },
  'it-IT': {
    automation: {
      tabs: {
        knowledge: 'Conoscenza',
        knowledgeDesc: 'Sfoglia pagine compilate, stato lint e risposte fondate.',
        evolution: 'Evoluzione',
        evolutionDesc: 'Esamina le evidenze di skill, runner e istruzioni.',
      },
    },
    knowledge: {
      eyebrow: 'Spazio conoscenza',
      title: 'Pagine compilate, stato lint e chiedi/archivia',
      description:
        'Ispeziona le pagine di conoscenza compilate, esegui manutenzione e poni domande fondate.',
      pageListHint:
        'Seleziona una pagina per esaminare fonti, pagine correlate e risposte archiviate.',
      askTitle: 'Chiedi allo spazio di conoscenza compilato',
      askHint:
        'I job di risposta privilegiano prima le pagine compilate e possono archiviare una buona risposta nel workspace.',
      archiveAnswer: 'Archivia questa risposta nel workspace di conoscenza',
    },
    evolution: {
      title: 'Console di autoriparazione ed evoluzione',
      subtitle:
        'Esamina skill, cambiamenti del runner e proposte di istruzioni con evidenze e approvazioni sicure.',
      tabs: {
        knowledge: 'Conoscenza',
        skills: 'Skill',
        runner: 'Runner',
        instructions: 'Coda di revisione',
      },
      lanes: {
        knowledgeDescription:
          'Radica il workspace in pagine compilate, query supportate da fonti e segnali di conflitti o lacune.',
        skillsDescription:
          'Esamina skill canoniche, revisioni accettate e preparazione al rollback.',
        runnerDescription:
          'Ispeziona candidati runner, evidenze di esecuzione e confini di switch.',
        instructionsDescription:
          'Esamina le proposte di istruzioni e mantieni esplicite le approvazioni di AGENTS.md.',
      },
    },
  },
  'nl-NL': {
    automation: {
      tabs: {
        knowledge: 'Kennis',
        knowledgeDesc: 'Bekijk gecompileerde pagina’s, lint-status en onderbouwde antwoorden.',
        evolution: 'Evolutie',
        evolutionDesc: 'Bekijk bewijs voor skills, runner en instructies.',
      },
    },
    knowledge: {
      eyebrow: 'Kennisruimte',
      title: 'Gecompileerde pagina’s, lint-status en vragen/archiveren',
      description:
        'Inspecteer gecompileerde kennispagina’s, voer onderhoud uit en stel onderbouwde vragen.',
      pageListHint:
        'Selecteer een pagina om bronnen, gerelateerde pagina’s en gearchiveerde antwoorden te bekijken.',
      askTitle: 'Vraag de gecompileerde kennisruimte',
      askHint:
        'Antwoordtaken geven voorrang aan gecompileerde pagina’s en kunnen een goed antwoord terug in de werkruimte archiveren.',
      archiveAnswer: 'Dit antwoord archiveren in de kenniswerkruimte',
    },
    evolution: {
      title: 'Console voor zelfherstel en evolutie',
      subtitle:
        'Bekijk skills, runner-wijzigingen en instructievoorstellen met bewijs en veilige goedkeuringen.',
      tabs: {
        knowledge: 'Kennis',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Beoordelingswachtrij',
      },
      lanes: {
        knowledgeDescription:
          'Veranker de werkruimte in gecompileerde pagina’s, bron-onderbouwde query’s en signalen van conflicten of hiaten.',
        skillsDescription:
          'Bekijk canonieke skills, geaccepteerde revisies en rollback-gereedheid.',
        runnerDescription:
          'Inspecteer runner-kandidaten, uitvoeringsbewijs en schakelgrenzen.',
        instructionsDescription:
          'Bekijk instructievoorstellen en houd AGENTS.md-goedkeuringen expliciet.',
      },
    },
  },
  'pl-PL': {
    automation: {
      tabs: {
        knowledge: 'Wiedza',
        knowledgeDesc: 'Przeglądaj skompilowane strony, stan lint i uzasadnione odpowiedzi.',
        evolution: 'Ewolucja',
        evolutionDesc: 'Przeglądaj dowody zmian skilli, runnera i instrukcji.',
      },
    },
    knowledge: {
      eyebrow: 'Przestrzeń wiedzy',
      title: 'Skompilowane strony, stan lint i pytaj/archiwizuj',
      description:
        'Przeglądaj skompilowane strony wiedzy, uruchamiaj utrzymanie i zadawaj uzasadnione pytania.',
      pageListHint:
        'Wybierz stronę, aby sprawdzić źródła, powiązane strony i zarchiwizowane odpowiedzi.',
      askTitle: 'Zapytaj skompilowaną przestrzeń wiedzy',
      askHint:
        'Zadania odpowiedzi najpierw korzystają ze skompilowanych stron i mogą archiwizować dobre odpowiedzi z powrotem do workspace.',
      archiveAnswer: 'Zarchiwizuj tę odpowiedź w obszarze wiedzy',
    },
    evolution: {
      title: 'Konsola samonaprawy i ewolucji',
      subtitle:
        'Przeglądaj skille, zmiany runnera i propozycje instrukcji z dowodami oraz bezpiecznymi akceptacjami.',
      tabs: {
        knowledge: 'Wiedza',
        skills: 'Skille',
        runner: 'Runner',
        instructions: 'Kolejka przeglądu',
      },
      lanes: {
        knowledgeDescription:
          'Oprzyj przestrzeń roboczą na skompilowanych stronach, zapytaniach popartych źródłami oraz sygnałach konfliktów lub luk.',
        skillsDescription:
          'Przeglądaj kanoniczne skille, zaakceptowane rewizje i gotowość do rollbacku.',
        runnerDescription:
          'Sprawdzaj kandydatów runnera, dowody wykonania i granice przełączenia.',
        instructionsDescription:
          'Przeglądaj propozycje instrukcji i utrzymuj jawne zatwierdzenia AGENTS.md.',
      },
    },
  },
  'ru-RU': {
    automation: {
      tabs: {
        knowledge: 'Знания',
        knowledgeDesc: 'Просматривайте скомпилированные страницы, lint и обоснованные ответы.',
        evolution: 'Эволюция',
        evolutionDesc: 'Проверяйте доказательства по навыкам, runner и инструкциям.',
      },
    },
    knowledge: {
      eyebrow: 'Пространство знаний',
      title: 'Скомпилированные страницы, lint и вопросы с архивацией',
      description:
        'Просматривайте скомпилированные страницы знаний, запускайте обслуживание и задавайте обоснованные вопросы.',
      pageListHint:
        'Выберите страницу, чтобы изучить источники, связанные страницы и архивные ответы.',
      askTitle: 'Спросить скомпилированное пространство знаний',
      askHint:
        'Задания ответов сначала используют скомпилированные страницы и могут архивировать хороший ответ обратно в workspace.',
      archiveAnswer: 'Архивировать этот ответ в пространство знаний',
    },
    evolution: {
      title: 'Консоль самовосстановления и эволюции',
      subtitle:
        'Проверяйте навыки, изменения runner и предложения по инструкциям с доказательствами и безопасными одобрениями.',
      tabs: {
        knowledge: 'Знания',
        skills: 'Навыки',
        runner: 'Runner',
        instructions: 'Очередь проверки',
      },
      lanes: {
        knowledgeDescription:
          'Сделайте основой рабочего пространства скомпилированные страницы, запросы с опорой на источники и сигналы конфликтов или пробелов.',
        skillsDescription:
          'Проверяйте канонические навыки, принятые ревизии и готовность к откату.',
        runnerDescription:
          'Проверяйте кандидатов runner, доказательства выполнения и границы переключения.',
        instructionsDescription:
          'Проверяйте предложения по инструкциям и сохраняйте явные одобрения AGENTS.md.',
      },
    },
  },
  'sv-SE': {
    automation: {
      tabs: {
        knowledge: 'Kunskap',
        knowledgeDesc: 'Bläddra bland kompilerade sidor, lint-status och välgrundade svar.',
        evolution: 'Evolution',
        evolutionDesc: 'Granska bevis för skills, runner och instruktioner.',
      },
    },
    knowledge: {
      eyebrow: 'Kunskapsyta',
      title: 'Kompilerade sidor, lint-status och fråga/arkivera',
      description:
        'Granska kompilerade kunskapssidor, kör underhåll och ställ välgrundade frågor.',
      pageListHint:
        'Välj en sida för att granska källor, relaterade sidor och arkiverade svar.',
      askTitle: 'Fråga den kompilerade kunskapsytan',
      askHint:
        'Svarsjobb prioriterar kompilerade sidor först och kan arkivera ett bra svar tillbaka till arbetsytan.',
      archiveAnswer: 'Arkivera detta svar i kunskapsarbetsytan',
    },
    evolution: {
      title: 'Konsol för självreparation och evolution',
      subtitle:
        'Granska skills, runner-ändringar och instruktionsförslag med bevis och säkra godkännanden.',
      tabs: {
        knowledge: 'Kunskap',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Granskningskö',
      },
      lanes: {
        knowledgeDescription:
          'Förankra arbetsytan i kompilerade sidor, källstödda frågor och signaler om konflikter eller luckor.',
        skillsDescription:
          'Granska kanoniska skills, accepterade revisioner och rollback-beredskap.',
        runnerDescription:
          'Granska runner-kandidater, körningsbevis och växlingsgränser.',
        instructionsDescription:
          'Granska instruktionsförslag och håll AGENTS.md-godkännanden uttryckliga.',
      },
    },
  },
  'nb-NO': {
    automation: {
      tabs: {
        knowledge: 'Kunnskap',
        knowledgeDesc: 'Bla gjennom kompilerte sider, lint-status og begrunnede svar.',
        evolution: 'Evolusjon',
        evolutionDesc: 'Gjennomgå bevis for ferdigheter, runner og instruksjoner.',
      },
    },
    knowledge: {
      eyebrow: 'Kunnskapsrom',
      title: 'Kompilerte sider, lint-status og spør/arkiver',
      description:
        'Inspiser kompilerte kunnskapssider, kjør vedlikehold og still begrunnede spørsmål.',
      pageListHint:
        'Velg en side for å inspisere kilder, relaterte sider og arkiverte svar.',
      askTitle: 'Spør det kompilerte kunnskapsrommet',
      askHint:
        'Svarjobber prioriterer kompilerte sider først og kan arkivere et godt svar tilbake til arbeidsområdet.',
      archiveAnswer: 'Arkiver dette svaret i kunnskapsarbeidsområdet',
    },
    evolution: {
      title: 'Konsoll for selvreparasjon og evolusjon',
      subtitle:
        'Gjennomgå ferdigheter, runner-endringer og instruksjonsforslag med bevis og trygge godkjenninger.',
      tabs: {
        knowledge: 'Kunnskap',
        skills: 'Ferdigheter',
        runner: 'Runner',
        instructions: 'Vurderingskø',
      },
      lanes: {
        knowledgeDescription:
          'Forankre arbeidsområdet i kompilerte sider, kildebaserte spørsmål og signaler om konflikter eller hull.',
        skillsDescription:
          'Gjennomgå kanoniske ferdigheter, aksepterte revisjoner og rollback-beredskap.',
        runnerDescription:
          'Inspiser runner-kandidater, kjørselsbevis og byttegrenser.',
        instructionsDescription:
          'Gjennomgå instruksjonsforslag og hold AGENTS.md-godkjenninger eksplisitte.',
      },
    },
  },
  'da-DK': {
    automation: {
      tabs: {
        knowledge: 'Viden',
        knowledgeDesc: 'Gennemse kompilerede sider, lint-status og velbegrundede svar.',
        evolution: 'Evolution',
        evolutionDesc: 'Gennemse evidens for skills, runner og instruktioner.',
      },
    },
    knowledge: {
      eyebrow: 'Vidensrum',
      title: 'Kompilerede sider, lint-status og spørg/arkivér',
      description:
        'Gennemse kompilerede videnssider, kør vedligeholdelse og stil velbegrundede spørgsmål.',
      pageListHint:
        'Vælg en side for at undersøge kilder, relaterede sider og arkiverede svar.',
      askTitle: 'Spørg det kompilerede vidensrum',
      askHint:
        'Svarjob prioriterer kompilerede sider først og kan arkivere et godt svar tilbage i arbejdsområdet.',
      archiveAnswer: 'Arkivér dette svar i vidensarbejdsområdet',
    },
    evolution: {
      title: 'Konsol for selvreparation og evolution',
      subtitle:
        'Gennemse skills, runner-ændringer og instruktionsforslag med evidens og sikre godkendelser.',
      tabs: {
        knowledge: 'Viden',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Gennemgangskø',
      },
      lanes: {
        knowledgeDescription:
          'Forankr arbejdsområdet i kompilerede sider, kildeunderbyggede forespørgsler og signaler om konflikter eller huller.',
        skillsDescription:
          'Gennemse kanoniske skills, accepterede revisioner og rollback-beredskab.',
        runnerDescription:
          'Undersøg runner-kandidater, udførelsesevidens og skiftegrænser.',
        instructionsDescription:
          'Gennemse instruktionsforslag og hold AGENTS.md-godkendelser eksplicitte.',
      },
    },
  },
  'cs-CZ': {
    automation: {
      tabs: {
        knowledge: 'Znalosti',
        knowledgeDesc: 'Procházejte kompilované stránky, stav lintu a podložené odpovědi.',
        evolution: 'Evoluce',
        evolutionDesc: 'Kontrolujte důkazy pro dovednosti, runner a instrukce.',
      },
    },
    knowledge: {
      eyebrow: 'Znalostní prostor',
      title: 'Kompilované stránky, stav lintu a dotazování s archivací',
      description:
        'Procházejte kompilované znalostní stránky, spouštějte údržbu a pokládejte podložené otázky.',
      pageListHint:
        'Vyberte stránku pro kontrolu zdrojů, souvisejících stránek a archivovaných odpovědí.',
      askTitle: 'Položit dotaz kompilovanému znalostnímu prostoru',
      askHint:
        'Úlohy odpovědí nejprve upřednostňují kompilované stránky a mohou dobrou odpověď archivovat zpět do workspace.',
      archiveAnswer: 'Archivovat tuto odpověď do znalostního workspace',
    },
    evolution: {
      title: 'Konzole samoopravy a evoluce',
      subtitle:
        'Kontrolujte dovednosti, změny runneru a návrhy instrukcí s důkazy a bezpečnými schváleními.',
      tabs: {
        knowledge: 'Znalosti',
        skills: 'Dovednosti',
        runner: 'Runner',
        instructions: 'Fronta revize',
      },
      lanes: {
        knowledgeDescription:
          'Ukotvěte pracovní prostor v kompilovaných stránkách, dotazech podložených zdroji a signálech konfliktů nebo mezer.',
        skillsDescription:
          'Kontrolujte kanonické dovednosti, přijaté revize a připravenost na rollback.',
        runnerDescription:
          'Prověřujte kandidáty runneru, důkazy z běhu a hranice přepnutí.',
        instructionsDescription:
          'Kontrolujte návrhy instrukcí a ponechte schválení AGENTS.md explicitní.',
      },
    },
  },
  'sk-SK': {
    automation: {
      tabs: {
        knowledge: 'Znalosti',
        knowledgeDesc: 'Prehliadajte kompilované stránky, stav lintu a podložené odpovede.',
        evolution: 'Evolúcia',
        evolutionDesc: 'Kontrolujte dôkazy pre zručnosti, runner a inštrukcie.',
      },
    },
    knowledge: {
      eyebrow: 'Priestor znalostí',
      title: 'Kompilované stránky, stav lintu a pýtanie s archiváciou',
      description:
        'Prehliadajte kompilované znalostné stránky, spúšťajte údržbu a kladte podložené otázky.',
      pageListHint:
        'Vyberte stránku na kontrolu zdrojov, súvisiacich stránok a archivovaných odpovedí.',
      askTitle: 'Položiť otázku kompilovanému priestoru znalostí',
      askHint:
        'Úlohy odpovedí najprv uprednostňujú kompilované stránky a môžu dobrú odpoveď archivovať späť do workspace.',
      archiveAnswer: 'Archivovať túto odpoveď do znalostného workspace',
    },
    evolution: {
      title: 'Konzola samoopravy a evolúcie',
      subtitle:
        'Kontrolujte zručnosti, zmeny runnera a návrhy inštrukcií s dôkazmi a bezpečnými schváleniami.',
      tabs: {
        knowledge: 'Znalosti',
        skills: 'Zručnosti',
        runner: 'Runner',
        instructions: 'Front kontrol',
      },
      lanes: {
        knowledgeDescription:
          'Ukotvite workspace v kompilovaných stránkach, otázkach podložených zdrojmi a signáloch konfliktov alebo medzier.',
        skillsDescription:
          'Kontrolujte kanonické zručnosti, prijaté revízie a pripravenosť na rollback.',
        runnerDescription:
          'Skontrolujte kandidátov runnera, dôkazy z behu a hranice prepnutia.',
        instructionsDescription:
          'Kontrolujte návrhy inštrukcií a ponechajte schválenia AGENTS.md explicitné.',
      },
    },
  },
  'ro-RO': {
    automation: {
      tabs: {
        knowledge: 'Cunoștințe',
        knowledgeDesc: 'Răsfoiți pagini compilate, starea lint și răspunsuri fundamentate.',
        evolution: 'Evoluție',
        evolutionDesc: 'Revizuiți dovezile pentru skill-uri, runner și instrucțiuni.',
      },
    },
    knowledge: {
      eyebrow: 'Spațiul de cunoștințe',
      title: 'Pagini compilate, starea lint și întreabă/arhivează',
      description:
        'Inspectați paginile de cunoștințe compilate, rulați mentenanță și puneți întrebări fundamentate.',
      pageListHint:
        'Selectați o pagină pentru a inspecta sursele, paginile conexe și răspunsurile arhivate.',
      askTitle: 'Întreabă spațiul de cunoștințe compilat',
      askHint:
        'Joburile de răspuns prioritizează mai întâi paginile compilate și pot arhiva un răspuns bun înapoi în workspace.',
      archiveAnswer: 'Arhivează acest răspuns în workspace-ul de cunoștințe',
    },
    evolution: {
      title: 'Consolă de auto-reparare și evoluție',
      subtitle:
        'Revizuiți skill-uri, schimbări ale runnerului și propuneri de instrucțiuni cu dovezi și aprobări sigure.',
      tabs: {
        knowledge: 'Cunoștințe',
        skills: 'Skill-uri',
        runner: 'Runner',
        instructions: 'Coada de revizuire',
      },
      lanes: {
        knowledgeDescription:
          'Ancorați workspace-ul în pagini compilate, interogări susținute de surse și semnale despre conflicte sau goluri.',
        skillsDescription:
          'Revizuiți skill-urile canonice, reviziile acceptate și pregătirea pentru rollback.',
        runnerDescription:
          'Inspectați candidații runner, dovezile de execuție și limitele de comutare.',
        instructionsDescription:
          'Revizuiți propunerile de instrucțiuni și păstrați aprobările AGENTS.md explicite.',
      },
    },
  },
  'hu-HU': {
    automation: {
      tabs: {
        knowledge: 'Tudás',
        knowledgeDesc: 'Böngéssze az összeállított oldalakat, a lint állapotát és a megalapozott válaszokat.',
        evolution: 'Evolúció',
        evolutionDesc: 'Vizsgálja a skillek, a runner és az utasítások bizonyítékait.',
      },
    },
    knowledge: {
      eyebrow: 'Tudástér',
      title: 'Összeállított oldalak, lint állapot és kérdezés/archiválás',
      description:
        'Vizsgálja az összeállított tudásoldalakat, futtasson karbantartást, és tegyen fel megalapozott kérdéseket.',
      pageListHint:
        'Válasszon oldalt a források, kapcsolódó oldalak és archivált válaszok áttekintéséhez.',
      askTitle: 'Kérdezzen az összeállított tudástértől',
      askHint:
        'A válaszfeladatok először az összeállított oldalakat részesítik előnyben, és egy jó választ vissza tudnak archiválni a workspace-be.',
      archiveAnswer: 'A válasz archiválása a tudás-workspace-be',
    },
    evolution: {
      title: 'Önjavítási és evolúciós konzol',
      subtitle:
        'Vizsgálja a skilleket, a runner változásait és az utasításjavaslatokat bizonyítékokkal és biztonságos jóváhagyásokkal.',
      tabs: {
        knowledge: 'Tudás',
        skills: 'Skillek',
        runner: 'Runner',
        instructions: 'Felülvizsgálati sor',
      },
      lanes: {
        knowledgeDescription:
          'Az workspace alapját az összeállított oldalak, forrásokkal alátámasztott lekérdezések, valamint az ütközésekre vagy hiányokra utaló jelzések adják.',
        skillsDescription:
          'Vizsgálja a kanonikus skilleket, az elfogadott revíziókat és a rollback-készültséget.',
        runnerDescription:
          'Vizsgálja a runner-jelölteket, a végrehajtási bizonyítékokat és az átváltási határokat.',
        instructionsDescription:
          'Vizsgálja az utasításjavaslatokat, és tartsa az AGENTS.md jóváhagyásokat egyértelműen kifejezettnek.',
      },
    },
  },
  'hr-HR': {
    automation: {
      tabs: {
        knowledge: 'Znanje',
        knowledgeDesc: 'Pregledajte kompilirane stranice, lint stanje i utemeljene odgovore.',
        evolution: 'Evolucija',
        evolutionDesc: 'Pregledajte dokaze za vještine, runner i upute.',
      },
    },
    knowledge: {
      eyebrow: 'Prostor znanja',
      title: 'Kompilirane stranice, lint stanje i pitaj/arhiviraj',
      description:
        'Pregledajte kompilirane stranice znanja, pokrenite održavanje i postavljajte utemeljena pitanja.',
      pageListHint:
        'Odaberite stranicu za pregled izvora, povezanih stranica i arhiviranih odgovora.',
      askTitle: 'Pitaju kompilirani prostor znanja',
      askHint:
        'Poslovi odgovora najprije daju prednost kompiliranim stranicama i mogu dobar odgovor arhivirati natrag u workspace.',
      archiveAnswer: 'Arhiviraj ovaj odgovor u radni prostor znanja',
    },
    evolution: {
      title: 'Konzola za samopopravak i evoluciju',
      subtitle:
        'Pregledajte vještine, promjene runnera i prijedloge uputa uz dokaze i sigurna odobrenja.',
      tabs: {
        knowledge: 'Znanje',
        skills: 'Vještine',
        runner: 'Runner',
        instructions: 'Red za pregled',
      },
      lanes: {
        knowledgeDescription:
          'Utemeljite radni prostor na kompiliranim stranicama, upitima potkrijepljenima izvorima te signalima sukoba ili praznina.',
        skillsDescription:
          'Pregledajte kanonske vještine, prihvaćene revizije i spremnost za rollback.',
        runnerDescription:
          'Pregledajte kandidate runnera, dokaze izvođenja i granice prebacivanja.',
        instructionsDescription:
          'Pregledajte prijedloge uputa i zadržite AGENTS.md odobrenja izričitima.',
      },
    },
  },
  'ga-IE': {
    automation: {
      tabs: {
        knowledge: 'Eolas',
        knowledgeDesc: 'Brabhsáil leathanaigh chomhdhlúite, stádas lint agus freagraí bunaithe.',
        evolution: 'Éabhlóid',
        evolutionDesc: 'Athbhreithnigh fianaise ar scileanna, runner agus treoracha.',
      },
    },
    knowledge: {
      eyebrow: 'Spás eolais',
      title: 'Leathanaigh chomhdhlúite, stádas lint agus ceist/cartlannú',
      description:
        'Scrúdaigh leathanaigh eolais chomhdhlúite, rith cothabháil agus cuir ceisteanna bunaithe.',
      pageListHint:
        'Roghnaigh leathanach chun foinsí, leathanaigh ghaolmhara agus freagraí cartlannaithe a scrúdú.',
      askTitle: 'Cuir ceist ar an spás eolais comhdhlúite',
      askHint:
        'Tugann poist freagartha tús áite do leathanaigh chomhdhlúite agus is féidir leo freagra maith a chartlannú ar ais sa workspace.',
      archiveAnswer: 'Cartlannaigh an freagra seo sa workspace eolais',
    },
    evolution: {
      title: 'Consól féin-dheisiúcháin agus éabhlóide',
      subtitle:
        'Athbhreithnigh scileanna, athruithe runner agus tograí treoracha le fianaise agus ceaduithe sábháilte.',
      tabs: {
        knowledge: 'Eolas',
        skills: 'Scileanna',
        runner: 'Runner',
        instructions: 'Scuaine athbhreithnithe',
      },
      lanes: {
        knowledgeDescription:
          'Bunaigh an workspace ar leathanaigh chomhdhlúite, ceisteanna le tacaíocht foinsí agus comharthaí coinbhleachta nó bearnaí.',
        skillsDescription:
          'Athbhreithnigh scileanna canónacha, leasuithe glactha agus ullmhacht rollback.',
        runnerDescription:
          'Scrúdaigh iarrthóirí runner, fianaise forghníomhaithe agus teorainneacha athraithe.',
        instructionsDescription:
          'Athbhreithnigh tograí treoracha agus coinnigh ceaduithe AGENTS.md soiléir.',
      },
    },
  },
  'ca-ES': {
    automation: {
      tabs: {
        knowledge: 'Coneixement',
        knowledgeDesc: 'Consulta pàgines compilades, salut de lint i respostes fonamentades.',
        evolution: 'Evolució',
        evolutionDesc: 'Revisa l’evidència de skills, runner i instruccions.',
      },
    },
    knowledge: {
      eyebrow: 'Espai de coneixement',
      title: 'Pàgines compilades, salut de lint i preguntar/arxivar',
      description:
        'Inspecciona les pàgines de coneixement compilades, executa manteniment i fes preguntes fonamentades.',
      pageListHint:
        'Selecciona una pàgina per inspeccionar fonts, pàgines relacionades i respostes arxivades.',
      askTitle: 'Pregunta a l’espai de coneixement compilat',
      askHint:
        'Les tasques de resposta prioritzen les pàgines compilades i poden arxivar una bona resposta al workspace.',
      archiveAnswer: 'Arxiva aquesta resposta a l’espai de coneixement',
    },
    evolution: {
      title: 'Consola d’autoreparació i evolució',
      subtitle:
        'Revisa skills, canvis del runner i propostes d’instruccions amb evidència i aprovacions segures.',
      tabs: {
        knowledge: 'Coneixement',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Cua de revisió',
      },
      lanes: {
        knowledgeDescription:
          'Arrela l’espai de treball en pàgines compilades, consultes sostingudes per fonts i senyals de conflictes o buits.',
        skillsDescription:
          'Revisa skills canòniques, revisions acceptades i preparació per al rollback.',
        runnerDescription:
          'Inspecciona candidats de runner, evidència d’execució i límits de canvi.',
        instructionsDescription:
          'Revisa propostes d’instruccions i mantén explícites les aprovacions d’AGENTS.md.',
      },
    },
  },
  'el-GR': {
    automation: {
      tabs: {
        knowledge: 'Γνώση',
        knowledgeDesc: 'Περιηγηθείτε σε μεταγλωττισμένες σελίδες, κατάσταση lint και τεκμηριωμένες απαντήσεις.',
        evolution: 'Εξέλιξη',
        evolutionDesc: 'Ελέγξτε αποδείξεις για skills, runner και οδηγίες.',
      },
    },
    knowledge: {
      eyebrow: 'Χώρος γνώσης',
      title: 'Μεταγλωττισμένες σελίδες, κατάσταση lint και ερώτηση/αρχειοθέτηση',
      description:
        'Επιθεωρήστε τις μεταγλωττισμένες σελίδες γνώσης, εκτελέστε συντήρηση και κάντε τεκμηριωμένες ερωτήσεις.',
      pageListHint:
        'Επιλέξτε μια σελίδα για να ελέγξετε πηγές, σχετικές σελίδες και αρχειοθετημένες απαντήσεις.',
      askTitle: 'Ρωτήστε τον μεταγλωττισμένο χώρο γνώσης',
      askHint:
        'Οι εργασίες απάντησης προτιμούν πρώτα τις μεταγλωττισμένες σελίδες και μπορούν να αρχειοθετήσουν μια καλή απάντηση πίσω στο workspace.',
      archiveAnswer: 'Αρχειοθετήστε αυτήν την απάντηση στον χώρο γνώσης',
    },
    evolution: {
      title: 'Κονσόλα αυτοεπιδιόρθωσης και εξέλιξης',
      subtitle:
        'Ελέγξτε skills, αλλαγές runner και προτάσεις οδηγιών με αποδείξεις και ασφαλείς εγκρίσεις.',
      tabs: {
        knowledge: 'Γνώση',
        skills: 'Skills',
        runner: 'Runner',
        instructions: 'Ουρά αξιολόγησης',
      },
      lanes: {
        knowledgeDescription:
          'Θεμελιώστε τον χώρο εργασίας σε μεταγλωττισμένες σελίδες, ερωτήματα με τεκμηρίωση από πηγές και σήματα συγκρούσεων ή κενών.',
        skillsDescription:
          'Ελέγξτε κανονικά skills, αποδεκτές αναθεωρήσεις και ετοιμότητα rollback.',
        runnerDescription:
          'Επιθεωρήστε υποψήφιους runner, αποδείξεις εκτέλεσης και όρια εναλλαγής.',
        instructionsDescription:
          'Ελέγξτε προτάσεις οδηγιών και διατηρήστε ρητές τις εγκρίσεις του AGENTS.md.',
      },
    },
  },
  'ml-IN': {
    automation: {
      tabs: {
        knowledge: 'ജ്ഞാനം',
        knowledgeDesc: 'കമ്പൈൽ ചെയ്ത പേജുകൾ, lint നില, അടിസ്ഥാനമുള്ള ഉത്തരങ്ങൾ എന്നിവ ബ്രൗസ് ചെയ്യുക.',
        evolution: 'പരിണാമം',
        evolutionDesc: 'സ്കില്ലുകൾ, runner, നിർദ്ദേശങ്ങൾ എന്നിവയുടെ തെളിവുകൾ പരിശോധിക്കുക.',
      },
    },
    knowledge: {
      eyebrow: 'ജ്ഞാന സ്ഥലം',
      title: 'കമ്പൈൽ ചെയ്ത പേജുകൾ, lint നില, ചോദിക്കുക/ആർക്കൈവ് ചെയ്യുക',
      description:
        'കമ്പൈൽ ചെയ്ത ജ്ഞാന പേജുകൾ പരിശോധിക്കുക, പരിപാലനം നടത്തുക, അടിസ്ഥാനമുള്ള ചോദ്യങ്ങൾ ചോദിക്കുക.',
      pageListHint:
        'സ്രോതസ്സുകൾ, ബന്ധപ്പെട്ട പേജുകൾ, ആർക്കൈവ് ചെയ്ത ഉത്തരങ്ങൾ എന്നിവ പരിശോധിക്കാൻ ഒരു പേജ് തിരഞ്ഞെടുക്കുക.',
      askTitle: 'കമ്പൈൽ ചെയ്ത ജ്ഞാന സ്ഥലത്തോട് ചോദിക്കുക',
      askHint:
        'ഉത്തര ജോലികൾ ആദ്യം കമ്പൈൽ ചെയ്ത പേജുകളെ മുൻഗണന നൽകുകയും നല്ല ഉത്തരങ്ങൾ workspace-ിലേക്ക് തിരികെ ആർക്കൈവ് ചെയ്യുകയും ചെയ്യും.',
      archiveAnswer: 'ഈ ഉത്തരം ജ്ഞാന workspace-ിൽ ആർക്കൈവ് ചെയ്യുക',
    },
    evolution: {
      title: 'സ്വയം നന്നാക്കലിന്റെയും പരിണാമത്തിന്റെയും കോൺസോൾ',
      subtitle:
        'തെളിവുകളും സുരക്ഷിതമായ അംഗീകാരങ്ങളും ഉപയോഗിച്ച് സ്കില്ലുകൾ, runner മാറ്റങ്ങൾ, നിർദ്ദേശ നിർദേശങ്ങൾ എന്നിവ പരിശോധിക്കുക.',
      tabs: {
        knowledge: 'ജ്ഞാനം',
        skills: 'സ്കില്ലുകൾ',
        runner: 'Runner',
        instructions: 'പരിശോധന ക്യൂ',
      },
      lanes: {
        knowledgeDescription:
          'കമ്പൈൽ ചെയ്ത പേജുകൾ, സ്രോതസ്സുകളുടെ പിന്തുണയുള്ള ചോദ്യങ്ങൾ, സംഘർഷങ്ങളുടെയോ പോരായ്മകളുടെയോ സൂചനകൾ എന്നിവയെ ഈ workspace-ന്റെ അടിത്തറയാക്കുക.',
        skillsDescription:
          'കാനോണിക്കൽ സ്കില്ലുകൾ, അംഗീകരിച്ച റിവിഷനുകൾ, rollback തയ്യാറെടുപ്പ് എന്നിവ പരിശോധിക്കുക.',
        runnerDescription:
          'Runner സ്ഥാനാർത്ഥികൾ, പ്രവർത്തന തെളിവുകൾ, സ്വിച്ച് അതിരുകൾ എന്നിവ പരിശോധിക്കുക.',
        instructionsDescription:
          'നിർദ്ദേശ നിർദേശങ്ങൾ പരിശോധിച്ച് AGENTS.md അംഗീകാരങ്ങൾ വ്യക്തമായി നിലനിർത്തുക.',
      },
    },
  },
}

export default automationKnowledgeEvolutionBackfills
