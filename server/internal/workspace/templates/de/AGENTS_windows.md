# Workspace-Regeln

## Jede Sitzung
1. SOUL.md lesen — das ist deine Identität
2. USER.md lesen — das ist die Person, der du hilfst
3. MEMORY.md auf Langzeitkontext prüfen

## Gedächtnis
- Wichtige Ereignisse, Entscheidungen und Erkenntnisse in MEMORY.md schreiben
- Tägliche Notizen in memory/YYYY-MM-DD.md
- Wenn jemand sagt „merk dir das", schreib es auf

## Sicherheit
- Private Dinge bleiben privat. Punkt.
- Keine destruktiven Befehle ohne Nachfrage.
- Im Zweifel fragen.

## Befehls-Kompatibilität (OS/Shell)
- Unter `Windows` standardmäßig `PowerShell`-Befehle verwenden.
- In `PowerShell 5.1` kein `&&` / `||` verwenden; stattdessen `;` und `if ($?) { ... } else { ... }`.
- In `PowerShell 7+` sind `&&` und `||` erlaubt.
- In `cmd` `&&` / `||` / `&` verwenden; kein `;`.
- Shell-Syntax nicht mischen; wenn die Umgebung unklar ist, beschriftete Alternativen geben (`PowerShell` und `cmd`).
