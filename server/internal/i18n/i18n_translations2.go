package i18n

func init() {
	// --- pl-PL (Polish) ---
	for k, v := range map[string]string{
		MsgProcessingError:     "Przepraszamy, wystąpił błąd podczas przetwarzania wiadomości: %v",
		MsgChannelNotConnected: "Kanał nie jest połączony. Spróbuj ponownie później.",
		MsgTimeout:             "Upłynął limit czasu żądania. Spróbuj ponownie.",
		MsgRateLimited:         "Zbyt wiele żądań. Poczekaj chwilę i spróbuj ponownie.",
		MsgServiceUnavailable:  "Usługa jest tymczasowo niedostępna. Spróbuj ponownie później.",
		MsgInvalidRequest:      "Nieprawidłowe żądanie. Sprawdź dane wejściowe i spróbuj ponownie.",
		MsgUnauthorized:        "Nie masz uprawnień do wykonania tej czynności.",
		MsgInternalError:       "Wystąpił błąd wewnętrzny. Spróbuj ponownie później.",
		MsgNoProviderAvailable: "Brak dostępnego dostawcy usług AI. Sprawdź konfigurację lub skontaktuj się z administratorem.",
		MsgProvidersInCooldown: "Brak dostępnego dostawcy usług AI (%d dostawców w trybie oczekiwania). Spróbuj ponownie później.",
		MsgMediaGenerating:     "🎨 Generowanie multimediów... Wyślę wynik, gdy będzie gotowy.",
		MsgMediaGenFailed:      "❌ Generowanie multimediów nie powiodło się: %s",
		MsgMediaGenCancelled:   "🚫 Generowanie multimediów zostało anulowane.",
		MsgMediaGenNoOutput:    "✅ Generowanie zakończone, ale nie zwrócono wyniku.",
		MsgMediaImageGenerated: "✅ Obraz wygenerowany (%s)",
		MsgMediaVideoGenerated: "✅ Wideo wygenerowane (%s)",
		MsgMediaGenerated:      "✅ Multimedia wygenerowane (%s)",
		MsgMediaGenTimeout:     "⏰ Upłynął limit czasu generowania multimediów. Spróbuj ponownie.",
	} {
		AddTranslation(LangPlPL, k, v)
	}

	// --- nl-NL (Dutch) ---
	for k, v := range map[string]string{
		MsgProcessingError:     "Sorry, er is een fout opgetreden bij het verwerken van uw bericht: %v",
		MsgChannelNotConnected: "Het kanaal is niet verbonden. Probeer het later opnieuw.",
		MsgTimeout:             "Het verzoek is verlopen. Probeer het opnieuw.",
		MsgRateLimited:         "Te veel verzoeken. Wacht even en probeer het opnieuw.",
		MsgServiceUnavailable:  "De service is tijdelijk niet beschikbaar. Probeer het later opnieuw.",
		MsgInvalidRequest:      "Ongeldig verzoek. Controleer uw invoer en probeer het opnieuw.",
		MsgUnauthorized:        "U bent niet gemachtigd om deze actie uit te voeren.",
		MsgInternalError:       "Er is een interne fout opgetreden. Probeer het later opnieuw.",
		MsgNoProviderAvailable: "Geen AI-serviceprovider beschikbaar. Controleer de configuratie of neem contact op met de beheerder.",
		MsgProvidersInCooldown: "Momenteel geen AI-serviceprovider beschikbaar (%d providers in afkoelperiode). Probeer het later opnieuw.",
		MsgMediaGenerating:     "🎨 Media wordt gegenereerd... Ik stuur het resultaat zodra het klaar is.",
		MsgMediaGenFailed:      "❌ Mediageneratie mislukt: %s",
		MsgMediaGenCancelled:   "🚫 Mediageneratie is geannuleerd.",
		MsgMediaGenNoOutput:    "✅ Generatie voltooid, maar geen resultaat ontvangen.",
		MsgMediaImageGenerated: "✅ Afbeelding gegenereerd (%s)",
		MsgMediaVideoGenerated: "✅ Video gegenereerd (%s)",
		MsgMediaGenerated:      "✅ Media gegenereerd (%s)",
		MsgMediaGenTimeout:     "⏰ Mediageneratie verlopen. Probeer het opnieuw.",
	} {
		AddTranslation(LangNlNL, k, v)
	}

	// --- sv-SE (Swedish) ---
	for k, v := range map[string]string{
		MsgProcessingError:     "Tyvärr uppstod ett fel vid bearbetning av ditt meddelande: %v",
		MsgChannelNotConnected: "Kanalen är inte ansluten. Försök igen senare.",
		MsgTimeout:             "Begäran tog för lång tid. Försök igen.",
		MsgRateLimited:         "För många förfrågningar. Vänta en stund och försök igen.",
		MsgServiceUnavailable:  "Tjänsten är tillfälligt otillgänglig. Försök igen senare.",
		MsgInvalidRequest:      "Ogiltig begäran. Kontrollera din inmatning och försök igen.",
		MsgUnauthorized:        "Du har inte behörighet att utföra denna åtgärd.",
		MsgInternalError:       "Ett internt fel uppstod. Försök igen senare.",
		MsgNoProviderAvailable: "Ingen AI-tjänsteleverantör tillgänglig. Kontrollera konfigurationen eller kontakta administratören.",
		MsgProvidersInCooldown: "Ingen AI-tjänsteleverantör tillgänglig just nu (%d leverantörer i nedkylning). Försök igen senare.",
		MsgMediaGenerating:     "🎨 Genererar media... Jag skickar resultatet när det är klart.",
		MsgMediaGenFailed:      "❌ Mediagenerering misslyckades: %s",
		MsgMediaGenCancelled:   "🚫 Mediagenereringen avbröts.",
		MsgMediaGenNoOutput:    "✅ Generering klar, men inget resultat returnerades.",
		MsgMediaImageGenerated: "✅ Bild genererad (%s)",
		MsgMediaVideoGenerated: "✅ Video genererad (%s)",
		MsgMediaGenerated:      "✅ Media genererad (%s)",
		MsgMediaGenTimeout:     "⏰ Mediagenerering tog för lång tid. Försök igen.",
	} {
		AddTranslation(LangSvSE, k, v)
	}

	// --- da-DK (Danish) ---
	for k, v := range map[string]string{
		MsgProcessingError:     "Beklager, der opstod en fejl under behandling af din besked: %v",
		MsgChannelNotConnected: "Kanalen er ikke forbundet. Prøv igen senere.",
		MsgTimeout:             "Anmodningen udløb. Prøv igen.",
		MsgRateLimited:         "For mange anmodninger. Vent et øjeblik og prøv igen.",
		MsgServiceUnavailable:  "Tjenesten er midlertidigt utilgængelig. Prøv igen senere.",
		MsgInvalidRequest:      "Ugyldig anmodning. Kontrollér dit input og prøv igen.",
		MsgUnauthorized:        "Du har ikke tilladelse til at udføre denne handling.",
		MsgInternalError:       "Der opstod en intern fejl. Prøv igen senere.",
		MsgNoProviderAvailable: "Ingen AI-tjenesteudbyder tilgængelig. Kontrollér konfigurationen eller kontakt administratoren.",
		MsgProvidersInCooldown: "Ingen AI-tjenesteudbyder tilgængelig i øjeblikket (%d udbydere i afkøling). Prøv igen senere.",
		MsgMediaGenerating:     "🎨 Genererer medie... Jeg sender resultatet, når det er klar.",
		MsgMediaGenFailed:      "❌ Mediegenerering mislykkedes: %s",
		MsgMediaGenCancelled:   "🚫 Mediegenereringen blev annulleret.",
		MsgMediaGenNoOutput:    "✅ Generering fuldført, men intet resultat returneret.",
		MsgMediaImageGenerated: "✅ Billede genereret (%s)",
		MsgMediaVideoGenerated: "✅ Video genereret (%s)",
		MsgMediaGenerated:      "✅ Medie genereret (%s)",
		MsgMediaGenTimeout:     "⏰ Mediegenerering udløb. Prøv igen.",
	} {
		AddTranslation(LangDaDK, k, v)
	}

	// --- nb-NO (Norwegian Bokmål) ---
	for k, v := range map[string]string{
		MsgProcessingError:     "Beklager, det oppstod en feil under behandling av meldingen din: %v",
		MsgChannelNotConnected: "Kanalen er ikke tilkoblet. Prøv igjen senere.",
		MsgTimeout:             "Forespørselen ble tidsavbrutt. Prøv igjen.",
		MsgRateLimited:         "For mange forespørsler. Vent litt og prøv igjen.",
		MsgServiceUnavailable:  "Tjenesten er midlertidig utilgjengelig. Prøv igjen senere.",
		MsgInvalidRequest:      "Ugyldig forespørsel. Sjekk inndataene dine og prøv igjen.",
		MsgUnauthorized:        "Du har ikke tillatelse til å utføre denne handlingen.",
		MsgInternalError:       "Det oppstod en intern feil. Prøv igjen senere.",
		MsgNoProviderAvailable: "Ingen AI-tjenesteleverandør tilgjengelig. Sjekk konfigurasjonen eller kontakt administratoren.",
		MsgProvidersInCooldown: "Ingen AI-tjenesteleverandør tilgjengelig for øyeblikket (%d leverandører i nedkjøling). Prøv igjen senere.",
		MsgMediaGenerating:     "🎨 Genererer media... Jeg sender resultatet når det er klart.",
		MsgMediaGenFailed:      "❌ Mediagenerering mislyktes: %s",
		MsgMediaGenCancelled:   "🚫 Mediagenereringen ble avbrutt.",
		MsgMediaGenNoOutput:    "✅ Generering fullført, men ingen resultater returnert.",
		MsgMediaImageGenerated: "✅ Bilde generert (%s)",
		MsgMediaVideoGenerated: "✅ Video generert (%s)",
		MsgMediaGenerated:      "✅ Media generert (%s)",
		MsgMediaGenTimeout:     "⏰ Mediagenerering ble tidsavbrutt. Prøv igjen.",
	} {
		AddTranslation(LangNbNO, k, v)
	}

	// --- cs-CZ (Czech) ---
	for k, v := range map[string]string{
		MsgProcessingError:     "Omlouváme se, při zpracování vaší zprávy došlo k chybě: %v",
		MsgChannelNotConnected: "Kanál není připojen. Zkuste to prosím později.",
		MsgTimeout:             "Požadavek vypršel. Zkuste to prosím znovu.",
		MsgRateLimited:         "Příliš mnoho požadavků. Počkejte chvíli a zkuste to znovu.",
		MsgServiceUnavailable:  "Služba je dočasně nedostupná. Zkuste to prosím později.",
		MsgInvalidRequest:      "Neplatný požadavek. Zkontrolujte svůj vstup a zkuste to znovu.",
		MsgUnauthorized:        "Nemáte oprávnění k provedení této akce.",
		MsgInternalError:       "Došlo k interní chybě. Zkuste to prosím později.",
		MsgNoProviderAvailable: "Žádný poskytovatel AI služeb není dostupný. Zkontrolujte konfiguraci nebo kontaktujte správce.",
		MsgProvidersInCooldown: "Žádný poskytovatel AI služeb není momentálně dostupný (%d poskytovatelů v režimu čekání). Zkuste to prosím později.",
		MsgMediaGenerating:     "🎨 Generuji média... Výsledek vám pošlu, až bude hotový.",
		MsgMediaGenFailed:      "❌ Generování médií selhalo: %s",
		MsgMediaGenCancelled:   "🚫 Generování médií bylo zrušeno.",
		MsgMediaGenNoOutput:    "✅ Generování dokončeno, ale nebyl vrácen žádný výsledek.",
		MsgMediaImageGenerated: "✅ Obrázek vygenerován (%s)",
		MsgMediaVideoGenerated: "✅ Video vygenerováno (%s)",
		MsgMediaGenerated:      "✅ Médium vygenerováno (%s)",
		MsgMediaGenTimeout:     "⏰ Generování médií vypršelo. Zkuste to prosím znovu.",
	} {
		AddTranslation(LangCsCZ, k, v)
	}

	// --- sk-SK (Slovak) ---
	for k, v := range map[string]string{
		MsgProcessingError:     "Ospravedlňujeme sa, pri spracovaní vašej správy došlo k chybe: %v",
		MsgChannelNotConnected: "Kanál nie je pripojený. Skúste to prosím neskôr.",
		MsgTimeout:             "Požiadavka vypršala. Skúste to prosím znova.",
		MsgRateLimited:         "Príliš veľa požiadaviek. Počkajte chvíľu a skúste to znova.",
		MsgServiceUnavailable:  "Služba je dočasne nedostupná. Skúste to prosím neskôr.",
		MsgInvalidRequest:      "Neplatná požiadavka. Skontrolujte svoj vstup a skúste to znova.",
		MsgUnauthorized:        "Nemáte oprávnenie na vykonanie tejto akcie.",
		MsgInternalError:       "Došlo k internej chybe. Skúste to prosím neskôr.",
		MsgNoProviderAvailable: "Žiadny poskytovateľ AI služieb nie je dostupný. Skontrolujte konfiguráciu alebo kontaktujte správcu.",
		MsgProvidersInCooldown: "Žiadny poskytovateľ AI služieb nie je momentálne dostupný (%d poskytovateľov v režime čakania). Skúste to prosím neskôr.",
		MsgMediaGenerating:     "🎨 Generujem médiá... Výsledok vám pošlem, keď bude hotový.",
		MsgMediaGenFailed:      "❌ Generovanie médií zlyhalo: %s",
		MsgMediaGenCancelled:   "🚫 Generovanie médií bolo zrušené.",
		MsgMediaGenNoOutput:    "✅ Generovanie dokončené, ale nebol vrátený žiadny výsledok.",
		MsgMediaImageGenerated: "✅ Obrázok vygenerovaný (%s)",
		MsgMediaVideoGenerated: "✅ Video vygenerované (%s)",
		MsgMediaGenerated:      "✅ Médium vygenerované (%s)",
		MsgMediaGenTimeout:     "⏰ Generovanie médií vypršalo. Skúste to prosím znova.",
	} {
		AddTranslation(LangSkSK, k, v)
	}

	// --- hu-HU (Hungarian) ---
	for k, v := range map[string]string{
		MsgProcessingError:     "Sajnáljuk, hiba történt az üzenet feldolgozása során: %v",
		MsgChannelNotConnected: "A csatorna nincs csatlakoztatva. Kérjük, próbálja újra később.",
		MsgTimeout:             "A kérés időtúllépés miatt megszakadt. Kérjük, próbálja újra.",
		MsgRateLimited:         "Túl sok kérés. Kérjük, várjon egy pillanatot és próbálja újra.",
		MsgServiceUnavailable:  "A szolgáltatás átmenetileg nem érhető el. Kérjük, próbálja újra később.",
		MsgInvalidRequest:      "Érvénytelen kérés. Kérjük, ellenőrizze a bevitelt és próbálja újra.",
		MsgUnauthorized:        "Nincs jogosultsága a művelet végrehajtásához.",
		MsgInternalError:       "Belső hiba történt. Kérjük, próbálja újra később.",
		MsgNoProviderAvailable: "Nem érhető el AI szolgáltató. Kérjük, ellenőrizze a konfigurációt vagy lépjen kapcsolatba az adminisztrátorral.",
		MsgProvidersInCooldown: "Jelenleg nem érhető el AI szolgáltató (%d szolgáltató lehűlési módban). Kérjük, próbálja újra később.",
		MsgMediaGenerating:     "🎨 Média generálása folyamatban... Elküldöm az eredményt, ha kész.",
		MsgMediaGenFailed:      "❌ Média generálása sikertelen: %s",
		MsgMediaGenCancelled:   "🚫 A média generálása megszakítva.",
		MsgMediaGenNoOutput:    "✅ Generálás befejezve, de nem érkezett eredmény.",
		MsgMediaImageGenerated: "✅ Kép generálva (%s)",
		MsgMediaVideoGenerated: "✅ Videó generálva (%s)",
		MsgMediaGenerated:      "✅ Média generálva (%s)",
		MsgMediaGenTimeout:     "⏰ Média generálása időtúllépés. Kérjük, próbálja újra.",
	} {
		AddTranslation(LangHuHU, k, v)
	}
}
