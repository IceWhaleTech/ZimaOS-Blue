# Reguli workspace

## Fiecare sesiune
1. Citește SOUL.md — aceasta este identitatea ta
2. Citește USER.md — aceasta este persoana pe care o ajuți
3. Verifică MEMORY.md pentru context pe termen lung

## Memorie
- Scrie evenimentele importante, deciziile și lecțiile în MEMORY.md
- Note zilnice în memory/YYYY-MM-DD.md
- Dacă cineva spune „ține minte asta", notează

## Securitate
- Informațiile private rămân private. Punct.
- Nu executa comenzi distructive fără a întreba.
- La îndoială, întreabă.

## Compatibilitatea Comenzilor (OS/Shell)
- Detectează mai întâi OS și shell: `uname` / `$OSTYPE` / `$PSVersionTable`.
- Pe `macOS`, folosește sintaxă BSD și evită opțiunile GNU-only (de exemplu, nu folosi `head -n -1`).
- Pe `Linux`, sintaxa GNU este permisă.
- Pe `Windows`, folosește implicit comenzi `PowerShell`.
- În `PowerShell 5.1`, nu folosi `&&` / `||`; folosește `;` și `if ($?) { ... } else { ... }`.
- În `PowerShell 7+`, `&&` și `||` sunt permise.
- În `cmd`, folosește `&&` / `||` / `&`; nu folosi `;`.
- Nu amesteca sintaxele shell; dacă mediul nu este clar, oferă alternative etichetate (`PowerShell` și `cmd`).
