import type { LocaleKey } from './locale-catalog'

const extensionsBrowseBackfills: Partial<Record<LocaleKey, object>> = {
  'ca-ES': {
    extensions: {
      browse: {
        skillManagementHint:
          'Les habilitats integrades es poden activar o desactivar aquí. La desinstal·lació només està disponible per a les habilitats locals.',
        toolManagementHint:
          "Les eines integrades es poden activar o desactivar aquí. La seva disponibilitat encara pot estar limitada per la política d'eines del runtime.",
        builtinSkillDetailHint:
          'Aquesta habilitat integrada es pot activar o desactivar aquí, però no es pot desinstal·lar.',
      },
    },
  },
  'cs-CZ': {
    extensions: {
      browse: {
        skillManagementHint:
          'Vestavěné dovednosti lze zde povolit nebo zakázat. Odinstalace je dostupná pouze pro místní dovednosti.',
        toolManagementHint:
          'Vestavěné nástroje lze zde povolit nebo zakázat. Jejich dostupnost může být stále omezena zásadami nástrojů běhového prostředí.',
        builtinSkillDetailHint:
          'Tuto vestavěnou dovednost lze zde povolit nebo zakázat, ale nelze ji odinstalovat.',
      },
    },
  },
  'da-DK': {
    extensions: {
      browse: {
        skillManagementHint:
          'Indbyggede færdigheder kan aktiveres eller deaktiveres her. Afinstallation er kun tilgængelig for lokale færdigheder.',
        toolManagementHint:
          'Indbyggede værktøjer kan aktiveres eller deaktiveres her. Deres tilgængelighed kan stadig være begrænset af runtime-værktøjspolitikken.',
        builtinSkillDetailHint:
          'Denne indbyggede færdighed kan aktiveres eller deaktiveres her, men den kan ikke afinstalleres.',
      },
    },
  },
  'de-DE': {
    extensions: {
      browse: {
        skillManagementHint:
          'Integrierte Skills können hier aktiviert oder deaktiviert werden. Eine Deinstallation ist nur für lokale Skills verfügbar.',
        toolManagementHint:
          'Integrierte Tools können hier aktiviert oder deaktiviert werden. Ihre Verfügbarkeit kann weiterhin durch die Laufzeit-Toolrichtlinie eingeschränkt sein.',
        builtinSkillDetailHint:
          'Dieser integrierte Skill kann hier aktiviert oder deaktiviert werden, aber er kann nicht deinstalliert werden.',
      },
    },
  },
  'el-GR': {
    extensions: {
      browse: {
        skillManagementHint:
          'Οι ενσωματωμένες δεξιότητες μπορούν να ενεργοποιηθούν ή να απενεργοποιηθούν εδώ. Η απεγκατάσταση είναι διαθέσιμη μόνο για τοπικές δεξιότητες.',
        toolManagementHint:
          'Τα ενσωματωμένα εργαλεία μπορούν να ενεργοποιηθούν ή να απενεργοποιηθούν εδώ. Η διαθεσιμότητά τους μπορεί ακόμη να περιορίζεται από την πολιτική εργαλείων του runtime.',
        builtinSkillDetailHint:
          'Αυτή η ενσωματωμένη δεξιότητα μπορεί να ενεργοποιηθεί ή να απενεργοποιηθεί εδώ, αλλά δεν μπορεί να απεγκατασταθεί.',
      },
    },
  },
  'en-GB': {
    extensions: {
      browse: {
        skillManagementHint:
          'Built-in skills can be enabled or disabled here. Uninstall is available only for local skills.',
        toolManagementHint:
          'Built-in tools can be enabled or disabled here. Their availability can still be limited by runtime tool policy.',
        builtinSkillDetailHint:
          'This built-in skill can be enabled or disabled here, but it cannot be uninstalled.',
      },
    },
  },
  'es-ES': {
    extensions: {
      browse: {
        skillManagementHint:
          'Las habilidades integradas se pueden activar o desactivar aquí. La desinstalación solo está disponible para las habilidades locales.',
        toolManagementHint:
          'Las herramientas integradas se pueden activar o desactivar aquí. Su disponibilidad aún puede estar limitada por la política de herramientas del runtime.',
        builtinSkillDetailHint:
          'Esta habilidad integrada se puede activar o desactivar aquí, pero no se puede desinstalar.',
      },
    },
  },
  'fr-FR': {
    extensions: {
      browse: {
        skillManagementHint:
          "Les compétences intégrées peuvent être activées ou désactivées ici. La désinstallation n'est disponible que pour les compétences locales.",
        toolManagementHint:
          "Les outils intégrés peuvent être activés ou désactivés ici. Leur disponibilité peut encore être limitée par la politique d'outils du runtime.",
        builtinSkillDetailHint:
          'Cette compétence intégrée peut être activée ou désactivée ici, mais elle ne peut pas être désinstallée.',
      },
    },
  },
  'ga-IE': {
    extensions: {
      browse: {
        skillManagementHint:
          'Is féidir scileanna ionsuite a chumasú nó a dhíchumasú anseo. Níl díshuiteáil ar fáil ach do scileanna áitiúla.',
        toolManagementHint:
          "Is féidir uirlisí ionsuite a chumasú nó a dhíchumasú anseo. D'fhéadfadh polasaí uirlisí an runtime a bheith fós ag cur teorainn lena n-infhaighteacht.",
        builtinSkillDetailHint:
          'Is féidir an scil ionsuite seo a chumasú nó a dhíchumasú anseo, ach ní féidir í a dhíshuiteáil.',
      },
    },
  },
  'hr-HR': {
    extensions: {
      browse: {
        skillManagementHint:
          'Ugrađene vještine ovdje se mogu omogućiti ili onemogućiti. Deinstalacija je dostupna samo za lokalne vještine.',
        toolManagementHint:
          'Ugrađeni alati ovdje se mogu omogućiti ili onemogućiti. Njihova dostupnost i dalje može biti ograničena pravilima alata tijekom izvođenja.',
        builtinSkillDetailHint:
          'Ova ugrađena vještina ovdje se može omogućiti ili onemogućiti, ali se ne može deinstalirati.',
      },
    },
  },
  'hu-HU': {
    extensions: {
      browse: {
        skillManagementHint:
          'A beépített készségek itt engedélyezhetők vagy letilthatók. Eltávolítás csak a helyi készségekhez érhető el.',
        toolManagementHint:
          'A beépített eszközök itt engedélyezhetők vagy letilthatók. Elérhetőségüket továbbra is korlátozhatja a futásidejű eszközszabályzat.',
        builtinSkillDetailHint:
          'Ez a beépített készség itt engedélyezhető vagy letiltható, de nem távolítható el.',
      },
    },
  },
  'it-IT': {
    extensions: {
      browse: {
        skillManagementHint:
          'Le skill integrate possono essere abilitate o disabilitate qui. La disinstallazione è disponibile solo per le skill locali.',
        toolManagementHint:
          'Gli strumenti integrati possono essere abilitati o disabilitati qui. La loro disponibilità può comunque essere limitata dalla policy degli strumenti di runtime.',
        builtinSkillDetailHint:
          'Questa skill integrata può essere abilitata o disabilitata qui, ma non può essere disinstallata.',
      },
    },
  },
  'ja-JP': {
    extensions: {
      browse: {
        skillManagementHint:
          '組み込みスキルはここで有効または無効にできます。アンインストールできるのはローカルスキルのみです。',
        toolManagementHint:
          '組み込みツールはここで有効または無効にできます。利用可否は引き続きランタイムのツールポリシーによって制限される場合があります。',
        builtinSkillDetailHint:
          'この組み込みスキルはここで有効または無効にできますが、アンインストールはできません。',
      },
    },
  },
  'ko-KR': {
    extensions: {
      browse: {
        skillManagementHint:
          '내장 스킬은 여기에서 활성화하거나 비활성화할 수 있습니다. 제거는 로컬 스킬에만 사용할 수 있습니다.',
        toolManagementHint:
          '내장 도구는 여기에서 활성화하거나 비활성화할 수 있습니다. 실제 사용 가능 여부는 여전히 런타임 도구 정책의 제한을 받을 수 있습니다.',
        builtinSkillDetailHint:
          '이 내장 스킬은 여기에서 활성화하거나 비활성화할 수 있지만 제거할 수는 없습니다.',
      },
    },
  },
  'ml-IN': {
    extensions: {
      browse: {
        skillManagementHint:
          'ഉൾനിർമ്മിത സ്കില്ലുകൾ ഇവിടെ പ്രവർത്തനക്ഷമമാക്കാനോ പ്രവർത്തനരഹിതമാക്കാനോ കഴിയും. അൺഇൻസ്റ്റാൾ ചെയ്യുക പ്രാദേശിക സ്കില്ലുകൾക്കു മാത്രമേ ലഭ്യമാകൂ.',
        toolManagementHint:
          'ഉൾനിർമ്മിത ടൂളുകൾ ഇവിടെ പ്രവർത്തനക്ഷമമാക്കാനോ പ്രവർത്തനരഹിതമാക്കാനോ കഴിയും. അവയുടെ ലഭ്യത റൺടൈം ടൂൾ നയത്തിന്റെ അടിസ്ഥാനത്തിൽ ഇപ്പോഴും നിയന്ത്രിക്കപ്പെടാം.',
        builtinSkillDetailHint:
          'ഈ ഉൾനിർമ്മിത സ്കിൽ ഇവിടെ പ്രവർത്തനക്ഷമമാക്കാനോ പ്രവർത്തനരഹിതമാക്കാനോ കഴിയും, പക്ഷേ അൺഇൻസ്റ്റാൾ ചെയ്യാൻ കഴിയില്ല.',
      },
    },
  },
  'nb-NO': {
    extensions: {
      browse: {
        skillManagementHint:
          'Innebygde ferdigheter kan aktiveres eller deaktiveres her. Avinstallering er bare tilgjengelig for lokale ferdigheter.',
        toolManagementHint:
          'Innebygde verktøy kan aktiveres eller deaktiveres her. Tilgjengeligheten deres kan fortsatt være begrenset av runtime-verktøypolicyen.',
        builtinSkillDetailHint:
          'Denne innebygde ferdigheten kan aktiveres eller deaktiveres her, men den kan ikke avinstalleres.',
      },
    },
  },
  'nl-NL': {
    extensions: {
      browse: {
        skillManagementHint:
          'Ingebouwde vaardigheden kunnen hier worden ingeschakeld of uitgeschakeld. Verwijderen is alleen beschikbaar voor lokale vaardigheden.',
        toolManagementHint:
          'Ingebouwde tools kunnen hier worden in- of uitgeschakeld. Hun beschikbaarheid kan nog steeds worden beperkt door het runtime-toolbeleid.',
        builtinSkillDetailHint:
          'Deze ingebouwde vaardigheid kan hier worden in- of uitgeschakeld, maar kan niet worden verwijderd.',
      },
    },
  },
  'pl-PL': {
    extensions: {
      browse: {
        skillManagementHint:
          'Wbudowane umiejętności można tutaj włączać lub wyłączać. Odinstalowanie jest dostępne tylko dla lokalnych umiejętności.',
        toolManagementHint:
          'Wbudowane narzędzia można tutaj włączać lub wyłączać. Ich dostępność może nadal być ograniczana przez zasady narzędzi środowiska uruchomieniowego.',
        builtinSkillDetailHint:
          'Tę wbudowaną umiejętność można tutaj włączać lub wyłączać, ale nie można jej odinstalować.',
      },
    },
  },
  'pt-BR': {
    extensions: {
      browse: {
        skillManagementHint:
          'As habilidades integradas podem ser ativadas ou desativadas aqui. A desinstalação está disponível apenas para habilidades locais.',
        toolManagementHint:
          'As ferramentas integradas podem ser ativadas ou desativadas aqui. A disponibilidade delas ainda pode ser limitada pela política de ferramentas do runtime.',
        builtinSkillDetailHint:
          'Esta habilidade integrada pode ser ativada ou desativada aqui, mas não pode ser desinstalada.',
      },
    },
  },
  'pt-PT': {
    extensions: {
      browse: {
        skillManagementHint:
          'As habilidades integradas podem ser ativadas ou desativadas aqui. A desinstalação só está disponível para habilidades locais.',
        toolManagementHint:
          'As ferramentas integradas podem ser ativadas ou desativadas aqui. A sua disponibilidade pode continuar limitada pela política de ferramentas do runtime.',
        builtinSkillDetailHint:
          'Esta habilidade integrada pode ser ativada ou desativada aqui, mas não pode ser desinstalada.',
      },
    },
  },
  'ro-RO': {
    extensions: {
      browse: {
        skillManagementHint:
          'Abilitățile integrate pot fi activate sau dezactivate aici. Dezinstalarea este disponibilă doar pentru abilitățile locale.',
        toolManagementHint:
          'Instrumentele integrate pot fi activate sau dezactivate aici. Disponibilitatea lor poate fi în continuare limitată de politica de instrumente a runtime-ului.',
        builtinSkillDetailHint:
          'Această abilitate integrată poate fi activată sau dezactivată aici, dar nu poate fi dezinstalată.',
      },
    },
  },
  'ru-RU': {
    extensions: {
      browse: {
        skillManagementHint:
          'Встроенные навыки можно включать и отключать здесь. Удаление доступно только для локальных навыков.',
        toolManagementHint:
          'Встроенные инструменты можно включать и отключать здесь. Их доступность по-прежнему может ограничиваться политикой инструментов во время выполнения.',
        builtinSkillDetailHint:
          'Этот встроенный навык можно включать и отключать здесь, но его нельзя удалить.',
      },
    },
  },
  'sk-SK': {
    extensions: {
      browse: {
        skillManagementHint:
          'Vstavané zručnosti tu možno povoliť alebo zakázať. Odinštalovanie je dostupné iba pre lokálne zručnosti.',
        toolManagementHint:
          'Vstavané nástroje tu možno povoliť alebo zakázať. Ich dostupnosť môže byť stále obmedzená politikou nástrojov za behu.',
        builtinSkillDetailHint:
          'Túto vstavanú zručnosť tu možno povoliť alebo zakázať, ale nemožno ju odinštalovať.',
      },
    },
  },
  'sv-SE': {
    extensions: {
      browse: {
        skillManagementHint:
          'Inbyggda färdigheter kan aktiveras eller inaktiveras här. Avinstallation är bara tillgänglig för lokala färdigheter.',
        toolManagementHint:
          'Inbyggda verktyg kan aktiveras eller inaktiveras här. Deras tillgänglighet kan fortfarande begränsas av runtime-verktygspolicyn.',
        builtinSkillDetailHint:
          'Den här inbyggda färdigheten kan aktiveras eller inaktiveras här, men den kan inte avinstalleras.',
      },
    },
  },
}

export default extensionsBrowseBackfills
