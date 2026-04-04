import type { LocaleKey } from './locale-catalog'

const githubAwesomeSkillsLabel = 'GitHub Awesome Skills'

function buildGitHubAwesomeSkillsBackfill(description: string, upstream: string) {
  return {
    marketplace: {
      sources: {
        githubAwesomeSkills: {
          label: githubAwesomeSkillsLabel,
          description,
        },
      },
      detail: {
        meta: {
          upstream,
        },
      },
    },
  }
}

const localeFollowupBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': {
    common: { backToTop: 'Torna a dalt' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          "Les habilitats integrades es poden desactivar, però no es poden desinstal·lar.",
        builtinToolUninstallBlocked:
          "Les eines integrades es poden desactivar, però no es poden desinstal·lar.",
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agrega fonts seleccionades de GitHub awesome skills.',
      'Origen original'
    ),
  },
  'cs-CZ': {
    common: { backToTop: 'Zpět nahoru' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Vestavěné dovednosti lze zakázat, ale nelze je odinstalovat.',
        builtinToolUninstallBlocked:
          'Vestavěné nástroje lze zakázat, ale nelze je odinstalovat.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregovaný výběr kurátorovaných zdrojů GitHub awesome skills.',
      'Původní zdroj'
    ),
  },
  'da-DK': {
    common: { backToTop: 'Tilbage til toppen' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Indbyggede færdigheder kan deaktiveres, men de kan ikke afinstalleres.',
        builtinToolUninstallBlocked:
          'Indbyggede værktøjer kan deaktiveres, men de kan ikke afinstalleres.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Samlet udvalg af kuraterede GitHub awesome skills-kilder.',
      'Oprindelig kilde'
    ),
  },
  'de-DE': {
    common: { backToTop: 'Zurück nach oben' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Integrierte Skills können deaktiviert, aber nicht deinstalliert werden.',
        builtinToolUninstallBlocked:
          'Integrierte Tools können deaktiviert, aber nicht deinstalliert werden.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Zusammengefasste kuratierte GitHub-Awesome-Skills-Quellen.',
      'Ursprungsquelle'
    ),
  },
  'el-GR': {
    common: { backToTop: 'Επιστροφή στην κορυφή' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Οι ενσωματωμένες δεξιότητες μπορούν να απενεργοποιηθούν, αλλά δεν μπορούν να απεγκατασταθούν.',
        builtinToolUninstallBlocked:
          'Τα ενσωματωμένα εργαλεία μπορούν να απενεργοποιηθούν, αλλά δεν μπορούν να απεγκατασταθούν.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Συγκεντρωμένες επιλεγμένες πηγές GitHub awesome skills.',
      'Αρχική πηγή'
    ),
  },
  'en-GB': {
    common: { backToTop: 'Back to top' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Built-in skills can be disabled, but they cannot be uninstalled.',
        builtinToolUninstallBlocked:
          'Built-in tools can be disabled, but they cannot be uninstalled.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Aggregated curated GitHub awesome skills upstreams.',
      'Upstream'
    ),
  },
  'es-ES': {
    common: { backToTop: 'Volver arriba' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Las habilidades integradas se pueden desactivar, pero no se pueden desinstalar.',
        builtinToolUninstallBlocked:
          'Las herramientas integradas se pueden desactivar, pero no se pueden desinstalar.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregado de fuentes seleccionadas de GitHub awesome skills.',
      'Origen'
    ),
  },
  'fr-FR': {
    common: { backToTop: 'Retour en haut' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Les compétences intégrées peuvent être désactivées, mais elles ne peuvent pas être désinstallées.',
        builtinToolUninstallBlocked:
          'Les outils intégrés peuvent être désactivés, mais ils ne peuvent pas être désinstallés.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agrégat de sources GitHub awesome skills sélectionnées.',
      "Source d'origine"
    ),
  },
  'ga-IE': {
    common: { backToTop: 'Ar ais go barr' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Is féidir scileanna ionsuite a dhíchumasú, ach ní féidir iad a dhíshuiteáil.',
        builtinToolUninstallBlocked:
          'Is féidir uirlisí ionsuite a dhíchumasú, ach ní féidir iad a dhíshuiteáil.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Comhiomlán de fhoinsí roghnaithe GitHub awesome skills.',
      'Foinse bhunaidh'
    ),
  },
  'hr-HR': {
    common: { backToTop: 'Natrag na vrh' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Ugrađene vještine mogu se onemogućiti, ali se ne mogu deinstalirati.',
        builtinToolUninstallBlocked:
          'Ugrađeni alati mogu se onemogućiti, ali se ne mogu deinstalirati.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Objedinjeni odabrani izvori GitHub awesome skills.',
      'Izvorni izvor'
    ),
  },
  'hu-HU': {
    common: { backToTop: 'Vissza a tetejére' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'A beépített készségek letilthatók, de nem távolíthatók el.',
        builtinToolUninstallBlocked:
          'A beépített eszközök letilthatók, de nem távolíthatók el.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Összesített, válogatott GitHub awesome skills források.',
      'Eredeti forrás'
    ),
  },
  'it-IT': {
    common: { backToTop: 'Torna in alto' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Le skill integrate possono essere disabilitate, ma non possono essere disinstallate.',
        builtinToolUninstallBlocked:
          'Gli strumenti integrati possono essere disabilitati, ma non possono essere disinstallati.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Raccolta di fonti GitHub awesome skills selezionate.',
      'Fonte originale'
    ),
  },
  'ja-JP': {
    common: { backToTop: 'トップに戻る' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          '組み込みスキルは無効にできますが、アンインストールはできません。',
        builtinToolUninstallBlocked:
          '組み込みツールは無効にできますが、アンインストールはできません。',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      '厳選された GitHub awesome skills ソースを集約した一覧です。',
      '元のソース'
    ),
  },
  'ko-KR': {
    common: { backToTop: '맨 위로' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          '내장 스킬은 비활성화할 수 있지만 제거할 수는 없습니다.',
        builtinToolUninstallBlocked:
          '내장 도구는 비활성화할 수 있지만 제거할 수는 없습니다.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      '엄선된 GitHub awesome skills 소스를 모아 둔 집계입니다.',
      '원본 출처'
    ),
  },
  'ml-IN': {
    common: { backToTop: 'മുകളിൽേക്ക് മടങ്ങുക' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'ഉൾനിർമ്മിത സ്കില്ലുകൾ പ്രവർത്തനരഹിതമാക്കാം, എന്നാൽ അൺഇൻസ്റ്റാൾ ചെയ്യാനാവില്ല.',
        builtinToolUninstallBlocked:
          'ഉൾനിർമ്മിത ടൂളുകൾ പ്രവർത്തനരഹിതമാക്കാം, എന്നാൽ അൺഇൻസ്റ്റാൾ ചെയ്യാനാവില്ല.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'തിരഞ്ഞെടുത്ത GitHub awesome skills ഉറവിടങ്ങളെ ഏകോപിപ്പിച്ച സമാഹാരം.',
      'മൂല ഉറവിടം'
    ),
  },
  'nb-NO': {
    common: { backToTop: 'Tilbake til toppen' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Innebygde ferdigheter kan deaktiveres, men de kan ikke avinstalleres.',
        builtinToolUninstallBlocked:
          'Innebygde verktøy kan deaktiveres, men de kan ikke avinstalleres.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Samlet oversikt over kuraterte GitHub awesome skills-kilder.',
      'Opprinnelig kilde'
    ),
  },
  'nl-NL': {
    common: { backToTop: 'Terug naar boven' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Ingebouwde vaardigheden kunnen worden uitgeschakeld, maar niet worden verwijderd.',
        builtinToolUninstallBlocked:
          'Ingebouwde tools kunnen worden uitgeschakeld, maar niet worden verwijderd.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Verzamelde selectie van GitHub awesome skills-bronnen.',
      'Oorspronkelijke bron'
    ),
  },
  'pl-PL': {
    common: { backToTop: 'Wróć na górę' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Wbudowane umiejętności można wyłączyć, ale nie można ich odinstalować.',
        builtinToolUninstallBlocked:
          'Wbudowane narzędzia można wyłączyć, ale nie można ich odinstalować.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Zbiorczy zestaw wyselekcjonowanych źródeł GitHub awesome skills.',
      'Źródło pierwotne'
    ),
  },
  'pt-BR': {
    common: { backToTop: 'Voltar ao topo' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'As habilidades integradas podem ser desativadas, mas não podem ser desinstaladas.',
        builtinToolUninstallBlocked:
          'As ferramentas integradas podem ser desativadas, mas não podem ser desinstaladas.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregado de fontes selecionadas de GitHub awesome skills.',
      'Fonte de origem'
    ),
  },
  'pt-PT': {
    common: { backToTop: 'Voltar ao topo' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'As habilidades integradas podem ser desativadas, mas não podem ser desinstaladas.',
        builtinToolUninstallBlocked:
          'As ferramentas integradas podem ser desativadas, mas não podem ser desinstaladas.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregado de fontes selecionadas de GitHub awesome skills.',
      'Fonte de origem'
    ),
  },
  'ro-RO': {
    common: { backToTop: 'Înapoi sus' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Abilitățile integrate pot fi dezactivate, dar nu pot fi dezinstalate.',
        builtinToolUninstallBlocked:
          'Instrumentele integrate pot fi dezactivate, dar nu pot fi dezinstalate.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregare de surse GitHub awesome skills selectate.',
      'Sursa de origine'
    ),
  },
  'ru-RU': {
    common: { backToTop: 'Наверх' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Встроенные навыки можно отключить, но нельзя удалить.',
        builtinToolUninstallBlocked:
          'Встроенные инструменты можно отключить, но нельзя удалить.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Собранные отобранные источники GitHub awesome skills.',
      'Исходный источник'
    ),
  },
  'sk-SK': {
    common: { backToTop: 'Späť hore' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Vstavané zručnosti možno zakázať, ale nemožno ich odinštalovať.',
        builtinToolUninstallBlocked:
          'Vstavané nástroje možno zakázať, ale nemožno ich odinštalovať.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Agregovaný výber kurátorovaných zdrojov GitHub awesome skills.',
      'Pôvodný zdroj'
    ),
  },
  'sv-SE': {
    common: { backToTop: 'Till toppen' },
    extensions: {
      browse: {
        builtinSkillUninstallBlocked:
          'Inbyggda färdigheter kan inaktiveras, men de kan inte avinstalleras.',
        builtinToolUninstallBlocked:
          'Inbyggda verktyg kan inaktiveras, men de kan inte avinstalleras.',
      },
    },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      'Samlad översikt över utvalda GitHub awesome skills-källor.',
      'Ursprungskälla'
    ),
  },
  'zh-TW': {
    common: { backToTop: '回到頂部' },
    skillStore: buildGitHubAwesomeSkillsBackfill(
      '聚合多個人工篩選的 GitHub awesome skills 上游來源。',
      '上游來源'
    ),
  },
}

export default localeFollowupBackfills
