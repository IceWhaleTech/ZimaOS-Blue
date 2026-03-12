# Pravidla workspace

## Každá relace
1. Přečti SOUL.md — to je tvá identita
2. Přečti USER.md — to je osoba, které pomáháš
3. Zkontroluj MEMORY.md pro dlouhodobý kontext

## Paměť
- Zapiš důležité události, rozhodnutí a poučení do MEMORY.md
- Denní poznámky do memory/YYYY-MM-DD.md
- Pokud někdo řekne „zapamatuj si to", zapiš to

## Bezpečnost
- Soukromé informace zůstávají soukromé. Tečka.
- Nespouštěj destruktivní příkazy bez dotazu.
- V případě pochybností se zeptej.

## Kompatibilita Příkazů (OS/Shell)
- Nejprve zjisti OS a shell: `uname` / `$OSTYPE` / `$PSVersionTable`.
- Na `macOS` používej BSD syntaxi a vyhýbej se GNU-only přepínačům (např. nepoužívej `head -n -1`).
- Na `Linux` je GNU syntaxe povolena.
- Na `Windows` ve výchozím stavu používej příkazy `PowerShell`.
- V `PowerShell 5.1` nepoužívej `&&` / `||`; používej `;` a `if ($?) { ... } else { ... }`.
- V `PowerShell 7+` jsou `&&` a `||` povoleny.
- V `cmd` používej `&&` / `||` / `&`; nepoužívej `;`.
- Nemíchej syntaxi shellů; pokud je prostředí nejasné, nabídni označené alternativy (`PowerShell` a `cmd`).
