type RalphLoopHoverOverridePack = {
  description: string
  plan: string
  act: string
  check: string
}

function buildLocaleOverrides(pack: RalphLoopHoverOverridePack) {
  return {
    chat: {
      ralphLoopHoverDescription: pack.description,
      ralphLoopHoverPlan: pack.plan,
      ralphLoopHoverAct: pack.act,
      ralphLoopHoverCheck: pack.check,
    },
  }
}

export default {
  'ca-ES': buildLocaleOverrides({
    description:
      "Deixa que l'agent planifiqui, faci servir eines, apliqui canvis i continuï iterant fins que la tasca quedi resolta netament.",
    plan: 'Planifica',
    act: 'Actua',
    check: 'Comprova',
  }),
  'cs-CZ': buildLocaleOverrides({
    description:
      'Nechte agenta plánovat, používat nástroje, provádět změny a dál iterovat, dokud úkol nebude čistě dokončen.',
    plan: 'Plán',
    act: 'Akce',
    check: 'Kontrola',
  }),
  'da-DK': buildLocaleOverrides({
    description:
      'Lad agenten planlægge, bruge værktøjer, anvende ændringer og fortsætte med at iterere, indtil opgaven er løst ordentligt.',
    plan: 'Plan',
    act: 'Udfør',
    check: 'Tjek',
  }),
  'de-DE': buildLocaleOverrides({
    description:
      'Lass den Agenten planen, Werkzeuge nutzen, Änderungen anwenden und so lange iterieren, bis die Aufgabe sauber erledigt ist.',
    plan: 'Planen',
    act: 'Ausführen',
    check: 'Prüfen',
  }),
  'el-GR': buildLocaleOverrides({
    description:
      'Αφήστε τον agent να σχεδιάσει, να χρησιμοποιήσει εργαλεία, να εφαρμόσει αλλαγές και να συνεχίσει να επαναλαμβάνει μέχρι να ολοκληρωθεί καθαρά η εργασία.',
    plan: 'Σχεδιασμός',
    act: 'Εκτέλεση',
    check: 'Έλεγχος',
  }),
  'en-GB': buildLocaleOverrides({
    description:
      'Let the agent plan, use tools, apply changes, and keep iterating until the task lands cleanly.',
    plan: 'Plan',
    act: 'Act',
    check: 'Check',
  }),
  'en-US': buildLocaleOverrides({
    description:
      'Let the agent plan, use tools, apply changes, and keep iterating until the task lands cleanly.',
    plan: 'Plan',
    act: 'Act',
    check: 'Check',
  }),
  'es-ES': buildLocaleOverrides({
    description:
      'Deja que el agente planifique, use herramientas, aplique cambios y siga iterando hasta que la tarea quede resuelta de forma limpia.',
    plan: 'Planificar',
    act: 'Actuar',
    check: 'Revisar',
  }),
  'fr-FR': buildLocaleOverrides({
    description:
      "Laissez l'agent planifier, utiliser des outils, appliquer des changements et continuer à itérer jusqu'à ce que la tâche soit proprement bouclée.",
    plan: 'Planifier',
    act: 'Agir',
    check: 'Vérifier',
  }),
  'ga-IE': buildLocaleOverrides({
    description:
      'Lig don ghníomhaire pleanáil, uirlisí a úsáid, athruithe a chur i bhfeidhm, agus leanúint ar aghaidh ag atriall go dtí go mbeidh an tasc curtha i gcrích go glan.',
    plan: 'Pleanáil',
    act: 'Gníomh',
    check: 'Seiceáil',
  }),
  'hr-HR': buildLocaleOverrides({
    description:
      'Pusti agentu da planira, koristi alate, primjenjuje izmjene i nastavi iterirati dok zadatak ne bude uredno dovršen.',
    plan: 'Plan',
    act: 'Izvedi',
    check: 'Provjeri',
  }),
  'hu-HU': buildLocaleOverrides({
    description:
      'Hagyd, hogy az ügynök tervezzen, eszközöket használjon, módosításokat alkalmazzon, és addig iteráljon, amíg a feladat tisztán célba nem ér.',
    plan: 'Terv',
    act: 'Végrehajtás',
    check: 'Ellenőrzés',
  }),
  'it-IT': buildLocaleOverrides({
    description:
      "Lascia che l'agente pianifichi, usi gli strumenti, applichi le modifiche e continui a iterare finché il compito non viene portato a termine in modo pulito.",
    plan: 'Pianifica',
    act: 'Agisci',
    check: 'Verifica',
  }),
  'ja-JP': buildLocaleOverrides({
    description:
      'エージェントに計画、ツール利用、変更の適用を任せ、タスクがきれいに完了するまで反復を続けます。',
    plan: '計画',
    act: '実行',
    check: '確認',
  }),
  'ko-KR': buildLocaleOverrides({
    description:
      '에이전트가 계획하고, 도구를 사용하고, 변경을 적용하며, 작업이 깔끔하게 마무리될 때까지 계속 반복하도록 합니다.',
    plan: '계획',
    act: '실행',
    check: '점검',
  }),
  'ml-IN': buildLocaleOverrides({
    description:
      'ഏജന്റിന് പദ്ധതിയിടാനും, ഉപകരണങ്ങൾ ഉപയോഗിക്കാനും, മാറ്റങ്ങൾ പ്രയോഗിക്കാനും, ജോലി വൃത്തിയായി പൂർത്തിയാകുന്നതുവരെ ആവർത്തിച്ച് തുടരാനും അനുവദിക്കുക.',
    plan: 'പദ്ധതി',
    act: 'നടപടി',
    check: 'പരിശോധനം',
  }),
  'nb-NO': buildLocaleOverrides({
    description:
      'La agenten planlegge, bruke verktøy, gjennomføre endringer og fortsette å iterere til oppgaven er løst på en ryddig måte.',
    plan: 'Plan',
    act: 'Utfør',
    check: 'Sjekk',
  }),
  'nl-NL': buildLocaleOverrides({
    description:
      'Laat de agent plannen, tools gebruiken, wijzigingen toepassen en blijven itereren totdat de taak netjes is afgerond.',
    plan: 'Plannen',
    act: 'Uitvoeren',
    check: 'Controleren',
  }),
  'pl-PL': buildLocaleOverrides({
    description:
      'Pozwól agentowi planować, używać narzędzi, wprowadzać zmiany i dalej iterować, aż zadanie zostanie czysto domknięte.',
    plan: 'Plan',
    act: 'Działaj',
    check: 'Sprawdź',
  }),
  'pt-BR': buildLocaleOverrides({
    description:
      'Deixe o agente planejar, usar ferramentas, aplicar mudanças e continuar iterando até que a tarefa seja concluída de forma limpa.',
    plan: 'Planejar',
    act: 'Agir',
    check: 'Verificar',
  }),
  'pt-PT': buildLocaleOverrides({
    description:
      'Deixe o agente planear, usar ferramentas, aplicar alterações e continuar a iterar até que a tarefa fique concluída de forma limpa.',
    plan: 'Planear',
    act: 'Agir',
    check: 'Verificar',
  }),
  'ro-RO': buildLocaleOverrides({
    description:
      'Lasă agentul să planifice, să folosească instrumente, să aplice modificări și să continue să itereze până când sarcina este încheiată curat.',
    plan: 'Plan',
    act: 'Acțiune',
    check: 'Verificare',
  }),
  'ru-RU': buildLocaleOverrides({
    description:
      'Позвольте агенту планировать, использовать инструменты, вносить изменения и продолжать итерации, пока задача не будет аккуратно доведена до результата.',
    plan: 'План',
    act: 'Действие',
    check: 'Проверка',
  }),
  'sk-SK': buildLocaleOverrides({
    description:
      'Nechajte agenta plánovať, používať nástroje, aplikovať zmeny a ďalej iterovať, kým úloha nebude čisto dokončená.',
    plan: 'Plán',
    act: 'Vykonať',
    check: 'Skontrolovať',
  }),
  'sv-SE': buildLocaleOverrides({
    description:
      'Låt agenten planera, använda verktyg, tillämpa ändringar och fortsätta iterera tills uppgiften landar rent.',
    plan: 'Planera',
    act: 'Agera',
    check: 'Kontrollera',
  }),
  'zh-CN': buildLocaleOverrides({
    description: '让 Agent 自主规划、调用工具、落地修改，并在任务完成前持续迭代。',
    plan: '规划',
    act: '执行',
    check: '复核',
  }),
  'zh-TW': buildLocaleOverrides({
    description: '讓 Agent 自主規劃、調用工具、落地修改，並在任務完成前持續迭代。',
    plan: '規劃',
    act: '執行',
    check: '複核',
  }),
}
