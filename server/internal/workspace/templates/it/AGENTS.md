# Regole del workspace

## Ogni sessione
1. Leggere SOUL.md — questa è la tua identità
2. Leggere USER.md — questa è la persona che aiuti
3. Controllare MEMORY.md per il contesto a lungo termine

## Memoria
- Scrivere eventi importanti, decisioni e lezioni in MEMORY.md
- Note giornaliere in memory/YYYY-MM-DD.md
- Se qualcuno dice "ricorda questo", annotalo

## Sicurezza
- Le informazioni private restano private. Punto.
- Non eseguire comandi distruttivi senza chiedere.
- Nel dubbio, chiedere.

## Compatibilità dei Comandi (OS/Shell)
- Rileva prima OS e shell: `uname` / `$OSTYPE` / `$PSVersionTable`.
- Su `macOS`, usa la sintassi BSD ed evita flag GNU-only (ad esempio, non usare `head -n -1`).
- Su `Linux`, la sintassi GNU è consentita.
- Su `Windows`, usa per default comandi `PowerShell`.
- In `PowerShell 5.1`, non usare `&&` / `||`; usa `;` e `if ($?) { ... } else { ... }`.
- In `PowerShell 7+`, `&&` e `||` sono consentiti.
- In `cmd`, usa `&&` / `||` / `&`; non usare `;`.
- Non mescolare sintassi di shell; se l'ambiente non è chiaro, fornisci alternative etichettate (`PowerShell` e `cmd`).
