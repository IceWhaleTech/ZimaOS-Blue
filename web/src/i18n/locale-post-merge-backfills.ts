import dashboardCardCopyOverrides from './dashboard-card-copy-overrides'
import type { LocaleKey } from './locale-catalog'

type LocaleLeaf = string | number | boolean | null
type LocaleNode = { [key: string]: LocaleLeaf | LocaleNode }

const execCardNoCommandLabels: Partial<Record<LocaleKey, string>> = {
  'ca-ES': 'Sense ordre',
  'cs-CZ': 'Bez příkazu',
  'da-DK': 'Ingen kommando',
  'el-GR': 'Χωρίς εντολή',
  'ga-IE': 'Gan ordú',
  'hr-HR': 'Bez naredbe',
  'hu-HU': 'Nincs parancs',
  'ml-IN': 'കമാൻഡ് ഇല്ല',
  'nb-NO': 'Ingen kommando',
  'pt-PT': 'Sem comando',
  'ro-RO': 'Fără comandă',
  'sk-SK': 'Bez príkazu',
}

const commonFilterLabels: Record<LocaleKey, string> = {
  'ca-ES': 'Filtre',
  'cs-CZ': 'Filtr',
  'da-DK': 'Filter',
  'de-DE': 'Filter',
  'el-GR': 'Φίλτρο',
  'en-GB': 'Filter',
  'en-US': 'Filter',
  'es-ES': 'Filtro',
  'fr-FR': 'Filtre',
  'ga-IE': 'Scagaire',
  'hr-HR': 'Filtar',
  'hu-HU': 'Szűrő',
  'it-IT': 'Filtro',
  'ja-JP': 'フィルター',
  'ko-KR': '필터',
  'ml-IN': 'ഫിൽട്ടർ',
  'nb-NO': 'Filter',
  'nl-NL': 'Filter',
  'pl-PL': 'Filtr',
  'pt-BR': 'Filtro',
  'pt-PT': 'Filtro',
  'ro-RO': 'Filtru',
  'ru-RU': 'Фильтр',
  'sk-SK': 'Filter',
  'sv-SE': 'Filter',
  'zh-CN': '筛选',
  'zh-TW': '篩選',
}

const localePostMergeOverrides: Partial<Record<LocaleKey, LocaleNode>> = {
  'de-DE': {
    skillStore: {
      modal: {
        skillNameOptional: 'Skill-Name (Optional)',
      },
      search: {
        sourceStore: 'Marktplatz',
      },
      tabs: {
        store: 'Marktplatz',
      },
    },
  },
  'el-GR': {
    skillStore: {
      marketplace: {
        security: {
          score: 'Βαθμολογία',
        },
      },
    },
  },
  'fr-FR': {
    chat: {
      taskHarnessVerificationStatus: 'Vérification',
    },
    skillStore: {
      detail: {
        sections: {
          description: 'Descriptif',
        },
      },
      marketplace: {
        detail: {
          title: 'Détails',
        },
      },
      status: {
        initializingProgress: 'Sources synchronisées {processed}/{total}',
      },
    },
  },
  'it-IT': {
    settings: {
      failover: {
        errorTypes: {
          timeout: 'Tempo scaduto',
        },
      },
    },
    skillStore: {
      search: {
        sourceStore: 'Negozio',
      },
      tabs: {
        store: 'Negozio',
      },
    },
  },
  'nb-NO': {
    skillStore: {
      marketplace: {
        embedding: {
          phaseStandby: 'I beredskap',
        },
      },
    },
  },
  'nl-NL': {
    chat: {
      taskHarnessRunStatus: 'Uitvoering',
    },
    settings: {
      failover: {
        chips: {
          open: 'Geopend',
        },
      },
    },
  },
  'pt-PT': {
    companion: {
      flow: {
        nodeTypes: {
          security: 'Seguranca',
        },
      },
    },
    skillStore: {
      marketplace: {
        actions: {
          blocked: 'Bloqueado',
        },
        artifactKinds: {
          unknown: 'Desconhecido',
        },
        detail: {
          title: 'Detalhes',
        },
        evidenceTypes: {
          permission: 'Permissoes',
        },
        filters: {
          category: 'Categoria',
          security: 'Seguranca',
          source: 'Fonte',
        },
        installTypes: {
          unknown: 'Desconhecido',
        },
        security: {
          score: 'Pontuacao',
          title: 'Seguranca',
        },
      },
    },
  },
  'zh-TW': {
    apiProxy: {
      prunerBackend: '後端',
      prunerDisabled: '上下文裁剪器已停用',
      prunerEnabled: '上下文裁剪器已啟用',
      prunerNoData: '尚未記錄任何裁剪請求。',
      prunerThreshold: '閾值',
    },
    askQuestion: {
      browserCheckpoint: {
        allowSite: '一律允許此網站',
        allowSiteDescription: '之後對 {site} 不再重複確認',
        cancelDescription: '封鎖這次瀏覽器操作',
        continueDescription: '只允許這次瀏覽器操作',
        continueOnce: '僅此一次繼續',
      },
    },
    authProviders: {
      placeholderClientId: 'OAuth 用戶端 ID',
      placeholderClientSecret: 'OAuth 用戶端密鑰',
      placeholderDomain: '例如：company.com',
      placeholderId: '例如：google、github',
      placeholderName: '例如：Google、GitHub',
      placeholderScopes: '例如：openid、profile、email',
    },
    channels: {
      feishuSessionMode: '會話模式（傳送時不回覆原訊息）',
    },
    chat: {
      activeTodo: {
        collapse: '收合待辦清單',
        completed: '已完成',
        expand: '展開待辦清單',
        inProgress: '進行中',
        jumpToMessage: '跳到清單訊息',
        progress: '已完成 {completed} / {total} 項任務',
      },
      contextWindowExceeded:
        '這次請求超出了模型的上下文視窗。請縮短對話、系統提示詞或工具內容後再試一次。',
      editAndResubmit: '編輯後重新送出',
      inputPlaceholderShort: '輸入訊息...',
      noProvider: {
        dismiss: '稍後再說',
        draftSaved: '您的訊息已儲存在本機。完成設定後即可繼續。',
        issueAuth: '驗證失敗。請重新檢查 API Key 或 OAuth 連線。',
        issueCertificate: '此 Provider 的 TLS 憑證驗證失敗。',
        issueEndpoint: '此 Provider 的 Endpoint 設定似乎不正確。',
        issueGeneric: '請開啟 Provider 設定查看最新健康狀態。',
        issueInactive: '請檢查 API Key、OAuth 連線或允許使用的模型是否已設定完成。',
        issueNetwork: '網路連線失敗。請檢查 Endpoint 與目前的網路環境。',
        issueTimeout: 'Provider 已逾時。請稍後再試，或切換到其他路由。',
        issueUnexpectedStatus: '此 Provider 回傳了非預期的狀態。',
        moreProviders: '還有 {count} 個 Provider 也需要處理。',
        providerSummary: '需要處理的已設定 Provider',
        review: '檢查 Provider',
        statusActive: '正常',
        statusError: '錯誤',
        statusInactive: '未就緒',
        unavailableDescription:
          '偵測到您已經啟用 Provider，但它們目前無法使用。請前往設定檢查連線、金鑰或模型狀態後再試。',
        unavailableEyebrow: '暫時無法使用',
        unavailableTitle: '目前沒有可用的 AI Provider',
        unconfiguredDescription:
          '您目前還沒有可用的 LLM Provider。完成設定後，就可以從剛剛中斷的地方繼續。',
        unconfiguredEyebrow: '還差一步',
        unconfiguredTitle: '先設定一個 AI Provider 再開始聊天',
      },
      requestBuildFailed:
        '上游中繼在建立這次請求時失敗，通常代表請求結構或工具參數無效。請簡化請求或檢查工具輸入後再試。',
      requestTooLarge:
        '這次請求過大，上游中繼無法建立請求。請縮短對話、附件或工具負載後再試。',
      saveAndResubmit: '儲存並重新送出',
    },
    common: {
      openLocation: '開啟所在位置',
      saveFailed: '儲存失敗',
      searchLogsPlaceholder: '搜尋日誌...',
    },
    companion: {
      platforms: {
        web: '網頁',
        'web-user': '網頁使用者',
      },
    },
    cron: {
      handlers: {
        http: 'HTTP 請求',
      },
      optionalSettings: '可選設定',
      optionalSettingsHint: '描述、逾時與處理器專屬的額外設定',
    },
    nav: {
      configuration: '設定',
      openWorkspaceIn: '在檔案管理員中開啟',
      openWorkspaceInExplorer: '在 Explorer 中開啟',
      openWorkspaceInFinder: '在 Finder 中開啟',
      workspace: '工作區',
      workspaceFiles: '檔案',
      workspaceFilesLoadFailed: '載入工作區檔案失敗',
      workspaceLoadFailed: '載入工作區資訊失敗',
      workspaceLoading: '正在載入工作區檔案...',
      workspaceNoFiles: '目前沒有可用的工作區檔案',
      workspaceOpenFailed: '無法在檔案管理員中開啟工作區',
      workspacePanelTitle: '工作區檔案',
      workspacePathUnavailable: '工作區路徑不可用',
      workspaceSelectFileHint: '選取檔案後即可預覽或下載',
    },
    onboarding: {
      skipForNow: '稍後再說',
    },
    providerPool: {
      apiFormatAutoDetected: '目前自動偵測的格式：{format}',
      apiFormatHint: '預設使用自動偵測，但您也可以固定指定 API 格式，變更會立即生效。',
      apiFormatLabel: '格式類型',
      apiFormatOptions: {
        auto: '自動偵測',
        openai: 'OpenAI 相容',
      },
    },
    search: {
      summaryTitle: '網頁搜尋',
    },
    settings: {
      antigravity: {
        tokenPlaceholder: '輸入您的 Antigravity 存取權杖',
      },
    },
    speech: {
      asrModelInfo: {
        sensevoiceSmall: {
          name: 'SenseVoice Small（多語言）',
        },
        zipformerEn: {
          name: 'Zipformer EN（串流）',
        },
      },
    },
    tools: {
      descriptions: {
        read: '讀取本機檔案並擷取支援的文件內容',
        write: '將文字內容寫入本機檔案',
      },
    },
    users: {
      confirmPasswordPlaceholder: '確認密碼',
      emailPlaceholder: '輸入電子郵件（選填）',
      newPasswordPlaceholder: '輸入新密碼',
      passwordPlaceholder: '輸入密碼',
      usernamePlaceholder: '輸入使用者名稱',
    },
    webFetchCard: {
      collapseContent: '收合網頁內容',
      collapseHint: '隱藏擷取的網頁內容',
      contentLabel: '網頁內容',
      expandContent: '展開網頁內容',
      expandHint: '查看擷取的網頁內容',
    },
    workflow: {
      descriptionPlaceholder: '這個工作流程是做什麼的？',
      namePlaceholder: '例如：每日報告',
      nodeNamePlaceholder: '例如：傳送電子郵件',
      searchPlaceholder: '搜尋工作流程...',
    },
  },
}

function isPlainObject(value: unknown): value is LocaleNode {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

function getString(root: unknown, path: string): string | null {
  const value = path.split('.').reduce<unknown>((current, segment) => {
    if (isPlainObject(current)) {
      return current[segment]
    }
    return undefined
  }, root)

  return typeof value === 'string' && value.length > 0 ? value : null
}

function hasKeys(node: LocaleNode): boolean {
  return Object.keys(node).length > 0
}

function mergeLocaleNodes(base: LocaleNode, patch: LocaleNode): LocaleNode {
  const merged: LocaleNode = { ...base }

  for (const [key, value] of Object.entries(patch)) {
    const baseValue = merged[key]
    if (isPlainObject(baseValue) && isPlainObject(value)) {
      merged[key] = mergeLocaleNodes(baseValue, value)
      continue
    }
    merged[key] = value
  }

  return merged
}

export function buildLocalePostMergeBackfill(
  localeKey: LocaleKey,
  messages: Record<string, unknown>
): LocaleNode {
  const failoverSource = dashboardCardCopyOverrides[localeKey]
  const commonPatch: LocaleNode = {}
  const execCardPatch: LocaleNode = {}
  const settingsPatch: LocaleNode = {}
  const failoverPatch: LocaleNode = {}
  const failoverChipsPatch: LocaleNode = {}
  const failoverErrorTypesPatch: LocaleNode = {}

  const mirrors: Array<[LocaleNode, string, string | null]> = [
    [
      failoverChipsPatch,
      'healthy',
      getString(messages, 'dashboard.healthy') ??
        getString(failoverSource, 'settings.failover.chips.healthy'),
    ],
    [
      failoverChipsPatch,
      'halfOpen',
      getString(failoverSource, 'settings.failover.chips.halfOpen'),
    ],
    [failoverChipsPatch, 'open', getString(failoverSource, 'settings.failover.chips.open')],
    [
      failoverErrorTypesPatch,
      'timeout',
      getString(failoverSource, 'settings.failover.errorTypes.timeout'),
    ],
    [
      failoverErrorTypesPatch,
      'unknown',
      getString(failoverSource, 'settings.failover.errorTypes.unknown'),
    ],
    [execCardPatch, 'noCommand', execCardNoCommandLabels[localeKey] ?? null],
  ]

  for (const [target, key, value] of mirrors) {
    if (value) {
      target[key] = value
    }
  }

  if (hasKeys(failoverChipsPatch)) {
    failoverPatch.chips = failoverChipsPatch
  }
  if (hasKeys(failoverErrorTypesPatch)) {
    failoverPatch.errorTypes = failoverErrorTypesPatch
  }
  if (hasKeys(failoverPatch)) {
    settingsPatch.failover = failoverPatch
  }

  const patch: LocaleNode = {}
  if (hasKeys(settingsPatch)) {
    patch.settings = settingsPatch
  }
  commonPatch.filter = commonFilterLabels[localeKey]
  if (hasKeys(commonPatch)) {
    patch.common = commonPatch
  }
  if (hasKeys(execCardPatch)) {
    patch.execCard = execCardPatch
  }

  return mergeLocaleNodes(patch, (localePostMergeOverrides[localeKey] ?? {}) as LocaleNode)
}
