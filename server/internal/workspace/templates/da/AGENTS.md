# Workspace-regler

## Hver session
1. Læs SOUL.md — dette er din identitet
2. Læs USER.md — dette er personen, du hjælper
3. Tjek MEMORY.md for langsigtet kontekst

## Hukommelse
- Skriv vigtige begivenheder, beslutninger og erfaringer i MEMORY.md
- Daglige noter i memory/YYYY-MM-DD.md
- Hvis nogen siger "husk dette", skriv det ned

## Sikkerhed
- Private oplysninger forbliver private. Punktum.
- Kør ikke destruktive kommandoer uden at spørge.
- Spørg ved tvivl.

## Kommando-kompatibilitet (OS/Shell)
- Registrer først OS og shell: `uname` / `$OSTYPE` / `$PSVersionTable`.
- På `macOS`, brug BSD-syntaks og undgå GNU-only flag (f.eks. brug ikke `head -n -1`).
- På `Linux` er GNU-syntaks tilladt.
- På `Windows`, brug `PowerShell`-kommandoer som standard.
- I `PowerShell 5.1`, brug ikke `&&` / `||`; brug `;` og `if ($?) { ... } else { ... }`.
- I `PowerShell 7+` er `&&` og `||` tilladt.
- I `cmd`, brug `&&` / `||` / `&`; brug ikke `;`.
- Bland ikke shell-syntaks; hvis miljøet er uklart, giv mærkede alternativer (`PowerShell` og `cmd`).
