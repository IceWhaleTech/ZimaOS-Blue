# Workspace szabályok

## Minden munkamenet
1. Olvasd el a SOUL.md-t — ez az identitásod
2. Olvasd el a USER.md-t — ez az, akinek segítesz
3. Ellenőrizd a MEMORY.md-t a hosszú távú kontextusért

## Memória
- Írd le a fontos eseményeket, döntéseket és tanulságokat a MEMORY.md-be
- Napi jegyzetek a memory/YYYY-MM-DD.md-be
- Ha valaki azt mondja „jegyezd meg", írd le

## Biztonság
- A magáninformációk magánjellegűek maradnak. Pont.
- Ne futtass destruktív parancsokat kérdezés nélkül.
- Kétség esetén kérdezz.

## Parancs-kompatibilitás (OS/Shell)
- Először észleld az OS-t és a shellt: `uname` / `$OSTYPE` / `$PSVersionTable`.
- `macOS` alatt BSD szintaxist használj, és kerüld a GNU-only kapcsolókat (pl. ne használd a `head -n -1` parancsot).
- `Linux` alatt a GNU szintaxis használható.
- `Windows` alatt alapértelmezetten `PowerShell` parancsokat adj.
- `PowerShell 5.1` esetén ne használd a `&&` / `||` operátorokat; használd a `;` jelet és az `if ($?) { ... } else { ... }` mintát.
- `PowerShell 7+` esetén a `&&` és `||` használható.
- `cmd` esetén használd a `&&` / `||` / `&` operátorokat; ne használd a `;` jelet.
- Ne keverd a különböző shell szintaxisokat; ha a környezet nem egyértelmű, adj címkézett alternatívákat (`PowerShell` és `cmd`).
