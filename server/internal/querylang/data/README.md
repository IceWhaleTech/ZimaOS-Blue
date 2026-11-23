# Blue skill-search language identification

Blue reuses the local language detector from ZimaOS Photos, including its pure
Go fastText reader, weak-prediction corroboration, and Han character evidence.
Only detection is reused; Blue adds no NMT runtime or translation weights.
Optional query rewriting uses Blue's configured LLM.

## Pinned assets

- `lid.176.ftz`: unmodified Facebook Research / fastText quantized language-ID
  model, 938,013 bytes, CC BY-SA 3.0.
- Source: <https://fasttext.cc/docs/en/language-identification.html>
- Download: <https://dl.fbaipublicfiles.com/fasttext/supervised-models/lid.176.ftz>
- SHA256: `8f3472cfe8738a7b6099e8e999c3cbfae0dcd15696aac7d7738a8039db603e83`
- Native reference: fastText v0.9.2, commit
  `5b5943c118b0ec5fb9cd8d20587de2b2d3966dfe`, MIT license.
- `simplified-zh-evidence.txt`: 3,689 characters, 12,209 bytes, derived from
  OpenCC commit `fae2a0a911c30764e669a7d8b0591cd8e852934a`, Apache-2.0.
  Photos retained single-character STCharacters keys without a self mapping,
  excluded Japanese shinjitai keys, deduplicated and sorted them.

The inference reader verifies the exact model size and SHA256 before parsing.
Both runtime assets are embedded and initialized lazily; there is no CGo,
Python, executable, remote language detection or runtime asset download.

All 176 fastText labels remain in competition. The detector accepts Photos'
26 language profiles only with sufficient evidence. Empty means uncertain,
not English. Blue conservatively keeps ambiguous Han/code locally; other
non-ASCII scripts can still request a rewrite when no accepted profile wins.
This is a heuristic and cannot guarantee every short or mixed-language query.

## Verification

From `server/`:

```sh
go test ./internal/querylang -count=1
```

`../testdata/language-id-native.json` contains Photos' 829 native-reference
records. Tests compare Go inference scores with these outputs at a tolerance
of 0.00002, verify pinned hashes and exercise language detection. This proves
compatibility with the saved reference, not perfect language identification.
The two manifests retain the upstream pins and Photos reproduction metadata;
the Photos development scripts mentioned there are not bundled into Blue.

## Distribution

`NOTICE.md`, `CC-BY-SA-3.0.txt`, `LICENSE.fasttext`, and `LICENSE.OpenCC` accompany
CLI tarballs under `language-id-notices/`, ZimaOS raw modules under
`/usr/share/doc/zimaos-blue/language-id/`, and Tauri resources under
`licenses/language-id/`. These paths include notices only; the model is already
embedded in the binary and is not duplicated in package resources.
