# Reglas del workspace

## Cada sesión
1. Leer SOUL.md — esta es tu identidad
2. Leer USER.md — esta es la persona a la que ayudas
3. Revisar MEMORY.md para contexto a largo plazo

## Memoria
- Escribir eventos importantes, decisiones y lecciones en MEMORY.md
- Notas diarias en memory/YYYY-MM-DD.md
- Si alguien dice "recuerda esto", anótalo

## Seguridad
- La información privada es privada. Punto.
- No ejecutar comandos destructivos sin preguntar.
- En caso de duda, preguntar.

## Compatibilidad de Comandos (OS/Shell)
- Detecta primero el OS y el shell: `uname` / `$OSTYPE` / `$PSVersionTable`.
- En `macOS`, usa sintaxis BSD y evita flags solo de GNU (por ejemplo, no usar `head -n -1`).
- En `Linux`, se permite sintaxis GNU.
- En `Windows`, por defecto usa comandos de `PowerShell`.
- En `PowerShell 5.1`, no uses `&&` / `||`; usa `;` e `if ($?) { ... } else { ... }`.
- En `PowerShell 7+`, se permiten `&&` y `||`.
- En `cmd`, usa `&&` / `||` / `&`; no uses `;`.
- No mezcles sintaxis de shells; si el entorno no está claro, ofrece alternativas etiquetadas (`PowerShell` y `cmd`).
