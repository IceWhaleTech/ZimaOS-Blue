# ZimaOS Blue v0.11.0

More reliable skill discovery, document workflows, and task completion.

## Improved

- Automatically select installed document skills such as FiLens for relevant tasks, including spreadsheet and cross-file calculations.
- Preserve shell evidence needed for follow-up reasoning and report request token usage through skill search and advice APIs.
- Check requested outcomes before finishing and stop once the requested results have been delivered.
- Rewrite conversational skill searches into useful search terms, with cached multilingual query planning and fallback when rewriting is unavailable.
- Reuse Photos' local language detection with a 938,013-byte model; no translation runtime or language packs are bundled.

## Fixed

- Correct macOS PDF form inspection and filling by using PDFKit's actual annotation keys and field type names.
- Refresh frontend dependencies to address the vulnerabilities reported by the release dependency audit.

## Packaging

- Align CLI, web, Rust, and desktop metadata at 0.11.0.
- Build macOS CLI and desktop packages for Apple Silicon and Intel, verify their architectures and signatures, require accepted Apple notarization, and validate the DMG's stapled ticket.
- Fail release preparation early when required signing credentials are unavailable. Manual validation builds do not publish release assets.

Release preparation is in progress. Successful Apple signing and notarization must be confirmed with the release workflow before publication.
