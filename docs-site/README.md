# ZimaOS Blue Docs

This directory contains the first public-style documentation site for ZimaOS Blue.

## What lives here

- Mintlify site config in `docs-site/docs.json`
- English canonical docs under `docs-site/`
- Simplified Chinese user-facing docs under `docs-site/zh-CN/`
- Local copies of required assets under `docs-site/assets/`
- A stable external wiki entry via `https://deepwiki.com/IceWhaleTech/ZimaOS-Blue`
- Recommended repository research flow: DeepWiki first for broad context, GitHub second for primary-source verification

## Local preview

From the repository root:

```bash
npm --prefix docs-site ci
npm --prefix docs-site run dev
```

## Production smoke check

```bash
npm --prefix docs-site ci
npm --prefix docs-site run build
```

## Deployment

This repo is prepared for Mintlify-managed deployment rather than a static-site publish workflow.

- Connect the repository in Mintlify via the GitHub App
- Set the docs root to `docs-site`
- Keep GitHub Actions focused on validation
- Swap `README.md` docs links to the public docs URL after the domain is live

The helper script in `docs-site/scripts/run-mint.mjs` prefers the pinned local `mint` CLI from `docs-site/package.json`, then falls back to global and `npx` paths if needed. The `build` script maps to Mint's strict `validate` command for CI-friendly smoke checks.

## Entry points

- GitHub README entry: `README.md`
- Docs site home: `docs-site/index.mdx`
- Curated docs hub: `docs-site/start/docs-directory.mdx`
- External wiki: `https://deepwiki.com/IceWhaleTech/ZimaOS-Blue`
