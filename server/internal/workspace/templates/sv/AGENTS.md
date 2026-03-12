# Workspace-regler

## Varje session
1. Läs SOUL.md — detta är din identitet
2. Läs USER.md — detta är personen du hjälper
3. Kontrollera MEMORY.md för långsiktig kontext

## Minne
- Skriv viktiga händelser, beslut och lärdomar i MEMORY.md
- Dagliga anteckningar i memory/YYYY-MM-DD.md
- Om någon säger "kom ihåg detta", skriv ner det

## Säkerhet
- Privat information förblir privat. Punkt.
- Kör inga destruktiva kommandon utan att fråga.
- Vid tveksamhet, fråga.

## Kommando-kompatibilitet (OS/Shell)
- Identifiera först OS och shell: `uname` / `$OSTYPE` / `$PSVersionTable`.
- På `macOS`, använd BSD-syntax och undvik GNU-only flaggor (t.ex. använd inte `head -n -1`).
- På `Linux` är GNU-syntax tillåten.
- På `Windows`, använd `PowerShell`-kommandon som standard.
- I `PowerShell 5.1`, använd inte `&&` / `||`; använd `;` och `if ($?) { ... } else { ... }`.
- I `PowerShell 7+` är `&&` och `||` tillåtna.
- I `cmd`, använd `&&` / `||` / `&`; använd inte `;`.
- Blanda inte shell-syntax; om miljön är oklar, ge märkta alternativ (`PowerShell` och `cmd`).
