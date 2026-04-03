import type { LocaleKey } from './locale-catalog'

const skillStoreHighRiskBackfills = {
  'ca-ES': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Risc alt', highRiskInstallTitle: "Confirmació d'instal·lació d'alt risc", highRiskInstallBody: "{name} està bloquejada actualment per la política de seguretat perquè està marcada com d'alt risc. Encara la pots instal·lar si entens i acceptes el risc.", confirmForceInstall: 'Instal·la igualment' } } },
  },
  'cs-CZ': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Vysoké riziko', highRiskInstallTitle: 'Potvrzení instalace s vysokým rizikem', highRiskInstallBody: '{name} je momentálně blokován bezpečnostní politikou, protože je označen jako vysoce rizikový. Stále jej můžete nainstalovat, pokud riziku rozumíte a přijímáte ho.', confirmForceInstall: 'Přesto nainstalovat' } } },
  },
  'da-DK': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Høj risiko', highRiskInstallTitle: 'Bekræft installation med høj risiko', highRiskInstallBody: '{name} er i øjeblikket blokeret af sikkerhedspolitikken, fordi den er markeret som høj risiko. Du kan stadig installere den, hvis du forstår og accepterer risikoen.', confirmForceInstall: 'Installer alligevel' } } },
  },
  'de-DE': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Hohes Risiko', highRiskInstallTitle: 'Bestätigung für Installation mit hohem Risiko', highRiskInstallBody: '{name} ist derzeit durch die Sicherheitsrichtlinie blockiert, weil es als hohes Risiko eingestuft ist. Sie können es trotzdem installieren, wenn Sie das Risiko verstehen und akzeptieren.', confirmForceInstall: 'Trotzdem installieren' } } },
  },
  'el-GR': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Υψηλός κίνδυνος', highRiskInstallTitle: 'Επιβεβαίωση εγκατάστασης υψηλού κινδύνου', highRiskInstallBody: '{name} έχει αποκλειστεί αυτή τη στιγμή από την πολιτική ασφαλείας επειδή έχει επισημανθεί ως υψηλού κινδύνου. Μπορείτε ακόμη να το εγκαταστήσετε αν κατανοείτε και αποδέχεστε τον κίνδυνο.', confirmForceInstall: "Εγκατάσταση παρ' όλα αυτά" } } },
  },
  'en-GB': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'High risk', highRiskInstallTitle: 'High-risk install confirmation', highRiskInstallBody: '{name} is currently blocked by the security policy because it is marked high risk. You can still install it if you understand and accept the risk.', confirmForceInstall: 'Install anyway' } } },
  },
  'en-US': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'High risk', highRiskInstallTitle: 'High-risk install confirmation', highRiskInstallBody: '{name} is currently blocked by the security policy because it is marked high risk. You can still install it if you understand and accept the risk.', confirmForceInstall: 'Install anyway' } } },
  },
  'es-ES': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Alto riesgo', highRiskInstallTitle: 'Confirmación de instalación de alto riesgo', highRiskInstallBody: '{name} está bloqueado actualmente por la política de seguridad porque está marcado como de alto riesgo. Todavía puedes instalarlo si entiendes y aceptas el riesgo.', confirmForceInstall: 'Instalar de todos modos' } } },
  },
  'fr-FR': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Risque élevé', highRiskInstallTitle: "Confirmation d'installation à haut risque", highRiskInstallBody: "{name} est actuellement bloqué par la politique de sécurité parce qu'il est marqué comme à haut risque. Vous pouvez quand même l'installer si vous comprenez et acceptez le risque.", confirmForceInstall: 'Installer quand même' } } },
  },
  'ga-IE': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Ardriosca', highRiskInstallTitle: 'Dearbhú suiteála ardriosca', highRiskInstallBody: 'Tá {name} bactha ag an mbeartas slándála faoi láthair toisc go bhfuil sé marcáilte mar ardriosca. Is féidir leat é a shuiteáil fós má thuigeann tú agus má ghlacann tú leis an riosca.', confirmForceInstall: 'Suiteáil mar sin féin' } } },
  },
  'hr-HR': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Visok rizik', highRiskInstallTitle: 'Potvrda instalacije visokog rizika', highRiskInstallBody: '{name} je trenutačno blokiran sigurnosnom politikom jer je označen kao visokorizičan. I dalje ga možete instalirati ako razumijete i prihvaćate rizik.', confirmForceInstall: 'Svejedno instaliraj' } } },
  },
  'hu-HU': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Magas kockázat', highRiskInstallTitle: 'Magas kockázatú telepítés megerősítése', highRiskInstallBody: '{name} jelenleg a biztonsági szabályzat miatt blokkolva van, mert magas kockázatúnak van jelölve. Továbbra is telepítheti, ha megérti és elfogadja a kockázatot.', confirmForceInstall: 'Telepítés mindenképp' } } },
  },
  'it-IT': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Rischio elevato', highRiskInstallTitle: "Conferma dell'installazione ad alto rischio", highRiskInstallBody: '{name} è attualmente bloccato dalla policy di sicurezza perché è contrassegnato come ad alto rischio. Puoi comunque installarlo se comprendi e accetti il rischio.', confirmForceInstall: 'Installa comunque' } } },
  },
  'ja-JP': {
    skillStore: { marketplace: { modal: { highRiskWarning: '高リスク', highRiskInstallTitle: '高リスクのインストール確認', highRiskInstallBody: '{name} は高リスクとしてマークされているため、現在セキュリティポリシーによってブロックされています。リスクを理解し、受け入れる場合は、それでもインストールできます。', confirmForceInstall: 'それでもインストール' } } },
  },
  'ko-KR': {
    skillStore: { marketplace: { modal: { highRiskWarning: '높은 위험', highRiskInstallTitle: '고위험 설치 확인', highRiskInstallBody: '{name}은(는) 고위험으로 표시되어 현재 보안 정책에 의해 차단되어 있습니다. 위험을 이해하고 수용한다면 그래도 설치할 수 있습니다.', confirmForceInstall: '그래도 설치' } } },
  },
  'ml-IN': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'ഉയർന്ന അപകടസാധ്യത', highRiskInstallTitle: 'ഉയർന്ന അപകടസാധ്യതയുള്ള ഇൻസ്റ്റാൾ സ്ഥിരീകരണം', highRiskInstallBody: '{name} ഉയർന്ന അപകടസാധ്യതയായി അടയാളപ്പെടുത്തിയതിനാൽ ഇപ്പോള്‍ സുരക്ഷാ നയം കാരണം തടഞ്ഞിരിക്കുന്നു. അപകടസാധ്യത മനസ്സിലാക്കി സ്വീകരിക്കുന്നുവെങ്കിൽ നിങ്ങള്‍ക്ക് ഇത് എന്നിരുന്നാലും ഇൻസ്റ്റാൾ ചെയ്യാം.', confirmForceInstall: 'എന്തായാലും ഇൻസ്റ്റാൾ ചെയ്യുക' } } },
  },
  'nb-NO': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Høy risiko', highRiskInstallTitle: 'Bekreftelse på høyrisiko-installasjon', highRiskInstallBody: '{name} er for øyeblikket blokkert av sikkerhetspolicyen fordi den er merket som høy risiko. Du kan fortsatt installere den hvis du forstår og aksepterer risikoen.', confirmForceInstall: 'Installer likevel' } } },
  },
  'nl-NL': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Hoog risico', highRiskInstallTitle: 'Bevestiging voor installatie met hoog risico', highRiskInstallBody: '{name} is momenteel geblokkeerd door het beveiligingsbeleid omdat het als hoog risico is gemarkeerd. Je kunt het nog steeds installeren als je het risico begrijpt en accepteert.', confirmForceInstall: 'Toch installeren' } } },
  },
  'pl-PL': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Wysokie ryzyko', highRiskInstallTitle: 'Potwierdzenie instalacji wysokiego ryzyka', highRiskInstallBody: '{name} jest obecnie zablokowany przez politykę bezpieczeństwa, ponieważ został oznaczony jako wysokiego ryzyka. Nadal możesz go zainstalować, jeśli rozumiesz i akceptujesz ryzyko.', confirmForceInstall: 'Zainstaluj mimo to' } } },
  },
  'pt-BR': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Alto risco', highRiskInstallTitle: 'Confirmação de instalação de alto risco', highRiskInstallBody: '{name} está bloqueado no momento pela política de segurança porque foi marcado como de alto risco. Você ainda pode instalá-lo se entender e aceitar o risco.', confirmForceInstall: 'Instalar mesmo assim' } } },
  },
  'pt-PT': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Alto risco', highRiskInstallTitle: 'Confirmação de instalação de alto risco', highRiskInstallBody: '{name} está atualmente bloqueado pela política de segurança porque foi marcado como de alto risco. Ainda o pode instalar se compreender e aceitar o risco.', confirmForceInstall: 'Instalar mesmo assim' } } },
  },
  'ro-RO': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Risc ridicat', highRiskInstallTitle: 'Confirmarea instalării cu risc ridicat', highRiskInstallBody: '{name} este blocat în prezent de politica de securitate deoarece este marcat ca având risc ridicat. Îl poți instala în continuare dacă înțelegi și accepți riscul.', confirmForceInstall: 'Instalează oricum' } } },
  },
  'ru-RU': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Высокий риск', highRiskInstallTitle: 'Подтверждение установки с высоким риском', highRiskInstallBody: '{name} сейчас заблокирован политикой безопасности, потому что помечен как высокорисковый. Вы все равно можете установить его, если понимаете и принимаете этот риск.', confirmForceInstall: 'Все равно установить' } } },
  },
  'sk-SK': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Vysoké riziko', highRiskInstallTitle: 'Potvrdenie inštalácie s vysokým rizikom', highRiskInstallBody: '{name} je momentálne blokovaný bezpečnostnou politikou, pretože je označený ako vysoko rizikový. Stále ho môžete nainštalovať, ak riziku rozumiete a prijímate ho.', confirmForceInstall: 'Napriek tomu nainštalovať' } } },
  },
  'sv-SE': {
    skillStore: { marketplace: { modal: { highRiskWarning: 'Hög risk', highRiskInstallTitle: 'Bekräfta installation med hög risk', highRiskInstallBody: '{name} är för närvarande blockerad av säkerhetspolicyn eftersom den är markerad som hög risk. Du kan fortfarande installera den om du förstår och accepterar risken.', confirmForceInstall: 'Installera ändå' } } },
  },
  'zh-CN': {
    skillStore: { marketplace: { modal: { highRiskWarning: '高风险', highRiskInstallTitle: '高风险安装确认', highRiskInstallBody: '{name} 因被标记为高风险，当前已被安全策略阻止。只要你理解并接受风险，仍然可以安装。', confirmForceInstall: '仍然安装' } } },
  },
  'zh-TW': {
    skillStore: { marketplace: { modal: { highRiskWarning: '高風險', highRiskInstallTitle: '高風險安裝確認', highRiskInstallBody: '{name} 因被標記為高風險，目前已被安全策略阻止。只要你理解並接受風險，仍然可以安裝。', confirmForceInstall: '仍要安裝' } } },
  },
} satisfies Partial<Record<LocaleKey, Record<string, unknown>>>

export default skillStoreHighRiskBackfills
