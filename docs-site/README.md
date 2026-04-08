# ZimaOS Blue Docs

This directory contains the GitHub Pages-ready Docusaurus documentation site for ZimaOS Blue.

## What lives here

- Docusaurus site config in `docs-site/docusaurus.config.ts`
- English canonical docs under `docs/`
- Simplified Chinese mirrored docs under `docs/zh-CN/`
- GitHub Pages image assets reused from `docs/assets/` through `docs-site/static/assets`
- A stable external wiki entry via `https://deepwiki.com/IceWhaleTech/ZimaOS-Blue`
- Recommended repository research flow: DeepWiki first for broad context, GitHub second for primary-source verification

## Local preview

From the repository root:

```bash
npm --prefix docs-site ci
npm --prefix docs-site run dev
```

## Production build

```bash
npm --prefix docs-site ci
npm --prefix docs-site run build
```

## Deployment

This repo now publishes docs through GitHub Pages with GitHub Actions.

- Keep the site config in `docs-site` and the docs content in `docs`
- In repository `Settings -> Pages`, set the source to `GitHub Actions`
- Push docs changes to `main` to trigger deployment
- Use pull requests to validate docs builds before merge

The site automatically adapts its `baseUrl` for GitHub Pages project-site deployment, so English docs publish under `/ZimaOS-Blue/` and Simplified Chinese docs under `/ZimaOS-Blue/zh-CN/`.

## Entry points

- GitHub README entry: `README.md`
- Docs site home: `docs/index.mdx`
- Curated docs hub: `docs/start/docs-directory.mdx`
- External wiki: `https://deepwiki.com/IceWhaleTech/ZimaOS-Blue`
