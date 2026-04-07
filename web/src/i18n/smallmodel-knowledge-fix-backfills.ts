import type { LocaleKey } from './locale-catalog'

const smallModelKnowledgeFixBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': {
    settings: {
      smallModel: {
        knowledgeFix: 'Acceleracio de reparacio wiki',
        knowledgeFixHint:
          'Prioritza el model lleuger quan la revisio de coneixement repara pagines wiki de baixa qualitat.',
      },
    },
  },
  'cs-CZ': {
    settings: {
      smallModel: {
        knowledgeFix: 'Zrychleni oprav wiki',
        knowledgeFixHint:
          'Uprednostni lehky model, kdyz lint znalosti opravuje wiki stranky nizke kvality.',
      },
    },
  },
  'da-DK': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki-reparationsacceleration',
        knowledgeFixHint:
          'Foretrak den lette model, nar knowledge lint reparerer wiki-sider af lav kvalitet.',
      },
    },
  },
  'de-DE': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki-Reparaturbeschleunigung',
        knowledgeFixHint:
          'Bevorzugt das leichte Modell, wenn der Knowledge-Lint Wiki-Seiten niedriger Qualitat repariert.',
      },
    },
  },
  'el-GR': {
    settings: {
      smallModel: {
        knowledgeFix: 'Επιταχυνση διορθωσης wiki',
        knowledgeFixHint:
          'Προτιμα το ελαφρυ μοντελο οταν το knowledge lint διορθωνει wiki σελιδες χαμηλης ποιοτητας.',
      },
    },
  },
  'en-GB': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki Fix Acceleration',
        knowledgeFixHint:
          'Prefer the lightweight model when knowledge lint repairs low-quality wiki pages.',
      },
    },
  },
  'en-US': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki Fix Acceleration',
        knowledgeFixHint:
          'Prefer the lightweight model when knowledge lint repairs low-quality wiki pages.',
      },
    },
  },
  'es-ES': {
    settings: {
      smallModel: {
        knowledgeFix: 'Aceleracion de correccion wiki',
        knowledgeFixHint:
          'Prioriza el modelo ligero cuando el lint de conocimiento repara paginas wiki de baja calidad.',
      },
    },
  },
  'fr-FR': {
    settings: {
      smallModel: {
        knowledgeFix: 'Acceleration des corrections wiki',
        knowledgeFixHint:
          'Privilegie le modele leger lorsque le lint de connaissances repare des pages wiki de faible qualite.',
      },
    },
  },
  'ga-IE': {
    settings: {
      smallModel: {
        knowledgeFix: 'Luasghniomh ceartaithe wiki',
        knowledgeFixHint:
          'Tosaionn leis an tsamhail eadrom nuair a dheisionn knowledge lint leathanaigh wiki ar chaighdean iseal.',
      },
    },
  },
  'hr-HR': {
    settings: {
      smallModel: {
        knowledgeFix: 'Ubrzanje wiki popravka',
        knowledgeFixHint:
          'Preferira lagani model kada knowledge lint popravlja wiki stranice niske kvalitete.',
      },
    },
  },
  'hu-HU': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki-javitas gyorsitas',
        knowledgeFixHint:
          'Az konnyu modellt reszesiti elonyben, amikor a knowledge lint gyenge minosegu wiki oldalakat javit.',
      },
    },
  },
  'it-IT': {
    settings: {
      smallModel: {
        knowledgeFix: 'Accelerazione correzione wiki',
        knowledgeFixHint:
          'Preferisce il modello leggero quando il lint della conoscenza ripara pagine wiki di bassa qualita.',
      },
    },
  },
  'ja-JP': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki 修正高速化',
        knowledgeFixHint: '知識 lint が低品質な wiki ページを修復する際に軽量モデルを優先します。',
      },
    },
  },
  'ko-KR': {
    settings: {
      smallModel: {
        knowledgeFix: '위키 수정 가속',
        knowledgeFixHint:
          '지식 lint가 품질이 낮은 위키 페이지를 수정할 때 경량 모델을 우선 사용합니다.',
      },
    },
  },
  'ml-IN': {
    settings: {
      smallModel: {
        knowledgeFix: 'വിക്കി തിരുത്തല് വേഗീകരണം',
        knowledgeFixHint:
          'knowledge lint താഴ്ന്ന ഗുണമേന്മയുള്ള wiki പേജുകള് തിരുത്തുമ്പോള് ലഘു മോഡലിന് മുന്‍ഗണന നല്‍കുന്നു.',
      },
    },
  },
  'nb-NO': {
    settings: {
      smallModel: {
        knowledgeFix: 'Akselerasjon for wiki-retting',
        knowledgeFixHint:
          'Foretrekker den lette modellen nar knowledge lint reparerer wiki-sider med lav kvalitet.',
      },
    },
  },
  'nl-NL': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki-fixversnelling',
        knowledgeFixHint:
          'Geeft de voorkeur aan het lichte model wanneer knowledge lint wiki-paginas van lage kwaliteit repareert.',
      },
    },
  },
  'pl-PL': {
    settings: {
      smallModel: {
        knowledgeFix: 'Przyspieszenie napraw wiki',
        knowledgeFixHint:
          'Preferuje lekki model, gdy knowledge lint naprawia strony wiki niskiej jakosci.',
      },
    },
  },
  'pt-BR': {
    settings: {
      smallModel: {
        knowledgeFix: 'Aceleracao de correcao wiki',
        knowledgeFixHint:
          'Prefere o modelo leve quando o lint de conhecimento corrige paginas wiki de baixa qualidade.',
      },
    },
  },
  'pt-PT': {
    settings: {
      smallModel: {
        knowledgeFix: 'Aceleracao de correcao wiki',
        knowledgeFixHint:
          'Prefere o modelo leve quando o lint de conhecimento corrige paginas wiki de baixa qualidade.',
      },
    },
  },
  'ro-RO': {
    settings: {
      smallModel: {
        knowledgeFix: 'Accelerare reparare wiki',
        knowledgeFixHint:
          'Prefera modelul usor cand knowledge lint repara pagini wiki de calitate scazuta.',
      },
    },
  },
  'ru-RU': {
    settings: {
      smallModel: {
        knowledgeFix: 'Ускорение исправления wiki',
        knowledgeFixHint:
          'Предпочитает легкую модель, когда knowledge lint исправляет wiki-страницы низкого качества.',
      },
    },
  },
  'sk-SK': {
    settings: {
      smallModel: {
        knowledgeFix: 'Zrychlenie oprav wiki',
        knowledgeFixHint:
          'Uprednostni lahky model, ked knowledge lint opravuje wiki stranky nizkej kvality.',
      },
    },
  },
  'sv-SE': {
    settings: {
      smallModel: {
        knowledgeFix: 'Acceleration for wiki-fixar',
        knowledgeFixHint:
          'Foredrar den lattviktiga modellen nar knowledge lint reparerar wiki-sidor med lag kvalitet.',
      },
    },
  },
  'zh-CN': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki 修复加速',
        knowledgeFixHint: '在知识检查修复低质量 wiki 页面时优先使用轻量模型。',
      },
    },
  },
  'zh-TW': {
    settings: {
      smallModel: {
        knowledgeFix: 'Wiki 修復加速',
        knowledgeFixHint: '在知識檢查修復低品質 wiki 頁面時優先使用輕量模型。',
      },
    },
  },
}

export default smallModelKnowledgeFixBackfills
