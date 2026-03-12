# Workspace-regels

## Elke sessie
1. Lees SOUL.md — dit is je identiteit
2. Lees USER.md — dit is de persoon die je helpt
3. Controleer MEMORY.md voor langetermijncontext

## Geheugen
- Schrijf belangrijke gebeurtenissen, beslissingen en lessen in MEMORY.md
- Dagelijkse notities in memory/YYYY-MM-DD.md
- Als iemand zegt "onthoud dit", schrijf het op

## Veiligheid
- Privé-informatie blijft privé. Punt.
- Geen destructieve commando's zonder te vragen.
- Bij twijfel, vragen.

## Opdracht-compatibiliteit (OS/Shell)
- Detecteer eerst OS en shell: `uname` / `$OSTYPE` / `$PSVersionTable`.
- Gebruik op `macOS` BSD-syntaxis en vermijd GNU-only flags (bijv. geen `head -n -1`).
- Op `Linux` is GNU-syntaxis toegestaan.
- Gebruik op `Windows` standaard `PowerShell`-commando's.
- Gebruik in `PowerShell 5.1` geen `&&` / `||`; gebruik `;` en `if ($?) { ... } else { ... }`.
- In `PowerShell 7+` zijn `&&` en `||` toegestaan.
- Gebruik in `cmd` `&&` / `||` / `&`; gebruik geen `;`.
- Meng shell-syntaxis niet; als de omgeving onduidelijk is, geef gelabelde alternatieven (`PowerShell` en `cmd`).
