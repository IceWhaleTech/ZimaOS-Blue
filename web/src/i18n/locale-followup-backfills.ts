import type { LocaleKey } from './locale-catalog'

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
  },
  'zh-TW': {
    common: { backToTop: '回到頂部' },
  },
}

export default localeFollowupBackfills
