package i18n

func init() {
	// --- ro-RO (Romanian) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Ne pare rău, a apărut o eroare la procesarea mesajului dvs.: %v",
		MsgChannelNotConnected:      "Canalul nu este conectat. Vă rugăm să încercați din nou mai târziu.",
		MsgTimeout:                  "Cererea a expirat. Vă rugăm să încercați din nou.",
		MsgRateLimited:              "Prea multe cereri. Vă rugăm să așteptați un moment și să încercați din nou.",
		MsgServiceUnavailable:       "Serviciul este temporar indisponibil. Vă rugăm să încercați din nou mai târziu.",
		MsgInvalidRequest:           "Cerere invalidă. Vă rugăm să verificați datele introduse și să încercați din nou.",
		MsgUnauthorized:             "Nu sunteți autorizat să efectuați această acțiune.",
		MsgInternalError:            "A apărut o eroare internă. Vă rugăm să încercați din nou mai târziu.",
		MsgNoProviderAvailable:      "Niciun furnizor de servicii AI disponibil. Verificați configurația sau contactați administratorul.",
		MsgProvidersInCooldown:      "Niciun furnizor de servicii AI disponibil momentan (%d furnizori în răcire). Vă rugăm să încercați din nou mai târziu.",
		MsgPathEscapesWorkspaceRoot: "Calea este în afara rădăcinii spațiului de lucru",
		MsgMediaGenerating:          "🎨 Se generează media... Voi trimite rezultatul când va fi gata.",
		MsgMediaGenFailed:           "❌ Generarea media a eșuat: %s",
		MsgMediaGenCancelled:        "🚫 Generarea media a fost anulată.",
		MsgMediaGenNoOutput:         "✅ Generare finalizată, dar nu a fost returnat niciun rezultat.",
		MsgMediaImageGenerated:      "✅ Imagine generată (%s)",
		MsgMediaVideoGenerated:      "✅ Video generat (%s)",
		MsgMediaGenerated:           "✅ Media generată (%s)",
		MsgMediaGenTimeout:          "⏰ Generarea media a expirat. Vă rugăm să încercați din nou.",
	} {
		AddTranslation(LangRoRO, k, v)
	}

	// --- hr-HR (Croatian) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Žao nam je, došlo je do pogreške pri obradi vaše poruke: %v",
		MsgChannelNotConnected:      "Kanal nije povezan. Pokušajte ponovo kasnije.",
		MsgTimeout:                  "Zahtjev je istekao. Pokušajte ponovo.",
		MsgRateLimited:              "Previše zahtjeva. Pričekajte trenutak i pokušajte ponovo.",
		MsgServiceUnavailable:       "Usluga je privremeno nedostupna. Pokušajte ponovo kasnije.",
		MsgInvalidRequest:           "Nevažeći zahtjev. Provjerite svoj unos i pokušajte ponovo.",
		MsgUnauthorized:             "Nemate ovlaštenje za izvršavanje ove radnje.",
		MsgInternalError:            "Došlo je do interne pogreške. Pokušajte ponovo kasnije.",
		MsgNoProviderAvailable:      "Nema dostupnog pružatelja AI usluga. Provjerite konfiguraciju ili kontaktirajte administratora.",
		MsgProvidersInCooldown:      "Trenutno nema dostupnog pružatelja AI usluga (%d pružatelja u hlađenju). Pokušajte ponovo kasnije.",
		MsgPathEscapesWorkspaceRoot: "Putanja je izvan korijenske mape radnog prostora",
		MsgMediaGenerating:          "🎨 Generiranje medija u tijeku... Poslat ću rezultat kad bude gotov.",
		MsgMediaGenFailed:           "❌ Generiranje medija nije uspjelo: %s",
		MsgMediaGenCancelled:        "🚫 Generiranje medija je otkazano.",
		MsgMediaGenNoOutput:         "✅ Generiranje završeno, ali nije vraćen rezultat.",
		MsgMediaImageGenerated:      "✅ Slika generirana (%s)",
		MsgMediaVideoGenerated:      "✅ Video generiran (%s)",
		MsgMediaGenerated:           "✅ Medij generiran (%s)",
		MsgMediaGenTimeout:          "⏰ Generiranje medija je isteklo. Pokušajte ponovo.",
	} {
		AddTranslation(LangHrHR, k, v)
	}

	// --- el-GR (Greek) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Λυπούμαστε, παρουσιάστηκε σφάλμα κατά την επεξεργασία του μηνύματός σας: %v",
		MsgChannelNotConnected:      "Το κανάλι δεν είναι συνδεδεμένο. Δοκιμάστε ξανά αργότερα.",
		MsgTimeout:                  "Το αίτημα έληξε. Δοκιμάστε ξανά.",
		MsgRateLimited:              "Πάρα πολλά αιτήματα. Περιμένετε λίγο και δοκιμάστε ξανά.",
		MsgServiceUnavailable:       "Η υπηρεσία είναι προσωρινά μη διαθέσιμη. Δοκιμάστε ξανά αργότερα.",
		MsgInvalidRequest:           "Μη έγκυρο αίτημα. Ελέγξτε την εισαγωγή σας και δοκιμάστε ξανά.",
		MsgUnauthorized:             "Δεν έχετε εξουσιοδότηση για αυτήν την ενέργεια.",
		MsgInternalError:            "Παρουσιάστηκε εσωτερικό σφάλμα. Δοκιμάστε ξανά αργότερα.",
		MsgNoProviderAvailable:      "Δεν υπάρχει διαθέσιμος πάροχος υπηρεσιών AI. Ελέγξτε τη διαμόρφωση ή επικοινωνήστε με τον διαχειριστή.",
		MsgProvidersInCooldown:      "Δεν υπάρχει διαθέσιμος πάροχος υπηρεσιών AI αυτή τη στιγμή (%d πάροχοι σε αναμονή). Δοκιμάστε ξανά αργότερα.",
		MsgPathEscapesWorkspaceRoot: "Η διαδρομή είναι εκτός του ριζικού καταλόγου του χώρου εργασίας",
		MsgMediaGenerating:          "🎨 Δημιουργία πολυμέσων... Θα σας στείλω το αποτέλεσμα όταν είναι έτοιμο.",
		MsgMediaGenFailed:           "❌ Η δημιουργία πολυμέσων απέτυχε: %s",
		MsgMediaGenCancelled:        "🚫 Η δημιουργία πολυμέσων ακυρώθηκε.",
		MsgMediaGenNoOutput:         "✅ Η δημιουργία ολοκληρώθηκε, αλλά δεν επιστράφηκε αποτέλεσμα.",
		MsgMediaImageGenerated:      "✅ Εικόνα δημιουργήθηκε (%s)",
		MsgMediaVideoGenerated:      "✅ Βίντεο δημιουργήθηκε (%s)",
		MsgMediaGenerated:           "✅ Πολυμέσο δημιουργήθηκε (%s)",
		MsgMediaGenTimeout:          "⏰ Η δημιουργία πολυμέσων έληξε. Δοκιμάστε ξανά.",
	} {
		AddTranslation(LangElGR, k, v)
	}

	// --- ca-ES (Catalan) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Ho sentim, s'ha produït un error en processar el vostre missatge: %v",
		MsgChannelNotConnected:      "El canal no està connectat. Torneu-ho a provar més tard.",
		MsgTimeout:                  "La sol·licitud ha expirat. Torneu-ho a provar.",
		MsgRateLimited:              "Massa sol·licituds. Espereu un moment i torneu-ho a provar.",
		MsgServiceUnavailable:       "El servei no està disponible temporalment. Torneu-ho a provar més tard.",
		MsgInvalidRequest:           "Sol·licitud no vàlida. Comproveu la vostra entrada i torneu-ho a provar.",
		MsgUnauthorized:             "No teniu autorització per realitzar aquesta acció.",
		MsgInternalError:            "S'ha produït un error intern. Torneu-ho a provar més tard.",
		MsgNoProviderAvailable:      "No hi ha cap proveïdor de serveis d'IA disponible. Comproveu la configuració o contacteu amb l'administrador.",
		MsgProvidersInCooldown:      "No hi ha cap proveïdor de serveis d'IA disponible en aquest moment (%d proveïdors en refredament). Torneu-ho a provar més tard.",
		MsgPathEscapesWorkspaceRoot: "El camí surt de l'arrel de l'espai de treball",
		MsgMediaGenerating:          "🎨 Generant contingut multimèdia... Us enviaré el resultat quan estigui llest.",
		MsgMediaGenFailed:           "❌ Error en la generació de multimèdia: %s",
		MsgMediaGenCancelled:        "🚫 La generació de multimèdia s'ha cancel·lat.",
		MsgMediaGenNoOutput:         "✅ Generació completada, però no s'ha obtingut cap resultat.",
		MsgMediaImageGenerated:      "✅ Imatge generada (%s)",
		MsgMediaVideoGenerated:      "✅ Vídeo generat (%s)",
		MsgMediaGenerated:           "✅ Multimèdia generat (%s)",
		MsgMediaGenTimeout:          "⏰ La generació de multimèdia ha expirat. Torneu-ho a provar.",
	} {
		AddTranslation(LangCaES, k, v)
	}

	// --- ga-IE (Irish) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "Ár leithscéal, tharla earráid agus do theachtaireacht á próiseáil: %v",
		MsgChannelNotConnected:      "Níl an cainéal ceangailte. Bain triail as arís níos déanaí.",
		MsgTimeout:                  "Chuaigh an t-iarratas thar am. Bain triail as arís.",
		MsgRateLimited:              "An iomarca iarratas. Fan nóiméad agus bain triail as arís.",
		MsgServiceUnavailable:       "Tá an tseirbhís gan fáil go sealadach. Bain triail as arís níos déanaí.",
		MsgInvalidRequest:           "Iarratas neamhbhailí. Seiceáil d'ionchur agus bain triail as arís.",
		MsgUnauthorized:             "Níl cead agat an gníomh seo a dhéanamh.",
		MsgInternalError:            "Tharla earráid inmheánach. Bain triail as arís níos déanaí.",
		MsgNoProviderAvailable:      "Níl aon soláthraí seirbhíse AI ar fáil. Seiceáil an chumraíocht nó déan teagmháil leis an riarthóir.",
		MsgProvidersInCooldown:      "Níl aon soláthraí seirbhíse AI ar fáil faoi láthair (%d soláthraí ag fuarú). Bain triail as arís níos déanaí.",
		MsgPathEscapesWorkspaceRoot: "Tá an cosán lasmuigh de fhréamh an spáis oibre",
		MsgMediaGenerating:          "🎨 Meáin á ghiniúint... Seolfaidh mé an toradh nuair a bheidh sé réidh.",
		MsgMediaGenFailed:           "❌ Theip ar ghiniúint meán: %s",
		MsgMediaGenCancelled:        "🚫 Cuireadh giniúint meán ar ceal.",
		MsgMediaGenNoOutput:         "✅ Giniúint críochnaithe, ach ní bhfuarthas aon toradh.",
		MsgMediaImageGenerated:      "✅ Íomhá ginte (%s)",
		MsgMediaVideoGenerated:      "✅ Físeán ginte (%s)",
		MsgMediaGenerated:           "✅ Meán ginte (%s)",
		MsgMediaGenTimeout:          "⏰ Chuaigh giniúint meán thar am. Bain triail as arís.",
	} {
		AddTranslation(LangGaIE, k, v)
	}

	// --- ml-IN (Malayalam) ---
	for k, v := range map[string]string{
		MsgProcessingError:          "ക്ഷമിക്കണം, നിങ്ങളുടെ സന്ദേശം പ്രോസസ്സ് ചെയ്യുന്നതിൽ ഒരു പിശക് സംഭവിച്ചു: %v",
		MsgChannelNotConnected:      "ചാനൽ കണക്റ്റ് ചെയ്തിട്ടില്ല. പിന്നീട് വീണ്ടും ശ്രമിക്കുക.",
		MsgTimeout:                  "അഭ്യർത്ഥന സമയം കഴിഞ്ഞു. വീണ്ടും ശ്രമിക്കുക.",
		MsgRateLimited:              "വളരെയധികം അഭ്യർത്ഥനകൾ. ഒരു നിമിഷം കാത്തിരുന്ന് വീണ്ടും ശ്രമിക്കുക.",
		MsgServiceUnavailable:       "സേവനം താൽക്കാലികമായി ലഭ്യമല്ല. പിന്നീട് വീണ്ടും ശ്രമിക്കുക.",
		MsgInvalidRequest:           "അസാധുവായ അഭ്യർത്ഥന. നിങ്ങളുടെ ഇൻപുട്ട് പരിശോധിച്ച് വീണ്ടും ശ്രമിക്കുക.",
		MsgUnauthorized:             "ഈ പ്രവർത്തനം നടത്താൻ നിങ്ങൾക്ക് അനുമതിയില്ല.",
		MsgInternalError:            "ഒരു ആന്തരിക പിശക് സംഭവിച്ചു. പിന്നീട് വീണ്ടും ശ്രമിക്കുക.",
		MsgNoProviderAvailable:      "AI സേവന ദാതാവ് ലഭ്യമല്ല. കോൺഫിഗറേഷൻ പരിശോധിക്കുക അല്ലെങ്കിൽ അഡ്മിനിസ്ട്രേറ്ററെ ബന്ധപ്പെടുക.",
		MsgProvidersInCooldown:      "നിലവിൽ AI സേവന ദാതാവ് ലഭ്യമല്ല (%d ദാതാക്കൾ കൂൾഡൗണിൽ). പിന്നീട് വീണ്ടും ശ്രമിക്കുക.",
		MsgPathEscapesWorkspaceRoot: "പാത വർക്ക്‌സ്‌പേസ് റൂട്ടിന് പുറത്താണ്",
		MsgMediaGenerating:          "🎨 മീഡിയ ജനറേറ്റ് ചെയ്യുന്നു... തയ്യാറാകുമ്പോൾ ഫലം അയയ്ക്കാം.",
		MsgMediaGenFailed:           "❌ മീഡിയ ജനറേഷൻ പരാജയപ്പെട്ടു: %s",
		MsgMediaGenCancelled:        "🚫 മീഡിയ ജനറേഷൻ റദ്ദാക്കി.",
		MsgMediaGenNoOutput:         "✅ ജനറേഷൻ പൂർത്തിയായി, പക്ഷേ ഫലം ലഭിച്ചില്ല.",
		MsgMediaImageGenerated:      "✅ ചിത്രം ജനറേറ്റ് ചെയ്തു (%s)",
		MsgMediaVideoGenerated:      "✅ വീഡിയോ ജനറേറ്റ് ചെയ്തു (%s)",
		MsgMediaGenerated:           "✅ മീഡിയ ജനറേറ്റ് ചെയ്തു (%s)",
		MsgMediaGenTimeout:          "⏰ മീഡിയ ജനറേഷൻ സമയം കഴിഞ്ഞു. വീണ്ടും ശ്രമിക്കുക.",
	} {
		AddTranslation(LangMlIN, k, v)
	}
}
