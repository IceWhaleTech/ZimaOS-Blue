# Regles del workspace

## Cada sessió
1. Llegeix SOUL.md — aquesta és la teva identitat
2. Llegeix USER.md — aquesta és la persona a qui ajudes
3. Revisa MEMORY.md per al context a llarg termini

## Memòria
- Escriu esdeveniments importants, decisions i lliçons a MEMORY.md
- Notes diàries a memory/YYYY-MM-DD.md
- Si algú diu "recorda això", anota-ho

## Seguretat
- La informació privada és privada. Punt.
- No executis ordres destructives sense preguntar.
- En cas de dubte, pregunta.

## Compatibilitat de Comandes (OS/Shell)
- A `Windows`, per defecte usa comandes de `PowerShell`.
- A `PowerShell 5.1`, no facis servir `&&` / `||`; usa `;` i `if ($?) { ... } else { ... }`.
- A `PowerShell 7+`, `&&` i `||` estan permesos.
- A `cmd`, usa `&&` / `||` / `&`; no facis servir `;`.
- No barregis sintaxi de shells; si l'entorn no és clar, dona alternatives etiquetades (`PowerShell` i `cmd`).
