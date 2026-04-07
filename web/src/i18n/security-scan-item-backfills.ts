import type { LocaleKey } from './locale-catalog'

const securityScanItemBackfills = {
  "ca-ES": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Requisits de complexitat de la contrasenya",
            "description": "Comproveu si els requisits de complexitat de la contrasenya estan configurats correctament"
          },
          "auth_account_lockout": {
            "name": "Política de bloqueig de comptes",
            "description": "Comproveu si el bloqueig del compte està configurat per evitar atacs de força bruta"
          },
          "auth_jwt_secret": {
            "name": "Força de la clau secreta JWT",
            "description": "Comproveu si el secret JWT és prou fort (es recomana més de 32 bytes)"
          },
          "auth_session_timeout": {
            "name": "Temps d'espera de la sessió",
            "description": "Comproveu si el temps d'espera de la sessió està configurat correctament"
          },
          "auth_active_sessions": {
            "name": "Seguiment actiu de sessions",
            "description": "Comproveu si s'està fent un seguiment de les sessions actives"
          },
          "input_threat_detection": {
            "name": "Detecció d'amenaces activa",
            "description": "Comproveu si la detecció d'amenaces d'entrada s'està executant"
          },
          "input_xss_protection": {
            "name": "Protecció XSS",
            "description": "Comproveu si la detecció d'atac XSS està activada"
          },
          "input_sql_injection": {
            "name": "Protecció d'injecció SQL",
            "description": "Comproveu si la detecció d'injecció SQL està habilitada"
          },
          "input_command_injection": {
            "name": "Protecció d'injecció de comandament",
            "description": "Comproveu si la detecció d'injecció d'ordres està habilitada"
          },
          "ai_output_validation": {
            "name": "Validació de la sortida de l'IA",
            "description": "Comproveu si les sortides d'AI estan validades abans de l'execució"
          },
          "ai_model_access_control": {
            "name": "Model de control d'accés",
            "description": "Comproveu si la llista blanca de models està configurada per restringir l'ús del model d'IA"
          },
          "ai_data_filtering": {
            "name": "Filtret de dades sensibles",
            "description": "Comproveu si les dades sensibles es filtren del context d'IA"
          },
          "network_rate_limiting": {
            "name": "Limitació de la taxa de l'API",
            "description": "Comproveu si la limitació de velocitat està configurada per evitar l'abús"
          },
          "network_cors": {
            "name": "Configuració CORS",
            "description": "Comproveu si CORS està restringit correctament"
          },
          "network_tls": {
            "name": "Configuració TLS/HTTPS",
            "description": "Comproveu si TLS està habilitat per a una comunicació segura"
          },
          "network_ip_blocking": {
            "name": "Capacitat de bloqueig d'IP",
            "description": "Comproveu si el bloqueig d'IP està disponible i actiu"
          },
          "network_binding": {
            "name": "Enllaç de la interfície de xarxa",
            "description": "Comproveu la configuració de l'enllaç de la xarxa del servidor"
          },
          "sandbox_enabled": {
            "name": "Execució Sandbox"
          },
          "sandbox_memory_limit": {
            "name": "Límits de memòria",
            "description": "Comproveu si els límits de memòria sandbox estan configurats"
          },
          "sandbox_timeout": {
            "name": "Temps d'espera d'execució",
            "description": "Comproveu si el temps d'espera d'execució està configurat"
          },
          "sandbox_network": {
            "name": "Aïllament de la xarxa",
            "description": "Comproveu si l'accés a la xarxa sandbox està restringit"
          },
          "data_audit_logging": {
            "name": "Registre d'esdeveniments de seguretat",
            "description": "Comproveu si s'estan registrant esdeveniments de seguretat"
          },
          "data_directory_security": {
            "name": "Seguretat del directori de dades",
            "description": "Comproveu si els directoris de dades tenen els permisos adequats"
          },
          "system_debug_mode": {
            "name": "Mode de depuració",
            "description": "Comproveu si el mode de depuració està desactivat en producció"
          },
          "system_error_handling": {
            "name": "Exposició d'informació d'error",
            "description": "Comproveu si els missatges d'error estan desinfectats correctament per evitar filtracions d'informació"
          },
          "system_error_logging": {
            "name": "Registre d'errors sensibles",
            "description": "Comproveu si les dades sensibles dels errors es gestionen correctament als registres"
          },
          "system_environment": {
            "name": "Configuració de l'entorn",
            "description": "Comproveu si l'entorn està configurat correctament"
          },
          "system_go_version": {
            "name": "Go versió d'idioma d'execució",
            "description": "Comproveu si el temps d'execució de l'idioma Go està actualitzat"
          },
          "system_memory": {
            "name": "Ús de la memòria",
            "description": "Comproveu l'ús actual de la memòria"
          },
          "ai_prompt_injection": {
            "description": "Comproveu si la detecció d'injecció ràpida està habilitada"
          }
        }
      }
    }
  },
  "cs-CZ": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Požadavky na složitost hesla",
            "description": "Zkontrolujte, zda jsou správně nakonfigurovány požadavky na složitost hesla"
          },
          "auth_account_lockout": {
            "name": "Zásady uzamčení účtu",
            "description": "Zkontrolujte, zda je nakonfigurováno uzamčení účtu, aby se zabránilo útokům hrubou silou"
          },
          "auth_jwt_secret": {
            "name": "Síla tajného klíče JWT",
            "description": "Zkontrolujte, zda je tajemství JWT dostatečně silné (doporučeno 32+ bajtů)"
          },
          "auth_session_timeout": {
            "name": "Časový limit relace",
            "description": "Zkontrolujte, zda je správně nakonfigurován časový limit relace"
          },
          "auth_active_sessions": {
            "name": "Sledování aktivních relací",
            "description": "Zkontrolujte, zda jsou sledovány aktivní relace"
          },
          "input_threat_detection": {
            "name": "Detekce hrozeb je aktivní",
            "description": "Zkontrolujte, zda je spuštěna detekce vstupní hrozby"
          },
          "input_xss_protection": {
            "name": "Ochrana XSS",
            "description": "Zkontrolujte, zda je povolena detekce útoků XSS"
          },
          "input_sql_injection": {
            "name": "Ochrana vstřikování SQL",
            "description": "Zkontrolujte, zda je povolena detekce vkládání SQL"
          },
          "input_command_injection": {
            "name": "Ochrana proti vstřikování příkazů",
            "description": "Zkontrolujte, zda je povolena detekce vstřikování příkazu"
          },
          "ai_output_validation": {
            "name": "Ověření výstupu AI",
            "description": "Před provedením zkontrolujte, zda jsou výstupy AI ověřeny"
          },
          "ai_model_access_control": {
            "name": "Řízení přístupu k modelu",
            "description": "Zkontrolujte, zda je seznam povolených modelů nakonfigurován tak, aby omezoval použití modelu AI"
          },
          "ai_data_filtering": {
            "name": "Filtrování citlivých dat",
            "description": "Zkontrolujte, zda jsou citlivá data filtrována z kontextu AI"
          },
          "network_rate_limiting": {
            "name": "Omezení rychlosti API",
            "description": "Zkontrolujte, zda je nakonfigurováno omezení rychlosti, aby se zabránilo zneužití"
          },
          "network_cors": {
            "name": "Konfigurace CORS",
            "description": "Zkontrolujte, zda je CORS správně omezen"
          },
          "network_tls": {
            "name": "Konfigurace TLS/HTTPS",
            "description": "Zkontrolujte, zda je pro zabezpečenou komunikaci povoleno TLS"
          },
          "network_ip_blocking": {
            "name": "Schopnost blokování IP",
            "description": "Zkontrolujte, zda je blokování IP dostupné a aktivní"
          },
          "network_binding": {
            "name": "Vazba síťového rozhraní",
            "description": "Zkontrolujte konfiguraci síťové vazby serveru"
          },
          "sandbox_enabled": {
            "name": "Provedení pískoviště"
          },
          "sandbox_memory_limit": {
            "name": "Limity paměti",
            "description": "Zkontrolujte, zda jsou nakonfigurovány limity paměti sandboxu"
          },
          "sandbox_timeout": {
            "name": "Časový limit provedení",
            "description": "Zkontrolujte, zda je nakonfigurován časový limit spuštění"
          },
          "sandbox_network": {
            "name": "Izolace sítě",
            "description": "Zkontrolujte, zda není omezen přístup k síti sandbox"
          },
          "data_audit_logging": {
            "name": "Protokolování bezpečnostních událostí",
            "description": "Zkontrolujte, zda jsou protokolovány události zabezpečení"
          },
          "data_directory_security": {
            "name": "Zabezpečení datového adresáře",
            "description": "Zkontrolujte, zda datové adresáře mají příslušná oprávnění"
          },
          "system_debug_mode": {
            "name": "Režim ladění",
            "description": "Zkontrolujte, zda není v produkci zakázán režim ladění"
          },
          "system_error_handling": {
            "name": "Informace o chybě",
            "description": "Zkontrolujte, zda jsou chybová hlášení správně dezinfikována, aby nedošlo k úniku informací"
          },
          "system_error_logging": {
            "name": "Citlivé protokolování chyb",
            "description": "Zkontrolujte, zda jsou citlivá data v chybách správně zpracována v protokolech"
          },
          "system_environment": {
            "name": "Konfigurace prostředí",
            "description": "Zkontrolujte, zda je prostředí správně nakonfigurováno"
          },
          "system_go_version": {
            "name": "Verze jazykového modulu Go",
            "description": "Zkontrolujte, zda je jazykový modul Go aktuální"
          },
          "system_memory": {
            "name": "Využití paměti",
            "description": "Zkontrolujte aktuální využití paměti"
          },
          "ai_prompt_injection": {
            "description": "Zkontrolujte, zda je povolena okamžitá detekce vstřikování"
          }
        }
      }
    }
  },
  "da-DK": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Adgangskodekompleksitetskrav",
            "description": "Kontroller, om kravene til adgangskodekompleksitet er korrekt konfigureret"
          },
          "auth_account_lockout": {
            "name": "Kontolåsepolitik",
            "description": "Tjek, om kontolås er konfigureret til at forhindre brute force-angreb"
          },
          "auth_jwt_secret": {
            "name": "JWTs hemmelige nøglestyrke",
            "description": "Tjek, om JWT-hemmeligheden er tilstrækkelig stærk (32+ bytes anbefales)"
          },
          "auth_session_timeout": {
            "name": "Session timeout",
            "description": "Tjek, om sessionstimeout er konfigureret korrekt"
          },
          "auth_active_sessions": {
            "name": "Overvågning af aktive sessioner",
            "description": "Tjek, om aktive sessioner spores"
          },
          "input_threat_detection": {
            "name": "Trusselsdetektion aktiv",
            "description": "Kontroller, om registrering af inputtrusler kører"
          },
          "input_xss_protection": {
            "name": "XSS beskyttelse",
            "description": "Tjek, om XSS-angrebsdetektion er aktiveret"
          },
          "input_sql_injection": {
            "name": "Beskyttelse mod SQL-injektion",
            "description": "Kontroller, om SQL-injektionsdetektion er aktiveret"
          },
          "input_command_injection": {
            "name": "Beskyttelse mod kommandoinjektion",
            "description": "Tjek, om kommandoindsprøjtningsdetektion er aktiveret"
          },
          "ai_output_validation": {
            "name": "AI-outputvalidering",
            "description": "Tjek, om AI-output er valideret før udførelse"
          },
          "ai_model_access_control": {
            "name": "Model Adgangskontrol",
            "description": "Tjek, om hvidlisten over modeller er konfigureret til at begrænse brugen af AI-modeller"
          },
          "ai_data_filtering": {
            "name": "Filtrering af følsomme data",
            "description": "Tjek, om følsomme data er filtreret fra AI-kontekst"
          },
          "network_rate_limiting": {
            "name": "API-hastighedsbegrænsning",
            "description": "Tjek, om hastighedsbegrænsning er konfigureret til at forhindre misbrug"
          },
          "network_cors": {
            "name": "CORS-konfiguration",
            "description": "Tjek, om CORS er korrekt begrænset"
          },
          "network_tls": {
            "name": "TLS/HTTPS-konfiguration",
            "description": "Tjek, om TLS er aktiveret for sikker kommunikation"
          },
          "network_ip_blocking": {
            "name": "IP-blokeringsevne",
            "description": "Tjek, om IP-blokering er tilgængelig og aktiv"
          },
          "network_binding": {
            "name": "Netværksgrænsefladebinding",
            "description": "Tjek serverens netværksbindingskonfiguration"
          },
          "sandbox_enabled": {
            "name": "Udførelse af sandkasse"
          },
          "sandbox_memory_limit": {
            "name": "Hukommelsesgrænser",
            "description": "Tjek, om sandkassehukommelsesgrænser er konfigureret"
          },
          "sandbox_timeout": {
            "name": "Timeout for udførelse",
            "description": "Tjek, om udførelsestimeout er konfigureret"
          },
          "sandbox_network": {
            "name": "Netværksisolering",
            "description": "Tjek, om sandbox-netværksadgang er begrænset"
          },
          "data_audit_logging": {
            "name": "Sikkerhedshændelseslogning",
            "description": "Tjek, om sikkerhedshændelser bliver logget"
          },
          "data_directory_security": {
            "name": "Data Directory Sikkerhed",
            "description": "Tjek, om datamapper har passende tilladelser"
          },
          "system_debug_mode": {
            "name": "Fejlretningstilstand",
            "description": "Tjek, om fejlretningstilstand er deaktiveret i produktionen"
          },
          "system_error_handling": {
            "name": "Fejlinformation Eksponering",
            "description": "Tjek, om fejlmeddelelser er korrekt renset for at forhindre informationslækage"
          },
          "system_error_logging": {
            "name": "Følsom fejllogning",
            "description": "Tjek, om følsomme data i fejl håndteres korrekt i logfiler"
          },
          "system_environment": {
            "name": "Miljøkonfiguration",
            "description": "Tjek om miljøet er korrekt konfigureret"
          },
          "system_go_version": {
            "name": "Go sprog runtime version",
            "description": "Tjek, om Go-sprogets runtime er opdateret"
          },
          "system_memory": {
            "name": "Hukommelsesbrug",
            "description": "Kontroller det aktuelle hukommelsesforbrug"
          },
          "ai_prompt_injection": {
            "description": "Kontroller, om prompt injektionsdetektion er aktiveret"
          }
        }
      }
    }
  },
  "de-DE": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Anforderungen an die Passwortkomplexität",
            "description": "Überprüfen Sie, ob die Anforderungen an die Passwortkomplexität ordnungsgemäß konfiguriert sind"
          },
          "auth_account_lockout": {
            "name": "Richtlinie zur Kontosperrung",
            "description": "Überprüfen Sie, ob die Kontosperrung konfiguriert ist, um Brute-Force-Angriffe zu verhindern"
          },
          "auth_jwt_secret": {
            "name": "Stärke des geheimen JWT-Schlüssels",
            "description": "Überprüfen Sie, ob das JWT-Geheimnis ausreichend stark ist (32+ Bytes empfohlen)"
          },
          "auth_session_timeout": {
            "name": "Sitzungszeitüberschreitung",
            "description": "Überprüfen Sie, ob das Sitzungszeitlimit ordnungsgemäß konfiguriert ist"
          },
          "auth_active_sessions": {
            "name": "Überwachung aktiver Sitzungen",
            "description": "Überprüfen Sie, ob aktive Sitzungen verfolgt werden"
          },
          "input_threat_detection": {
            "name": "Bedrohungserkennung aktiv",
            "description": "Überprüfen Sie, ob die Eingabebedrohungserkennung ausgeführt wird"
          },
          "input_xss_protection": {
            "name": "XSS-Schutz",
            "description": "Überprüfen Sie, ob die XSS-Angriffserkennung aktiviert ist"
          },
          "input_sql_injection": {
            "name": "SQL-Injection-Schutz",
            "description": "Überprüfen Sie, ob die SQL-Injection-Erkennung aktiviert ist"
          },
          "input_command_injection": {
            "name": "Befehlsinjektionsschutz",
            "description": "Überprüfen Sie, ob die Befehlsinjektionserkennung aktiviert ist"
          },
          "ai_output_validation": {
            "name": "KI-Ausgabevalidierung",
            "description": "Überprüfen Sie vor der Ausführung, ob die KI-Ausgaben validiert sind"
          },
          "ai_model_access_control": {
            "name": "Modellzugriffskontrolle",
            "description": "Überprüfen Sie, ob die Modell-Whitelist so konfiguriert ist, dass die Verwendung von KI-Modellen eingeschränkt wird"
          },
          "ai_data_filtering": {
            "name": "Sensible Datenfilterung",
            "description": "Überprüfen Sie, ob sensible Daten aus dem KI-Kontext gefiltert werden"
          },
          "network_rate_limiting": {
            "name": "API-Ratenbegrenzung",
            "description": "Überprüfen Sie, ob die Ratenbegrenzung konfiguriert ist, um Missbrauch zu verhindern"
          },
          "network_cors": {
            "name": "CORS-Konfiguration",
            "description": "Überprüfen Sie, ob CORS ordnungsgemäß eingeschränkt ist"
          },
          "network_tls": {
            "name": "TLS/HTTPS-Konfiguration",
            "description": "Überprüfen Sie, ob TLS für sichere Kommunikation aktiviert ist"
          },
          "network_ip_blocking": {
            "name": "IP-Blockierungsfunktion",
            "description": "Überprüfen Sie, ob die IP-Blockierung verfügbar und aktiv ist"
          },
          "network_binding": {
            "name": "Netzwerkschnittstellenbindung",
            "description": "Überprüfen Sie die Netzwerkbindungskonfiguration des Servers"
          },
          "sandbox_enabled": {
            "name": "Sandbox-Ausführung"
          },
          "sandbox_memory_limit": {
            "name": "Speichergrenzen",
            "description": "Überprüfen Sie, ob Sandbox-Speichergrenzen konfiguriert sind"
          },
          "sandbox_timeout": {
            "name": "Ausführungszeitüberschreitung",
            "description": "Überprüfen Sie, ob das Ausführungszeitlimit konfiguriert ist"
          },
          "sandbox_network": {
            "name": "Netzwerkisolation",
            "description": "Überprüfen Sie, ob der Sandbox-Netzwerkzugriff eingeschränkt ist"
          },
          "data_audit_logging": {
            "name": "Protokollierung von Sicherheitsereignissen",
            "description": "Überprüfen Sie, ob Sicherheitsereignisse protokolliert werden"
          },
          "data_directory_security": {
            "name": "Datenverzeichnissicherheit",
            "description": "Überprüfen Sie, ob Datenverzeichnisse über die entsprechenden Berechtigungen verfügen"
          },
          "system_debug_mode": {
            "name": "Debug-Modus",
            "description": "Überprüfen Sie, ob der Debugmodus in der Produktion deaktiviert ist"
          },
          "system_error_handling": {
            "name": "Offenlegung von Fehlerinformationen",
            "description": "Überprüfen Sie, ob Fehlermeldungen ordnungsgemäß bereinigt sind, um Informationslecks zu verhindern"
          },
          "system_error_logging": {
            "name": "Sensible Fehlerprotokollierung",
            "description": "Überprüfen Sie, ob vertrauliche Daten in Fehlern in den Protokollen ordnungsgemäß behandelt werden"
          },
          "system_environment": {
            "name": "Umgebungskonfiguration",
            "description": "Überprüfen Sie, ob die Umgebung ordnungsgemäß konfiguriert ist"
          },
          "system_go_version": {
            "name": "Go-Sprachlaufzeitversion",
            "description": "Überprüfen Sie, ob die Go-Sprachlaufzeitumgebung auf dem neuesten Stand ist"
          },
          "system_memory": {
            "name": "Speichernutzung",
            "description": "Überprüfen Sie die aktuelle Speichernutzung"
          },
          "ai_prompt_injection": {
            "description": "Überprüfen Sie, ob die sofortige Injektionserkennung aktiviert ist"
          }
        }
      }
    }
  },
  "el-GR": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Απαιτήσεις πολυπλοκότητας κωδικού πρόσβασης",
            "description": "Ελέγξτε εάν οι απαιτήσεις πολυπλοκότητας κωδικού πρόσβασης έχουν ρυθμιστεί σωστά"
          },
          "auth_account_lockout": {
            "name": "Πολιτική κλειδώματος λογαριασμού",
            "description": "Ελέγξτε εάν το κλείδωμα λογαριασμού έχει ρυθμιστεί για την αποτροπή επιθέσεων ωμής βίας"
          },
          "auth_jwt_secret": {
            "name": "Ισχύς μυστικού κλειδιού JWT",
            "description": "Ελέγξτε εάν το μυστικό JWT είναι αρκετά ισχυρό (συνιστώνται 32+ byte)"
          },
          "auth_session_timeout": {
            "name": "Χρονικό όριο περιόδου λειτουργίας",
            "description": "Ελέγξτε εάν το χρονικό όριο λήξης περιόδου λειτουργίας έχει ρυθμιστεί κατάλληλα"
          },
          "auth_active_sessions": {
            "name": "Ενεργή Παρακολούθηση Συνεδριών",
            "description": "Ελέγξτε εάν παρακολουθούνται οι ενεργές περίοδοι σύνδεσης"
          },
          "input_threat_detection": {
            "name": "Ενεργή ανίχνευση απειλών",
            "description": "Ελέγξτε εάν εκτελείται η ανίχνευση απειλής εισόδου"
          },
          "input_xss_protection": {
            "name": "Προστασία XSS",
            "description": "Ελέγξτε εάν η ανίχνευση επιθέσεων XSS είναι ενεργοποιημένη"
          },
          "input_sql_injection": {
            "name": "Προστασία από έγχυση SQL",
            "description": "Ελέγξτε εάν η ανίχνευση έγχυσης SQL είναι ενεργοποιημένη"
          },
          "input_command_injection": {
            "name": "Προστασία από έγχυση εντολών",
            "description": "Ελέγξτε εάν η ανίχνευση έγχυσης εντολών είναι ενεργοποιημένη"
          },
          "ai_output_validation": {
            "name": "Επικύρωση εξόδου AI",
            "description": "Ελέγξτε εάν οι έξοδοι AI έχουν επικυρωθεί πριν από την εκτέλεση"
          },
          "ai_model_access_control": {
            "name": "Έλεγχος πρόσβασης μοντέλου",
            "description": "Ελέγξτε εάν η λίστα επιτρεπόμενων μοντέλων έχει διαμορφωθεί ώστε να περιορίζει τη χρήση μοντέλων τεχνητής νοημοσύνης"
          },
          "ai_data_filtering": {
            "name": "Φιλτράρισμα ευαίσθητων δεδομένων",
            "description": "Ελέγξτε εάν τα ευαίσθητα δεδομένα φιλτράρονται από το περιβάλλον τεχνητής νοημοσύνης"
          },
          "network_rate_limiting": {
            "name": "Περιορισμός ρυθμού API",
            "description": "Ελέγξτε εάν ο περιορισμός ρυθμού έχει ρυθμιστεί για την αποφυγή κατάχρησης"
          },
          "network_cors": {
            "name": "Διαμόρφωση CORS",
            "description": "Ελέγξτε εάν το CORS είναι σωστά περιορισμένο"
          },
          "network_tls": {
            "name": "Διαμόρφωση TLS/HTTPS",
            "description": "Ελέγξτε εάν το TLS είναι ενεργοποιημένο για ασφαλή επικοινωνία"
          },
          "network_ip_blocking": {
            "name": "Δυνατότητα αποκλεισμού IP",
            "description": "Ελέγξτε εάν ο αποκλεισμός IP είναι διαθέσιμος και ενεργός"
          },
          "network_binding": {
            "name": "Δέσμευση διεπαφής δικτύου",
            "description": "Ελέγξτε τη διαμόρφωση σύνδεσης δικτύου διακομιστή"
          },
          "sandbox_enabled": {
            "name": "Εκτέλεση Sandbox"
          },
          "sandbox_memory_limit": {
            "name": "Όρια μνήμης",
            "description": "Ελέγξτε εάν έχουν ρυθμιστεί τα όρια μνήμης sandbox"
          },
          "sandbox_timeout": {
            "name": "Χρονικό όριο εκτέλεσης",
            "description": "Ελέγξτε εάν έχει ρυθμιστεί το χρονικό όριο εκτέλεσης"
          },
          "sandbox_network": {
            "name": "Απομόνωση δικτύου",
            "description": "Ελέγξτε εάν η πρόσβαση στο δίκτυο sandbox είναι περιορισμένη"
          },
          "data_audit_logging": {
            "name": "Καταγραφή συμβάντων ασφαλείας",
            "description": "Ελέγξτε εάν καταγράφονται συμβάντα ασφαλείας"
          },
          "data_directory_security": {
            "name": "Ασφάλεια καταλόγου δεδομένων",
            "description": "Ελέγξτε εάν οι κατάλογοι δεδομένων έχουν τα κατάλληλα δικαιώματα"
          },
          "system_debug_mode": {
            "name": "Λειτουργία εντοπισμού σφαλμάτων",
            "description": "Ελέγξτε εάν η λειτουργία εντοπισμού σφαλμάτων είναι απενεργοποιημένη στην παραγωγή"
          },
          "system_error_handling": {
            "name": "Έκθεση πληροφοριών σφάλματος",
            "description": "Ελέγξτε εάν τα μηνύματα σφάλματος έχουν απολυμανθεί σωστά για να αποτρέψετε τη διαρροή πληροφοριών"
          },
          "system_error_logging": {
            "name": "Καταγραφή ευαίσθητων σφαλμάτων",
            "description": "Ελέγξτε εάν τα ευαίσθητα δεδομένα σε σφάλματα αντιμετωπίζονται σωστά στα αρχεία καταγραφής"
          },
          "system_environment": {
            "name": "Διαμόρφωση περιβάλλοντος",
            "description": "Ελέγξτε εάν το περιβάλλον έχει ρυθμιστεί σωστά"
          },
          "system_go_version": {
            "name": "Έκδοση χρόνου εκτέλεσης γλώσσας Go",
            "description": "Ελέγξτε εάν ο χρόνος εκτέλεσης της γλώσσας Go είναι ενημερωμένος"
          },
          "system_memory": {
            "name": "Χρήση Μνήμης",
            "description": "Ελέγξτε την τρέχουσα χρήση μνήμης"
          },
          "ai_prompt_injection": {
            "description": "Ελέγξτε εάν είναι ενεργοποιημένη η άμεση ανίχνευση έγχυσης"
          }
        }
      }
    }
  },
  "en-GB": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Password Complexity Requirements",
            "description": "Check if password complexity requirements are properly configured"
          },
          "auth_account_lockout": {
            "name": "Account Lockout Policy",
            "description": "Check if account lockout is configured to prevent brute force attacks"
          },
          "auth_jwt_secret": {
            "name": "JWT Secret Strength",
            "description": "Check if JWT secret is sufficiently strong (32+ bytes recommended)"
          },
          "auth_session_timeout": {
            "name": "Session Timeout",
            "description": "Check if session timeout is configured appropriately"
          },
          "auth_active_sessions": {
            "name": "Active Sessions Monitoring",
            "description": "Check if active sessions are being tracked"
          },
          "input_threat_detection": {
            "name": "Threat Detection Active",
            "description": "Check if input threat detection is running"
          },
          "input_xss_protection": {
            "name": "XSS Protection",
            "description": "Check if XSS attack detection is enabled"
          },
          "input_sql_injection": {
            "name": "SQL Injection Protection",
            "description": "Check if SQL injection detection is enabled"
          },
          "input_command_injection": {
            "name": "Command Injection Protection",
            "description": "Check if command injection detection is enabled"
          },
          "ai_output_validation": {
            "name": "AI Output Validation",
            "description": "Check if AI outputs are validated before execution"
          },
          "ai_model_access_control": {
            "name": "Model Access Control",
            "description": "Check if model whitelist is configured to restrict AI model usage"
          },
          "ai_data_filtering": {
            "name": "Sensitive Data Filtering",
            "description": "Check if sensitive data is filtered from AI context"
          },
          "network_rate_limiting": {
            "name": "API Rate Limiting",
            "description": "Check if rate limiting is configured to prevent abuse"
          },
          "network_cors": {
            "name": "CORS Configuration",
            "description": "Check if CORS is properly restricted"
          },
          "network_tls": {
            "name": "TLS/HTTPS Configuration",
            "description": "Check if TLS is enabled for secure communication"
          },
          "network_ip_blocking": {
            "name": "IP Blocking Capability",
            "description": "Check if IP blocking is available and active"
          },
          "network_binding": {
            "name": "Network Interface Binding",
            "description": "Check server network binding configuration"
          },
          "sandbox_enabled": {
            "name": "Sandbox Execution"
          },
          "sandbox_memory_limit": {
            "name": "Memory Limits",
            "description": "Check if sandbox memory limits are configured"
          },
          "sandbox_timeout": {
            "name": "Execution Timeout",
            "description": "Check if execution timeout is configured"
          },
          "sandbox_network": {
            "name": "Network Isolation",
            "description": "Check if sandbox network access is restricted"
          },
          "data_audit_logging": {
            "name": "Security Event Logging",
            "description": "Check if security events are being logged"
          },
          "data_directory_security": {
            "name": "Data Directory Security",
            "description": "Check if data directories have appropriate permissions"
          },
          "system_debug_mode": {
            "name": "Debug Mode",
            "description": "Check if debug mode is disabled in production"
          },
          "system_error_handling": {
            "name": "Error Information Exposure",
            "description": "Check if error messages are properly sanitized to prevent information leakage"
          },
          "system_error_logging": {
            "name": "Sensitive Error Logging",
            "description": "Check if sensitive data in errors is properly handled in logs"
          },
          "system_environment": {
            "name": "Environment Configuration",
            "description": "Check if environment is properly configured"
          },
          "system_go_version": {
            "name": "Go Runtime Version",
            "description": "Check if Go runtime is up to date"
          },
          "system_memory": {
            "name": "Memory Usage",
            "description": "Check current memory usage"
          },
          "ai_prompt_injection": {
            "description": "Check if prompt injection detection is enabled"
          }
        }
      }
    }
  },
  "en-US": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Password Complexity Requirements",
            "description": "Check if password complexity requirements are properly configured"
          },
          "auth_account_lockout": {
            "name": "Account Lockout Policy",
            "description": "Check if account lockout is configured to prevent brute force attacks"
          },
          "auth_jwt_secret": {
            "name": "JWT Secret Strength",
            "description": "Check if JWT secret is sufficiently strong (32+ bytes recommended)"
          },
          "auth_session_timeout": {
            "name": "Session Timeout",
            "description": "Check if session timeout is configured appropriately"
          },
          "auth_active_sessions": {
            "name": "Active Sessions Monitoring",
            "description": "Check if active sessions are being tracked"
          },
          "input_threat_detection": {
            "name": "Threat Detection Active",
            "description": "Check if input threat detection is running"
          },
          "input_xss_protection": {
            "name": "XSS Protection",
            "description": "Check if XSS attack detection is enabled"
          },
          "input_sql_injection": {
            "name": "SQL Injection Protection",
            "description": "Check if SQL injection detection is enabled"
          },
          "input_command_injection": {
            "name": "Command Injection Protection",
            "description": "Check if command injection detection is enabled"
          },
          "ai_output_validation": {
            "name": "AI Output Validation",
            "description": "Check if AI outputs are validated before execution"
          },
          "ai_model_access_control": {
            "name": "Model Access Control",
            "description": "Check if model whitelist is configured to restrict AI model usage"
          },
          "ai_data_filtering": {
            "name": "Sensitive Data Filtering",
            "description": "Check if sensitive data is filtered from AI context"
          },
          "network_rate_limiting": {
            "name": "API Rate Limiting",
            "description": "Check if rate limiting is configured to prevent abuse"
          },
          "network_cors": {
            "name": "CORS Configuration",
            "description": "Check if CORS is properly restricted"
          },
          "network_tls": {
            "name": "TLS/HTTPS Configuration",
            "description": "Check if TLS is enabled for secure communication"
          },
          "network_ip_blocking": {
            "name": "IP Blocking Capability",
            "description": "Check if IP blocking is available and active"
          },
          "network_binding": {
            "name": "Network Interface Binding",
            "description": "Check server network binding configuration"
          },
          "sandbox_enabled": {
            "name": "Sandbox Execution"
          },
          "sandbox_memory_limit": {
            "name": "Memory Limits",
            "description": "Check if sandbox memory limits are configured"
          },
          "sandbox_timeout": {
            "name": "Execution Timeout",
            "description": "Check if execution timeout is configured"
          },
          "sandbox_network": {
            "name": "Network Isolation",
            "description": "Check if sandbox network access is restricted"
          },
          "data_audit_logging": {
            "name": "Security Event Logging",
            "description": "Check if security events are being logged"
          },
          "data_directory_security": {
            "name": "Data Directory Security",
            "description": "Check if data directories have appropriate permissions"
          },
          "system_debug_mode": {
            "name": "Debug Mode",
            "description": "Check if debug mode is disabled in production"
          },
          "system_error_handling": {
            "name": "Error Information Exposure",
            "description": "Check if error messages are properly sanitized to prevent information leakage"
          },
          "system_error_logging": {
            "name": "Sensitive Error Logging",
            "description": "Check if sensitive data in errors is properly handled in logs"
          },
          "system_environment": {
            "name": "Environment Configuration",
            "description": "Check if environment is properly configured"
          },
          "system_go_version": {
            "name": "Go Runtime Version",
            "description": "Check if Go runtime is up to date"
          },
          "system_memory": {
            "name": "Memory Usage",
            "description": "Check current memory usage"
          },
          "ai_prompt_injection": {
            "description": "Check if prompt injection detection is enabled"
          }
        }
      }
    }
  },
  "es-ES": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Requisitos de complejidad de la contraseña",
            "description": "Compruebe si los requisitos de complejidad de la contraseña están configurados correctamente"
          },
          "auth_account_lockout": {
            "name": "Política de bloqueo de cuenta",
            "description": "Compruebe si el bloqueo de cuenta está configurado para evitar ataques de fuerza bruta"
          },
          "auth_jwt_secret": {
            "name": "Fortaleza de la clave secreta JWT",
            "description": "Compruebe si el secreto JWT es lo suficientemente fuerte (se recomiendan más de 32 bytes)"
          },
          "auth_session_timeout": {
            "name": "Tiempo de espera de sesión",
            "description": "Compruebe si el tiempo de espera de la sesión está configurado correctamente"
          },
          "auth_active_sessions": {
            "name": "Monitoreo de sesiones activas",
            "description": "Comprobar si se están realizando un seguimiento de las sesiones activas"
          },
          "input_threat_detection": {
            "name": "Detección de amenazas activa",
            "description": "Compruebe si la detección de amenazas de entrada se está ejecutando"
          },
          "input_xss_protection": {
            "name": "Protección XSS",
            "description": "Compruebe si la detección de ataques XSS está habilitada"
          },
          "input_sql_injection": {
            "name": "Protección de inyección SQL",
            "description": "Compruebe si la detección de inyección SQL está habilitada"
          },
          "input_command_injection": {
            "name": "Protección de inyección de comando",
            "description": "Compruebe si la detección de inyección de comandos está habilitada"
          },
          "ai_output_validation": {
            "name": "Validación de salida de IA",
            "description": "Compruebe si las salidas de IA están validadas antes de la ejecución"
          },
          "ai_model_access_control": {
            "name": "Control de acceso modelo",
            "description": "Compruebe si la lista blanca de modelos está configurada para restringir el uso del modelo de IA"
          },
          "ai_data_filtering": {
            "name": "Filtrado de datos confidenciales",
            "description": "Compruebe si los datos confidenciales se filtran del contexto de IA"
          },
          "network_rate_limiting": {
            "name": "Limitación de tasa API",
            "description": "Compruebe si la limitación de velocidad está configurada para evitar abusos"
          },
          "network_cors": {
            "name": "Configuración CORS",
            "description": "Compruebe si CORS está restringido correctamente"
          },
          "network_tls": {
            "name": "Configuración TLS/HTTPS",
            "description": "Compruebe si TLS está habilitado para una comunicación segura"
          },
          "network_ip_blocking": {
            "name": "Capacidad de bloqueo de IP",
            "description": "Compruebe si el bloqueo de IP está disponible y activo"
          },
          "network_binding": {
            "name": "Enlace de interfaz de red",
            "description": "Verifique la configuración de enlace de red del servidor"
          },
          "sandbox_enabled": {
            "name": "Ejecución de zona de pruebas"
          },
          "sandbox_memory_limit": {
            "name": "Límites de memoria",
            "description": "Compruebe si los límites de memoria de la zona de pruebas están configurados"
          },
          "sandbox_timeout": {
            "name": "Tiempo de espera de ejecución",
            "description": "Compruebe si el tiempo de espera de ejecución está configurado"
          },
          "sandbox_network": {
            "name": "Aislamiento de red",
            "description": "Compruebe si el acceso a la red sandbox está restringido"
          },
          "data_audit_logging": {
            "name": "Registro de eventos de seguridad",
            "description": "Comprobar si se están registrando eventos de seguridad"
          },
          "data_directory_security": {
            "name": "Seguridad del directorio de datos",
            "description": "Compruebe si los directorios de datos tienen los permisos adecuados"
          },
          "system_debug_mode": {
            "name": "Modo de depuración",
            "description": "Compruebe si el modo de depuración está deshabilitado en producción"
          },
          "system_error_handling": {
            "name": "Exposición de información de error",
            "description": "Verifique si los mensajes de error están correctamente desinfectados para evitar la fuga de información"
          },
          "system_error_logging": {
            "name": "Registro de errores sensibles",
            "description": "Compruebe si los datos confidenciales en los errores se manejan correctamente en los registros"
          },
          "system_environment": {
            "name": "Configuración del entorno",
            "description": "Compruebe si el entorno está configurado correctamente"
          },
          "system_go_version": {
            "name": "Ir a la versión en tiempo de ejecución del idioma",
            "description": "Compruebe si el tiempo de ejecución del idioma Go está actualizado"
          },
          "system_memory": {
            "name": "Uso de la memoria",
            "description": "Verificar el uso actual de la memoria"
          },
          "ai_prompt_injection": {
            "description": "Compruebe si la detección rápida de inyección está habilitada"
          }
        }
      }
    }
  },
  "fr-FR": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Exigences de complexité du mot de passe",
            "description": "Vérifiez si les exigences de complexité des mots de passe sont correctement configurées"
          },
          "auth_account_lockout": {
            "name": "Politique de verrouillage de compte",
            "description": "Vérifiez si le verrouillage du compte est configuré pour empêcher les attaques par force brute"
          },
          "auth_jwt_secret": {
            "name": "Force de la clé secrète JWT",
            "description": "Vérifiez si le secret JWT est suffisamment fort (plus de 32 octets recommandés)"
          },
          "auth_session_timeout": {
            "name": "Expiration de la session",
            "description": "Vérifiez si le délai d'expiration de la session est configuré de manière appropriée"
          },
          "auth_active_sessions": {
            "name": "Surveillance des sessions actives",
            "description": "Vérifiez si les sessions actives sont suivies"
          },
          "input_threat_detection": {
            "name": "Détection des menaces active",
            "description": "Vérifiez si la détection des menaces d'entrée est en cours d'exécution"
          },
          "input_xss_protection": {
            "name": "Protection XSS",
            "description": "Vérifiez si la détection des attaques XSS est activée"
          },
          "input_sql_injection": {
            "name": "Protection contre les injections SQL",
            "description": "Vérifiez si la détection d'injection SQL est activée"
          },
          "input_command_injection": {
            "name": "Protection contre les injections de commande",
            "description": "Vérifiez si la détection d'injection de commande est activée"
          },
          "ai_output_validation": {
            "name": "Validation des sorties IA",
            "description": "Vérifiez si les sorties de l'IA sont validées avant l'exécution"
          },
          "ai_model_access_control": {
            "name": "Modèle de contrôle d'accès",
            "description": "Vérifiez si la liste blanche des modèles est configurée pour restreindre l'utilisation du modèle IA"
          },
          "ai_data_filtering": {
            "name": "Filtrage des données sensibles",
            "description": "Vérifiez si les données sensibles sont filtrées du contexte de l'IA"
          },
          "network_rate_limiting": {
            "name": "Limitation du débit de l'API",
            "description": "Vérifiez si la limitation du débit est configurée pour éviter les abus"
          },
          "network_cors": {
            "name": "Configuration CORS",
            "description": "Vérifiez si CORS est correctement restreint"
          },
          "network_tls": {
            "name": "Configuration TLS/HTTPS",
            "description": "Vérifiez si TLS est activé pour une communication sécurisée"
          },
          "network_ip_blocking": {
            "name": "Capacité de blocage IP",
            "description": "Vérifiez si le blocage IP est disponible et actif"
          },
          "network_binding": {
            "name": "Liaison d'interface réseau",
            "description": "Vérifier la configuration de la liaison réseau du serveur"
          },
          "sandbox_enabled": {
            "name": "Exécution du bac à sable"
          },
          "sandbox_memory_limit": {
            "name": "Limites de mémoire",
            "description": "Vérifiez si les limites de mémoire du bac à sable sont configurées"
          },
          "sandbox_timeout": {
            "name": "Délai d'exécution",
            "description": "Vérifiez si le délai d'exécution est configuré"
          },
          "sandbox_network": {
            "name": "Isolation du réseau",
            "description": "Vérifiez si l'accès au réseau sandbox est restreint"
          },
          "data_audit_logging": {
            "name": "Journalisation des événements de sécurité",
            "description": "Vérifiez si les événements de sécurité sont enregistrés"
          },
          "data_directory_security": {
            "name": "Sécurité du répertoire de données",
            "description": "Vérifiez si les répertoires de données disposent des autorisations appropriées"
          },
          "system_debug_mode": {
            "name": "Mode débogage",
            "description": "Vérifiez si le mode débogage est désactivé en production"
          },
          "system_error_handling": {
            "name": "Exposition aux informations d’erreur",
            "description": "Vérifiez si les messages d'erreur sont correctement nettoyés pour éviter les fuites d'informations"
          },
          "system_error_logging": {
            "name": "Journalisation des erreurs sensibles",
            "description": "Vérifiez si les données sensibles des erreurs sont correctement traitées dans les journaux"
          },
          "system_environment": {
            "name": "Configuration de l'environnement",
            "description": "Vérifiez si l'environnement est correctement configuré"
          },
          "system_go_version": {
            "name": "Accéder à la version d'exécution du langage",
            "description": "Vérifiez si le runtime du langage Go est à jour"
          },
          "system_memory": {
            "name": "Utilisation de la mémoire",
            "description": "Vérifier l'utilisation actuelle de la mémoire"
          },
          "ai_prompt_injection": {
            "description": "Vérifiez si la détection d'injection rapide est activée"
          }
        }
      }
    }
  },
  "ga-IE": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Riachtanais Castacht Pasfhocal",
            "description": "Seiceáil an bhfuil riachtanais castachta pasfhocail cumraithe i gceart"
          },
          "auth_account_lockout": {
            "name": "Beartas Frithdhúnadh Cuntas",
            "description": "Seiceáil an bhfuil frithdhúnadh cuntais cumraithe chun ionsaithe fórsa brúidiúil a chosc"
          },
          "auth_jwt_secret": {
            "name": "Neart eochair rúnda JWT",
            "description": "Seiceáil an bhfuil rún JWT sách láidir (moltar 32+ beart)"
          },
          "auth_session_timeout": {
            "name": "Teorainn Ama an tSeisiúin",
            "description": "Seiceáil an bhfuil teorainn ama an tseisiúin cumraithe go cuí"
          },
          "auth_active_sessions": {
            "name": "Monatóireacht ar Sheisiúin Ghníomhach",
            "description": "Seiceáil an bhfuil seisiúin ghníomhacha á rianú"
          },
          "input_threat_detection": {
            "name": "Brath Bagairt Gníomhach",
            "description": "Seiceáil an bhfuil braite bagairt ionchuir ar siúl"
          },
          "input_xss_protection": {
            "name": "Cosaint XSS",
            "description": "Seiceáil an bhfuil braite ionsaithe XSS cumasaithe"
          },
          "input_sql_injection": {
            "name": "Cosaint Instealladh SQL",
            "description": "Seiceáil an bhfuil braite insteallta SQL cumasaithe"
          },
          "input_command_injection": {
            "name": "Cosaint Instealladh Ordú",
            "description": "Seiceáil an bhfuil braite insteallta ordaithe cumasaithe"
          },
          "ai_output_validation": {
            "name": "Bailíochtú Aschuir AI",
            "description": "Seiceáil an bhfuil aschuir AI deimhnithe roimh fhorghníomhú"
          },
          "ai_model_access_control": {
            "name": "Rialú Rochtana Múnla",
            "description": "Seiceáil an bhfuil liosta bán na samhla cumraithe chun úsáid mhúnla AI a shrianadh"
          },
          "ai_data_filtering": {
            "name": "Scagadh Sonraí Íogaire",
            "description": "Seiceáil an bhfuil sonraí íogaire scagtha ó chomhthéacs AI"
          },
          "network_rate_limiting": {
            "name": "Teorainn Ráta API",
            "description": "Seiceáil an bhfuil teorannú rátaí cumraithe chun mí-úsáid a chosc"
          },
          "network_cors": {
            "name": "Cumraíocht CORS",
            "description": "Seiceáil an bhfuil srian cuí ar CORS"
          },
          "network_tls": {
            "name": "Cumraíocht TLS/HTTPS",
            "description": "Seiceáil an bhfuil TLS cumasaithe le haghaidh cumarsáide slán"
          },
          "network_ip_blocking": {
            "name": "Cumas Blocáil IP",
            "description": "Seiceáil an bhfuil blocáil IP ar fáil agus gníomhach"
          },
          "network_binding": {
            "name": "Ceangal Comhéadain Líonra",
            "description": "Seiceáil cumraíocht ceangailteach líonra an fhreastalaí"
          },
          "sandbox_enabled": {
            "name": "Forghníomhú Bosca Gainimh"
          },
          "sandbox_memory_limit": {
            "name": "Teorainneacha Cuimhne",
            "description": "Seiceáil an bhfuil teorainneacha cuimhne an bhosca gainimh cumraithe"
          },
          "sandbox_timeout": {
            "name": "Teorainn Ama Forghníomhaithe",
            "description": "Seiceáil an bhfuil teorainn ama forghníomhaithe cumraithe"
          },
          "sandbox_network": {
            "name": "Leithlisiú Líonra",
            "description": "Seiceáil an bhfuil srian ar rochtain líonra bosca gainimh"
          },
          "data_audit_logging": {
            "name": "Logáil Imeachtaí Slándála",
            "description": "Seiceáil an bhfuil imeachtaí slándála á logáil"
          },
          "data_directory_security": {
            "name": "Slándáil Eolaire Sonraí",
            "description": "Seiceáil an bhfuil ceadanna cuí ag eolairí sonraí"
          },
          "system_debug_mode": {
            "name": "Mód Dífhabhtaithe",
            "description": "Seiceáil an bhfuil an modh dífhabhtaithe díchumasaithe sa táirgeadh"
          },
          "system_error_handling": {
            "name": "Nochtadh Faisnéise Earráid",
            "description": "Seiceáil an bhfuil teachtaireachtaí earráide sláintithe i gceart chun sceitheadh faisnéise a chosc"
          },
          "system_error_logging": {
            "name": "Logáil Earráide Íogaire",
            "description": "Seiceáil an láimhseáiltear sonraí íogaire i gcás earráidí i gceart i logaí"
          },
          "system_environment": {
            "name": "Cumraíocht Timpeallachta",
            "description": "Seiceáil an bhfuil an timpeallacht cumraithe i gceart"
          },
          "system_go_version": {
            "name": "Téigh leagan ama rite teanga",
            "description": "Seiceáil an bhfuil an t-am rite teanga Téigh cothrom le dáta"
          },
          "system_memory": {
            "name": "Úsáid Cuimhne",
            "description": "Seiceáil úsáid chuimhne reatha"
          },
          "ai_prompt_injection": {
            "description": "Seiceáil an bhfuil braite instealladh pras cumasaithe"
          }
        }
      }
    }
  },
  "hr-HR": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Zahtjevi za složenost lozinke",
            "description": "Provjerite jesu li zahtjevi za složenost lozinke ispravno konfigurirani"
          },
          "auth_account_lockout": {
            "name": "Politika zaključavanja računa",
            "description": "Provjerite je li zaključavanje računa konfigurirano za sprječavanje napada brutalnom silom"
          },
          "auth_jwt_secret": {
            "name": "Snaga JWT tajnog ključa",
            "description": "Provjerite je li JWT tajna dovoljno jaka (preporučuje se 32+ bajta)"
          },
          "auth_session_timeout": {
            "name": "Vremensko ograničenje sesije",
            "description": "Provjerite je li vrijeme čekanja sesije ispravno konfigurirano"
          },
          "auth_active_sessions": {
            "name": "Praćenje aktivnih sesija",
            "description": "Provjerite prate li se aktivne sesije"
          },
          "input_threat_detection": {
            "name": "Aktivno otkrivanje prijetnji",
            "description": "Provjerite radi li otkrivanje ulazne prijetnje"
          },
          "input_xss_protection": {
            "name": "XSS zaštita",
            "description": "Provjerite je li omogućeno otkrivanje XSS napada"
          },
          "input_sql_injection": {
            "name": "Zaštita od SQL ubacivanja",
            "description": "Provjerite je li omogućeno otkrivanje SQL ubacivanja"
          },
          "input_command_injection": {
            "name": "Zaštita od ubrizgavanja naredbe",
            "description": "Provjerite je li omogućeno otkrivanje ubrizgavanja naredbi"
          },
          "ai_output_validation": {
            "name": "Validacija AI izlaza",
            "description": "Provjerite jesu li AI izlazi potvrđeni prije izvršenja"
          },
          "ai_model_access_control": {
            "name": "Model kontrole pristupa",
            "description": "Provjerite je li popis dopuštenih modela konfiguriran za ograničavanje upotrebe AI modela"
          },
          "ai_data_filtering": {
            "name": "Filtriranje osjetljivih podataka",
            "description": "Provjerite jesu li osjetljivi podaci filtrirani iz AI konteksta"
          },
          "network_rate_limiting": {
            "name": "Ograničenje brzine API-ja",
            "description": "Provjerite je li ograničenje brzine konfigurirano za sprječavanje zlouporabe"
          },
          "network_cors": {
            "name": "CORS konfiguracija",
            "description": "Provjerite je li CORS ispravno ograničen"
          },
          "network_tls": {
            "name": "TLS/HTTPS konfiguracija",
            "description": "Provjerite je li TLS omogućen za sigurnu komunikaciju"
          },
          "network_ip_blocking": {
            "name": "Mogućnost IP blokiranja",
            "description": "Provjerite je li IP blokiranje dostupno i aktivno"
          },
          "network_binding": {
            "name": "Vezanje mrežnog sučelja",
            "description": "Provjerite konfiguraciju vezanja mreže poslužitelja"
          },
          "sandbox_enabled": {
            "name": "Izvršenje sandboxa"
          },
          "sandbox_memory_limit": {
            "name": "Ograničenja memorije",
            "description": "Provjerite jesu li ograničenja memorije sandboxa konfigurirana"
          },
          "sandbox_timeout": {
            "name": "Istek vremena izvršenja",
            "description": "Provjerite je li konfigurirano vremensko ograničenje izvršenja"
          },
          "sandbox_network": {
            "name": "Izolacija mreže",
            "description": "Provjerite je li pristup mreži sandboxa ograničen"
          },
          "data_audit_logging": {
            "name": "Zapisivanje sigurnosnih događaja",
            "description": "Provjerite bilježe li se sigurnosni događaji"
          },
          "data_directory_security": {
            "name": "Sigurnost imenika podataka",
            "description": "Provjerite imaju li direktoriji podataka odgovarajuća dopuštenja"
          },
          "system_debug_mode": {
            "name": "Način otklanjanja pogrešaka",
            "description": "Provjerite je li način otklanjanja pogrešaka onemogućen u produkciji"
          },
          "system_error_handling": {
            "name": "Izlaganje informacija o pogrešci",
            "description": "Provjerite jesu li poruke o pogreškama ispravno dezinficirane kako biste spriječili curenje informacija"
          },
          "system_error_logging": {
            "name": "Osjetljivo bilježenje pogrešaka",
            "description": "Provjerite jesu li osjetljivi podaci u pogreškama ispravno obrađeni u zapisnicima"
          },
          "system_environment": {
            "name": "Konfiguracija okruženja",
            "description": "Provjerite je li okruženje ispravno konfigurirano"
          },
          "system_go_version": {
            "name": "Izvedbena verzija jezika Go",
            "description": "Provjerite je li runtime jezika Go ažuriran"
          },
          "system_memory": {
            "name": "Upotreba memorije",
            "description": "Provjerite trenutnu upotrebu memorije"
          },
          "ai_prompt_injection": {
            "description": "Provjerite je li omogućeno otkrivanje brzog ubrizgavanja"
          }
        }
      }
    }
  },
  "hu-HU": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Jelszó bonyolultsági követelmények",
            "description": "Ellenőrizze, hogy a jelszó összetettségi követelményei megfelelően vannak-e konfigurálva"
          },
          "auth_account_lockout": {
            "name": "Fiókzárolási szabályzat",
            "description": "Ellenőrizze, hogy a fiókzárolás be van-e állítva a brutális erőszak elleni támadások megelőzésére"
          },
          "auth_jwt_secret": {
            "name": "JWT titkos kulcs erőssége",
            "description": "Ellenőrizze, hogy a JWT titkossága elég erős-e (32+ bájt ajánlott)"
          },
          "auth_session_timeout": {
            "name": "Munkamenet időtúllépés",
            "description": "Ellenőrizze, hogy a munkamenet időtúllépése megfelelően van-e konfigurálva"
          },
          "auth_active_sessions": {
            "name": "Aktív munkamenetek figyelése",
            "description": "Ellenőrizze, hogy a rendszer követi-e az aktív munkameneteket"
          },
          "input_threat_detection": {
            "name": "Fenyegetésészlelés aktív",
            "description": "Ellenőrizze, hogy fut-e a bemeneti fenyegetésészlelés"
          },
          "input_xss_protection": {
            "name": "XSS védelem",
            "description": "Ellenőrizze, hogy az XSS támadásészlelés engedélyezve van-e"
          },
          "input_sql_injection": {
            "name": "SQL-befecskendezés elleni védelem",
            "description": "Ellenőrizze, hogy az SQL-injektálás észlelése engedélyezve van-e"
          },
          "input_command_injection": {
            "name": "Injekció elleni védelem parancs",
            "description": "Ellenőrizze, hogy engedélyezve van-e a parancsinjektálás észlelése"
          },
          "ai_output_validation": {
            "name": "AI kimenet érvényesítése",
            "description": "Ellenőrizze, hogy az AI kimenetek érvényesek-e a végrehajtás előtt"
          },
          "ai_model_access_control": {
            "name": "Modell hozzáférés-vezérlés",
            "description": "Ellenőrizze, hogy a modell engedélyezési listája nincs-e konfigurálva az AI-modell használatának korlátozására"
          },
          "ai_data_filtering": {
            "name": "Érzékeny adatok szűrése",
            "description": "Ellenőrizze, hogy az érzékeny adatok ki vannak-e szűrve az AI-környezetből"
          },
          "network_rate_limiting": {
            "name": "API sebességkorlátozás",
            "description": "Ellenőrizze, hogy a sebességkorlátozás be van-e állítva a visszaélések megelőzése érdekében"
          },
          "network_cors": {
            "name": "CORS konfiguráció",
            "description": "Ellenőrizze, hogy a CORS megfelelően korlátozva van-e"
          },
          "network_tls": {
            "name": "TLS/HTTPS konfiguráció",
            "description": "Ellenőrizze, hogy a TLS engedélyezve van-e a biztonságos kommunikáció érdekében"
          },
          "network_ip_blocking": {
            "name": "IP blokkolási képesség",
            "description": "Ellenőrizze, hogy az IP-blokkolás elérhető és aktív-e"
          },
          "network_binding": {
            "name": "Hálózati interfész kötés",
            "description": "Ellenőrizze a kiszolgáló hálózati kötési konfigurációját"
          },
          "sandbox_enabled": {
            "name": "Sandbox végrehajtás"
          },
          "sandbox_memory_limit": {
            "name": "Memória korlátok",
            "description": "Ellenőrizze, hogy be vannak-e állítva a sandbox memóriakorlátok"
          },
          "sandbox_timeout": {
            "name": "Végrehajtási időtúllépés",
            "description": "Ellenőrizze, hogy a végrehajtási időtúllépés be van-e állítva"
          },
          "sandbox_network": {
            "name": "Hálózati elkülönítés",
            "description": "Ellenőrizze, hogy nincs-e korlátozva a sandbox hálózati hozzáférés"
          },
          "data_audit_logging": {
            "name": "Biztonsági eseménynaplózás",
            "description": "Ellenőrizze, hogy a biztonsági események naplózva vannak-e"
          },
          "data_directory_security": {
            "name": "Data Directory biztonság",
            "description": "Ellenőrizze, hogy az adatkönyvtárak rendelkeznek-e megfelelő jogosultságokkal"
          },
          "system_debug_mode": {
            "name": "Hibakeresési mód",
            "description": "Ellenőrizze, hogy a hibakeresési mód le van-e tiltva az éles környezetben"
          },
          "system_error_handling": {
            "name": "Hibainformációk kitettsége",
            "description": "Ellenőrizze, hogy a hibaüzenetek megfelelően fertőtlenítettek-e az információszivárgás megelőzése érdekében"
          },
          "system_error_logging": {
            "name": "Érzékeny hibanaplózás",
            "description": "Ellenőrizze, hogy a hibákban lévő érzékeny adatok megfelelően vannak-e kezelve a naplókban"
          },
          "system_environment": {
            "name": "Környezet konfigurációja",
            "description": "Ellenőrizze, hogy a környezet megfelelően van-e konfigurálva"
          },
          "system_go_version": {
            "name": "Ugrás a nyelvi futásidejű verzióra",
            "description": "Ellenőrizze, hogy a Go nyelvi futtatókörnyezet naprakész-e"
          },
          "system_memory": {
            "name": "Memóriahasználat",
            "description": "Ellenőrizze az aktuális memóriahasználatot"
          },
          "ai_prompt_injection": {
            "description": "Ellenőrizze, hogy engedélyezve van-e az azonnali injekció észlelése"
          }
        }
      }
    }
  },
  "it-IT": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Requisiti di complessità della password",
            "description": "Controlla se i requisiti di complessità della password sono configurati correttamente"
          },
          "auth_account_lockout": {
            "name": "Politica di blocco dell'account",
            "description": "Controlla se il blocco dell'account è configurato per prevenire attacchi di forza bruta"
          },
          "auth_jwt_secret": {
            "name": "Punto di forza della chiave segreta JWT",
            "description": "Controlla se il segreto JWT è sufficientemente forte (si consigliano più di 32 byte)"
          },
          "auth_session_timeout": {
            "name": "Timeout della sessione",
            "description": "Controlla se il timeout della sessione è configurato correttamente"
          },
          "auth_active_sessions": {
            "name": "Monitoraggio delle sessioni attive",
            "description": "Controlla se le sessioni attive vengono monitorate"
          },
          "input_threat_detection": {
            "name": "Rilevamento minacce attivo",
            "description": "Controlla se il rilevamento delle minacce di input è in esecuzione"
          },
          "input_xss_protection": {
            "name": "Protezione XSS",
            "description": "Controlla se il rilevamento degli attacchi XSS è abilitato"
          },
          "input_sql_injection": {
            "name": "Protezione dall'iniezione SQL",
            "description": "Controlla se il rilevamento dell'iniezione SQL è abilitato"
          },
          "input_command_injection": {
            "name": "Protezione dall'iniezione di comando",
            "description": "Controllare se il rilevamento dell'iniezione di comando è abilitato"
          },
          "ai_output_validation": {
            "name": "Convalida dell'output AI",
            "description": "Controlla se gli output AI sono convalidati prima dell'esecuzione"
          },
          "ai_model_access_control": {
            "name": "Controllo degli accessi ai modelli",
            "description": "Controlla se la whitelist del modello è configurata per limitare l'utilizzo del modello AI"
          },
          "ai_data_filtering": {
            "name": "Filtraggio dei dati sensibili",
            "description": "Controlla se i dati sensibili vengono filtrati dal contesto AI"
          },
          "network_rate_limiting": {
            "name": "Limitazione della velocità API",
            "description": "Controlla se la limitazione della velocità è configurata per prevenire abusi"
          },
          "network_cors": {
            "name": "Configurazione CORS",
            "description": "Controlla se CORS è correttamente limitato"
          },
          "network_tls": {
            "name": "Configurazione TLS/HTTPS",
            "description": "Controlla se TLS è abilitato per la comunicazione sicura"
          },
          "network_ip_blocking": {
            "name": "Funzionalità di blocco IP",
            "description": "Controlla se il blocco IP è disponibile e attivo"
          },
          "network_binding": {
            "name": "Associazione dell'interfaccia di rete",
            "description": "Controllare la configurazione del collegamento di rete del server"
          },
          "sandbox_enabled": {
            "name": "Esecuzione sandbox"
          },
          "sandbox_memory_limit": {
            "name": "Limiti di memoria",
            "description": "Controlla se i limiti di memoria sandbox sono configurati"
          },
          "sandbox_timeout": {
            "name": "Timeout di esecuzione",
            "description": "Controlla se il timeout di esecuzione è configurato"
          },
          "sandbox_network": {
            "name": "Isolamento della rete",
            "description": "Controlla se l'accesso alla rete sandbox è limitato"
          },
          "data_audit_logging": {
            "name": "Registrazione eventi di sicurezza",
            "description": "Controlla se gli eventi di sicurezza vengono registrati"
          },
          "data_directory_security": {
            "name": "Sicurezza della directory dei dati",
            "description": "Controlla se le directory dei dati dispongono delle autorizzazioni appropriate"
          },
          "system_debug_mode": {
            "name": "Modalità di debug",
            "description": "Controlla se la modalità debug è disabilitata in produzione"
          },
          "system_error_handling": {
            "name": "Esposizione delle informazioni sugli errori",
            "description": "Controlla se i messaggi di errore sono stati adeguatamente disinfettati per evitare perdite di informazioni"
          },
          "system_error_logging": {
            "name": "Registrazione degli errori sensibili",
            "description": "Controlla se i dati sensibili negli errori vengono gestiti correttamente nei log"
          },
          "system_environment": {
            "name": "Configurazione dell'ambiente",
            "description": "Controlla se l'ambiente è configurato correttamente"
          },
          "system_go_version": {
            "name": "Vai alla versione runtime della lingua",
            "description": "Controlla se il runtime della lingua Go è aggiornato"
          },
          "system_memory": {
            "name": "Utilizzo della memoria",
            "description": "Controlla l'utilizzo corrente della memoria"
          },
          "ai_prompt_injection": {
            "description": "Controllare se il rilevamento rapido dell'iniezione è abilitato"
          }
        }
      }
    }
  },
  "ja-JP": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "パスワードの複雑さの要件",
            "description": "パスワードの複雑さの要件が適切に設定されているかどうかを確認する"
          },
          "auth_account_lockout": {
            "name": "アカウント ロックアウト ポリシー",
            "description": "ブルート フォース攻撃を防ぐためにアカウント ロックアウトが設定されているかどうかを確認する"
          },
          "auth_jwt_secret": {
            "name": "JWT秘密鍵の強度",
            "description": "JWT シークレットが十分に強力かどうかを確認します (32 バイト以上を推奨)"
          },
          "auth_session_timeout": {
            "name": "セッションタイムアウト",
            "description": "セッションタイムアウトが適切に設定されているかどうかを確認します"
          },
          "auth_active_sessions": {
            "name": "アクティブなセッションの監視",
            "description": "アクティブなセッションが追跡されているかどうかを確認する"
          },
          "input_threat_detection": {
            "name": "脅威検出がアクティブです",
            "description": "入力脅威検出が実行されているかどうかを確認する"
          },
          "input_xss_protection": {
            "name": "XSS 保護",
            "description": "XSS 攻撃検出が有効になっているかどうかを確認する"
          },
          "input_sql_injection": {
            "name": "SQLインジェクション保護",
            "description": "SQLインジェクション検出が有効になっているかどうかを確認する"
          },
          "input_command_injection": {
            "name": "コマンドインジェクション保護",
            "description": "コマンドインジェクション検出が有効になっているかどうかを確認する"
          },
          "ai_output_validation": {
            "name": "AI出力の検証",
            "description": "AI 出力が実行前に検証されているかどうかを確認する"
          },
          "ai_model_access_control": {
            "name": "モデルのアクセス制御",
            "description": "モデルのホワイトリストが AI モデルの使用を制限するように構成されているかどうかを確認する"
          },
          "ai_data_filtering": {
            "name": "機密データのフィルタリング",
            "description": "機密データが AI コンテキストからフィルタリングされているかどうかを確認する"
          },
          "network_rate_limiting": {
            "name": "API レート制限",
            "description": "不正行為を防止するためにレート制限が設定されているかどうかを確認する"
          },
          "network_cors": {
            "name": "CORS の構成",
            "description": "CORS が適切に制限されているかどうかを確認する"
          },
          "network_tls": {
            "name": "TLS/HTTPS 構成",
            "description": "安全な通信のために TLS が有効になっているかどうかを確認する"
          },
          "network_ip_blocking": {
            "name": "IPブロッキング機能",
            "description": "IP ブロッキングが利用可能でアクティブであるかどうかを確認する"
          },
          "network_binding": {
            "name": "ネットワークインターフェースバインディング",
            "description": "サーバーのネットワーク バインディング構成を確認する"
          },
          "sandbox_enabled": {
            "name": "サンドボックス実行"
          },
          "sandbox_memory_limit": {
            "name": "メモリ制限",
            "description": "サンドボックスのメモリ制限が設定されているかどうかを確認する"
          },
          "sandbox_timeout": {
            "name": "実行タイムアウト",
            "description": "実行タイムアウトが設定されているかどうかを確認する"
          },
          "sandbox_network": {
            "name": "ネットワークの分離",
            "description": "サンドボックス ネットワーク アクセスが制限されているかどうかを確認する"
          },
          "data_audit_logging": {
            "name": "セキュリティイベントのログ記録",
            "description": "セキュリティ イベントがログに記録されているかどうかを確認する"
          },
          "data_directory_security": {
            "name": "データディレクトリのセキュリティ",
            "description": "データ ディレクトリに適切な権限があるかどうかを確認する"
          },
          "system_debug_mode": {
            "name": "デバッグモード",
            "description": "本番環境でデバッグモードが無効になっているかどうかを確認する"
          },
          "system_error_handling": {
            "name": "エラー情報の暴露",
            "description": "情報漏洩を防ぐためにエラーメッセージが適切にサニタイズされているかを確認する"
          },
          "system_error_logging": {
            "name": "機密エラーのログ記録",
            "description": "エラー中の機密データがログで適切に処理されているかどうかを確認する"
          },
          "system_environment": {
            "name": "環境構成",
            "description": "環境が適切に構成されているかどうかを確認する"
          },
          "system_go_version": {
            "name": "Go言語ランタイムバージョン",
            "description": "Go 言語ランタイムが最新かどうかを確認する"
          },
          "system_memory": {
            "name": "メモリ使用量",
            "description": "現在のメモリ使用量を確認する"
          },
          "ai_prompt_injection": {
            "description": "プロンプトインジェクション検出が有効になっているかどうかを確認します"
          }
        }
      }
    }
  },
  "ko-KR": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "비밀번호 복잡성 요구 사항",
            "description": "비밀번호 복잡성 요구 사항이 올바르게 구성되었는지 확인하세요."
          },
          "auth_account_lockout": {
            "name": "계정 잠금 정책",
            "description": "무차별 대입 공격을 방지하도록 계정 잠금이 구성되어 있는지 확인하세요."
          },
          "auth_jwt_secret": {
            "name": "JWT 비밀 키 강도",
            "description": "JWT 비밀이 충분히 강력한지 확인하세요(32바이트 이상 권장)."
          },
          "auth_session_timeout": {
            "name": "세션 시간 초과",
            "description": "세션 시간 초과가 적절하게 구성되었는지 확인하세요."
          },
          "auth_active_sessions": {
            "name": "활성 세션 모니터링",
            "description": "활성 세션이 추적되고 있는지 확인"
          },
          "input_threat_detection": {
            "name": "위협 감지 활성",
            "description": "입력 위협 감지가 실행 중인지 확인"
          },
          "input_xss_protection": {
            "name": "XSS 보호",
            "description": "XSS 공격 탐지가 활성화되어 있는지 확인"
          },
          "input_sql_injection": {
            "name": "SQL 주입 방지",
            "description": "SQL 주입 감지가 활성화되어 있는지 확인하세요."
          },
          "input_command_injection": {
            "name": "명령 주입 보호",
            "description": "명령 주입 감지가 활성화되어 있는지 확인하세요."
          },
          "ai_output_validation": {
            "name": "AI 출력 검증",
            "description": "실행 전에 AI 출력이 검증되었는지 확인"
          },
          "ai_model_access_control": {
            "name": "모델 액세스 제어",
            "description": "AI 모델 사용을 제한하도록 모델 화이트리스트가 구성되어 있는지 확인하세요."
          },
          "ai_data_filtering": {
            "name": "민감한 데이터 필터링",
            "description": "민감한 데이터가 AI 컨텍스트에서 필터링되는지 확인"
          },
          "network_rate_limiting": {
            "name": "API 속도 제한",
            "description": "남용을 방지하기 위해 속도 제한이 구성되어 있는지 확인하십시오."
          },
          "network_cors": {
            "name": "CORS 구성",
            "description": "CORS가 올바르게 제한되었는지 확인하세요."
          },
          "network_tls": {
            "name": "TLS/HTTPS 구성",
            "description": "보안 통신을 위해 TLS가 활성화되어 있는지 확인하세요."
          },
          "network_ip_blocking": {
            "name": "IP 차단 기능",
            "description": "IP 차단이 가능하고 활성화되어 있는지 확인하세요."
          },
          "network_binding": {
            "name": "네트워크 인터페이스 바인딩",
            "description": "서버 네트워크 바인딩 구성 확인"
          },
          "sandbox_enabled": {
            "name": "샌드박스 실행"
          },
          "sandbox_memory_limit": {
            "name": "메모리 제한",
            "description": "샌드박스 메모리 제한이 구성되어 있는지 확인하세요."
          },
          "sandbox_timeout": {
            "name": "실행 시간 초과",
            "description": "실행 제한 시간이 구성되어 있는지 확인하세요."
          },
          "sandbox_network": {
            "name": "네트워크 격리",
            "description": "샌드박스 네트워크 액세스가 제한되어 있는지 확인하세요."
          },
          "data_audit_logging": {
            "name": "보안 이벤트 로깅",
            "description": "보안 이벤트가 기록되고 있는지 확인"
          },
          "data_directory_security": {
            "name": "데이터 디렉토리 보안",
            "description": "데이터 디렉터리에 적절한 권한이 있는지 확인하세요."
          },
          "system_debug_mode": {
            "name": "디버그 모드",
            "description": "프로덕션에서 디버그 모드가 비활성화되어 있는지 확인"
          },
          "system_error_handling": {
            "name": "오류 정보 노출",
            "description": "정보 유출을 방지하기 위해 오류 메시지가 제대로 삭제되었는지 확인하세요."
          },
          "system_error_logging": {
            "name": "민감한 오류 로깅",
            "description": "오류가 발생한 민감한 데이터가 로그에서 제대로 처리되었는지 확인"
          },
          "system_environment": {
            "name": "환경 구성",
            "description": "환경이 올바르게 구성되었는지 확인"
          },
          "system_go_version": {
            "name": "Go 언어 런타임 버전",
            "description": "Go 언어 런타임이 최신인지 확인하세요."
          },
          "system_memory": {
            "name": "메모리 사용량",
            "description": "현재 메모리 사용량 확인"
          },
          "ai_prompt_injection": {
            "description": "프롬프트 주입 감지가 활성화되어 있는지 확인하십시오."
          }
        }
      }
    }
  },
  "ml-IN": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "പാസ്‌വേഡ് സങ്കീർണ്ണത ആവശ്യകതകൾ",
            "description": "പാസ്‌വേഡ് സങ്കീർണ്ണത ആവശ്യകതകൾ ശരിയായി ക്രമീകരിച്ചിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "auth_account_lockout": {
            "name": "അക്കൗണ്ട് ലോക്കൗട്ട് നയം",
            "description": "ബ്രൂട്ട് ഫോഴ്‌സ് ആക്രമണങ്ങൾ തടയാൻ അക്കൗണ്ട് ലോക്കൗട്ട് കോൺഫിഗർ ചെയ്‌തിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "auth_jwt_secret": {
            "name": "JWT രഹസ്യ കീ ശക്തി",
            "description": "JWT രഹസ്യം വേണ്ടത്ര ശക്തമാണോയെന്ന് പരിശോധിക്കുക (32+ ബൈറ്റുകൾ ശുപാർശ ചെയ്യുന്നു)"
          },
          "auth_session_timeout": {
            "name": "സെഷൻ ടൈംഔട്ട്",
            "description": "സെഷൻ ടൈംഔട്ട് ഉചിതമായി ക്രമീകരിച്ചിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "auth_active_sessions": {
            "name": "സജീവമായ സെഷൻസ് മോണിറ്ററിംഗ്",
            "description": "സജീവമായ സെഷനുകൾ ട്രാക്ക് ചെയ്യുന്നുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "input_threat_detection": {
            "name": "ഭീഷണി കണ്ടെത്തൽ സജീവമാണ്",
            "description": "ഇൻപുട്ട് ഭീഷണി കണ്ടെത്തൽ പ്രവർത്തിക്കുന്നുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "input_xss_protection": {
            "name": "XSS സംരക്ഷണം",
            "description": "XSS ആക്രമണം കണ്ടെത്തൽ പ്രവർത്തനക്ഷമമാണോയെന്ന് പരിശോധിക്കുക"
          },
          "input_sql_injection": {
            "name": "SQL കുത്തിവയ്പ്പ് സംരക്ഷണം",
            "description": "SQL ഇഞ്ചക്ഷൻ കണ്ടെത്തൽ പ്രവർത്തനക്ഷമമാണോയെന്ന് പരിശോധിക്കുക"
          },
          "input_command_injection": {
            "name": "കമാൻഡ് ഇൻജക്ഷൻ സംരക്ഷണം",
            "description": "കമാൻഡ് ഇൻജക്ഷൻ കണ്ടെത്തൽ പ്രവർത്തനക്ഷമമാണോയെന്ന് പരിശോധിക്കുക"
          },
          "ai_output_validation": {
            "name": "AI ഔട്ട്പുട്ട് മൂല്യനിർണ്ണയം",
            "description": "നിർവ്വഹിക്കുന്നതിന് മുമ്പ് AI ഔട്ട്പുട്ടുകൾ സാധൂകരിക്കപ്പെട്ടിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "ai_model_access_control": {
            "name": "മോഡൽ ആക്സസ് കൺട്രോൾ",
            "description": "AI മോഡൽ ഉപയോഗം നിയന്ത്രിക്കാൻ മോഡൽ വൈറ്റ്‌ലിസ്റ്റ് കോൺഫിഗർ ചെയ്‌തിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "ai_data_filtering": {
            "name": "സെൻസിറ്റീവ് ഡാറ്റ ഫിൽട്ടറിംഗ്",
            "description": "AI സന്ദർഭത്തിൽ നിന്ന് സെൻസിറ്റീവ് ഡാറ്റ ഫിൽട്ടർ ചെയ്തിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "network_rate_limiting": {
            "name": "API നിരക്ക് പരിമിതപ്പെടുത്തൽ",
            "description": "ദുരുപയോഗം തടയാൻ നിരക്ക് പരിധി ക്രമീകരിച്ചിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "network_cors": {
            "name": "CORS കോൺഫിഗറേഷൻ",
            "description": "CORS ശരിയായി നിയന്ത്രിച്ചിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "network_tls": {
            "name": "TLS/HTTPS കോൺഫിഗറേഷൻ",
            "description": "സുരക്ഷിതമായ ആശയവിനിമയത്തിനായി TLS പ്രവർത്തനക്ഷമമാക്കിയിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "network_ip_blocking": {
            "name": "ഐപി തടയൽ ശേഷി",
            "description": "ഐപി തടയൽ ലഭ്യമാണോ എന്നും സജീവമാണോ എന്നും പരിശോധിക്കുക"
          },
          "network_binding": {
            "name": "നെറ്റ്‌വർക്ക് ഇൻ്റർഫേസ് ബൈൻഡിംഗ്",
            "description": "സെർവർ നെറ്റ്‌വർക്ക് ബൈൻഡിംഗ് കോൺഫിഗറേഷൻ പരിശോധിക്കുക"
          },
          "sandbox_enabled": {
            "name": "സാൻഡ്ബോക്സ് എക്സിക്യൂഷൻ"
          },
          "sandbox_memory_limit": {
            "name": "മെമ്മറി പരിധികൾ",
            "description": "സാൻഡ്‌ബോക്‌സ് മെമ്മറി പരിധികൾ ക്രമീകരിച്ചിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "sandbox_timeout": {
            "name": "എക്സിക്യൂഷൻ ടൈംഔട്ട്",
            "description": "എക്സിക്യൂഷൻ ടൈംഔട്ട് കോൺഫിഗർ ചെയ്തിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "sandbox_network": {
            "name": "നെറ്റ്‌വർക്ക് ഐസൊലേഷൻ",
            "description": "സാൻഡ്‌ബോക്‌സ് നെറ്റ്‌വർക്ക് ആക്‌സസ് നിയന്ത്രിച്ചിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "data_audit_logging": {
            "name": "സുരക്ഷാ ഇവൻ്റ് ലോഗിംഗ്",
            "description": "സുരക്ഷാ ഇവൻ്റുകൾ ലോഗിൻ ചെയ്തിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "data_directory_security": {
            "name": "ഡാറ്റ ഡയറക്ടറി സുരക്ഷ",
            "description": "ഡാറ്റ ഡയറക്‌ടറികൾക്ക് ഉചിതമായ അനുമതികളുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "system_debug_mode": {
            "name": "ഡീബഗ് മോഡ്",
            "description": "നിർമ്മാണത്തിൽ ഡീബഗ് മോഡ് പ്രവർത്തനരഹിതമാണോയെന്ന് പരിശോധിക്കുക"
          },
          "system_error_handling": {
            "name": "വിവരങ്ങളുടെ വെളിപ്പെടുത്തൽ പിശക്",
            "description": "വിവര ചോർച്ച തടയാൻ പിശക് സന്ദേശങ്ങൾ ശരിയായി അണുവിമുക്തമാക്കിയിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "system_error_logging": {
            "name": "സെൻസിറ്റീവ് പിശക് ലോഗിംഗ്",
            "description": "പിശകുകളിലെ സെൻസിറ്റീവ് ഡാറ്റ ലോഗുകളിൽ ശരിയായി കൈകാര്യം ചെയ്തിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "system_environment": {
            "name": "പരിസ്ഥിതി കോൺഫിഗറേഷൻ",
            "description": "പരിസ്ഥിതി ശരിയായി ക്രമീകരിച്ചിട്ടുണ്ടോയെന്ന് പരിശോധിക്കുക"
          },
          "system_go_version": {
            "name": "ഭാഷ റൺടൈം പതിപ്പിലേക്ക് പോകുക",
            "description": "Go ഭാഷ റൺടൈം കാലികമാണോയെന്ന് പരിശോധിക്കുക"
          },
          "system_memory": {
            "name": "മെമ്മറി ഉപയോഗം",
            "description": "നിലവിലെ മെമ്മറി ഉപയോഗം പരിശോധിക്കുക"
          },
          "ai_prompt_injection": {
            "description": "പെട്ടെന്നുള്ള കുത്തിവയ്പ്പ് കണ്ടെത്തൽ പ്രവർത്തനക്ഷമമാണോയെന്ന് പരിശോധിക്കുക"
          }
        }
      }
    }
  },
  "nb-NO": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Krav til passordkompleksitet",
            "description": "Sjekk om kravene til passordkompleksitet er riktig konfigurert"
          },
          "auth_account_lockout": {
            "name": "Retningslinjer for låsing av kontoer",
            "description": "Sjekk om kontosperring er konfigurert for å forhindre brute force-angrep"
          },
          "auth_jwt_secret": {
            "name": "JWT hemmelig nøkkelstyrke",
            "description": "Sjekk om JWT-hemmeligheten er tilstrekkelig sterk (32+ byte anbefales)"
          },
          "auth_session_timeout": {
            "name": "Tidsavbrudd for økten",
            "description": "Sjekk om timeout for økten er riktig konfigurert"
          },
          "auth_active_sessions": {
            "name": "Overvåking av aktive økter",
            "description": "Sjekk om aktive økter spores"
          },
          "input_threat_detection": {
            "name": "Trusseldeteksjon aktiv",
            "description": "Sjekk om gjenkjenning av inngangstrussel kjører"
          },
          "input_xss_protection": {
            "name": "XSS beskyttelse",
            "description": "Sjekk om XSS-angrepsdeteksjon er aktivert"
          },
          "input_sql_injection": {
            "name": "SQL-injeksjonsbeskyttelse",
            "description": "Sjekk om SQL-injeksjonsdeteksjon er aktivert"
          },
          "input_command_injection": {
            "name": "Kommando-injeksjonsbeskyttelse",
            "description": "Sjekk om kommandoinjeksjonsdeteksjon er aktivert"
          },
          "ai_output_validation": {
            "name": "Validering av AI-utdata",
            "description": "Sjekk om AI-utganger er validert før kjøring"
          },
          "ai_model_access_control": {
            "name": "Modelltilgangskontroll",
            "description": "Sjekk om modellhvitelisten er konfigurert til å begrense bruken av AI-modeller"
          },
          "ai_data_filtering": {
            "name": "Sensitive datafiltrering",
            "description": "Sjekk om sensitive data er filtrert fra AI-kontekst"
          },
          "network_rate_limiting": {
            "name": "API-hastighetsbegrensning",
            "description": "Sjekk om hastighetsbegrensning er konfigurert for å forhindre misbruk"
          },
          "network_cors": {
            "name": "CORS-konfigurasjon",
            "description": "Sjekk om CORS er riktig begrenset"
          },
          "network_tls": {
            "name": "TLS/HTTPS-konfigurasjon",
            "description": "Sjekk om TLS er aktivert for sikker kommunikasjon"
          },
          "network_ip_blocking": {
            "name": "IP-blokkeringsevne",
            "description": "Sjekk om IP-blokkering er tilgjengelig og aktiv"
          },
          "network_binding": {
            "name": "Nettverksgrensesnittbinding",
            "description": "Sjekk servernettverksbindingskonfigurasjonen"
          },
          "sandbox_enabled": {
            "name": "Sandkasseutførelse"
          },
          "sandbox_memory_limit": {
            "name": "Minnegrenser",
            "description": "Sjekk om minnegrenser for sandkasse er konfigurert"
          },
          "sandbox_timeout": {
            "name": "Tidsavbrudd for utførelse",
            "description": "Sjekk om utføringstidsavbrudd er konfigurert"
          },
          "sandbox_network": {
            "name": "Nettverksisolasjon",
            "description": "Sjekk om tilgangen til sandkassenettverket er begrenset"
          },
          "data_audit_logging": {
            "name": "Logging av sikkerhetshendelser",
            "description": "Sjekk om sikkerhetshendelser blir logget"
          },
          "data_directory_security": {
            "name": "Datakatalogsikkerhet",
            "description": "Sjekk om datakataloger har passende tillatelser"
          },
          "system_debug_mode": {
            "name": "Feilsøkingsmodus",
            "description": "Sjekk om feilsøkingsmodus er deaktivert i produksjonen"
          },
          "system_error_handling": {
            "name": "Feilinformasjon Eksponering",
            "description": "Sjekk om feilmeldinger er ordentlig renset for å forhindre informasjonslekkasje"
          },
          "system_error_logging": {
            "name": "Sensitiv feillogging",
            "description": "Sjekk om sensitive data i feil er riktig håndtert i logger"
          },
          "system_environment": {
            "name": "Miljøkonfigurasjon",
            "description": "Sjekk om miljøet er riktig konfigurert"
          },
          "system_go_version": {
            "name": "Go language runtime versjon",
            "description": "Sjekk om Go-språkets kjøretid er oppdatert"
          },
          "system_memory": {
            "name": "Minnebruk",
            "description": "Sjekk gjeldende minnebruk"
          },
          "ai_prompt_injection": {
            "description": "Sjekk om prompt injeksjonsdeteksjon er aktivert"
          }
        }
      }
    }
  },
  "nl-NL": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Vereisten voor wachtwoordcomplexiteit",
            "description": "Controleer of de vereisten voor wachtwoordcomplexiteit correct zijn geconfigureerd"
          },
          "auth_account_lockout": {
            "name": "Beleid voor accountvergrendeling",
            "description": "Controleer of accountvergrendeling is geconfigureerd om brute force-aanvallen te voorkomen"
          },
          "auth_jwt_secret": {
            "name": "JWT geheime sleutelsterkte",
            "description": "Controleer of het JWT-geheim voldoende sterk is (32+ bytes aanbevolen)"
          },
          "auth_session_timeout": {
            "name": "Sessietime-out",
            "description": "Controleer of de sessietime-out correct is geconfigureerd"
          },
          "auth_active_sessions": {
            "name": "Actieve sessiebewaking",
            "description": "Controleer of actieve sessies worden bijgehouden"
          },
          "input_threat_detection": {
            "name": "Bedreigingsdetectie actief",
            "description": "Controleer of detectie van invoerbedreigingen actief is"
          },
          "input_xss_protection": {
            "name": "XSS-bescherming",
            "description": "Controleer of XSS-aanvalsdetectie is ingeschakeld"
          },
          "input_sql_injection": {
            "name": "Bescherming tegen SQL-injectie",
            "description": "Controleer of SQL-injectiedetectie is ingeschakeld"
          },
          "input_command_injection": {
            "name": "Commando-injectiebescherming",
            "description": "Controleer of detectie van commando-injectie is ingeschakeld"
          },
          "ai_output_validation": {
            "name": "Validatie van AI-uitvoer",
            "description": "Controleer of AI-uitvoer wordt gevalideerd voordat deze wordt uitgevoerd"
          },
          "ai_model_access_control": {
            "name": "Modeltoegangscontrole",
            "description": "Controleer of de witte lijst van modellen is geconfigureerd om het gebruik van AI-modellen te beperken"
          },
          "ai_data_filtering": {
            "name": "Gevoelige gegevensfiltering",
            "description": "Controleer of gevoelige gegevens uit de AI-context worden gefilterd"
          },
          "network_rate_limiting": {
            "name": "API-snelheidslimiet",
            "description": "Controleer of snelheidsbeperking is geconfigureerd om misbruik te voorkomen"
          },
          "network_cors": {
            "name": "CORS-configuratie",
            "description": "Controleer of CORS op de juiste manier is beperkt"
          },
          "network_tls": {
            "name": "TLS/HTTPS-configuratie",
            "description": "Controleer of TLS is ingeschakeld voor beveiligde communicatie"
          },
          "network_ip_blocking": {
            "name": "IP-blokkeermogelijkheid",
            "description": "Controleer of IP-blokkering beschikbaar en actief is"
          },
          "network_binding": {
            "name": "Netwerkinterfacebinding",
            "description": "Controleer de configuratie van de servernetwerkbinding"
          },
          "sandbox_enabled": {
            "name": "Sandbox-uitvoering"
          },
          "sandbox_memory_limit": {
            "name": "Geheugenlimieten",
            "description": "Controleer of de sandbox-geheugenlimieten zijn geconfigureerd"
          },
          "sandbox_timeout": {
            "name": "Time-out voor uitvoering",
            "description": "Controleer of een time-out voor uitvoering is geconfigureerd"
          },
          "sandbox_network": {
            "name": "Netwerkisolatie",
            "description": "Controleer of de toegang tot het sandboxnetwerk beperkt is"
          },
          "data_audit_logging": {
            "name": "Registratie van beveiligingsgebeurtenissen",
            "description": "Controleer of beveiligingsgebeurtenissen worden geregistreerd"
          },
          "data_directory_security": {
            "name": "Beveiliging van gegevensdirectory's",
            "description": "Controleer of gegevensmappen de juiste machtigingen hebben"
          },
          "system_debug_mode": {
            "name": "Foutopsporingsmodus",
            "description": "Controleer of de foutopsporingsmodus is uitgeschakeld in de productie"
          },
          "system_error_handling": {
            "name": "Foutinformatie Blootstelling",
            "description": "Controleer of foutmeldingen op de juiste manier zijn opgeschoond om het lekken van informatie te voorkomen"
          },
          "system_error_logging": {
            "name": "Gevoelige foutregistratie",
            "description": "Controleer of gevoelige gegevens bij fouten op de juiste manier worden verwerkt in logboeken"
          },
          "system_environment": {
            "name": "Omgevingsconfiguratie",
            "description": "Controleer of de omgeving correct is geconfigureerd"
          },
          "system_go_version": {
            "name": "Ga naar de taalruntimeversie",
            "description": "Controleer of de Go-taalruntime up-to-date is"
          },
          "system_memory": {
            "name": "Geheugengebruik",
            "description": "Controleer het huidige geheugengebruik"
          },
          "ai_prompt_injection": {
            "description": "Controleer of snelle injectiedetectie is ingeschakeld"
          }
        }
      }
    }
  },
  "pl-PL": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Wymagania dotyczące złożoności hasła",
            "description": "Sprawdź, czy wymagania dotyczące złożoności hasła są prawidłowo skonfigurowane"
          },
          "auth_account_lockout": {
            "name": "Polityka blokady konta",
            "description": "Sprawdź, czy skonfigurowano blokadę konta, aby zapobiec atakom typu brute-force"
          },
          "auth_jwt_secret": {
            "name": "Siła tajnego klucza JWT",
            "description": "Sprawdź, czy sekret JWT jest wystarczająco silny (zalecane ponad 32 bajty)"
          },
          "auth_session_timeout": {
            "name": "Limit czasu sesji",
            "description": "Sprawdź, czy limit czasu sesji jest odpowiednio skonfigurowany"
          },
          "auth_active_sessions": {
            "name": "Monitorowanie aktywnych sesji",
            "description": "Sprawdź, czy śledzone są aktywne sesje"
          },
          "input_threat_detection": {
            "name": "Wykrywanie zagrożeń aktywne",
            "description": "Sprawdź, czy działa wykrywanie zagrożeń wejściowych"
          },
          "input_xss_protection": {
            "name": "Ochrona XSS",
            "description": "Sprawdź, czy włączone jest wykrywanie ataków XSS"
          },
          "input_sql_injection": {
            "name": "Ochrona przed wtryskiem SQL",
            "description": "Sprawdź, czy jest włączone wykrywanie iniekcji SQL"
          },
          "input_command_injection": {
            "name": "Ochrona wtrysku poleceń",
            "description": "Sprawdź, czy włączone jest wykrywanie wstrzykiwania poleceń"
          },
          "ai_output_validation": {
            "name": "Walidacja wyników AI",
            "description": "Przed wykonaniem sprawdź, czy wyniki AI są sprawdzane"
          },
          "ai_model_access_control": {
            "name": "Modelowa kontrola dostępu",
            "description": "Sprawdź, czy biała lista modeli jest skonfigurowana tak, aby ograniczać użycie modelu AI"
          },
          "ai_data_filtering": {
            "name": "Wrażliwe filtrowanie danych",
            "description": "Sprawdź, czy wrażliwe dane są filtrowane z kontekstu AI"
          },
          "network_rate_limiting": {
            "name": "Ograniczanie szybkości interfejsu API",
            "description": "Sprawdź, czy skonfigurowano ograniczanie szybkości, aby zapobiec nadużyciom"
          },
          "network_cors": {
            "name": "Konfiguracja CORS-a",
            "description": "Sprawdź, czy CORS jest odpowiednio ograniczony"
          },
          "network_tls": {
            "name": "Konfiguracja TLS/HTTPS",
            "description": "Sprawdź, czy TLS jest włączony dla bezpiecznej komunikacji"
          },
          "network_ip_blocking": {
            "name": "Możliwość blokowania adresów IP",
            "description": "Sprawdź, czy blokowanie adresów IP jest dostępne i aktywne"
          },
          "network_binding": {
            "name": "Powiązanie interfejsu sieciowego",
            "description": "Sprawdź konfigurację powiązania sieciowego serwera"
          },
          "sandbox_enabled": {
            "name": "Wykonanie piaskownicy"
          },
          "sandbox_memory_limit": {
            "name": "Limity pamięci",
            "description": "Sprawdź, czy skonfigurowano limity pamięci piaskownicy"
          },
          "sandbox_timeout": {
            "name": "Limit czasu wykonania",
            "description": "Sprawdź, czy skonfigurowano limit czasu wykonania"
          },
          "sandbox_network": {
            "name": "Izolacja sieci",
            "description": "Sprawdź, czy dostęp do sieci piaskownicy jest ograniczony"
          },
          "data_audit_logging": {
            "name": "Rejestrowanie zdarzeń związanych z bezpieczeństwem",
            "description": "Sprawdź, czy rejestrowane są zdarzenia związane z bezpieczeństwem"
          },
          "data_directory_security": {
            "name": "Bezpieczeństwo katalogu danych",
            "description": "Sprawdź, czy katalogi danych mają odpowiednie uprawnienia"
          },
          "system_debug_mode": {
            "name": "Tryb debugowania",
            "description": "Sprawdź, czy tryb debugowania jest wyłączony w środowisku produkcyjnym"
          },
          "system_error_handling": {
            "name": "Ujawnienie informacji o błędzie",
            "description": "Sprawdź, czy komunikaty o błędach są odpowiednio oczyszczone, aby zapobiec wyciekowi informacji"
          },
          "system_error_logging": {
            "name": "Rejestrowanie wrażliwych błędów",
            "description": "Sprawdź, czy wrażliwe dane w błędach są prawidłowo obsługiwane w logach"
          },
          "system_environment": {
            "name": "Konfiguracja środowiska",
            "description": "Sprawdź, czy środowisko jest poprawnie skonfigurowane"
          },
          "system_go_version": {
            "name": "Przejdź do wersji uruchomieniowej języka",
            "description": "Sprawdź, czy środowisko wykonawcze języka Go jest aktualne"
          },
          "system_memory": {
            "name": "Wykorzystanie pamięci",
            "description": "Sprawdź bieżące wykorzystanie pamięci"
          },
          "ai_prompt_injection": {
            "description": "Sprawdź, czy włączone jest szybkie wykrywanie wtrysku"
          }
        }
      }
    }
  },
  "pt-BR": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Requisitos de complexidade de senha",
            "description": "Verifique se os requisitos de complexidade da senha estão configurados corretamente"
          },
          "auth_account_lockout": {
            "name": "Política de bloqueio de conta",
            "description": "Verifique se o bloqueio da conta está configurado para evitar ataques de força bruta"
          },
          "auth_jwt_secret": {
            "name": "Força da chave secreta JWT",
            "description": "Verifique se o segredo JWT é suficientemente forte (recomenda-se mais de 32 bytes)"
          },
          "auth_session_timeout": {
            "name": "Tempo limite da sessão",
            "description": "Verifique se o tempo limite da sessão está configurado adequadamente"
          },
          "auth_active_sessions": {
            "name": "Monitoramento de sessões ativas",
            "description": "Verifique se as sessões ativas estão sendo rastreadas"
          },
          "input_threat_detection": {
            "name": "Detecção de ameaças ativa",
            "description": "Verifique se a detecção de ameaças de entrada está em execução"
          },
          "input_xss_protection": {
            "name": "Proteção XSS",
            "description": "Verifique se a detecção de ataque XSS está habilitada"
          },
          "input_sql_injection": {
            "name": "Proteção contra injeção SQL",
            "description": "Verifique se a detecção de injeção SQL está habilitada"
          },
          "input_command_injection": {
            "name": "Proteção de injeção de comando",
            "description": "Verifique se a detecção de injeção de comando está habilitada"
          },
          "ai_output_validation": {
            "name": "Validação de saída de IA",
            "description": "Verifique se as saídas de IA são validadas antes da execução"
          },
          "ai_model_access_control": {
            "name": "Controle de acesso ao modelo",
            "description": "Verifique se a lista de permissões do modelo está configurada para restringir o uso do modelo de IA"
          },
          "ai_data_filtering": {
            "name": "Filtragem de dados confidenciais",
            "description": "Verifique se os dados confidenciais são filtrados do contexto de IA"
          },
          "network_rate_limiting": {
            "name": "Limitação de taxa de API",
            "description": "Verifique se a limitação de taxa está configurada para evitar abusos"
          },
          "network_cors": {
            "name": "Configuração do CORS",
            "description": "Verifique se o CORS está devidamente restrito"
          },
          "network_tls": {
            "name": "Configuração TLS/HTTPS",
            "description": "Verifique se o TLS está habilitado para comunicação segura"
          },
          "network_ip_blocking": {
            "name": "Capacidade de bloqueio de IP",
            "description": "Verifique se o bloqueio de IP está disponível e ativo"
          },
          "network_binding": {
            "name": "Vinculação de interface de rede",
            "description": "Verifique a configuração de ligação de rede do servidor"
          },
          "sandbox_enabled": {
            "name": "Execução de sandbox"
          },
          "sandbox_memory_limit": {
            "name": "Limites de memória",
            "description": "Verifique se os limites de memória do sandbox estão configurados"
          },
          "sandbox_timeout": {
            "name": "Tempo limite de execução",
            "description": "Verifique se o tempo limite de execução está configurado"
          },
          "sandbox_network": {
            "name": "Isolamento de rede",
            "description": "Verifique se o acesso à rede sandbox é restrito"
          },
          "data_audit_logging": {
            "name": "Registro de eventos de segurança",
            "description": "Verifique se os eventos de segurança estão sendo registrados"
          },
          "data_directory_security": {
            "name": "Segurança do diretório de dados",
            "description": "Verifique se os diretórios de dados têm permissões apropriadas"
          },
          "system_debug_mode": {
            "name": "Modo de depuração",
            "description": "Verifique se o modo de depuração está desabilitado na produção"
          },
          "system_error_handling": {
            "name": "Exposição de informações de erro",
            "description": "Verifique se as mensagens de erro estão devidamente higienizadas para evitar vazamento de informações"
          },
          "system_error_logging": {
            "name": "Registro de erros confidenciais",
            "description": "Verifique se os dados confidenciais em erros são tratados corretamente nos logs"
          },
          "system_environment": {
            "name": "Configuração do ambiente",
            "description": "Verifique se o ambiente está configurado corretamente"
          },
          "system_go_version": {
            "name": "Versão de tempo de execução do idioma Go",
            "description": "Verifique se o tempo de execução da linguagem Go está atualizado"
          },
          "system_memory": {
            "name": "Uso de memória",
            "description": "Verifique o uso atual da memória"
          },
          "ai_prompt_injection": {
            "description": "Verifique se a detecção imediata de injeção está habilitada"
          }
        }
      }
    }
  },
  "pt-PT": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Requisitos de complexidade da palavra-passe",
            "description": "Verifique se os requisitos de complexidade da palavra-passe estão configurados corretamente"
          },
          "auth_account_lockout": {
            "name": "Política de bloqueio de conta",
            "description": "Verifique se o bloqueio da conta está configurado para evitar ataques de força bruta"
          },
          "auth_jwt_secret": {
            "name": "Força da chave secreta JWT",
            "description": "Verifique se o segredo JWT é suficientemente forte (recomenda-se mais de 32 bytes)"
          },
          "auth_session_timeout": {
            "name": "Tempo limite da sessão",
            "description": "Verifique se o tempo limite da sessão está configurado adequadamente"
          },
          "auth_active_sessions": {
            "name": "Monitorização de sessões ativas",
            "description": "Verifique se as sessões ativas estão a ser rastreadas"
          },
          "input_threat_detection": {
            "name": "Detecção de ameaças activa",
            "description": "Verifique se a deteção de ameaças de entrada está em execução"
          },
          "input_xss_protection": {
            "name": "Proteção XSS",
            "description": "Verifique se a deteção de ataque XSS está ativada"
          },
          "input_sql_injection": {
            "name": "Proteção contra injeção SQL",
            "description": "Verifique se a deteção de injeção SQL está ativada"
          },
          "input_command_injection": {
            "name": "Proteção de injeção de comando",
            "description": "Verifique se a deteção de injeção de comando está ativada"
          },
          "ai_output_validation": {
            "name": "Validação de saída de IA",
            "description": "Verifique se as saídas de IA são validadas antes da execução"
          },
          "ai_model_access_control": {
            "name": "Controlo de acesso ao modelo",
            "description": "Verifique se a lista de permissões do modelo está configurada para restringir a utilização do modelo de IA"
          },
          "ai_data_filtering": {
            "name": "Filtragem de dados confidenciais",
            "description": "Verifique se os dados confidenciais são filtrados do contexto de IA"
          },
          "network_rate_limiting": {
            "name": "Limitação de taxa API",
            "description": "Verifique se a limitação de taxa está configurada para evitar abusos"
          },
          "network_cors": {
            "name": "Configuração do CORS",
            "description": "Verifique se o CORS está devidamente restringido"
          },
          "network_tls": {
            "name": "Configuração TLS/HTTPS",
            "description": "Verifique se o TLS está ativado para uma comunicação segura"
          },
          "network_ip_blocking": {
            "name": "Capacidade de bloqueio de IP",
            "description": "Verifique se o bloqueio de IP está disponível e ativo"
          },
          "network_binding": {
            "name": "Ligação de interface de rede",
            "description": "Verifique a configuração de ligação de rede do servidor"
          },
          "sandbox_enabled": {
            "name": "Execução de sandbox"
          },
          "sandbox_memory_limit": {
            "name": "Limites de memória",
            "description": "Verifique se os limites de memória do sandbox estão configurados"
          },
          "sandbox_timeout": {
            "name": "Tempo limite de execução",
            "description": "Verifique se o tempo limite de execução está configurado"
          },
          "sandbox_network": {
            "name": "Isolamento de rede",
            "description": "Verifique se o acesso à rede sandbox é restrito"
          },
          "data_audit_logging": {
            "name": "Registo de eventos de segurança",
            "description": "Verifique se os eventos de segurança estão a ser registados"
          },
          "data_directory_security": {
            "name": "Segurança do diretório de dados",
            "description": "Verifique se os diretórios de dados têm permissões apropriadas"
          },
          "system_debug_mode": {
            "name": "Modo de depuração",
            "description": "Verifique se o modo de depuração está desativado na produção"
          },
          "system_error_handling": {
            "name": "Exposição de informação de erro",
            "description": "Verifique se as mensagens de erro estão devidamente higienizadas para evitar fugas de informação"
          },
          "system_error_logging": {
            "name": "Registo de erros confidenciais",
            "description": "Verifique se os dados confidenciais em erros são tratados corretamente nos registos"
          },
          "system_environment": {
            "name": "Configuração do ambiente",
            "description": "Verifique se o ambiente está corretamente configurado"
          },
          "system_go_version": {
            "name": "Versão de tempo de execução do idioma Go",
            "description": "Verifique se o tempo de execução da linguagem Go está atualizado"
          },
          "system_memory": {
            "name": "Uso de memória",
            "description": "Verifique o uso atual da memória"
          },
          "ai_prompt_injection": {
            "description": "Verifique se a deteção imediata de injeção está ativada"
          }
        }
      }
    }
  },
  "ro-RO": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Cerințe de complexitate a parolei",
            "description": "Verificați dacă cerințele de complexitate a parolei sunt configurate corect"
          },
          "auth_account_lockout": {
            "name": "Politica de blocare a contului",
            "description": "Verificați dacă blocarea contului este configurată pentru a preveni atacurile cu forță brută"
          },
          "auth_jwt_secret": {
            "name": "Puterea cheii secrete JWT",
            "description": "Verificați dacă secretul JWT este suficient de puternic (se recomandă 32+ octeți)"
          },
          "auth_session_timeout": {
            "name": "Timp de expirare a sesiunii",
            "description": "Verificați dacă expirarea sesiunii este configurată corespunzător"
          },
          "auth_active_sessions": {
            "name": "Monitorizarea sesiunilor active",
            "description": "Verificați dacă sesiunile active sunt urmărite"
          },
          "input_threat_detection": {
            "name": "Detectarea amenințărilor activă",
            "description": "Verificați dacă funcția de detectare a amenințărilor de intrare rulează"
          },
          "input_xss_protection": {
            "name": "Protecție XSS",
            "description": "Verificați dacă detectarea atacurilor XSS este activată"
          },
          "input_sql_injection": {
            "name": "Protecție prin injecție SQL",
            "description": "Verificați dacă detectarea injecției SQL este activată"
          },
          "input_command_injection": {
            "name": "Comanda protectie injectie",
            "description": "Verificați dacă detectarea injecției de comandă este activată"
          },
          "ai_output_validation": {
            "name": "Validarea ieșirii AI",
            "description": "Verificați dacă ieșirile AI sunt validate înainte de execuție"
          },
          "ai_model_access_control": {
            "name": "Control acces model",
            "description": "Verificați dacă lista albă a modelelor este configurată pentru a restricționa utilizarea modelului AI"
          },
          "ai_data_filtering": {
            "name": "Filtrarea datelor sensibile",
            "description": "Verificați dacă datele sensibile sunt filtrate din contextul AI"
          },
          "network_rate_limiting": {
            "name": "Limitarea ratei API",
            "description": "Verificați dacă limitarea ratei este configurată pentru a preveni abuzul"
          },
          "network_cors": {
            "name": "Configurație CORS",
            "description": "Verificați dacă CORS este restricționat corespunzător"
          },
          "network_tls": {
            "name": "Configurare TLS/HTTPS",
            "description": "Verificați dacă TLS este activat pentru comunicarea securizată"
          },
          "network_ip_blocking": {
            "name": "Capacitate de blocare IP",
            "description": "Verificați dacă blocarea IP este disponibilă și activă"
          },
          "network_binding": {
            "name": "Legarea interfeței de rețea",
            "description": "Verificați configurația legăturii rețelei serverului"
          },
          "sandbox_enabled": {
            "name": "Execuție Sandbox"
          },
          "sandbox_memory_limit": {
            "name": "Limite de memorie",
            "description": "Verificați dacă limitele de memorie sandbox sunt configurate"
          },
          "sandbox_timeout": {
            "name": "Timp de execuție",
            "description": "Verificați dacă timeout-ul de execuție este configurat"
          },
          "sandbox_network": {
            "name": "Izolarea rețelei",
            "description": "Verificați dacă accesul la rețea sandbox este restricționat"
          },
          "data_audit_logging": {
            "name": "Înregistrare evenimente de securitate",
            "description": "Verificați dacă sunt înregistrate evenimente de securitate"
          },
          "data_directory_security": {
            "name": "Securitatea directorului de date",
            "description": "Verificați dacă directoarele de date au permisiunile corespunzătoare"
          },
          "system_debug_mode": {
            "name": "Modul de depanare",
            "description": "Verificați dacă modul de depanare este dezactivat în producție"
          },
          "system_error_handling": {
            "name": "Expunerea la informații despre eroare",
            "description": "Verificați dacă mesajele de eroare sunt igienizate corespunzător pentru a preveni scurgerea de informații"
          },
          "system_error_logging": {
            "name": "Înregistrare sensibilă a erorilor",
            "description": "Verificați dacă datele sensibile din erori sunt tratate corect în jurnalele"
          },
          "system_environment": {
            "name": "Configurarea mediului",
            "description": "Verificați dacă mediul este configurat corect"
          },
          "system_go_version": {
            "name": "Go versiunea de rulare a limbii",
            "description": "Verificați dacă durata de execuție a limbii Go este actualizată"
          },
          "system_memory": {
            "name": "Utilizarea memoriei",
            "description": "Verificați utilizarea curentă a memoriei"
          },
          "ai_prompt_injection": {
            "description": "Verificați dacă detectarea promptă a injecției este activată"
          }
        }
      }
    }
  },
  "ru-RU": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Требования к сложности пароля",
            "description": "Проверьте, правильно ли настроены требования к сложности пароля."
          },
          "auth_account_lockout": {
            "name": "Политика блокировки учетной записи",
            "description": "Проверьте, настроена ли блокировка учетной записи для предотвращения атак методом перебора."
          },
          "auth_jwt_secret": {
            "name": "Сила секретного ключа JWT",
            "description": "Проверьте, достаточно ли надежен секрет JWT (рекомендуется 32+ байта)."
          },
          "auth_session_timeout": {
            "name": "Тайм-аут сеанса",
            "description": "Проверьте, правильно ли настроен тайм-аут сеанса"
          },
          "auth_active_sessions": {
            "name": "Мониторинг активных сессий",
            "description": "Проверьте, отслеживаются ли активные сеансы"
          },
          "input_threat_detection": {
            "name": "Обнаружение угроз активно",
            "description": "Проверьте, запущено ли обнаружение входных угроз"
          },
          "input_xss_protection": {
            "name": "XSS-защита",
            "description": "Проверьте, включено ли обнаружение XSS-атак."
          },
          "input_sql_injection": {
            "name": "Защита от SQL-инъекций",
            "description": "Проверьте, включено ли обнаружение SQL-инъекций"
          },
          "input_command_injection": {
            "name": "Защита от ввода команд",
            "description": "Проверьте, включено ли обнаружение внедрения команд"
          },
          "ai_output_validation": {
            "name": "Проверка вывода AI",
            "description": "Проверьте, проверены ли выходные данные AI перед выполнением"
          },
          "ai_model_access_control": {
            "name": "Модель контроля доступа",
            "description": "Проверьте, настроен ли белый список моделей для ограничения использования модели AI."
          },
          "ai_data_filtering": {
            "name": "Фильтрация конфиденциальных данных",
            "description": "Проверьте, фильтруются ли конфиденциальные данные из контекста AI."
          },
          "network_rate_limiting": {
            "name": "Ограничение скорости API",
            "description": "Проверьте, настроено ли ограничение скорости для предотвращения злоупотреблений."
          },
          "network_cors": {
            "name": "Конфигурация CORS",
            "description": "Проверьте, правильно ли ограничен CORS"
          },
          "network_tls": {
            "name": "Конфигурация TLS/HTTPS",
            "description": "Проверьте, включен ли TLS для безопасной связи."
          },
          "network_ip_blocking": {
            "name": "Возможность блокировки IP",
            "description": "Проверьте, доступна ли и активна ли блокировка IP"
          },
          "network_binding": {
            "name": "Привязка сетевого интерфейса",
            "description": "Проверьте конфигурацию привязки сети сервера"
          },
          "sandbox_enabled": {
            "name": "Выполнение в песочнице"
          },
          "sandbox_memory_limit": {
            "name": "Ограничения памяти",
            "description": "Проверьте, настроены ли ограничения памяти песочницы"
          },
          "sandbox_timeout": {
            "name": "Тайм-аут выполнения",
            "description": "Проверьте, настроен ли тайм-аут выполнения"
          },
          "sandbox_network": {
            "name": "Сетевая изоляция",
            "description": "Проверьте, ограничен ли доступ к сети песочницы"
          },
          "data_audit_logging": {
            "name": "Регистрация событий безопасности",
            "description": "Проверьте, регистрируются ли события безопасности"
          },
          "data_directory_security": {
            "name": "Безопасность каталога данных",
            "description": "Проверьте, имеют ли каталоги данных соответствующие разрешения."
          },
          "system_debug_mode": {
            "name": "Режим отладки",
            "description": "Проверьте, отключен ли режим отладки в производстве"
          },
          "system_error_handling": {
            "name": "Предоставление информации об ошибках",
            "description": "Проверьте, правильно ли обработаны сообщения об ошибках, чтобы предотвратить утечку информации."
          },
          "system_error_logging": {
            "name": "Чувствительная регистрация ошибок",
            "description": "Проверьте, правильно ли обрабатываются конфиденциальные данные в ошибках в журналах."
          },
          "system_environment": {
            "name": "Конфигурация среды",
            "description": "Проверьте, правильно ли настроена среда"
          },
          "system_go_version": {
            "name": "Версия среды выполнения Go на языке",
            "description": "Проверьте, обновлена ли среда выполнения языка Go."
          },
          "system_memory": {
            "name": "Использование памяти",
            "description": "Проверьте текущее использование памяти"
          },
          "ai_prompt_injection": {
            "description": "Проверьте, включено ли обнаружение быстрой инъекции"
          }
        }
      }
    }
  },
  "sk-SK": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Požiadavky na zložitosť hesla",
            "description": "Skontrolujte, či sú požiadavky na zložitosť hesla správne nakonfigurované"
          },
          "auth_account_lockout": {
            "name": "Zásady uzamknutia účtu",
            "description": "Skontrolujte, či je nakonfigurované uzamknutie účtu, aby sa zabránilo útokom hrubou silou"
          },
          "auth_jwt_secret": {
            "name": "Sila tajného kľúča JWT",
            "description": "Skontrolujte, či je tajomstvo JWT dostatočne silné (odporúča sa 32+ bajtov)"
          },
          "auth_session_timeout": {
            "name": "Časový limit relácie",
            "description": "Skontrolujte, či je správne nakonfigurovaný časový limit relácie"
          },
          "auth_active_sessions": {
            "name": "Monitorovanie aktívnych relácií",
            "description": "Skontrolujte, či sa aktívne relácie sledujú"
          },
          "input_threat_detection": {
            "name": "Detekcia hrozieb je aktívna",
            "description": "Skontrolujte, či je spustená detekcia vstupných hrozieb"
          },
          "input_xss_protection": {
            "name": "Ochrana XSS",
            "description": "Skontrolujte, či je povolená detekcia útokov XSS"
          },
          "input_sql_injection": {
            "name": "Ochrana pred vstrekovaním SQL",
            "description": "Skontrolujte, či je povolená detekcia vstrekovania SQL"
          },
          "input_command_injection": {
            "name": "Ochrana príkazového vstrekovania",
            "description": "Skontrolujte, či je aktivovaná detekcia vstrekovania príkazov"
          },
          "ai_output_validation": {
            "name": "Overenie výstupu AI",
            "description": "Pred vykonaním skontrolujte, či sú výstupy AI overené"
          },
          "ai_model_access_control": {
            "name": "Kontrola prístupu modelu",
            "description": "Skontrolujte, či je zoznam povolených modelov nakonfigurovaný na obmedzenie používania modelu AI"
          },
          "ai_data_filtering": {
            "name": "Filtrovanie citlivých údajov",
            "description": "Skontrolujte, či sú citlivé údaje filtrované z kontextu AI"
          },
          "network_rate_limiting": {
            "name": "Obmedzenie rýchlosti API",
            "description": "Skontrolujte, či je nakonfigurované obmedzenie rýchlosti, aby sa zabránilo zneužitiu"
          },
          "network_cors": {
            "name": "Konfigurácia CORS",
            "description": "Skontrolujte, či je CORS správne obmedzený"
          },
          "network_tls": {
            "name": "Konfigurácia TLS/HTTPS",
            "description": "Skontrolujte, či je povolené TLS pre zabezpečenú komunikáciu"
          },
          "network_ip_blocking": {
            "name": "Schopnosť blokovania IP",
            "description": "Skontrolujte, či je blokovanie IP dostupné a aktívne"
          },
          "network_binding": {
            "name": "Väzba sieťového rozhrania",
            "description": "Skontrolujte konfiguráciu sieťovej väzby servera"
          },
          "sandbox_enabled": {
            "name": "Spustenie v karanténe"
          },
          "sandbox_memory_limit": {
            "name": "Pamäťové limity",
            "description": "Skontrolujte, či sú nakonfigurované limity pamäte karantény"
          },
          "sandbox_timeout": {
            "name": "Časový limit vykonania",
            "description": "Skontrolujte, či je nakonfigurovaný časový limit vykonania"
          },
          "sandbox_network": {
            "name": "Izolácia siete",
            "description": "Skontrolujte, či nie je obmedzený prístup k sieti sandbox"
          },
          "data_audit_logging": {
            "name": "Zapisovanie bezpečnostných udalostí",
            "description": "Skontrolujte, či sa zaznamenávajú bezpečnostné udalosti"
          },
          "data_directory_security": {
            "name": "Zabezpečenie adresára údajov",
            "description": "Skontrolujte, či majú dátové adresáre príslušné povolenia"
          },
          "system_debug_mode": {
            "name": "Režim ladenia",
            "description": "Skontrolujte, či je v produkcii zakázaný režim ladenia"
          },
          "system_error_handling": {
            "name": "Vystavenie informácií o chybe",
            "description": "Skontrolujte, či sú chybové hlásenia správne vyčistené, aby sa zabránilo úniku informácií"
          },
          "system_error_logging": {
            "name": "Citlivé zaznamenávanie chýb",
            "description": "Skontrolujte, či sú citlivé údaje v chybách správne spracované v protokoloch"
          },
          "system_environment": {
            "name": "Konfigurácia prostredia",
            "description": "Skontrolujte, či je prostredie správne nakonfigurované"
          },
          "system_go_version": {
            "name": "Verzia jazykového modulu Go",
            "description": "Skontrolujte, či je jazykový modul Go aktuálny"
          },
          "system_memory": {
            "name": "Využitie pamäte",
            "description": "Skontrolujte aktuálne využitie pamäte"
          },
          "ai_prompt_injection": {
            "description": "Skontrolujte, či je aktivovaná detekcia rýchleho vstreknutia"
          }
        }
      }
    }
  },
  "sv-SE": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "Lösenordskomplexitetskrav",
            "description": "Kontrollera om kraven på lösenordskomplexitet är korrekt konfigurerade"
          },
          "auth_account_lockout": {
            "name": "Policy för låsning av konto",
            "description": "Kontrollera om kontolåsning är konfigurerad för att förhindra brute force-attacker"
          },
          "auth_jwt_secret": {
            "name": "JWT hemliga nyckelstyrka",
            "description": "Kontrollera om JWT-hemligheten är tillräckligt stark (32+ byte rekommenderas)"
          },
          "auth_session_timeout": {
            "name": "Timeout för session",
            "description": "Kontrollera om tidsgränsen för sessionen är korrekt konfigurerad"
          },
          "auth_active_sessions": {
            "name": "Övervakning av aktiva sessioner",
            "description": "Kontrollera om aktiva sessioner spåras"
          },
          "input_threat_detection": {
            "name": "Hotdetektion aktiv",
            "description": "Kontrollera om upptäckt av ingångshot körs"
          },
          "input_xss_protection": {
            "name": "XSS-skydd",
            "description": "Kontrollera om XSS-attackdetektering är aktiverat"
          },
          "input_sql_injection": {
            "name": "SQL-injektionsskydd",
            "description": "Kontrollera om SQL-injektionsdetektering är aktiverat"
          },
          "input_command_injection": {
            "name": "Kommando Injektionsskydd",
            "description": "Kontrollera om detektering av kommandoinsprutning är aktiverad"
          },
          "ai_output_validation": {
            "name": "AI-utgångsvalidering",
            "description": "Kontrollera om AI-utgångar är validerade innan exekvering"
          },
          "ai_model_access_control": {
            "name": "Modell Access Control",
            "description": "Kontrollera om modellvitlistan är konfigurerad för att begränsa användningen av AI-modeller"
          },
          "ai_data_filtering": {
            "name": "Filtrering av känsliga data",
            "description": "Kontrollera om känslig data filtreras från AI-kontext"
          },
          "network_rate_limiting": {
            "name": "API-hastighetsbegränsning",
            "description": "Kontrollera om hastighetsbegränsning är konfigurerad för att förhindra missbruk"
          },
          "network_cors": {
            "name": "CORS-konfiguration",
            "description": "Kontrollera om CORS är ordentligt begränsad"
          },
          "network_tls": {
            "name": "TLS/HTTPS-konfiguration",
            "description": "Kontrollera om TLS är aktiverat för säker kommunikation"
          },
          "network_ip_blocking": {
            "name": "IP-blockeringsförmåga",
            "description": "Kontrollera om IP-blockering är tillgänglig och aktiv"
          },
          "network_binding": {
            "name": "Nätverksgränssnittsbindning",
            "description": "Kontrollera serverns nätverksbindningskonfiguration"
          },
          "sandbox_enabled": {
            "name": "Utförande av sandlåda"
          },
          "sandbox_memory_limit": {
            "name": "Minnesgränser",
            "description": "Kontrollera om minnesgränser för sandlådan är konfigurerade"
          },
          "sandbox_timeout": {
            "name": "Exekveringstidsgräns",
            "description": "Kontrollera om exekveringstimeout är konfigurerad"
          },
          "sandbox_network": {
            "name": "Nätverksisolering",
            "description": "Kontrollera om nätverksåtkomsten till sandlådan är begränsad"
          },
          "data_audit_logging": {
            "name": "Loggning av säkerhetshändelser",
            "description": "Kontrollera om säkerhetshändelser loggas"
          },
          "data_directory_security": {
            "name": "Datakatalogsäkerhet",
            "description": "Kontrollera om datakataloger har lämpliga behörigheter"
          },
          "system_debug_mode": {
            "name": "Felsökningsläge",
            "description": "Kontrollera om felsökningsläget är inaktiverat i produktionen"
          },
          "system_error_handling": {
            "name": "Felinformation Exponering",
            "description": "Kontrollera om felmeddelanden är korrekt sanerade för att förhindra informationsläckage"
          },
          "system_error_logging": {
            "name": "Känslig felloggning",
            "description": "Kontrollera om känslig information i fel hanteras korrekt i loggar"
          },
          "system_environment": {
            "name": "Miljökonfiguration",
            "description": "Kontrollera om miljön är korrekt konfigurerad"
          },
          "system_go_version": {
            "name": "Go language runtime version",
            "description": "Kontrollera om Go-språkets körtid är uppdaterad"
          },
          "system_memory": {
            "name": "Minnesanvändning",
            "description": "Kontrollera aktuell minnesanvändning"
          },
          "ai_prompt_injection": {
            "description": "Kontrollera om snabb injektionsdetektering är aktiverad"
          }
        }
      }
    }
  },
  "zh-CN": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "密码复杂性要求",
            "description": "检查密码复杂性要求是否配置正确"
          },
          "auth_account_lockout": {
            "name": "账户锁定政策",
            "description": "检查是否配置了帐户锁定以防止暴力攻击"
          },
          "auth_jwt_secret": {
            "name": "JWT 密钥强度",
            "description": "检查 JWT 密钥是否足够强（建议 32+ 字节）"
          },
          "auth_session_timeout": {
            "name": "会话超时",
            "description": "检查会话超时是否配置正确"
          },
          "auth_active_sessions": {
            "name": "活动会话监控",
            "description": "检查是否正在跟踪活动会话"
          },
          "input_threat_detection": {
            "name": "威胁检测活跃",
            "description": "检查输入威胁检测是否正在运行"
          },
          "input_xss_protection": {
            "name": "XSS防护",
            "description": "检查是否开启XSS攻击检测"
          },
          "input_sql_injection": {
            "name": "SQL注入保护",
            "description": "检查是否启用SQL注入检测"
          },
          "input_command_injection": {
            "name": "命令注入保护",
            "description": "检查命令注入检测是否启用"
          },
          "ai_output_validation": {
            "name": "AI输出验证",
            "description": "执行前检查 AI 输出是否经过验证"
          },
          "ai_model_access_control": {
            "name": "模型访问控制",
            "description": "检查是否配置模型白名单限制AI模型使用"
          },
          "ai_data_filtering": {
            "name": "敏感数据过滤",
            "description": "检查敏感数据是否从 AI 上下文中过滤掉"
          },
          "network_rate_limiting": {
            "name": "API速率限制",
            "description": "检查是否配置了速率限制以防止滥用"
          },
          "network_cors": {
            "name": "CORS配置",
            "description": "检查 CORS 是否受到适当限制"
          },
          "network_tls": {
            "name": "TLS/HTTPS 配置",
            "description": "检查是否启用了 TLS 以实现安全通信"
          },
          "network_ip_blocking": {
            "name": "IP封锁能力",
            "description": "检查 IP 阻止是否可用且处于活动状态"
          },
          "network_binding": {
            "name": "网络接口绑定",
            "description": "检查服务器网络绑定配置"
          },
          "sandbox_enabled": {
            "name": "沙箱执行"
          },
          "sandbox_memory_limit": {
            "name": "内存限制",
            "description": "检查是否配置了沙箱内存限制"
          },
          "sandbox_timeout": {
            "name": "执行超时",
            "description": "检查是否配置了执行超时"
          },
          "sandbox_network": {
            "name": "网络隔离",
            "description": "检查沙盒网络访问是否受到限制"
          },
          "data_audit_logging": {
            "name": "安全事件记录",
            "description": "检查是否记录了安全事件"
          },
          "data_directory_security": {
            "name": "数据目录安全",
            "description": "检查数据目录是否有适当的权限"
          },
          "system_debug_mode": {
            "name": "调试模式",
            "description": "检查生产中是否禁用调试模式"
          },
          "system_error_handling": {
            "name": "错误信息暴露",
            "description": "检查错误消息是否已正确清理以防止信息泄露"
          },
          "system_error_logging": {
            "name": "敏感错误记录",
            "description": "检查日志中错误的敏感数据是否得到正确处理"
          },
          "system_environment": {
            "name": "环境配置",
            "description": "检查环境是否配置正确"
          },
          "system_go_version": {
            "name": "Go语言运行时版本",
            "description": "检查Go语言运行时是否是最新的"
          },
          "system_memory": {
            "name": "内存使用情况",
            "description": "检查当前内存使用情况"
          },
          "ai_prompt_injection": {
            "description": "检查是否启用提示注入检测"
          }
        }
      }
    }
  },
  "zh-TW": {
    "security": {
      "scan": {
        "items": {
          "auth_password_complexity": {
            "name": "密碼複雜度要求",
            "description": "檢查密碼複雜度要求是否配置正確"
          },
          "auth_account_lockout": {
            "name": "帳戶鎖定政策",
            "description": "檢查是否配置了帳戶鎖定以防止暴力攻擊"
          },
          "auth_jwt_secret": {
            "name": "JWT 密鑰強度",
            "description": "檢查 JWT 密鑰是否足夠強（建議 32+ 位元組）"
          },
          "auth_session_timeout": {
            "name": "會話逾時",
            "description": "檢查會話逾時是否配置正確"
          },
          "auth_active_sessions": {
            "name": "活動會話監控",
            "description": "檢查是否正在追蹤活動會話"
          },
          "input_threat_detection": {
            "name": "威脅偵測活躍",
            "description": "檢查輸入威脅偵測是否正在執行"
          },
          "input_xss_protection": {
            "name": "XSS防護",
            "description": "檢查是否開啟XSS攻擊偵測"
          },
          "input_sql_injection": {
            "name": "SQL注入保護",
            "description": "檢查是否啟用SQL注入偵測"
          },
          "input_command_injection": {
            "name": "命令注入保護",
            "description": "檢查命令注入檢測是否啟用"
          },
          "ai_output_validation": {
            "name": "AI輸出驗證",
            "description": "執行前檢查 AI 輸出是否經過驗證"
          },
          "ai_model_access_control": {
            "name": "模型存取控制",
            "description": "檢查是否配置模型白名單限制AI模型使用"
          },
          "ai_data_filtering": {
            "name": "敏感資料過濾",
            "description": "檢查敏感資料是否從 AI 上下文中過濾掉"
          },
          "network_rate_limiting": {
            "name": "API速率限制",
            "description": "檢查是否配置了速率限制以防止濫用"
          },
          "network_cors": {
            "name": "CORS配置",
            "description": "檢查 CORS 是否受到適當限制"
          },
          "network_tls": {
            "name": "TLS/HTTPS 配置",
            "description": "檢查是否啟用了 TLS 以實現安全通信"
          },
          "network_ip_blocking": {
            "name": "IP封鎖能力",
            "description": "檢查 IP 封鎖是否可用且處於活動狀態"
          },
          "network_binding": {
            "name": "網路介面綁定",
            "description": "檢查伺服器網路綁定配置"
          },
          "sandbox_enabled": {
            "name": "沙箱執行"
          },
          "sandbox_memory_limit": {
            "name": "記憶體限制",
            "description": "檢查是否配置了沙箱記憶體限制"
          },
          "sandbox_timeout": {
            "name": "執行逾時",
            "description": "檢查是否配置了執行逾時"
          },
          "sandbox_network": {
            "name": "網路隔離",
            "description": "檢查沙盒網路存取是否受到限制"
          },
          "data_audit_logging": {
            "name": "安全事件記錄",
            "description": "檢查是否記錄了安全事件"
          },
          "data_directory_security": {
            "name": "資料目錄安全",
            "description": "檢查資料目錄是否有適當的權限"
          },
          "system_debug_mode": {
            "name": "偵錯模式",
            "description": "檢查生產中是否停用調試模式"
          },
          "system_error_handling": {
            "name": "錯誤訊息暴露",
            "description": "檢查錯誤訊息是否已正確清理以防止資訊洩露"
          },
          "system_error_logging": {
            "name": "敏感錯誤記錄",
            "description": "檢查日誌中錯誤的敏感資料是否已正確處理"
          },
          "system_environment": {
            "name": "環境配置",
            "description": "檢查環境是否配置正確"
          },
          "system_go_version": {
            "name": "Go 語言執行階段版本",
            "description": "檢查 Go 語言執行階段是否為最新版本"
          },
          "system_memory": {
            "name": "記憶體使用情況",
            "description": "檢查當前記憶體使用情況"
          },
          "ai_prompt_injection": {
            "description": "檢查是否啟用提示注入偵測"
          }
        }
      }
    }
  }
} as Partial<Record<LocaleKey, object>>

export default securityScanItemBackfills
