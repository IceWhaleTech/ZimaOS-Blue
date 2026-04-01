# Lightpanda vs Chromium Local Benchmark

Date: 2026-03-25
Updated: 2026-03-26

Scope:
- This report summarizes local benchmark data collected during Blue browser-routing evaluation.
- It now includes two distinct Lightpanda paths:
  - `lightpanda_shim`: Blue's current read-layer implementation.
  - `lightpanda_binary`: Blue's browser-lite adapter attached to an upstream Lightpanda runtime.
- Chromium numbers were collected from real browser runs on the local machine.

Primary local sources:
- `~/.codex/archived_sessions/rollout-2026-03-24T22-24-16-019d203b-6333-7070-a33e-300afbac44f8.jsonl`
- `docs/reports/lightpanda-binary-vs-chromium-benchmark-2026-03-26-native-rerun-refresh.json`

## Engine Data Availability

| Engine detail | Layer | Confirmed local archived benchmark data | Safe to use in this report | Notes |
|---|---|---|---|---|
| `lightpanda_shim` | Read | Yes | Yes | This is the measured "Lightpanda" path in all tables below |
| `lightpanda_binary` | Browser-lite | Yes | Yes, with caveat | Measured on 2026-03-26 through Blue `browserbench` against a locally downloaded native Lightpanda nightly binary; `tab not found` no longer reproduces, but real-site stability still trails Chromium |
| `chromium_managed` / `chromium_relay` | Full browser | Yes, but not consistently split out by engine detail | Partially | Current local record is safe as a Chromium baseline, not as a strict managed-vs-relay comparison |

## Environment

| Item | Value |
|---|---|
| Machine | Apple M2 |
| Memory | 8 GB |
| OS | macOS Darwin 25.3.0 |
| Lightpanda binary runtime | `~/Library/Caches/zimaos-blue/browser/lightpanda/darwin-arm64/lightpanda` (`Mach-O arm64`) |
| Chromium binary | `~/.cache/rod/browser/chromium-1321438/.../Chromium` |
| Local Chrome binary | `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome` |

## Definitions

| Metric | Meaning |
|---|---|
| cold | First run including engine startup cost |
| warm p50 | Median latency after the browser/runtime is already hot |

## Synthetic Scenarios

| Scenario | Lightpanda cold | Chromium cold | Cold winner | Lightpanda warm p50 | Chromium warm p50 | Warm winner | Notes |
|---|---:|---:|---|---:|---:|---|---|
| Static article / readable tree | 2.0 ms | 4020.3 ms | Lightpanda | 1.0 ms | 273.5 ms | Lightpanda | Pure read-only structured extraction |
| Dense DOM / readable tree | 1.7 ms | 2569.4 ms | Lightpanda | 2.1 ms | 345.5 ms | Lightpanda | Large DOM enumeration |
| JS shell / escalation path | 0.3 ms fail | 3673.5 ms | Lightpanda fails fast | 0.2 ms fail | 257.1 ms | Chromium | Hybrid should escalate after one Lightpanda failure |
| Screenshot baseline | unsupported | 3805.4 ms | Chromium | unsupported | 546.0 ms | Chromium | Screenshot path is Chromium-only |

## Real Sites

| Site | Lightpanda cold | Chromium cold | Cold winner | Lightpanda warm p50 | Chromium warm p50 | Warm winner | Notes |
|---|---:|---:|---|---:|---:|---|---|
| `https://example.com` | 494.0 ms | 3377.6 ms | Lightpanda | 487.4 ms | 96.8 ms | Chromium | Static site |
| `https://developer.mozilla.org/en-US/docs/Web/HTML` | 10178.5 ms | 4212.4 ms | Chromium | 6893.4 ms | 207.0 ms | Chromium | Lightpanda hit `TLS handshake timeout` in this environment |
| `https://www.apple.com/` | 1748.2 ms | 19166.8 ms | Lightpanda | 1238.5 ms | 667.4 ms | Chromium | Chromium cold-start cost was especially high |
| `https://github.com/` | 807.2 ms | 3105.0 ms | Lightpanda | 782.1 ms | 687.2 ms | Chromium | Warm gap is small |
| `https://www.figma.com/` | 1449.8 ms | 3300.0 ms | Lightpanda | 1158.8 ms | 334.0 ms | Chromium | JS-heavy site; Chromium clearly better when warm |

## Standalone Lightpanda Binary (Official Nightly Native Runtime)

Source:
- `docs/reports/lightpanda-binary-vs-chromium-benchmark-2026-03-26-native-rerun-refresh.json`
- Run date: 2026-03-26 (`generated_at=2026-03-26T08:04:58Z`)
- Runtime detail: Blue `lightpanda_binary` browser-lite backend attached to a locally downloaded upstream Lightpanda native binary

### Synthetic Scenarios

| Scenario | `lightpanda_binary` cold | Chromium cold | Cold winner | `lightpanda_binary` warm p50 | Chromium warm p50 | Warm winner | Notes |
|---|---:|---:|---|---:|---:|---|---|
| Static article / readable tree | 1221.5 ms | 1287.2 ms | Lightpanda | 2.7 ms | 10.4 ms | Lightpanda | Native runtime now completes cold and warm reads successfully |
| Dense DOM / readable tree | 269.5 ms | 1235.0 ms | Lightpanda | 24.3 ms | 73.9 ms | Lightpanda | Large local DOM stays stable through warm reuse |
| JS shell / hydrated app | 215.4 ms | 1159.2 ms | Lightpanda | 2.4 ms | 9.5 ms | Lightpanda | Both engines render the client-side shell; Lightpanda is faster here |
| Screenshot baseline | unsupported | 1404.3 ms | Chromium | unsupported | 328.6 ms | Chromium | Screenshot remains outside Blue browser-lite capability |

### Real Sites

| Site | `lightpanda_binary` cold | Chromium cold | Cold winner | `lightpanda_binary` warm p50 | Chromium warm p50 | Warm winner | Notes |
|---|---:|---:|---|---:|---:|---|---|
| `https://example.com` | 968.7 ms | 2061.4 ms | Lightpanda | 175.0 ms | 12.9 ms | Chromium | Cold path is fine; warm Chromium is still much faster |
| `https://developer.mozilla.org/en-US/docs/Web/HTML` | 2053.9 ms | 3029.8 ms | Lightpanda | 2023.0 ms | 86.1 ms | Chromium | `tab not found` is gone, but warm Lightpanda remains much slower |
| `https://www.apple.com/` | 1616.4 ms | 11548.0 ms | Lightpanda | 6456.8 ms | 573.1 ms | Chromium | Strong cold result for Lightpanda, but warm Chromium wins decisively |
| `https://github.com/` | 23758.4 ms fail (`connection reset by peer`) | 5150.0 ms | Chromium | - | 394.3 ms | Chromium | Current native adapter is still unstable on GitHub |
| `https://www.figma.com/` | 8703.3 ms | 2137.2 ms | Chromium | 1227.0 ms | 208.5 ms | Chromium | One Lightpanda warm sample reset the connection; Chromium stayed stable |

## Screenshot Baseline

| Site | Chromium cold | Chromium warm p50 |
|---|---:|---:|
| `https://www.apple.com/` screenshot | 3587.9 ms | 1316.4 ms |

## Interpretation

| Condition | Recommended default |
|---|---|
| Cold start, simple read-only, tree/text extraction | Try `lightpanda_shim` first |
| Upstream `lightpanda_binary` under the current Blue browser-lite adapter | Keep experimental only; do not make it a default route yet |
| Hot local Chrome or hot Chromium already available | Chromium is often the better default even for simple reads |
| Screenshot, vision, interaction, login, session continuity, long transaction | Chromium |
| JS-heavy site or Lightpanda failure (`js_required`, empty tree, navigation blocked, unstable DOM) | Escalate to Chromium once |

## Practical Conclusion

- The current Blue `lightpanda_shim` often wins cold-start latency by a large margin on simple read-only tasks.
- The native `lightpanda_binary` rerun is now locally reproducible without Docker, and the earlier `tab not found` failure mode did not reproduce in this run.
- The native binary performs well on local synthetic reads and can win cold-start latency on several real sites, but warm latency on complex sites is still usually worse than Chromium.
- Real-site stability is still not good enough for a blanket default: GitHub failed cold with `connection reset by peer`, and Figma only produced 2 of 3 warm samples.
- On real sites, once Chromium is already warm, Chromium frequently wins on absolute latency and stability.
- Based on these measurements alone, Blue should not blindly route all read-only traffic to Lightpanda.
- A better routing rule is:
  - use Lightpanda first for cold, simple, read-only extraction;
  - treat any `lightpanda_binary` default as opportunistic and failure-aware, not as a blanket route;
  - prefer Chromium when a hot local Chrome/Chromium session already exists;
  - always use Chromium for screenshot/vision/interaction/login/continuity.

## Caveats

- The `lightpanda_shim` tables still represent Blue's current read-layer shim versus Chromium.
- The `lightpanda_binary` tables now come from a locally downloaded nightly native binary, not from the older Docker relay experiment.
- These `lightpanda_binary` numbers still measure Blue's current browser-lite adapter behavior, not an idealized upstream best-case benchmark.
- The authoritative local records are the archived session above and `docs/reports/lightpanda-binary-vs-chromium-benchmark-2026-03-26-native-rerun-refresh.json`.
