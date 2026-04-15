import type { LocaleKey } from './locale-catalog'

type BrowserLegacyResultCardBackfill = {
  resultCard: {
    messages: {
      browser_interactive_elements_section: string
      browser_page_structure_section: string
      browser_scroll_direction_down: string
      browser_scroll_direction_left: string
      browser_scroll_direction_right: string
      browser_scroll_direction_up: string
    }
    messageTemplates: {
      browser_action_performed_on_ref: string
      browser_large_dom_note: string
      browser_open_tabs: string
      browser_page: string
      browser_page_main_content: string
      browser_page_main_content_with_section: string
      browser_page_with_interactive_count: string
      browser_page_with_screenshot_interactive_count: string
      browser_recipes_available: string
      browser_scrolled_page: string
      browser_tab_closed: string
    }
  }
}

export const browserLegacyResultCardBackfills: Partial<
  Record<LocaleKey, BrowserLegacyResultCardBackfill>
> = {
  'ca-ES': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Elements interactius',
        browser_page_structure_section: 'Estructura de la pàgina',
        browser_scroll_direction_down: 'avall',
        browser_scroll_direction_left: "l'esquerra",
        browser_scroll_direction_right: 'la dreta',
        browser_scroll_direction_up: 'amunt',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "S'ha executat l'acció {action} a {'@'}{ref}",
        browser_large_dom_note:
          'La pàgina té {count} elements interactius i un DOM gran. S\'utilitza la llista d\'elements interactius. Fes servir "screenshot" per al disseny visual.',
        browser_open_tabs: '{count} pestanyes obertes',
        browser_page: 'Pàgina: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Pàgina: {title} ({url})\n\nContingut principal:\n{content}',
        browser_page_main_content_with_section:
          'Pàgina: {title} ({url})\n\nContingut principal:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Pàgina: {title} ({url}) — {count} elements interactius\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Pàgina: {title} ({url}) — captura de pantalla + {count} elements interactius\n\n{body}',
        browser_recipes_available: '{count} receptes disponibles',
        browser_scrolled_page: "S'ha desplaçat la pàgina cap a {direction}",
        browser_tab_closed: 'Pestanya {target} tancada',
      },
    },
  },
  'cs-CZ': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interaktivní prvky',
        browser_page_structure_section: 'Struktura stránky',
        browser_scroll_direction_down: 'dolů',
        browser_scroll_direction_left: 'doleva',
        browser_scroll_direction_right: 'doprava',
        browser_scroll_direction_up: 'nahoru',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Akce {action} byla provedena na {'@'}{ref}",
        browser_large_dom_note:
          'Stránka má {count} interaktivních prvků a velký DOM. Používá se seznam interaktivních prvků. Pro vizuální rozvržení použijte „screenshot“.',
        browser_open_tabs: '{count} otevřených karet',
        browser_page: 'Stránka: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Stránka: {title} ({url})\n\nHlavní obsah:\n{content}',
        browser_page_main_content_with_section:
          'Stránka: {title} ({url})\n\nHlavní obsah:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Stránka: {title} ({url}) — {count} interaktivních prvků\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Stránka: {title} ({url}) — snímek obrazovky + {count} interaktivních prvků\n\n{body}',
        browser_recipes_available: '{count} receptů k dispozici',
        browser_scrolled_page: 'Stránka byla posunuta {direction}',
        browser_tab_closed: 'Karta {target} byla zavřena',
      },
    },
  },
  'da-DK': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interaktive elementer',
        browser_page_structure_section: 'Sidestruktur',
        browser_scroll_direction_down: 'ned',
        browser_scroll_direction_left: 'til venstre',
        browser_scroll_direction_right: 'til højre',
        browser_scroll_direction_up: 'op',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Handlingen {action} blev udført på {'@'}{ref}",
        browser_large_dom_note:
          "Siden har {count} interaktive elementer og en stor DOM. Listen over interaktive elementer bruges. Brug 'screenshot' til det visuelle layout.",
        browser_open_tabs: '{count} åbne faner',
        browser_page: 'Side: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Side: {title} ({url})\n\nHovedindhold:\n{content}',
        browser_page_main_content_with_section:
          'Side: {title} ({url})\n\nHovedindhold:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Side: {title} ({url}) — {count} interaktive elementer\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Side: {title} ({url}) — skærmbillede + {count} interaktive elementer\n\n{body}',
        browser_recipes_available: '{count} opskrifter tilgængelige',
        browser_scrolled_page: 'Siden blev rullet {direction}',
        browser_tab_closed: 'Fanen {target} blev lukket',
      },
    },
  },
  'de-DE': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interaktive Elemente',
        browser_page_structure_section: 'Seitenstruktur',
        browser_scroll_direction_down: 'unten',
        browser_scroll_direction_left: 'links',
        browser_scroll_direction_right: 'rechts',
        browser_scroll_direction_up: 'oben',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Aktion {action} für {'@'}{ref} ausgeführt",
        browser_large_dom_note:
          'Die Seite hat {count} interaktive Elemente und ein großes DOM. Es wird die Liste der interaktiven Elemente verwendet. Nutzen Sie „screenshot“ für das visuelle Layout.',
        browser_open_tabs: '{count} offene Tabs',
        browser_page: 'Seite: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Seite: {title} ({url})\n\nHauptinhalt:\n{content}',
        browser_page_main_content_with_section:
          'Seite: {title} ({url})\n\nHauptinhalt:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Seite: {title} ({url}) — {count} interaktive Elemente\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Seite: {title} ({url}) — Screenshot + {count} interaktive Elemente\n\n{body}',
        browser_recipes_available: '{count} Rezepte verfügbar',
        browser_scrolled_page: 'Seite nach {direction} gescrollt',
        browser_tab_closed: 'Tab {target} geschlossen',
      },
    },
  },
  'el-GR': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Διαδραστικά στοιχεία',
        browser_page_structure_section: 'Δομή σελίδας',
        browser_scroll_direction_down: 'κάτω',
        browser_scroll_direction_left: 'αριστερά',
        browser_scroll_direction_right: 'δεξιά',
        browser_scroll_direction_up: 'πάνω',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Η ενέργεια {action} εκτελέστηκε στο {'@'}{ref}",
        browser_large_dom_note:
          "Η σελίδα έχει {count} διαδραστικά στοιχεία και μεγάλο DOM. Χρησιμοποιείται η λίστα διαδραστικών στοιχείων. Χρησιμοποίησε το 'screenshot' για την οπτική διάταξη.",
        browser_open_tabs: '{count} ανοιχτές καρτέλες',
        browser_page: 'Σελίδα: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Σελίδα: {title} ({url})\n\nΚύριο περιεχόμενο:\n{content}',
        browser_page_main_content_with_section:
          'Σελίδα: {title} ({url})\n\nΚύριο περιεχόμενο:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Σελίδα: {title} ({url}) — {count} διαδραστικά στοιχεία\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Σελίδα: {title} ({url}) — στιγμιότυπο οθόνης + {count} διαδραστικά στοιχεία\n\n{body}',
        browser_recipes_available: '{count} διαθέσιμες συνταγές',
        browser_scrolled_page: 'Η σελίδα μετακινήθηκε προς τα {direction}',
        browser_tab_closed: 'Η καρτέλα {target} έκλεισε',
      },
    },
  },
  'en-GB': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interactive elements',
        browser_page_structure_section: 'Page structure',
        browser_scroll_direction_down: 'down',
        browser_scroll_direction_left: 'left',
        browser_scroll_direction_right: 'right',
        browser_scroll_direction_up: 'up',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Performed {action} on {'@'}{ref}",
        browser_large_dom_note:
          "Page has {count} interactive elements and a large DOM. Using interactive elements list. Use 'screenshot' for visual layout.",
        browser_open_tabs: '{count} open tabs',
        browser_page: 'Page: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Page: {title} ({url})\n\nMain content:\n{content}',
        browser_page_main_content_with_section:
          'Page: {title} ({url})\n\nMain content:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Page: {title} ({url}) — {count} interactive elements\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Page: {title} ({url}) — screenshot + {count} interactive elements\n\n{body}',
        browser_recipes_available: '{count} recipes available',
        browser_scrolled_page: 'Scrolled page {direction}',
        browser_tab_closed: 'Tab {target} closed',
      },
    },
  },
  'en-US': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interactive elements',
        browser_page_structure_section: 'Page structure',
        browser_scroll_direction_down: 'down',
        browser_scroll_direction_left: 'left',
        browser_scroll_direction_right: 'right',
        browser_scroll_direction_up: 'up',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Performed {action} on {'@'}{ref}",
        browser_large_dom_note:
          "Page has {count} interactive elements and a large DOM. Using interactive elements list. Use 'screenshot' for visual layout.",
        browser_open_tabs: '{count} open tabs',
        browser_page: 'Page: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Page: {title} ({url})\n\nMain content:\n{content}',
        browser_page_main_content_with_section:
          'Page: {title} ({url})\n\nMain content:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Page: {title} ({url}) — {count} interactive elements\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Page: {title} ({url}) — screenshot + {count} interactive elements\n\n{body}',
        browser_recipes_available: '{count} recipes available',
        browser_scrolled_page: 'Scrolled page {direction}',
        browser_tab_closed: 'Tab {target} closed',
      },
    },
  },
  'es-ES': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Elementos interactivos',
        browser_page_structure_section: 'Estructura de la página',
        browser_scroll_direction_down: 'abajo',
        browser_scroll_direction_left: 'la izquierda',
        browser_scroll_direction_right: 'la derecha',
        browser_scroll_direction_up: 'arriba',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Se realizó la acción {action} en {'@'}{ref}",
        browser_large_dom_note:
          'La página tiene {count} elementos interactivos y un DOM grande. Se usa la lista de elementos interactivos. Usa "screenshot" para el diseño visual.',
        browser_open_tabs: '{count} pestañas abiertas',
        browser_page: 'Página: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Página: {title} ({url})\n\nContenido principal:\n{content}',
        browser_page_main_content_with_section:
          'Página: {title} ({url})\n\nContenido principal:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Página: {title} ({url}) — {count} elementos interactivos\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Página: {title} ({url}) — captura de pantalla + {count} elementos interactivos\n\n{body}',
        browser_recipes_available: '{count} recetas disponibles',
        browser_scrolled_page: 'Se desplazó la página hacia {direction}',
        browser_tab_closed: 'Pestaña {target} cerrada',
      },
    },
  },
  'fr-FR': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Éléments interactifs',
        browser_page_structure_section: 'Structure de la page',
        browser_scroll_direction_down: 'bas',
        browser_scroll_direction_left: 'gauche',
        browser_scroll_direction_right: 'droite',
        browser_scroll_direction_up: 'haut',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Action {action} effectuée sur {'@'}{ref}",
        browser_large_dom_note:
          'La page contient {count} éléments interactifs et un DOM volumineux. Utilisation de la liste des éléments interactifs. Utilisez « screenshot » pour la mise en page visuelle.',
        browser_open_tabs: '{count} onglets ouverts',
        browser_page: 'Page : {title} ({url})\n\n{body}',
        browser_page_main_content: 'Page : {title} ({url})\n\nContenu principal :\n{content}',
        browser_page_main_content_with_section:
          'Page : {title} ({url})\n\nContenu principal :\n{content}\n\n{section} :\n{body}',
        browser_page_with_interactive_count:
          'Page : {title} ({url}) — {count} éléments interactifs\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Page : {title} ({url}) — capture d’écran + {count} éléments interactifs\n\n{body}',
        browser_recipes_available: '{count} recettes disponibles',
        browser_scrolled_page: 'Page défilée vers le {direction}',
        browser_tab_closed: 'Onglet {target} fermé',
      },
    },
  },
  'ga-IE': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Eilimintí idirghníomhacha',
        browser_page_structure_section: 'Struchtúr an leathanaigh',
        browser_scroll_direction_down: 'síos',
        browser_scroll_direction_left: 'ar chlé',
        browser_scroll_direction_right: 'ar dheis',
        browser_scroll_direction_up: 'suas',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Rinneadh an gníomh {action} ar {'@'}{ref}",
        browser_large_dom_note:
          "Tá {count} eilimint idirghníomhach ar an leathanach agus DOM mór ann. Tá liosta na n-eilimintí idirghníomhacha á úsáid. Úsáid 'screenshot' don leagan amach amhairc.",
        browser_open_tabs: '{count} cluaisín oscailte',
        browser_page: 'Leathanach: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Leathanach: {title} ({url})\n\nPríomhábhar:\n{content}',
        browser_page_main_content_with_section:
          'Leathanach: {title} ({url})\n\nPríomhábhar:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Leathanach: {title} ({url}) — {count} eilimint idirghníomhach\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Leathanach: {title} ({url}) — gabháil scáileáin + {count} eilimint idirghníomhach\n\n{body}',
        browser_recipes_available: '{count} oideas ar fáil',
        browser_scrolled_page: 'Scrolláladh an leathanach {direction}',
        browser_tab_closed: 'Dúnadh cluaisín {target}',
      },
    },
  },
  'hr-HR': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interaktivni elementi',
        browser_page_structure_section: 'Struktura stranice',
        browser_scroll_direction_down: 'dolje',
        browser_scroll_direction_left: 'lijevo',
        browser_scroll_direction_right: 'desno',
        browser_scroll_direction_up: 'gore',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Radnja {action} izvršena je na {'@'}{ref}",
        browser_large_dom_note:
          "Stranica ima {count} interaktivnih elemenata i velik DOM. Koristi se popis interaktivnih elemenata. Upotrijebite 'screenshot' za vizualni raspored.",
        browser_open_tabs: '{count} otvorenih kartica',
        browser_page: 'Stranica: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Stranica: {title} ({url})\n\nGlavni sadržaj:\n{content}',
        browser_page_main_content_with_section:
          'Stranica: {title} ({url})\n\nGlavni sadržaj:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Stranica: {title} ({url}) — {count} interaktivnih elemenata\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Stranica: {title} ({url}) — snimka zaslona + {count} interaktivnih elemenata\n\n{body}',
        browser_recipes_available: '{count} dostupnih recepata',
        browser_scrolled_page: 'Stranica je pomaknuta {direction}',
        browser_tab_closed: 'Kartica {target} je zatvorena',
      },
    },
  },
  'hu-HU': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interaktív elemek',
        browser_page_structure_section: 'Oldalstruktúra',
        browser_scroll_direction_down: 'le',
        browser_scroll_direction_left: 'balra',
        browser_scroll_direction_right: 'jobbra',
        browser_scroll_direction_up: 'fel',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "A(z) {action} művelet végrehajtva itt: {'@'}{ref}",
        browser_large_dom_note:
          'Az oldalon {count} interaktív elem van, és a DOM nagy. Az interaktív elemek listája lesz használva. A vizuális elrendezéshez használja a „screenshot” parancsot.',
        browser_open_tabs: '{count} megnyitott lap',
        browser_page: 'Oldal: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Oldal: {title} ({url})\n\nFő tartalom:\n{content}',
        browser_page_main_content_with_section:
          'Oldal: {title} ({url})\n\nFő tartalom:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Oldal: {title} ({url}) — {count} interaktív elem\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Oldal: {title} ({url}) — képernyőkép + {count} interaktív elem\n\n{body}',
        browser_recipes_available: '{count} elérhető recept',
        browser_scrolled_page: 'Az oldal {direction} lett görgetve',
        browser_tab_closed: 'A(z) {target} lap bezárva',
      },
    },
  },
  'it-IT': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Elementi interattivi',
        browser_page_structure_section: 'Struttura della pagina',
        browser_scroll_direction_down: 'il basso',
        browser_scroll_direction_left: 'sinistra',
        browser_scroll_direction_right: 'destra',
        browser_scroll_direction_up: "l'alto",
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Azione {action} eseguita su {'@'}{ref}",
        browser_large_dom_note:
          'La pagina ha {count} elementi interattivi e un DOM esteso. Verrà utilizzato l\'elenco degli elementi interattivi. Usa "screenshot" per il layout visivo.',
        browser_open_tabs: '{count} schede aperte',
        browser_page: 'Pagina: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Pagina: {title} ({url})\n\nContenuto principale:\n{content}',
        browser_page_main_content_with_section:
          'Pagina: {title} ({url})\n\nContenuto principale:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Pagina: {title} ({url}) — {count} elementi interattivi\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Pagina: {title} ({url}) — screenshot + {count} elementi interattivi\n\n{body}',
        browser_recipes_available: '{count} ricette disponibili',
        browser_scrolled_page: 'Pagina fatta scorrere verso {direction}',
        browser_tab_closed: 'Scheda {target} chiusa',
      },
    },
  },
  'ja-JP': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'インタラクティブ要素',
        browser_page_structure_section: 'ページ構造',
        browser_scroll_direction_down: '下',
        browser_scroll_direction_left: '左',
        browser_scroll_direction_right: '右',
        browser_scroll_direction_up: '上',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "{action} を {'@'}{ref} に対して実行しました",
        browser_large_dom_note:
          'このページには {count} 個のインタラクティブ要素があり、DOM も大きいため、インタラクティブ要素の一覧を使用しています。視覚的なレイアウトには「screenshot」を使ってください。',
        browser_open_tabs: '開いているタブは {count} 件です',
        browser_page: 'ページ: {title} ({url})\n\n{body}',
        browser_page_main_content: 'ページ: {title} ({url})\n\n主な内容:\n{content}',
        browser_page_main_content_with_section:
          'ページ: {title} ({url})\n\n主な内容:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'ページ: {title} ({url}) — {count} 個のインタラクティブ要素\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'ページ: {title} ({url}) — スクリーンショット + {count} 個のインタラクティブ要素\n\n{body}',
        browser_recipes_available: '利用可能なレシピは {count} 件です',
        browser_scrolled_page: 'ページを{direction}にスクロールしました',
        browser_tab_closed: 'タブ {target} を閉じました',
      },
    },
  },
  'ko-KR': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: '상호작용 요소',
        browser_page_structure_section: '페이지 구조',
        browser_scroll_direction_down: '아래',
        browser_scroll_direction_left: '왼쪽',
        browser_scroll_direction_right: '오른쪽',
        browser_scroll_direction_up: '위',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "{action} 작업을 {'@'}{ref}에서 수행했습니다",
        browser_large_dom_note:
          "이 페이지에는 상호작용 요소가 {count}개 있고 DOM이 큽니다. 상호작용 요소 목록을 사용합니다. 시각적 레이아웃은 'screenshot'을 사용하세요.",
        browser_open_tabs: '열린 탭 {count}개',
        browser_page: '페이지: {title} ({url})\n\n{body}',
        browser_page_main_content: '페이지: {title} ({url})\n\n주요 내용:\n{content}',
        browser_page_main_content_with_section:
          '페이지: {title} ({url})\n\n주요 내용:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          '페이지: {title} ({url}) — 상호작용 요소 {count}개\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          '페이지: {title} ({url}) — 스크린샷 + 상호작용 요소 {count}개\n\n{body}',
        browser_recipes_available: '사용 가능한 레시피 {count}개',
        browser_scrolled_page: '페이지를 {direction}로 스크롤했습니다',
        browser_tab_closed: '탭 {target}을(를) 닫았습니다',
      },
    },
  },
  'ml-IN': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'ഇന്ററാക്ടീവ് ഘടകങ്ങൾ',
        browser_page_structure_section: 'പേജ് ഘടന',
        browser_scroll_direction_down: 'താഴേക്ക്',
        browser_scroll_direction_left: 'ഇടത്തേക്ക്',
        browser_scroll_direction_right: 'വലത്തേക്ക്',
        browser_scroll_direction_up: 'മുകളിൽ',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "{'@'}{ref}-ൽ {action} പ്രവർത്തനം നിർവഹിച്ചു",
        browser_large_dom_note:
          "ഈ പേജിൽ {count} ഇന്ററാക്ടീവ് ഘടകങ്ങളും വലിയ DOM ഉം ഉണ്ട്. ഇന്ററാക്ടീവ് ഘടകങ്ങളുടെ പട്ടികയാണ് ഉപയോഗിക്കുന്നത്. ദൃശ്യ ലേഔട്ടിനായി 'screenshot' ഉപയോഗിക്കുക.",
        browser_open_tabs: '{count} തുറന്ന ടാബുകൾ',
        browser_page: 'പേജ്: {title} ({url})\n\n{body}',
        browser_page_main_content: 'പേജ്: {title} ({url})\n\nപ്രധാന ഉള്ളടക്കം:\n{content}',
        browser_page_main_content_with_section:
          'പേജ്: {title} ({url})\n\nപ്രധാന ഉള്ളടക്കം:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'പേജ്: {title} ({url}) — {count} ഇന്ററാക്ടീവ് ഘടകങ്ങൾ\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'പേജ്: {title} ({url}) — സ്ക്രീൻഷോട്ട് + {count} ഇന്ററാക്ടീവ് ഘടകങ്ങൾ\n\n{body}',
        browser_recipes_available: '{count} റെസിപ്പികൾ ലഭ്യമാണ്',
        browser_scrolled_page: 'പേജ് {direction} ആയി സ്ക്രോൾ ചെയ്തു',
        browser_tab_closed: 'ടാബ് {target} അടച്ചു',
      },
    },
  },
  'nb-NO': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interaktive elementer',
        browser_page_structure_section: 'Sidestruktur',
        browser_scroll_direction_down: 'ned',
        browser_scroll_direction_left: 'til venstre',
        browser_scroll_direction_right: 'til høyre',
        browser_scroll_direction_up: 'opp',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Handlingen {action} ble utført på {'@'}{ref}",
        browser_large_dom_note:
          "Siden har {count} interaktive elementer og en stor DOM. Listen over interaktive elementer brukes. Bruk 'screenshot' for visuelt oppsett.",
        browser_open_tabs: '{count} åpne faner',
        browser_page: 'Side: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Side: {title} ({url})\n\nHovedinnhold:\n{content}',
        browser_page_main_content_with_section:
          'Side: {title} ({url})\n\nHovedinnhold:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Side: {title} ({url}) — {count} interaktive elementer\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Side: {title} ({url}) — skjermbilde + {count} interaktive elementer\n\n{body}',
        browser_recipes_available: '{count} oppskrifter tilgjengelige',
        browser_scrolled_page: 'Siden ble rullet {direction}',
        browser_tab_closed: 'Fanen {target} ble lukket',
      },
    },
  },
  'nl-NL': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interactieve elementen',
        browser_page_structure_section: 'Paginastructuur',
        browser_scroll_direction_down: 'beneden',
        browser_scroll_direction_left: 'links',
        browser_scroll_direction_right: 'rechts',
        browser_scroll_direction_up: 'boven',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Actie {action} uitgevoerd op {'@'}{ref}",
        browser_large_dom_note:
          "De pagina heeft {count} interactieve elementen en een grote DOM. De lijst met interactieve elementen wordt gebruikt. Gebruik 'screenshot' voor de visuele lay-out.",
        browser_open_tabs: '{count} open tabbladen',
        browser_page: 'Pagina: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Pagina: {title} ({url})\n\nHoofdinhoud:\n{content}',
        browser_page_main_content_with_section:
          'Pagina: {title} ({url})\n\nHoofdinhoud:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Pagina: {title} ({url}) — {count} interactieve elementen\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Pagina: {title} ({url}) — screenshot + {count} interactieve elementen\n\n{body}',
        browser_recipes_available: '{count} recepten beschikbaar',
        browser_scrolled_page: 'Pagina naar {direction} gescrold',
        browser_tab_closed: 'Tabblad {target} gesloten',
      },
    },
  },
  'pl-PL': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Elementy interaktywne',
        browser_page_structure_section: 'Struktura strony',
        browser_scroll_direction_down: 'dół',
        browser_scroll_direction_left: 'lewo',
        browser_scroll_direction_right: 'prawo',
        browser_scroll_direction_up: 'górę',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Wykonano akcję {action} na {'@'}{ref}",
        browser_large_dom_note:
          'Strona ma {count} interaktywnych elementów i duży DOM. Używana jest lista interaktywnych elementów. Użyj „screenshot” do układu wizualnego.',
        browser_open_tabs: '{count} otwartych kart',
        browser_page: 'Strona: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Strona: {title} ({url})\n\nGłówna treść:\n{content}',
        browser_page_main_content_with_section:
          'Strona: {title} ({url})\n\nGłówna treść:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Strona: {title} ({url}) — {count} interaktywnych elementów\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Strona: {title} ({url}) — zrzut ekranu + {count} interaktywnych elementów\n\n{body}',
        browser_recipes_available: '{count} dostępnych przepisów',
        browser_scrolled_page: 'Przewinięto stronę w {direction}',
        browser_tab_closed: 'Karta {target} zamknięta',
      },
    },
  },
  'pt-BR': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Elementos interativos',
        browser_page_structure_section: 'Estrutura da página',
        browser_scroll_direction_down: 'baixo',
        browser_scroll_direction_left: 'esquerda',
        browser_scroll_direction_right: 'direita',
        browser_scroll_direction_up: 'cima',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Ação {action} executada em {'@'}{ref}",
        browser_large_dom_note:
          'A página tem {count} elementos interativos e um DOM grande. Usando a lista de elementos interativos. Use "screenshot" para o layout visual.',
        browser_open_tabs: '{count} abas abertas',
        browser_page: 'Página: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Página: {title} ({url})\n\nConteúdo principal:\n{content}',
        browser_page_main_content_with_section:
          'Página: {title} ({url})\n\nConteúdo principal:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Página: {title} ({url}) — {count} elementos interativos\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Página: {title} ({url}) — captura de tela + {count} elementos interativos\n\n{body}',
        browser_recipes_available: '{count} receitas disponíveis',
        browser_scrolled_page: 'Página rolada para {direction}',
        browser_tab_closed: 'Aba {target} fechada',
      },
    },
  },
  'pt-PT': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Elementos interativos',
        browser_page_structure_section: 'Estrutura da página',
        browser_scroll_direction_down: 'baixo',
        browser_scroll_direction_left: 'a esquerda',
        browser_scroll_direction_right: 'a direita',
        browser_scroll_direction_up: 'cima',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Ação {action} executada em {'@'}{ref}",
        browser_large_dom_note:
          'A página tem {count} elementos interativos e um DOM grande. Está a ser utilizada a lista de elementos interativos. Utilize "screenshot" para a disposição visual.',
        browser_open_tabs: '{count} separadores abertos',
        browser_page: 'Página: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Página: {title} ({url})\n\nConteúdo principal:\n{content}',
        browser_page_main_content_with_section:
          'Página: {title} ({url})\n\nConteúdo principal:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Página: {title} ({url}) — {count} elementos interativos\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Página: {title} ({url}) — captura de ecrã + {count} elementos interativos\n\n{body}',
        browser_recipes_available: '{count} receitas disponíveis',
        browser_scrolled_page: 'Página deslocada para {direction}',
        browser_tab_closed: 'Separador {target} fechado',
      },
    },
  },
  'ro-RO': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Elemente interactive',
        browser_page_structure_section: 'Structura paginii',
        browser_scroll_direction_down: 'în jos',
        browser_scroll_direction_left: 'la stânga',
        browser_scroll_direction_right: 'la dreapta',
        browser_scroll_direction_up: 'în sus',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Acțiunea {action} a fost executată pe {'@'}{ref}",
        browser_large_dom_note:
          'Pagina are {count} elemente interactive și un DOM mare. Se folosește lista elementelor interactive. Folosește „screenshot” pentru aspectul vizual.',
        browser_open_tabs: '{count} file deschise',
        browser_page: 'Pagină: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Pagină: {title} ({url})\n\nConținut principal:\n{content}',
        browser_page_main_content_with_section:
          'Pagină: {title} ({url})\n\nConținut principal:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Pagină: {title} ({url}) — {count} elemente interactive\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Pagină: {title} ({url}) — captură de ecran + {count} elemente interactive\n\n{body}',
        browser_recipes_available: '{count} rețete disponibile',
        browser_scrolled_page: 'Pagina a fost derulată {direction}',
        browser_tab_closed: 'Fila {target} a fost închisă',
      },
    },
  },
  'ru-RU': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Интерактивные элементы',
        browser_page_structure_section: 'Структура страницы',
        browser_scroll_direction_down: 'вниз',
        browser_scroll_direction_left: 'влево',
        browser_scroll_direction_right: 'вправо',
        browser_scroll_direction_up: 'вверх',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Действие {action} выполнено для {'@'}{ref}",
        browser_large_dom_note:
          "На странице {count} интерактивных элементов и большой DOM. Используется список интерактивных элементов. Для визуальной компоновки используйте 'screenshot'.",
        browser_open_tabs: '{count} открытых вкладок',
        browser_page: 'Страница: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Страница: {title} ({url})\n\nОсновное содержимое:\n{content}',
        browser_page_main_content_with_section:
          'Страница: {title} ({url})\n\nОсновное содержимое:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Страница: {title} ({url}) — {count} интерактивных элементов\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Страница: {title} ({url}) — скриншот + {count} интерактивных элементов\n\n{body}',
        browser_recipes_available: '{count} доступных рецептов',
        browser_scrolled_page: 'Страница прокручена {direction}',
        browser_tab_closed: 'Вкладка {target} закрыта',
      },
    },
  },
  'sk-SK': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interaktívne prvky',
        browser_page_structure_section: 'Štruktúra stránky',
        browser_scroll_direction_down: 'nadol',
        browser_scroll_direction_left: 'doľava',
        browser_scroll_direction_right: 'doprava',
        browser_scroll_direction_up: 'nahor',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Akcia {action} bola vykonaná na {'@'}{ref}",
        browser_large_dom_note:
          'Stránka má {count} interaktívnych prvkov a veľký DOM. Používa sa zoznam interaktívnych prvkov. Pre vizuálne rozloženie použite „screenshot“.',
        browser_open_tabs: '{count} otvorených kariet',
        browser_page: 'Stránka: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Stránka: {title} ({url})\n\nHlavný obsah:\n{content}',
        browser_page_main_content_with_section:
          'Stránka: {title} ({url})\n\nHlavný obsah:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Stránka: {title} ({url}) — {count} interaktívnych prvkov\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Stránka: {title} ({url}) — snímka obrazovky + {count} interaktívnych prvkov\n\n{body}',
        browser_recipes_available: '{count} receptov k dispozícii',
        browser_scrolled_page: 'Stránka bola posunutá {direction}',
        browser_tab_closed: 'Karta {target} bola zatvorená',
      },
    },
  },
  'sv-SE': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: 'Interaktiva element',
        browser_page_structure_section: 'Sidstruktur',
        browser_scroll_direction_down: 'ned',
        browser_scroll_direction_left: 'vänster',
        browser_scroll_direction_right: 'höger',
        browser_scroll_direction_up: 'upp',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "Åtgärden {action} utfördes på {'@'}{ref}",
        browser_large_dom_note:
          "Sidan har {count} interaktiva element och en stor DOM. Listan över interaktiva element används. Använd 'screenshot' för den visuella layouten.",
        browser_open_tabs: '{count} öppna flikar',
        browser_page: 'Sida: {title} ({url})\n\n{body}',
        browser_page_main_content: 'Sida: {title} ({url})\n\nHuvudinnehåll:\n{content}',
        browser_page_main_content_with_section:
          'Sida: {title} ({url})\n\nHuvudinnehåll:\n{content}\n\n{section}:\n{body}',
        browser_page_with_interactive_count:
          'Sida: {title} ({url}) — {count} interaktiva element\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          'Sida: {title} ({url}) — skärmbild + {count} interaktiva element\n\n{body}',
        browser_recipes_available: '{count} recept tillgängliga',
        browser_scrolled_page: 'Sidan rullades {direction}',
        browser_tab_closed: 'Fliken {target} stängdes',
      },
    },
  },
  'zh-CN': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: '交互元素',
        browser_page_structure_section: '页面结构',
        browser_scroll_direction_down: '下',
        browser_scroll_direction_left: '左',
        browser_scroll_direction_right: '右',
        browser_scroll_direction_up: '上',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "已在 {'@'}{ref} 上执行 {action}",
        browser_large_dom_note:
          '页面有 {count} 个交互元素且 DOM 很大。正在使用交互元素列表。若要查看视觉布局，请使用“screenshot”。',
        browser_open_tabs: '{count} 个打开的标签页',
        browser_page: '页面：{title} ({url})\n\n{body}',
        browser_page_main_content: '页面：{title} ({url})\n\n主要内容：\n{content}',
        browser_page_main_content_with_section:
          '页面：{title} ({url})\n\n主要内容：\n{content}\n\n{section}：\n{body}',
        browser_page_with_interactive_count: '页面：{title} ({url}) — {count} 个交互元素\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          '页面：{title} ({url}) — 截图 + {count} 个交互元素\n\n{body}',
        browser_recipes_available: '{count} 个可用配方',
        browser_scrolled_page: '页面已向{direction}滚动',
        browser_tab_closed: '标签页 {target} 已关闭',
      },
    },
  },
  'zh-TW': {
    resultCard: {
      messages: {
        browser_interactive_elements_section: '互動元素',
        browser_page_structure_section: '頁面結構',
        browser_scroll_direction_down: '下',
        browser_scroll_direction_left: '左',
        browser_scroll_direction_right: '右',
        browser_scroll_direction_up: '上',
      },
      messageTemplates: {
        browser_action_performed_on_ref: "操作 {action} 已在 {'@'}{ref} 上執行",
        browser_large_dom_note:
          '頁面有 {count} 個互動元素且 DOM 很大。正在使用互動元素清單。若要查看視覺版面，請使用「screenshot」。',
        browser_open_tabs: '{count} 個開啟的分頁',
        browser_page: '頁面：{title} ({url})\n\n{body}',
        browser_page_main_content: '頁面：{title} ({url})\n\n主要內容：\n{content}',
        browser_page_main_content_with_section:
          '頁面：{title} ({url})\n\n主要內容：\n{content}\n\n{section}：\n{body}',
        browser_page_with_interactive_count: '頁面：{title} ({url}) — {count} 個互動元素\n\n{body}',
        browser_page_with_screenshot_interactive_count:
          '頁面：{title} ({url}) — 螢幕截圖 + {count} 個互動元素\n\n{body}',
        browser_recipes_available: '{count} 個可用配方',
        browser_scrolled_page: '頁面已向{direction}捲動',
        browser_tab_closed: '分頁 {target} 已關閉',
      },
    },
  },
}
