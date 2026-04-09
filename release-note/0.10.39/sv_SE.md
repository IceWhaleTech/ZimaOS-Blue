# ZimaOS Blue v0.10.39

Släppt: 2026-04-09

Enhetliga Research-arbetsflöden, nya arbetsytor för Knowledge och Evolution, starkare återställning av konversationer samt förbättrad tillförlitlighet för dokumentextraktion och körning.

## Höjdpunkter

- Ett mer enhetligt arbetsflöde för research-, knowledge- och evolution-uppgifter
- Bättre återställning när du återvänder till aktiva eller väntande konversationer
- Starkare tillförlitlighet för dokumentextraktion och kontrollerad körning

## Nytt

- Lade till en enhetlig `Research`-ingång samtidigt som lägesspecifik utdata bevaras
- Lade till en `Knowledge`-arbetsyta för kompilerade sidor, lint-status och ask-and-archive-jobb
- Lade till en `Evolution`-konsol och lokala `blue audit`-verktyg för granskning och diagnostik

## Förbättrat

- Förbättrade bootstrap för konversationer så att återöppnade chattar återställer mer aktivt tillstånd och väntande godkännanden
- Förbättrade Harness-datasetflöden för mer upprepningsbara utvärderingskörningar
- Förbättrade kontextkomprimering, failover-hantering och lokaliseringskonsistens

## Åtgärdat

- Åtgärdade uppdateringsloopar i verifieringen av leverantörskatalogen
- Åtgärdade scenarier med upprepade läsloopar med artifact recovery
- Åtgärdade reparation av felaktiga tecken vid PDF-textextraktion

## Säkerhet

- Härdade kommandokörning med `blue exec` och nivåstyrd sandbox-routing

## Anteckningar

Om du stöter på några problem kan du gå med i Zima-communityn på Discord för att få stöd från över 43 000 medlemmar:

https://zimaboard.com/discord
