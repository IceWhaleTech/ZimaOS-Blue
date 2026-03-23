// Result-card messages emitted by the backend often arrive as raw English text.
// Keep their localized copies in one place so CardResult can translate them
// without duplicating strings across the main locale files.
export default {
  'ca-ES': {
    resultCard: {
      messages: {
        file_written_successfully: 'Fitxer escrit correctament',
        search_completed: 'Cerca completada',
        auto_answered_silent_mode: 'Resposta automàtica enviada (mode silenciós)',
        screenshot_captured: 'Captura de pantalla feta',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de pantalla feta (elements interactius no disponibles)',
      },
      messageTemplates: {
        found_results: "S'han trobat {count} resultats",
        reminder_count: '{count} recordatoris',
        cleared_reminders: "S'han esborrat {count} recordatoris",
        entries_in_path: '{count} entrades a {path}',
        single_entry_in_path: '1 entrada a {path}',
        no_entries_in_path: 'No hi ha entrades a {path}',
        showing_first_entries_in_path:
          "S'estan mostrant les primeres {count} entrades de {path} (se n'han omès més)",
        screenshot_captured_for: 'Captura de pantalla feta per a {target}',
      },
      warnings: {
        listing_truncated: "La llista s'ha truncat; acota el camí o augmenta max_entries.",
      },
    },
  },
  'cs-CZ': {
    resultCard: {
      messages: {
        file_written_successfully: 'Soubor byl úspěšně zapsán',
        search_completed: 'Vyhledávání dokončeno',
        auto_answered_silent_mode: 'Zodpovězeno automaticky (tichý režim)',
        screenshot_captured: 'Snímek obrazovky pořízen',
        screenshot_captured_interactive_elements_unavailable:
          'Snímek obrazovky pořízen (interaktivní prvky nejsou k dispozici)',
      },
      messageTemplates: {
        found_results: 'Nalezeno {count} výsledků',
        reminder_count: '{count} připomenutí',
        cleared_reminders: 'Smazáno {count} připomenutí',
        entries_in_path: '{count} položek v {path}',
        single_entry_in_path: '1 položka v {path}',
        no_entries_in_path: 'V {path} nejsou žádné položky',
        showing_first_entries_in_path:
          'Zobrazuje se prvních {count} položek v {path} (další byly vynechány)',
        screenshot_captured_for: 'Snímek obrazovky pořízen pro {target}',
      },
      warnings: {
        listing_truncated: 'Výpis byl zkrácen; zúžte cestu nebo zvyšte max_entries.',
      },
    },
  },
  'da-DK': {
    resultCard: {
      messages: {
        file_written_successfully: 'Filen blev skrevet',
        search_completed: 'Søgning fuldført',
        auto_answered_silent_mode: 'Besvaret automatisk (lydløs tilstand)',
        screenshot_captured: 'Skærmbillede taget',
        screenshot_captured_interactive_elements_unavailable:
          'Skærmbillede taget (interaktive elementer er ikke tilgængelige)',
      },
      messageTemplates: {
        found_results: 'Fandt {count} resultater',
        reminder_count: '{count} påmindelser',
        cleared_reminders: 'Ryddede {count} påmindelser',
        entries_in_path: '{count} elementer i {path}',
        single_entry_in_path: '1 element i {path}',
        no_entries_in_path: 'Ingen elementer i {path}',
        showing_first_entries_in_path: 'Viser de første {count} elementer i {path} (flere udeladt)',
        screenshot_captured_for: 'Skærmbillede taget for {target}',
      },
      warnings: {
        listing_truncated: 'Listen blev afkortet; indsnævr stien eller øg max_entries.',
      },
    },
  },
  'de-DE': {
    resultCard: {
      messages: {
        file_written_successfully: 'Datei erfolgreich geschrieben',
        search_completed: 'Suche abgeschlossen',
        auto_answered_silent_mode: 'Automatisch beantwortet (stiller Modus)',
        screenshot_captured: 'Screenshot aufgenommen',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot aufgenommen (interaktive Elemente nicht verfügbar)',
      },
      messageTemplates: {
        found_results: '{count} Ergebnisse gefunden',
        reminder_count: '{count} Erinnerungen',
        cleared_reminders: '{count} Erinnerungen gelöscht',
        entries_in_path: '{count} Einträge in {path}',
        single_entry_in_path: '1 Eintrag in {path}',
        no_entries_in_path: 'Keine Einträge in {path}',
        showing_first_entries_in_path:
          'Erste {count} Einträge in {path} werden angezeigt (weitere ausgelassen)',
        screenshot_captured_for: 'Screenshot für {target} aufgenommen',
      },
      warnings: {
        listing_truncated:
          'Liste wurde gekürzt; grenzen Sie den Pfad ein oder erhöhen Sie max_entries.',
      },
    },
  },
  'el-GR': {
    resultCard: {
      messages: {
        file_written_successfully: 'Το αρχείο γράφτηκε με επιτυχία',
        search_completed: 'Η αναζήτηση ολοκληρώθηκε',
        auto_answered_silent_mode: 'Απαντήθηκε αυτόματα (σιωπηλή λειτουργία)',
        screenshot_captured: 'Λήφθηκε στιγμιότυπο οθόνης',
        screenshot_captured_interactive_elements_unavailable:
          'Λήφθηκε στιγμιότυπο οθόνης (τα διαδραστικά στοιχεία δεν είναι διαθέσιμα)',
      },
      messageTemplates: {
        found_results: 'Βρέθηκαν {count} αποτελέσματα',
        reminder_count: '{count} υπενθυμίσεις',
        cleared_reminders: 'Εκκαθαρίστηκαν {count} υπενθυμίσεις',
        entries_in_path: '{count} καταχωρίσεις στο {path}',
        single_entry_in_path: '1 καταχώριση στο {path}',
        no_entries_in_path: 'Δεν υπάρχουν καταχωρίσεις στο {path}',
        showing_first_entries_in_path:
          'Εμφανίζονται οι πρώτες {count} καταχωρίσεις στο {path} (οι υπόλοιπες παραλείφθηκαν)',
        screenshot_captured_for: 'Λήφθηκε στιγμιότυπο οθόνης για το {target}',
      },
      warnings: {
        listing_truncated: 'Η λίστα περικόπηκε· περιορίστε τη διαδρομή ή αυξήστε το max_entries.',
      },
    },
  },
  'en-GB': {
    resultCard: {
      messages: {
        file_written_successfully: 'File written successfully',
        search_completed: 'Search completed',
        auto_answered_silent_mode: 'Auto-answered (silent mode)',
        screenshot_captured: 'Screenshot captured',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot captured (interactive elements unavailable)',
      },
      messageTemplates: {
        found_results: 'Found {count} results',
        reminder_count: '{count} reminders',
        cleared_reminders: 'Cleared {count} reminders',
        entries_in_path: '{count} entries in {path}',
        single_entry_in_path: '1 entry in {path}',
        no_entries_in_path: 'No entries in {path}',
        showing_first_entries_in_path: 'Showing first {count} entries in {path} (more omitted)',
        screenshot_captured_for: 'Screenshot captured for {target}',
      },
      warnings: {
        listing_truncated: 'Listing was truncated; narrow the path or increase max_entries.',
      },
    },
  },
  'en-US': {
    resultCard: {
      messages: {
        file_written_successfully: 'File written successfully',
        search_completed: 'Search completed',
        auto_answered_silent_mode: 'Auto-answered (silent mode)',
        screenshot_captured: 'Screenshot captured',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot captured (interactive elements unavailable)',
      },
      messageTemplates: {
        found_results: 'Found {count} results',
        reminder_count: '{count} reminders',
        cleared_reminders: 'Cleared {count} reminders',
        entries_in_path: '{count} entries in {path}',
        single_entry_in_path: '1 entry in {path}',
        no_entries_in_path: 'No entries in {path}',
        showing_first_entries_in_path: 'Showing first {count} entries in {path} (more omitted)',
        screenshot_captured_for: 'Screenshot captured for {target}',
      },
      warnings: {
        listing_truncated: 'Listing was truncated; narrow the path or increase max_entries.',
      },
    },
  },
  'es-ES': {
    resultCard: {
      messages: {
        file_written_successfully: 'Archivo escrito correctamente',
        search_completed: 'Búsqueda completada',
        auto_answered_silent_mode: 'Respondido automáticamente (modo silencioso)',
        screenshot_captured: 'Captura de pantalla realizada',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de pantalla realizada (elementos interactivos no disponibles)',
      },
      messageTemplates: {
        found_results: 'Se encontraron {count} resultados',
        reminder_count: '{count} recordatorios',
        cleared_reminders: 'Se borraron {count} recordatorios',
        entries_in_path: '{count} entradas en {path}',
        single_entry_in_path: '1 entrada en {path}',
        no_entries_in_path: 'No hay entradas en {path}',
        showing_first_entries_in_path:
          'Mostrando las primeras {count} entradas de {path} (se omitieron más)',
        screenshot_captured_for: 'Captura de pantalla realizada para {target}',
      },
      warnings: {
        listing_truncated: 'La lista se truncó; acota la ruta o aumenta max_entries.',
      },
    },
  },
  'fr-FR': {
    resultCard: {
      messages: {
        file_written_successfully: 'Fichier écrit avec succès',
        search_completed: 'Recherche terminée',
        auto_answered_silent_mode: 'Réponse automatique envoyée (mode silencieux)',
        screenshot_captured: 'Capture d’écran effectuée',
        screenshot_captured_interactive_elements_unavailable:
          'Capture d’écran effectuée (éléments interactifs indisponibles)',
      },
      messageTemplates: {
        found_results: '{count} résultats trouvés',
        reminder_count: '{count} rappels',
        cleared_reminders: '{count} rappels supprimés',
        entries_in_path: '{count} éléments dans {path}',
        single_entry_in_path: '1 élément dans {path}',
        no_entries_in_path: 'Aucun élément dans {path}',
        showing_first_entries_in_path:
          'Affichage des {count} premiers éléments dans {path} (autres omis)',
        screenshot_captured_for: 'Capture d’écran effectuée pour {target}',
      },
      warnings: {
        listing_truncated: 'La liste a été tronquée ; réduisez le chemin ou augmentez max_entries.',
      },
    },
  },
  'ga-IE': {
    resultCard: {
      messages: {
        file_written_successfully: 'Scríobhadh an comhad go rathúil',
        search_completed: 'Cuardach críochnaithe',
        auto_answered_silent_mode: 'Freagraíodh go huathoibríoch (mód ciúin)',
        screenshot_captured: 'Tógadh seat scáileáin',
        screenshot_captured_interactive_elements_unavailable:
          'Tógadh seat scáileáin (níl eilimintí idirghníomhacha ar fáil)',
      },
      messageTemplates: {
        found_results: 'Fuarthas {count} torthaí',
        reminder_count: '{count} meabhrúcháin',
        cleared_reminders: 'Glanadh {count} meabhrúcháin',
        entries_in_path: '{count} iontráil i {path}',
        single_entry_in_path: '1 iontráil i {path}',
        no_entries_in_path: 'Níl aon iontrálacha i {path}',
        showing_first_entries_in_path:
          'Ag taispeáint na chéad {count} iontráil i {path} (fágadh cinn eile ar lár)',
        screenshot_captured_for: 'Tógadh seat scáileáin do {target}',
      },
      warnings: {
        listing_truncated: 'Gearradh an liosta; caolaigh an cosán nó méadaigh max_entries.',
      },
    },
  },
  'hr-HR': {
    resultCard: {
      messages: {
        file_written_successfully: 'Datoteka je uspješno zapisana',
        search_completed: 'Pretraživanje dovršeno',
        auto_answered_silent_mode: 'Automatski odgovoreno (tihi način)',
        screenshot_captured: 'Snimka zaslona je zabilježena',
        screenshot_captured_interactive_elements_unavailable:
          'Snimka zaslona je zabilježena (interaktivni elementi nisu dostupni)',
      },
      messageTemplates: {
        found_results: 'Pronađeno je {count} rezultata',
        reminder_count: '{count} podsjetnika',
        cleared_reminders: 'Obrisano {count} podsjetnika',
        entries_in_path: '{count} stavki u {path}',
        single_entry_in_path: '1 stavka u {path}',
        no_entries_in_path: 'Nema stavki u {path}',
        showing_first_entries_in_path:
          'Prikazuje se prvih {count} stavki u {path} (ostale su izostavljene)',
        screenshot_captured_for: 'Snimka zaslona zabilježena za {target}',
      },
      warnings: {
        listing_truncated: 'Popis je skraćen; suzite putanju ili povećajte max_entries.',
      },
    },
  },
  'hu-HU': {
    resultCard: {
      messages: {
        file_written_successfully: 'A fájl írása sikeresen befejeződött',
        search_completed: 'Keresés befejezve',
        auto_answered_silent_mode: 'Automatikusan megválaszolva (csendes mód)',
        screenshot_captured: 'Képernyőkép elkészült',
        screenshot_captured_interactive_elements_unavailable:
          'Képernyőkép elkészült (az interaktív elemek nem érhetők el)',
      },
      messageTemplates: {
        found_results: '{count} találat',
        reminder_count: '{count} emlékeztető',
        cleared_reminders: '{count} emlékeztető törölve',
        entries_in_path: '{path} alatt {count} bejegyzés',
        single_entry_in_path: '{path} alatt 1 bejegyzés',
        no_entries_in_path: 'Nincs bejegyzés itt: {path}',
        showing_first_entries_in_path:
          'Az első {count} bejegyzés látható itt: {path} (a többi ki van hagyva)',
        screenshot_captured_for: 'Képernyőkép készült erről: {target}',
      },
      warnings: {
        listing_truncated:
          'A lista csonkítva lett; szűkítse az útvonalat vagy növelje a max_entries értékét.',
      },
    },
  },
  'it-IT': {
    resultCard: {
      messages: {
        file_written_successfully: 'File scritto con successo',
        search_completed: 'Ricerca completata',
        auto_answered_silent_mode: 'Risposta automatica inviata (modalità silenziosa)',
        screenshot_captured: 'Screenshot acquisito',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot acquisito (elementi interattivi non disponibili)',
      },
      messageTemplates: {
        found_results: 'Trovati {count} risultati',
        reminder_count: '{count} promemoria',
        cleared_reminders: 'Cancellati {count} promemoria',
        entries_in_path: '{count} elementi in {path}',
        single_entry_in_path: '1 elemento in {path}',
        no_entries_in_path: 'Nessun elemento in {path}',
        showing_first_entries_in_path: 'Mostra i primi {count} elementi in {path} (altri omessi)',
        screenshot_captured_for: 'Screenshot acquisito per {target}',
      },
      warnings: {
        listing_truncated:
          "L'elenco è stato troncato; restringi il percorso o aumenta max_entries.",
      },
    },
  },
  'ja-JP': {
    resultCard: {
      messages: {
        file_written_successfully: 'ファイルの書き込みが完了しました',
        search_completed: '検索が完了しました',
        auto_answered_silent_mode: '自動回答しました（サイレントモード）',
        screenshot_captured: 'スクリーンショットを取得しました',
        screenshot_captured_interactive_elements_unavailable:
          'スクリーンショットを取得しました（操作要素は利用できません）',
      },
      messageTemplates: {
        found_results: '{count} 件の結果が見つかりました',
        reminder_count: '{count} 件のリマインダー',
        cleared_reminders: '{count} 件のリマインダーを削除しました',
        entries_in_path: '{path} に {count} 件の項目があります',
        single_entry_in_path: '{path} に 1 件の項目があります',
        no_entries_in_path: '{path} に項目はありません',
        showing_first_entries_in_path:
          '{path} の先頭 {count} 件の項目を表示しています（残りは省略）',
        screenshot_captured_for: '{target} のスクリーンショットを取得しました',
      },
      warnings: {
        listing_truncated:
          '一覧が切り詰められました。パスを絞り込むか、max_entries を増やしてください。',
      },
    },
  },
  'ko-KR': {
    resultCard: {
      messages: {
        file_written_successfully: '파일이 성공적으로 저장되었습니다',
        search_completed: '검색 완료',
        auto_answered_silent_mode: '자동 응답됨(무음 모드)',
        screenshot_captured: '스크린샷 캡처됨',
        screenshot_captured_interactive_elements_unavailable:
          '스크린샷 캡처됨(상호작용 요소를 사용할 수 없음)',
      },
      messageTemplates: {
        found_results: '결과 {count}개를 찾았습니다',
        reminder_count: '알림 {count}개',
        cleared_reminders: '알림 {count}개를 지웠습니다',
        entries_in_path: '{path}에 항목 {count}개',
        single_entry_in_path: '{path}에 항목 1개',
        no_entries_in_path: '{path}에 항목이 없습니다',
        showing_first_entries_in_path:
          '{path}의 처음 {count}개 항목을 표시하는 중(더 많은 항목은 생략됨)',
        screenshot_captured_for: '{target}의 스크린샷을 캡처했습니다',
      },
      warnings: {
        listing_truncated: '목록이 잘렸습니다. 경로를 더 좁히거나 max_entries를 늘리세요.',
      },
    },
  },
  'ml-IN': {
    resultCard: {
      messages: {
        file_written_successfully: 'ഫയൽ വിജയകരമായി എഴുതിയിരിക്കുന്നു',
        search_completed: 'തിരയൽ പൂർത്തിയായി',
        auto_answered_silent_mode: 'സ്വയമേവ മറുപടി നൽകി (നിശ്ശബ്ദ മോഡ്)',
        screenshot_captured: 'സ്ക്രീൻഷോട്ട് എടുത്തു',
        screenshot_captured_interactive_elements_unavailable:
          'സ്ക്രീൻഷോട്ട് എടുത്തു (ഇന്ററാക്ടീവ് ഘടകങ്ങൾ ലഭ്യമല്ല)',
      },
      messageTemplates: {
        found_results: '{count} ഫലങ്ങൾ കണ്ടെത്തി',
        reminder_count: '{count} ഓർമ്മിപ്പികൾ',
        cleared_reminders: '{count} ഓർമ്മിപ്പികൾ നീക്കി',
        entries_in_path: '{path} ൽ {count} എൻട്രികൾ',
        single_entry_in_path: '{path} ൽ 1 എൻട്രി',
        no_entries_in_path: '{path} ൽ എൻട്രികളില്ല',
        showing_first_entries_in_path:
          '{path} ൽ ആദ്യ {count} എൻട്രികൾ കാണിക്കുന്നു (കൂടുതൽ ഒഴിവാക്കി)',
        screenshot_captured_for: '{target} ന് വേണ്ടി സ്ക്രീൻഷോട്ട് എടുത്തു',
      },
      warnings: {
        listing_truncated:
          'പട്ടിക മുറിച്ചിരിക്കുന്നു; പാത ചുരുക്കുക അല്ലെങ്കിൽ max_entries വർധിപ്പിക്കുക.',
      },
    },
  },
  'nb-NO': {
    resultCard: {
      messages: {
        file_written_successfully: 'Filen ble skrevet',
        search_completed: 'Søk fullført',
        auto_answered_silent_mode: 'Automatisk besvart (stille modus)',
        screenshot_captured: 'Skjermbilde tatt',
        screenshot_captured_interactive_elements_unavailable:
          'Skjermbilde tatt (interaktive elementer er ikke tilgjengelige)',
      },
      messageTemplates: {
        found_results: 'Fant {count} resultater',
        reminder_count: '{count} påminnelser',
        cleared_reminders: 'Fjernet {count} påminnelser',
        entries_in_path: '{count} oppføringer i {path}',
        single_entry_in_path: '1 oppføring i {path}',
        no_entries_in_path: 'Ingen oppføringer i {path}',
        showing_first_entries_in_path:
          'Viser de første {count} oppføringene i {path} (flere utelatt)',
        screenshot_captured_for: 'Skjermbilde tatt for {target}',
      },
      warnings: {
        listing_truncated: 'Listen ble avkortet; snevre inn stien eller øk max_entries.',
      },
    },
  },
  'nl-NL': {
    resultCard: {
      messages: {
        file_written_successfully: 'Bestand succesvol weggeschreven',
        search_completed: 'Zoeken voltooid',
        auto_answered_silent_mode: 'Automatisch beantwoord (stille modus)',
        screenshot_captured: 'Screenshot gemaakt',
        screenshot_captured_interactive_elements_unavailable:
          'Screenshot gemaakt (interactieve elementen niet beschikbaar)',
      },
      messageTemplates: {
        found_results: '{count} resultaten gevonden',
        reminder_count: '{count} herinneringen',
        cleared_reminders: '{count} herinneringen verwijderd',
        entries_in_path: '{count} items in {path}',
        single_entry_in_path: '1 item in {path}',
        no_entries_in_path: 'Geen items in {path}',
        showing_first_entries_in_path:
          'De eerste {count} items in {path} worden getoond (meer weggelaten)',
        screenshot_captured_for: 'Screenshot gemaakt voor {target}',
      },
      warnings: {
        listing_truncated: 'De lijst is afgekapt; beperk het pad of verhoog max_entries.',
      },
    },
  },
  'pl-PL': {
    resultCard: {
      messages: {
        file_written_successfully: 'Plik został pomyślnie zapisany',
        search_completed: 'Wyszukiwanie zakończone',
        auto_answered_silent_mode: 'Odpowiedziano automatycznie (tryb cichy)',
        screenshot_captured: 'Zrzut ekranu został wykonany',
        screenshot_captured_interactive_elements_unavailable:
          'Zrzut ekranu został wykonany (elementy interaktywne są niedostępne)',
      },
      messageTemplates: {
        found_results: 'Znaleziono {count} wyników',
        reminder_count: '{count} przypomnień',
        cleared_reminders: 'Usunięto {count} przypomnień',
        entries_in_path: '{count} elementów w {path}',
        single_entry_in_path: '1 element w {path}',
        no_entries_in_path: 'Brak elementów w {path}',
        showing_first_entries_in_path:
          'Wyświetlanie pierwszych {count} elementów w {path} (pozostałe pominięto)',
        screenshot_captured_for: 'Zrzut ekranu został wykonany dla {target}',
      },
      warnings: {
        listing_truncated: 'Lista została skrócona; zawęź ścieżkę lub zwiększ max_entries.',
      },
    },
  },
  'pt-BR': {
    resultCard: {
      messages: {
        file_written_successfully: 'Arquivo gravado com sucesso',
        search_completed: 'Pesquisa concluída',
        auto_answered_silent_mode: 'Respondido automaticamente (modo silencioso)',
        screenshot_captured: 'Captura de tela realizada',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de tela realizada (elementos interativos indisponíveis)',
      },
      messageTemplates: {
        found_results: 'Encontrados {count} resultados',
        reminder_count: '{count} lembretes',
        cleared_reminders: '{count} lembretes removidos',
        entries_in_path: '{count} entradas em {path}',
        single_entry_in_path: '1 entrada em {path}',
        no_entries_in_path: 'Nenhuma entrada em {path}',
        showing_first_entries_in_path:
          'Mostrando as primeiras {count} entradas em {path} (outras omitidas)',
        screenshot_captured_for: 'Captura de tela realizada para {target}',
      },
      warnings: {
        listing_truncated: 'A listagem foi truncada; restrinja o caminho ou aumente max_entries.',
      },
    },
  },
  'pt-PT': {
    resultCard: {
      messages: {
        file_written_successfully: 'Ficheiro escrito com sucesso',
        search_completed: 'Pesquisa concluída',
        auto_answered_silent_mode: 'Respondido automaticamente (modo silencioso)',
        screenshot_captured: 'Captura de ecrã efetuada',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de ecrã efetuada (elementos interativos indisponíveis)',
      },
      messageTemplates: {
        found_results: 'Encontrados {count} resultados',
        reminder_count: '{count} lembretes',
        cleared_reminders: '{count} lembretes removidos',
        entries_in_path: '{count} entradas em {path}',
        single_entry_in_path: '1 entrada em {path}',
        no_entries_in_path: 'Sem entradas em {path}',
        showing_first_entries_in_path:
          'A mostrar as primeiras {count} entradas em {path} (outras omitidas)',
        screenshot_captured_for: 'Captura de ecrã efetuada para {target}',
      },
      warnings: {
        listing_truncated: 'A listagem foi truncada; restrinja o caminho ou aumente max_entries.',
      },
    },
  },
  'ro-RO': {
    resultCard: {
      messages: {
        file_written_successfully: 'Fișierul a fost scris cu succes',
        search_completed: 'Căutarea s-a încheiat',
        auto_answered_silent_mode: 'S-a răspuns automat (mod silențios)',
        screenshot_captured: 'Captura de ecran a fost realizată',
        screenshot_captured_interactive_elements_unavailable:
          'Captura de ecran a fost realizată (elementele interactive nu sunt disponibile)',
      },
      messageTemplates: {
        found_results: 'Au fost găsite {count} rezultate',
        reminder_count: '{count} mementouri',
        cleared_reminders: 'Au fost șterse {count} mementouri',
        entries_in_path: '{count} intrări în {path}',
        single_entry_in_path: '1 intrare în {path}',
        no_entries_in_path: 'Nu există intrări în {path}',
        showing_first_entries_in_path:
          'Se afișează primele {count} intrări din {path} (restul au fost omise)',
        screenshot_captured_for: 'Captura de ecran a fost realizată pentru {target}',
      },
      warnings: {
        listing_truncated: 'Lista a fost trunchiată; restrânge calea sau mărește max_entries.',
      },
    },
  },
  'ru-RU': {
    resultCard: {
      messages: {
        file_written_successfully: 'Файл успешно записан',
        search_completed: 'Поиск завершён',
        auto_answered_silent_mode: 'Ответ дан автоматически (тихий режим)',
        screenshot_captured: 'Снимок экрана сделан',
        screenshot_captured_interactive_elements_unavailable:
          'Снимок экрана сделан (интерактивные элементы недоступны)',
      },
      messageTemplates: {
        found_results: 'Найдено {count} результатов',
        reminder_count: '{count} напоминаний',
        cleared_reminders: 'Очищено {count} напоминаний',
        entries_in_path: '{count} элементов в {path}',
        single_entry_in_path: '1 элемент в {path}',
        no_entries_in_path: 'В {path} нет элементов',
        showing_first_entries_in_path:
          'Показаны первые {count} элементов в {path} (остальные скрыты)',
        screenshot_captured_for: 'Сделан снимок экрана для {target}',
      },
      warnings: {
        listing_truncated: 'Список был усечён; сузьте путь или увеличьте max_entries.',
      },
    },
  },
  'sk-SK': {
    resultCard: {
      messages: {
        file_written_successfully: 'Súbor bol úspešne zapísaný',
        search_completed: 'Vyhľadávanie dokončené',
        auto_answered_silent_mode: 'Zodpovedané automaticky (tichý režim)',
        screenshot_captured: 'Snímka obrazovky bola vytvorená',
        screenshot_captured_interactive_elements_unavailable:
          'Snímka obrazovky bola vytvorená (interaktívne prvky nie sú k dispozícii)',
      },
      messageTemplates: {
        found_results: 'Nájdených {count} výsledkov',
        reminder_count: '{count} pripomienok',
        cleared_reminders: 'Vymazaných {count} pripomienok',
        entries_in_path: '{count} položiek v {path}',
        single_entry_in_path: '1 položka v {path}',
        no_entries_in_path: 'V {path} nie sú žiadne položky',
        showing_first_entries_in_path:
          'Zobrazuje sa prvých {count} položiek v {path} (ďalšie boli vynechané)',
        screenshot_captured_for: 'Snímka obrazovky bola vytvorená pre {target}',
      },
      warnings: {
        listing_truncated: 'Zoznam bol skrátený; zúžte cestu alebo zvýšte max_entries.',
      },
    },
  },
  'sv-SE': {
    resultCard: {
      messages: {
        file_written_successfully: 'Filen skrevs',
        search_completed: 'Sökning slutförd',
        auto_answered_silent_mode: 'Besvarades automatiskt (tyst läge)',
        screenshot_captured: 'Skärmbild tagen',
        screenshot_captured_interactive_elements_unavailable:
          'Skärmbild tagen (interaktiva element är inte tillgängliga)',
      },
      messageTemplates: {
        found_results: 'Hittade {count} resultat',
        reminder_count: '{count} påminnelser',
        cleared_reminders: 'Rensade {count} påminnelser',
        entries_in_path: '{count} poster i {path}',
        single_entry_in_path: '1 post i {path}',
        no_entries_in_path: 'Inga poster i {path}',
        showing_first_entries_in_path:
          'Visar de första {count} posterna i {path} (fler utelämnade)',
        screenshot_captured_for: 'Skärmbild tagen för {target}',
      },
      warnings: {
        listing_truncated: 'Listan trunkerades; begränsa sökvägen eller öka max_entries.',
      },
    },
  },
  'zh-CN': {
    resultCard: {
      messages: {
        file_written_successfully: '文件写入成功',
        search_completed: '搜索完成',
        auto_answered_silent_mode: '已自动回答（静默模式）',
        screenshot_captured: '已捕获截图',
        screenshot_captured_interactive_elements_unavailable: '已捕获截图（交互元素不可用）',
      },
      messageTemplates: {
        found_results: '找到 {count} 条结果',
        reminder_count: '{count} 个提醒',
        cleared_reminders: '已清除 {count} 个提醒',
        entries_in_path: '{path} 中有 {count} 个条目',
        single_entry_in_path: '{path} 中有 1 个条目',
        no_entries_in_path: '{path} 中没有条目',
        showing_first_entries_in_path: '显示 {path} 中前 {count} 个条目（更多已省略）',
        screenshot_captured_for: '已为 {target} 捕获截图',
      },
      warnings: {
        listing_truncated: '列表已截断；请缩小路径范围或增大 max_entries。',
      },
    },
  },
  'zh-TW': {
    resultCard: {
      messages: {
        file_written_successfully: '檔案寫入成功',
        search_completed: '搜尋完成',
        auto_answered_silent_mode: '已自動回答（靜默模式）',
        screenshot_captured: '已擷取截圖',
        screenshot_captured_interactive_elements_unavailable: '已擷取截圖（互動元素無法使用）',
      },
      messageTemplates: {
        found_results: '找到 {count} 筆結果',
        reminder_count: '{count} 個提醒',
        cleared_reminders: '已清除 {count} 個提醒',
        entries_in_path: '{path} 中有 {count} 個項目',
        single_entry_in_path: '{path} 中有 1 個項目',
        no_entries_in_path: '{path} 中沒有項目',
        showing_first_entries_in_path: '顯示 {path} 中前 {count} 個項目（更多已省略）',
        screenshot_captured_for: '已為 {target} 擷取截圖',
      },
      warnings: {
        listing_truncated: '清單已截斷；請縮小路徑範圍或提高 max_entries。',
      },
    },
  },
} as const
