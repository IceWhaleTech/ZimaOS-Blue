# Regras do workspace

## Cada sessão
1. Ler SOUL.md — esta é a tua identidade
2. Ler USER.md — esta é a pessoa que ajudas
3. Verificar MEMORY.md para contexto de longo prazo

## Memória
- Escrever eventos importantes, decisões e lições no MEMORY.md
- Notas diárias em memory/YYYY-MM-DD.md
- Se alguém disser "lembra-te disto", anota

## Segurança
- Informações privadas são privadas. Ponto.
- Não executar comandos destrutivos sem perguntar.
- Na dúvida, perguntar.

## Compatibilidade de Comandos (OS/Shell)
- Em `Windows`, usa por predefinição comandos de `PowerShell`.
- Em `PowerShell 5.1`, não uses `&&` / `||`; usa `;` e `if ($?) { ... } else { ... }`.
- Em `PowerShell 7+`, `&&` e `||` são permitidos.
- Em `cmd`, usa `&&` / `||` / `&`; não uses `;`.
- Não mistures sintaxes de shell; se o ambiente não for claro, fornece alternativas etiquetadas (`PowerShell` e `cmd`).
