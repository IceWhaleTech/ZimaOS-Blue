# Pravila radnog prostora

## Svaka sesija
1. Pročitaj SOUL.md — to je tvoj identitet
2. Pročitaj USER.md — to je osoba kojoj pomažeš
3. Provjeri MEMORY.md za dugoročni kontekst

## Memorija
- Zapiši važne događaje, odluke i pouke u MEMORY.md
- Dnevne bilješke u memory/YYYY-MM-DD.md
- Ako netko kaže "zapamti ovo", zapiši

## Sigurnost
- Privatne informacije ostaju privatne. Točka.
- Ne pokreći destruktivne naredbe bez pitanja.
- U slučaju sumnje, pitaj.

## Kompatibilnost Naredbi (OS/Shell)
- Na `Windows` zadano koristi `PowerShell` naredbe.
- U `PowerShell 5.1` nemoj koristiti `&&` / `||`; koristi `;` i `if ($?) { ... } else { ... }`.
- U `PowerShell 7+` `&&` i `||` su dopušteni.
- U `cmd` koristi `&&` / `||` / `&`; nemoj koristiti `;`.
- Ne miješaj shell sintakse; ako okruženje nije jasno, ponudi označene alternative (`PowerShell` i `cmd`).
