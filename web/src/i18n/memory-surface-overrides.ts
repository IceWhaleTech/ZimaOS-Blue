type MemorySurfaceOverridePack = {
  memorySurface: string
  memoryManagement: string
  recallSettings: string
}

function buildLocaleOverrides(pack: MemorySurfaceOverridePack) {
  return {
    memory: {
      recallSettings: pack.recallSettings,
    },
    settings: {
      memorySurface: pack.memorySurface,
      memoryManagement: pack.memoryManagement,
    },
  }
}

export default {
  'ca-ES': buildLocaleOverrides({
    memorySurface: 'Memòria',
    memoryManagement: 'Ús i gestió de la memòria',
    recallSettings: 'Mode de recuperació de memòria',
  }),
  'cs-CZ': buildLocaleOverrides({
    memorySurface: 'Paměť',
    memoryManagement: 'Využití a správa paměti',
    recallSettings: 'Režim vyvolávání paměti',
  }),
  'da-DK': buildLocaleOverrides({
    memorySurface: 'Hukommelse',
    memoryManagement: 'Brug og administration af hukommelse',
    recallSettings: 'Tilstand for hukommelsesgenkaldelse',
  }),
  'de-DE': buildLocaleOverrides({
    memorySurface: 'Gedächtnis',
    memoryManagement: 'Verwendung und Verwaltung des Gedächtnisses',
    recallSettings: 'Speicherabrufmodus',
  }),
  'el-GR': buildLocaleOverrides({
    memorySurface: 'Μνήμη',
    memoryManagement: 'Χρήση και διαχείριση μνήμης',
    recallSettings: 'Λειτουργία ανάκλησης μνήμης',
  }),
  'en-GB': buildLocaleOverrides({
    memorySurface: 'Memory',
    memoryManagement: 'Memory Usage & Management',
    recallSettings: 'Recall Settings',
  }),
  'en-US': buildLocaleOverrides({
    memorySurface: 'Memory',
    memoryManagement: 'Memory Usage & Management',
    recallSettings: 'Recall Settings',
  }),
  'es-ES': buildLocaleOverrides({
    memorySurface: 'Memoria',
    memoryManagement: 'Uso y gestión de la memoria',
    recallSettings: 'Modo de recuperación de memoria',
  }),
  'fr-FR': buildLocaleOverrides({
    memorySurface: 'Mémoire',
    memoryManagement: 'Utilisation et gestion de la mémoire',
    recallSettings: 'Mode de rappel mémoire',
  }),
  'ga-IE': buildLocaleOverrides({
    memorySurface: 'Cuimhne',
    memoryManagement: 'Úsáid agus bainistíocht na cuimhne',
    recallSettings: 'Mód athghairme cuimhne',
  }),
  'hr-HR': buildLocaleOverrides({
    memorySurface: 'Pamćenje',
    memoryManagement: 'Upotreba i upravljanje pamćenjem',
    recallSettings: 'Način prisjećanja memorije',
  }),
  'hu-HU': buildLocaleOverrides({
    memorySurface: 'Memória',
    memoryManagement: 'Memória használata és kezelése',
    recallSettings: 'Memóriafelidézési mód',
  }),
  'it-IT': buildLocaleOverrides({
    memorySurface: 'Memoria',
    memoryManagement: 'Uso e gestione della memoria',
    recallSettings: 'Modalità di richiamo memoria',
  }),
  'ja-JP': buildLocaleOverrides({
    memorySurface: '記憶',
    memoryManagement: '記憶の使用と管理',
    recallSettings: 'メモリー想起モード',
  }),
  'ko-KR': buildLocaleOverrides({
    memorySurface: '기억',
    memoryManagement: '기억 사용 및 관리',
    recallSettings: '메모리 회상 모드',
  }),
  'ml-IN': buildLocaleOverrides({
    memorySurface: 'ഓർമ്മ',
    memoryManagement: 'ഓർമ്മയുടെ ഉപയോഗവും മാനേജ്മെന്റും',
    recallSettings: 'മെമ്മറി റീകോൾ മോഡ്',
  }),
  'nb-NO': buildLocaleOverrides({
    memorySurface: 'Hukommelse',
    memoryManagement: 'Bruk og administrasjon av hukommelse',
    recallSettings: 'Minnegjenkallingsmodus',
  }),
  'nl-NL': buildLocaleOverrides({
    memorySurface: 'Geheugen',
    memoryManagement: 'Gebruik en beheer van geheugen',
    recallSettings: 'Geheugenophaalmodus',
  }),
  'pl-PL': buildLocaleOverrides({
    memorySurface: 'Pamięć',
    memoryManagement: 'Użycie i zarządzanie pamięcią',
    recallSettings: 'Tryb przywoływania pamięci',
  }),
  'pt-BR': buildLocaleOverrides({
    memorySurface: 'Memória',
    memoryManagement: 'Uso e gerenciamento da memória',
    recallSettings: 'Modo de recuperação de memória',
  }),
  'pt-PT': buildLocaleOverrides({
    memorySurface: 'Memória',
    memoryManagement: 'Utilização e gestão da memória',
    recallSettings: 'Modo de recuperação de memória',
  }),
  'ro-RO': buildLocaleOverrides({
    memorySurface: 'Memorie',
    memoryManagement: 'Utilizarea și gestionarea memoriei',
    recallSettings: 'Mod de reamintire a memoriei',
  }),
  'ru-RU': buildLocaleOverrides({
    memorySurface: 'Память',
    memoryManagement: 'Использование и управление памятью',
    recallSettings: 'Режим вызова памяти',
  }),
  'sk-SK': buildLocaleOverrides({
    memorySurface: 'Pamäť',
    memoryManagement: 'Používanie a správa pamäte',
    recallSettings: 'Režim vyvolania pamäte',
  }),
  'sv-SE': buildLocaleOverrides({
    memorySurface: 'Minne',
    memoryManagement: 'Användning och hantering av minne',
    recallSettings: 'Minnesåterkallningsläge',
  }),
  'zh-CN': buildLocaleOverrides({
    memorySurface: '记忆管理',
    memoryManagement: '记忆使用与管理',
    recallSettings: '记忆使用方式',
  }),
  'zh-TW': buildLocaleOverrides({
    memorySurface: '記憶管理',
    memoryManagement: '記憶使用與管理',
    recallSettings: '記憶召回模式',
  }),
}
