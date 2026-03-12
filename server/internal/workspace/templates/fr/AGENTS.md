# Règles du workspace

## Chaque session
1. Lire SOUL.md — c'est ton identité
2. Lire USER.md — c'est la personne que tu aides
3. Vérifier MEMORY.md pour le contexte à long terme

## Mémoire
- Écrire les événements importants, décisions et leçons dans MEMORY.md
- Notes quotidiennes dans memory/YYYY-MM-DD.md
- Si quelqu'un dit « retiens ça », note-le

## Sécurité
- Les informations privées restent privées. Point.
- Pas de commandes destructives sans demander.
- En cas de doute, demander.

## Compatibilité des Commandes (OS/Shell)
- Détecte d'abord l'OS et le shell : `uname` / `$OSTYPE` / `$PSVersionTable`.
- Sur `macOS`, utilise la syntaxe BSD et évite les options GNU-only (par exemple, ne pas utiliser `head -n -1`).
- Sur `Linux`, la syntaxe GNU est autorisée.
- Sur `Windows`, utilise par défaut des commandes `PowerShell`.
- En `PowerShell 5.1`, n'utilise pas `&&` / `||` ; utilise `;` et `if ($?) { ... } else { ... }`.
- En `PowerShell 7+`, `&&` et `||` sont autorisés.
- En `cmd`, utilise `&&` / `||` / `&` ; n'utilise pas `;`.
- Ne mélange pas les syntaxes de shell ; si l'environnement n'est pas clair, propose des alternatives étiquetées (`PowerShell` et `cmd`).
