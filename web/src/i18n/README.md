# i18n Layout

This directory uses two localization layers on purpose.

## `locales/*.ts`

Use raw locale files for stable product copy that should remain the primary
source of truth for each language.

Typical examples:

- shared UI labels that appear across the app
- critical recovery or settings copy that should stay visible in the base locale
- long-lived text that product/design expects translators to maintain directly

## `*-backfills.ts`

Use backfill catalogs for centralized overlays that would otherwise require the
same structural change across all 27 locale files at once.

Typical examples:

- late-added feature bundles such as selector debug
- structural compatibility patches
- post-merge locale repairs and derived runtime labels

## Rule of thumb

- If the copy is a stable user-facing surface, prefer `locales/*.ts`.
- If the copy is a shared overlay or compatibility patch, prefer backfills.
- If a backfill becomes long-lived product copy, migrate it into raw locale
  files and leave the backfill layer for compatibility only.
