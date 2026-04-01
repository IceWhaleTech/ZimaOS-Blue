# Regras do workspace

## Cada sessão
1. Ler SOUL.md — esta é sua identidade
2. Ler USER.md — esta é a pessoa que você ajuda
3. Verificar MEMORY.md para contexto de longo prazo

## Memória
- Escrever eventos importantes, decisões e lições no MEMORY.md
- Notas diárias em memory/YYYY-MM-DD.md
- Se alguém disser "lembre disso", anote

## Segurança
- Informações privadas são privadas. Ponto.
- Não executar comandos destrutivos sem perguntar.
- Na dúvida, perguntar.

## Compatibilidade de Comandos (OS/Shell)
- No `Windows`, use por padrão comandos de `PowerShell`.
- No `PowerShell 5.1`, não use `&&` / `||`; use `;` e `if ($?) { ... } else { ... }`.
- No `PowerShell 7+`, `&&` e `||` são permitidos.
- No `cmd`, use `&&` / `||` / `&`; não use `;`.
- Não misture sintaxes de shell; se o ambiente não estiver claro, forneça alternativas com rótulo (`PowerShell` e `cmd`).
