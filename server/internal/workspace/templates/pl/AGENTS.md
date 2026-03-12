# Zasady workspace

## Każda sesja
1. Przeczytaj SOUL.md — to twoja tożsamość
2. Przeczytaj USER.md — to osoba, której pomagasz
3. Sprawdź MEMORY.md dla długoterminowego kontekstu

## Pamięć
- Zapisuj ważne wydarzenia, decyzje i wnioski w MEMORY.md
- Codzienne notatki w memory/YYYY-MM-DD.md
- Jeśli ktoś mówi „zapamiętaj to", zapisz

## Bezpieczeństwo
- Prywatne informacje pozostają prywatne. Kropka.
- Nie wykonuj destrukcyjnych poleceń bez pytania.
- W razie wątpliwości pytaj.

## Zgodność Poleceń (OS/Shell)
- Najpierw wykryj OS i shell: `uname` / `$OSTYPE` / `$PSVersionTable`.
- W `macOS` używaj składni BSD i unikaj opcji GNU-only (np. nie używaj `head -n -1`).
- W `Linux` dozwolona jest składnia GNU.
- W `Windows` domyślnie używaj poleceń `PowerShell`.
- W `PowerShell 5.1` nie używaj `&&` / `||`; użyj `;` oraz `if ($?) { ... } else { ... }`.
- W `PowerShell 7+` `&&` i `||` są dozwolone.
- W `cmd` używaj `&&` / `||` / `&`; nie używaj `;`.
- Nie mieszaj składni shelli; jeśli środowisko jest niejasne, podaj opisane alternatywy (`PowerShell` i `cmd`).
