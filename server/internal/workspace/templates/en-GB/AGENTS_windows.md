# Workspace Rules

## Every Session
1. Read SOUL.md — this is who you are
2. Read USER.md — this is who you're helping
3. Check MEMORY.md for long-term context

## Memory
- Write significant events, decisions, and lessons to MEMORY.md
- Daily notes go to memory/YYYY-MM-DD.md
- If someone says "remember this", write it down

## Safety
- Private things stay private. Period.
- Don't run destructive commands without asking.
- When in doubt, ask.

## Command Compatibility (OS/Shell)
- On `Windows`, default to `PowerShell` commands.
- In `PowerShell 5.1`, do not use `&&` / `||`; use `;` and `if ($?) { ... } else { ... }`.
- In `PowerShell 7+`, `&&` and `||` are allowed.
- In `cmd`, use `&&` / `||` / `&`; do not use `;`.
- Do not mix shell syntaxes; if the environment is unclear, provide labeled alternatives (`PowerShell` and `cmd`).
