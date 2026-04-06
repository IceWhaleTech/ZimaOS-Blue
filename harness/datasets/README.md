# Harness Dataset Bundles

First-party Harness datasets can live in this repo under `harness/datasets/<bundle>/`.

Third-party datasets can live in a separate GitHub repository as long as they use the same bundle layout:

```text
<bundle>/
  dataset.yaml
  versions/
    <version>/
      manifest.json
      eval-specs/
        *.yaml
```

V1 is manual-only:

- import a local bundle with `blue harness dataset import --path harness/datasets/<bundle>`
- pull a GitHub-hosted bundle with `blue harness dataset pull --source <github-tree-url>`
- repo URLs also work with `blue harness dataset pull --source <github-repo-url> --bundle-path <path>`
- both commands also support `--version <version>` and `--owner <user-id>`
- the Harness UI also exposes `Automation -> Harness -> Datasets & versions -> Import bundle`
- the UI always previews the selected bundle version before import

Remote GitHub pull support is declarative only in this phase:

- dataset metadata
- one selected `DatasetVersion` manifest
- optional reusable `EvalSpec` templates

Remote scorer code, drivers, plugins, and background sync are intentionally out of scope for V1.

The CLI imports one version at a time:

- use `dataset.yaml` `default_version` by default
- override it with `--version <version>` when needed

The source-preview and source-import APIs are:

- `POST /api/v1/harness/dataset-bundles/preview-source`
- `POST /api/v1/harness/dataset-bundles/import-source`

For GitHub-hosted bundles, list eval spec filenames in `dataset.yaml` under `versions.<version>.eval_specs` so the CLI can fetch the exact files without directory listing support.

GitHub file fetches follow this fallback order:

1. `raw.githubusercontent.com`
2. `raw.gitmirror.com`
3. `cdn.jsdelivr.net/gh`
4. `ghproxy.com` proxying raw GitHub

The preview step shows:

- dataset name and subject
- selected bundle version
- manifest SHA-256
- case count
- bundled eval spec names

Imported content is frozen into ordinary Harness `DatasetVersion` and `EvalSpec` records. There is no background sync or remote code loading in this phase.
