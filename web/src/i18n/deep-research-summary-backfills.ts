import type { LocaleKey } from './locale-catalog'

export type DeepResearchSummaryCopy = {
  summaryGapCoverage: string
  summaryOfficialGap: string
  summaryResolvedCoverage: string
  summaryDomainCoverage: string
  summaryFreshnessCoverage: string
  summaryClaimSupportCoverage: string
  openQuestionPrimarySourceVerification: string
}

export const deepResearchSummaryBackfills: Record<LocaleKey, DeepResearchSummaryCopy> = {
  'ca-ES': {
    summaryGapCoverage:
      "{focus} només cobreix {evidenceCount} prova/es en {domainCount} domini/s; cal més recerca.",
    summaryOfficialGap: "{focus} encara no té fonts primàries o oficials estables.",
    summaryResolvedCoverage:
      "{focus} està cobert per {evidenceCount} prova/es en {domainCount} domini/s.",
    summaryDomainCoverage: "La cobertura s'estén a {domainCount} dominis únics.",
    summaryFreshnessCoverage: "L'evidència recent arriba fins a {year}.",
    summaryClaimSupportCoverage:
      "Les conclusions principals estan recolzades per {supportCount} grup(s) d'afirmacions.",
    openQuestionPrimarySourceVerification: 'Reviseu fonts primàries per a la verificació final.',
  },
  'cs-CZ': {
    summaryGapCoverage:
      '{focus} pokrývá jen {evidenceCount} důkaz(ů) napříč {domainCount} doménami; je potřeba další průzkum.',
    summaryOfficialGap: '{focus} zatím postrádá stabilní primární nebo oficiální zdroje.',
    summaryResolvedCoverage:
      '{focus} je pokryto {evidenceCount} důkaz(y) napříč {domainCount} doménami.',
    summaryDomainCoverage: 'Pokrytí zahrnuje {domainCount} jedinečných domén.',
    summaryFreshnessCoverage: 'Aktuální důkazy sahají do roku {year}.',
    summaryClaimSupportCoverage:
      'Hlavní závěry podporuje {supportCount} skupin tvrzení.',
    openQuestionPrimarySourceVerification:
      'Pro závěrečné ověření zkontrolujte primární zdroje.',
  },
  'da-DK': {
    summaryGapCoverage:
      '{focus} dækker kun {evidenceCount} bevispunkt(er) på tværs af {domainCount} domæne(r); yderligere research er nødvendig.',
    summaryOfficialGap: '{focus} mangler stadig stabile primære eller officielle kilder.',
    summaryResolvedCoverage:
      '{focus} er dækket af {evidenceCount} bevispunkt(er) på tværs af {domainCount} domæne(r).',
    summaryDomainCoverage: 'Dækningen spænder over {domainCount} unikke domæne(r).',
    summaryFreshnessCoverage: 'Frisk evidens når frem til {year}.',
    summaryClaimSupportCoverage:
      'Kernekonklusionerne understøttes af {supportCount} påstandsgruppe(r).',
    openQuestionPrimarySourceVerification:
      'Kontrollér primære kilder for endelig verifikation.',
  },
  'de-DE': {
    summaryGapCoverage:
      '{focus} deckt nur {evidenceCount} Beleg(e) über {domainCount} Domain(s) ab; weitere Recherche ist nötig.',
    summaryOfficialGap: '{focus} hat noch keine stabilen Primär- oder offiziellen Quellen.',
    summaryResolvedCoverage:
      '{focus} ist durch {evidenceCount} Beleg(e) über {domainCount} Domain(s) abgedeckt.',
    summaryDomainCoverage: 'Die Abdeckung umfasst {domainCount} eindeutige Domain(s).',
    summaryFreshnessCoverage: 'Aktuelle Evidenz reicht bis {year}.',
    summaryClaimSupportCoverage:
      'Die Kernaussagen werden von {supportCount} Claim-Gruppe(n) gestützt.',
    openQuestionPrimarySourceVerification:
      'Bitte zur finalen Verifikation Primärquellen prüfen.',
  },
  'el-GR': {
    summaryGapCoverage:
      'Το {focus} καλύπτει μόνο {evidenceCount} στοιχείο/α τεκμηρίωσης σε {domainCount} τομέα/είς· απαιτείται περαιτέρω έρευνα.',
    summaryOfficialGap:
      'Το {focus} εξακολουθεί να μην διαθέτει σταθερές πρωτογενείς ή επίσημες πηγές.',
    summaryResolvedCoverage:
      'Το {focus} καλύπτεται από {evidenceCount} στοιχείο/α τεκμηρίωσης σε {domainCount} τομέα/είς.',
    summaryDomainCoverage: 'Η κάλυψη εκτείνεται σε {domainCount} μοναδικούς τομείς.',
    summaryFreshnessCoverage: 'Τα πρόσφατα στοιχεία φτάνουν έως το {year}.',
    summaryClaimSupportCoverage:
      'Τα βασικά συμπεράσματα υποστηρίζονται από {supportCount} ομάδες ισχυρισμών.',
    openQuestionPrimarySourceVerification:
      'Ελέγξτε πρωτογενείς πηγές για τελική επαλήθευση.',
  },
  'en-GB': {
    summaryGapCoverage:
      '{focus} only covers {evidenceCount} evidence item(s) across {domainCount} domain(s); follow-up research is needed.',
    summaryOfficialGap: '{focus} still lacks stable primary or official sources.',
    summaryResolvedCoverage:
      '{focus} is covered by {evidenceCount} evidence item(s) across {domainCount} domain(s).',
    summaryDomainCoverage: 'Coverage spans {domainCount} unique domain(s).',
    summaryFreshnessCoverage: 'Fresh evidence reaches {year}.',
    summaryClaimSupportCoverage:
      'Core conclusions are supported across {supportCount} claim group(s).',
    openQuestionPrimarySourceVerification:
      'Check primary sources for final verification.',
  },
  'en-US': {
    summaryGapCoverage:
      '{focus} only covers {evidenceCount} evidence item(s) across {domainCount} domain(s); follow-up research is needed.',
    summaryOfficialGap: '{focus} still lacks stable primary or official sources.',
    summaryResolvedCoverage:
      '{focus} is covered by {evidenceCount} evidence item(s) across {domainCount} domain(s).',
    summaryDomainCoverage: 'Coverage spans {domainCount} unique domain(s).',
    summaryFreshnessCoverage: 'Fresh evidence reaches {year}.',
    summaryClaimSupportCoverage:
      'Core conclusions are supported across {supportCount} claim group(s).',
    openQuestionPrimarySourceVerification:
      'Check primary sources for final verification.',
  },
  'es-ES': {
    summaryGapCoverage:
      '{focus} solo cubre {evidenceCount} evidencia(s) en {domainCount} dominio(s); se necesita investigación adicional.',
    summaryOfficialGap: '{focus} aún carece de fuentes primarias u oficiales estables.',
    summaryResolvedCoverage:
      '{focus} está respaldado por {evidenceCount} evidencia(s) en {domainCount} dominio(s).',
    summaryDomainCoverage: 'La cobertura abarca {domainCount} dominio(s) único(s).',
    summaryFreshnessCoverage: 'La evidencia reciente alcanza {year}.',
    summaryClaimSupportCoverage:
      'Las conclusiones principales están respaldadas por {supportCount} grupo(s) de afirmaciones.',
    openQuestionPrimarySourceVerification:
      'Revise fuentes primarias para la verificación final.',
  },
  'fr-FR': {
    summaryGapCoverage:
      '{focus} ne couvre que {evidenceCount} élément(s) de preuve sur {domainCount} domaine(s) ; une recherche complémentaire est nécessaire.',
    summaryOfficialGap:
      "{focus} ne dispose pas encore de sources primaires ou officielles stables.",
    summaryResolvedCoverage:
      '{focus} est couvert par {evidenceCount} élément(s) de preuve sur {domainCount} domaine(s).',
    summaryDomainCoverage: 'La couverture s\'étend à {domainCount} domaine(s) unique(s).',
    summaryFreshnessCoverage: 'Les preuves récentes vont jusqu\'à {year}.',
    summaryClaimSupportCoverage:
      "Les conclusions principales sont soutenues par {supportCount} groupe(s) d'affirmations.",
    openQuestionPrimarySourceVerification:
      'Vérifiez les sources primaires pour la validation finale.',
  },
  'ga-IE': {
    summaryGapCoverage:
      'Ní chlúdaíonn {focus} ach {evidenceCount} mír/míreanna fianaise thar {domainCount} fhearann/fhearainn; tá tuilleadh taighde de dhíth.',
    summaryOfficialGap:
      'Níl príomhfhoinsí ná foinsí oifigiúla cobhsaí ag {focus} fós.',
    summaryResolvedCoverage:
      'Tá {focus} clúdaithe ag {evidenceCount} mír/míreanna fianaise thar {domainCount} fhearann/fhearainn.',
    summaryDomainCoverage:
      'Síneann an clúdach thar {domainCount} fhearann/fhearainn uathúil.',
    summaryFreshnessCoverage: 'Sroicheann an fhianaise úr suas go {year}.',
    summaryClaimSupportCoverage:
      'Tacaíonn {supportCount} grúpa éileamh leis na príomhchonclúidí.',
    openQuestionPrimarySourceVerification:
      'Seiceáil príomhfhoinsí le haghaidh fíoraithe deiridh.',
  },
  'hr-HR': {
    summaryGapCoverage:
      '{focus} pokriva samo {evidenceCount} dokaz(a) na {domainCount} domena; potrebno je dodatno istraživanje.',
    summaryOfficialGap: '{focus} još nema stabilne primarne ili službene izvore.',
    summaryResolvedCoverage:
      '{focus} je pokriven s {evidenceCount} dokaz(a) na {domainCount} domena.',
    summaryDomainCoverage: 'Pokrivenost obuhvaća {domainCount} jedinstvenih domena.',
    summaryFreshnessCoverage: 'Novi dokazi dosežu do {year}.',
    summaryClaimSupportCoverage:
      'Glavne zaključke podupire {supportCount} skupina tvrdnji.',
    openQuestionPrimarySourceVerification:
      'Provjerite primarne izvore za završnu verifikaciju.',
  },
  'hu-HU': {
    summaryGapCoverage:
      '{focus} csak {evidenceCount} bizonyítékot fed le {domainCount} doménen; további kutatás szükséges.',
    summaryOfficialGap:
      '{focus} még mindig nem rendelkezik stabil elsődleges vagy hivatalos forrásokkal.',
    summaryResolvedCoverage:
      '{focus} {evidenceCount} bizonyítékkal és {domainCount} doménnel fedett.',
    summaryDomainCoverage: 'A lefedettség {domainCount} egyedi domént ölel fel.',
    summaryFreshnessCoverage: 'A friss bizonyítékok {year}-ig terjednek.',
    summaryClaimSupportCoverage:
      'A fő következtetéseket {supportCount} állításcsoport támasztja alá.',
    openQuestionPrimarySourceVerification:
      'A végső ellenőrzéshez vizsgálja meg az elsődleges forrásokat.',
  },
  'it-IT': {
    summaryGapCoverage:
      '{focus} copre solo {evidenceCount} elemento/i di prova in {domainCount} dominio/i; sono necessarie ulteriori ricerche.',
    summaryOfficialGap:
      '{focus} non dispone ancora di fonti primarie o ufficiali stabili.',
    summaryResolvedCoverage:
      '{focus} è supportato da {evidenceCount} elemento/i di prova in {domainCount} dominio/i.',
    summaryDomainCoverage: 'La copertura si estende a {domainCount} dominio/i unici.',
    summaryFreshnessCoverage: 'Le prove recenti arrivano fino al {year}.',
    summaryClaimSupportCoverage:
      'Le conclusioni principali sono supportate da {supportCount} gruppo/i di affermazioni.',
    openQuestionPrimarySourceVerification:
      'Verifica le fonti primarie per la validazione finale.',
  },
  'ja-JP': {
    summaryGapCoverage:
      '{focus} は {evidenceCount} 件の根拠・{domainCount} 件のドメインしか確認できておらず、追加調査が必要です。',
    summaryOfficialGap:
      '{focus} にはまだ安定した一次情報または公式ソースの裏付けがありません。',
    summaryResolvedCoverage:
      '{focus} は {evidenceCount} 件の根拠と {domainCount} 件のドメインで裏付けられています。',
    summaryDomainCoverage:
      'カバレッジは {domainCount} 件のユニークドメインに広がっています。',
    summaryFreshnessCoverage: '新しい根拠は {year} 年まで確認できています。',
    summaryClaimSupportCoverage:
      '主要な結論は {supportCount} 組の支持シグナルで裏付けられています。',
    openQuestionPrimarySourceVerification:
      '最終確認のため一次情報を確認してください。',
  },
  'ko-KR': {
    summaryGapCoverage:
      '{focus} 항목은 근거 {evidenceCount}건, 도메인 {domainCount}곳만 확보되어 추가 조사가 필요합니다.',
    summaryOfficialGap:
      '{focus} 항목은 아직 안정적인 1차 또는 공식 출처로 뒷받침되지 않았습니다.',
    summaryResolvedCoverage:
      '{focus} 항목은 근거 {evidenceCount}건과 도메인 {domainCount}곳으로 확인되었습니다.',
    summaryDomainCoverage:
      '현재 커버리지는 고유 도메인 {domainCount}곳에 걸쳐 있습니다.',
    summaryFreshnessCoverage: '최신 근거는 {year}년까지 확보되었습니다.',
    summaryClaimSupportCoverage:
      '핵심 결론은 지지 신호 {supportCount}개 그룹으로 뒷받침됩니다.',
    openQuestionPrimarySourceVerification:
      '최종 검증을 위해 1차 출처를 확인하세요.',
  },
  'ml-IN': {
    summaryGapCoverage:
      '{focus} ന് {evidenceCount} തെളിവുകളും {domainCount} ഡൊമൈനുകളും മാത്രമാണ് ഉൾപ്പെടുന്നത്; കൂടുതൽ ഗവേഷണം ആവശ്യമാണ്.',
    summaryOfficialGap:
      '{focus} ന് ഇനിയും സ്ഥിരമായ പ്രാഥമികമോ ഔദ്യോഗികമോ ആയ ഉറവിടങ്ങൾ ലഭ്യമല്ല.',
    summaryResolvedCoverage:
      '{focus} ന് {evidenceCount} തെളിവുകളും {domainCount} ഡൊമൈനുകളും വഴി പിന്തുണ ലഭിക്കുന്നു.',
    summaryDomainCoverage:
      'കവറേജ് {domainCount} ഏകീകൃത ഡൊമൈനുകളിലേക്ക് വ്യാപിക്കുന്നു.',
    summaryFreshnessCoverage: 'പുതിയ തെളിവുകൾ {year} വരെ എത്തുന്നു.',
    summaryClaimSupportCoverage:
      'പ്രധാന നിഗമനങ്ങൾക്ക് {supportCount} അവകാശവാദ ഗ്രൂപ്പുകളുടെ പിന്തുണയുണ്ട്.',
    openQuestionPrimarySourceVerification:
      'അവസാന സ്ഥിരീകരണത്തിനായി പ്രാഥമിക ഉറവിടങ്ങൾ പരിശോധിക്കുക.',
  },
  'nb-NO': {
    summaryGapCoverage:
      '{focus} dekker bare {evidenceCount} bevispunkt(er) på tvers av {domainCount} domene(r); videre undersøkelser trengs.',
    summaryOfficialGap:
      '{focus} mangler fortsatt stabile primære eller offisielle kilder.',
    summaryResolvedCoverage:
      '{focus} er dekket av {evidenceCount} bevispunkt(er) på tvers av {domainCount} domene(r).',
    summaryDomainCoverage: 'Dekningen omfatter {domainCount} unike domene(r).',
    summaryFreshnessCoverage: 'Fersk dokumentasjon går frem til {year}.',
    summaryClaimSupportCoverage:
      'Kjernkonklusjonene støttes av {supportCount} påstandsgruppe(r).',
    openQuestionPrimarySourceVerification:
      'Sjekk primærkilder for endelig verifisering.',
  },
  'nl-NL': {
    summaryGapCoverage:
      '{focus} dekt slechts {evidenceCount} bewijsitem(s) over {domainCount} domein(en); vervolgonderzoek is nodig.',
    summaryOfficialGap:
      '{focus} mist nog stabiele primaire of officiële bronnen.',
    summaryResolvedCoverage:
      '{focus} wordt gedekt door {evidenceCount} bewijsitem(s) over {domainCount} domein(en).',
    summaryDomainCoverage: 'De dekking beslaat {domainCount} unieke domein(en).',
    summaryFreshnessCoverage: 'Recent bewijs reikt tot {year}.',
    summaryClaimSupportCoverage:
      'Kernconclusies worden ondersteund door {supportCount} claimgroep(en).',
    openQuestionPrimarySourceVerification:
      'Controleer primaire bronnen voor definitieve verificatie.',
  },
  'pl-PL': {
    summaryGapCoverage:
      '{focus} obejmuje tylko {evidenceCount} dowod(ów) w {domainCount} domenach; potrzebne jest dalsze badanie.',
    summaryOfficialGap:
      '{focus} nadal nie ma stabilnych źródeł pierwotnych ani oficjalnych.',
    summaryResolvedCoverage:
      '{focus} jest objęte {evidenceCount} dowod(ami) z {domainCount} domen.',
    summaryDomainCoverage: 'Pokrycie obejmuje {domainCount} unikalnych domen.',
    summaryFreshnessCoverage: 'Świeże dowody sięgają {year} r.',
    summaryClaimSupportCoverage:
      'Główne wnioski są wspierane przez {supportCount} grup(y) twierdzeń.',
    openQuestionPrimarySourceVerification:
      'Sprawdź źródła pierwotne dla końcowej weryfikacji.',
  },
  'pt-BR': {
    summaryGapCoverage:
      '{focus} cobre apenas {evidenceCount} evidência(s) em {domainCount} domínio(s); é necessária pesquisa adicional.',
    summaryOfficialGap:
      '{focus} ainda não conta com fontes primárias ou oficiais estáveis.',
    summaryResolvedCoverage:
      '{focus} está coberto por {evidenceCount} evidência(s) em {domainCount} domínio(s).',
    summaryDomainCoverage: 'A cobertura abrange {domainCount} domínio(s) único(s).',
    summaryFreshnessCoverage: 'A evidência recente chega até {year}.',
    summaryClaimSupportCoverage:
      'As conclusões principais são sustentadas por {supportCount} grupo(s) de alegações.',
    openQuestionPrimarySourceVerification:
      'Verifique fontes primárias para validação final.',
  },
  'pt-PT': {
    summaryGapCoverage:
      '{focus} cobre apenas {evidenceCount} evidência(s) em {domainCount} domínio(s); é necessária pesquisa adicional.',
    summaryOfficialGap:
      '{focus} ainda não conta com fontes primárias ou oficiais estáveis.',
    summaryResolvedCoverage:
      '{focus} está coberto por {evidenceCount} evidência(s) em {domainCount} domínio(s).',
    summaryDomainCoverage: 'A cobertura abrange {domainCount} domínio(s) único(s).',
    summaryFreshnessCoverage: 'A evidência recente chega até {year}.',
    summaryClaimSupportCoverage:
      'As conclusões principais são sustentadas por {supportCount} grupo(s) de alegações.',
    openQuestionPrimarySourceVerification:
      'Verifique fontes primárias para validação final.',
  },
  'ro-RO': {
    summaryGapCoverage:
      '{focus} acoperă doar {evidenceCount} dovezi în {domainCount} domenii; este necesară cercetare suplimentară.',
    summaryOfficialGap:
      '{focus} încă nu are surse primare sau oficiale stabile.',
    summaryResolvedCoverage:
      '{focus} este acoperit de {evidenceCount} dovezi din {domainCount} domenii.',
    summaryDomainCoverage: 'Acoperirea se întinde pe {domainCount} domenii unice.',
    summaryFreshnessCoverage: 'Dovezile recente ajung până în {year}.',
    summaryClaimSupportCoverage:
      'Concluziile principale sunt susținute de {supportCount} grup(uri) de afirmații.',
    openQuestionPrimarySourceVerification:
      'Verificați sursele primare pentru validarea finală.',
  },
  'ru-RU': {
    summaryGapCoverage:
      '{focus} охватывает только {evidenceCount} единиц доказательств по {domainCount} домен(ам); требуется дополнительное исследование.',
    summaryOfficialGap:
      'Для {focus} пока не найдено устойчивых первичных или официальных источников.',
    summaryResolvedCoverage:
      '{focus} подтверждается {evidenceCount} единицами доказательств по {domainCount} домен(ам).',
    summaryDomainCoverage: 'Покрытие охватывает {domainCount} уникальных домен(ов).',
    summaryFreshnessCoverage: 'Свежие доказательства доходят до {year} года.',
    summaryClaimSupportCoverage:
      'Ключевые выводы поддерживаются {supportCount} групп(ой/ами) утверждений.',
    openQuestionPrimarySourceVerification:
      'Проверьте первоисточники для финальной верификации.',
  },
  'sk-SK': {
    summaryGapCoverage:
      '{focus} pokrýva len {evidenceCount} dôkaz(ov) naprieč {domainCount} doménami; je potrebný ďalší prieskum.',
    summaryOfficialGap:
      '{focus} zatiaľ nemá stabilné primárne alebo oficiálne zdroje.',
    summaryResolvedCoverage:
      '{focus} je pokryté {evidenceCount} dôkaz(mi) naprieč {domainCount} doménami.',
    summaryDomainCoverage: 'Pokrytie zahŕňa {domainCount} jedinečných domén.',
    summaryFreshnessCoverage: 'Aktuálne dôkazy siahajú do roku {year}.',
    summaryClaimSupportCoverage:
      'Hlavné závery podporuje {supportCount} skupín tvrdení.',
    openQuestionPrimarySourceVerification:
      'Na záverečné overenie skontrolujte primárne zdroje.',
  },
  'sv-SE': {
    summaryGapCoverage:
      '{focus} täcker bara {evidenceCount} bevispunkt(er) över {domainCount} domän(er); fortsatt efterforskning behövs.',
    summaryOfficialGap:
      '{focus} saknar fortfarande stabila primära eller officiella källor.',
    summaryResolvedCoverage:
      '{focus} täcks av {evidenceCount} bevispunkt(er) över {domainCount} domän(er).',
    summaryDomainCoverage: 'Täckningen omfattar {domainCount} unika domän(er).',
    summaryFreshnessCoverage: 'Färsk evidens når fram till {year}.',
    summaryClaimSupportCoverage:
      'Kärnslutsatserna stöds av {supportCount} påståendegupp(er).',
    openQuestionPrimarySourceVerification:
      'Kontrollera primärkällor för slutlig verifiering.',
  },
  'zh-CN': {
    summaryGapCoverage:
      '{focus}仅覆盖 {evidenceCount} 条证据 / {domainCount} 个来源域名，需要继续深挖。',
    summaryOfficialGap: '{focus}尚未拿到稳定的一手/官方来源支撑。',
    summaryResolvedCoverage:
      '{focus}已覆盖 {evidenceCount} 条证据 / {domainCount} 个来源域名。',
    summaryDomainCoverage: '当前已覆盖 {domainCount} 个来源域名。',
    summaryFreshnessCoverage: '已覆盖到 {year} 年的较新来源。',
    summaryClaimSupportCoverage: '主要结论已被 {supportCount} 组支持信号覆盖。',
    openQuestionPrimarySourceVerification: '建议回查一手来源并进行最终核验。',
  },
  'zh-TW': {
    summaryGapCoverage:
      '{focus}僅覆蓋 {evidenceCount} 條證據 / {domainCount} 個來源網域，需要繼續深挖。',
    summaryOfficialGap: '{focus}尚未取得穩定的一手/官方來源支撐。',
    summaryResolvedCoverage:
      '{focus}已覆蓋 {evidenceCount} 條證據 / {domainCount} 個來源網域。',
    summaryDomainCoverage: '目前已覆蓋 {domainCount} 個來源網域。',
    summaryFreshnessCoverage: '已覆蓋到 {year} 年的較新來源。',
    summaryClaimSupportCoverage: '主要結論已被 {supportCount} 組支持訊號覆蓋。',
    openQuestionPrimarySourceVerification: '建議回查一手來源並進行最終核驗。',
  },
}
