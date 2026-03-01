# Chat Stream Benchmark Template

## 1. Goal

Measure end-to-end chat streaming experience before/after optimization:

- Send -> first visible token latency (TTFT)
- Total response latency
- SSE delta event count
- Backend delta batching efficiency
- Frontend render stability (long task / FPS jank)

## 2. Test Matrix

Use the same prompt set and environment for both baseline and optimized builds.

| Scenario | Prompt Style | Expected Output Size | Tool Calls |
| --- | --- | --- | --- |
| S1 | short Q&A | small | no |
| S2 | long explanation | medium/large | no |
| S3 | plan + execute | medium/large | yes |
| S4 | tiny token-heavy stream | large | no |

Recommended runs per scenario: `N=10` (discard first warm-up run, keep 9).

## 3. Environment Lock

Record and keep fixed:

- Server commit SHA
- Web commit SHA
- Model/provider
- Hardware (CPU/memory)
- Network (LAN/WAN, proxy on/off)
- Browser and version

## 4. Data Collection

### 4.1 Backend (Stream Completion Logs)

Use the stream completion log line:

- key fields:
  - `latency_ms`
  - `ttft_ms`
  - `tokens_per_sec`
  - `delta_chunks_in`
  - `delta_flushes_out`
  - `delta_bytes_out`

### 4.2 Frontend (Perceived UX)

For each run, collect:

- `TTFT_UI_ms`: time from clicking send to first visible token
- `Total_UI_ms`: time from send to final done state
- `SSE_Delta_Events`: parsed from response stream
- `LongTasks_>50ms`: from browser Performance panel

## 5. Result Table

Fill one row per scenario with P50/P95.

| Scenario | Version | TTFT_UI_P50 | TTFT_UI_P95 | Total_UI_P50 | Total_UI_P95 | SSE_Delta_Events_P50 | delta_chunks_in_P50 | delta_flushes_out_P50 | Flush Reduction | Notes |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| S1 | baseline |  |  |  |  |  |  |  |  |  |
| S1 | optimized |  |  |  |  |  |  |  |  |  |
| S2 | baseline |  |  |  |  |  |  |  |  |  |
| S2 | optimized |  |  |  |  |  |  |  |  |  |
| S3 | baseline |  |  |  |  |  |  |  |  |  |
| S3 | optimized |  |  |  |  |  |  |  |  |  |
| S4 | baseline |  |  |  |  |  |  |  |  |  |
| S4 | optimized |  |  |  |  |  |  |  |  |  |

`Flush Reduction = 1 - (delta_flushes_out / delta_chunks_in)`

## 6. Acceptance Targets (Suggested)

- No TTFT regression for S1/S2 (P50 not worse than baseline by >5%)
- `delta_flushes_out` significantly lower than `delta_chunks_in` in S2/S4
- Equal output correctness (no missing/duplicated tokens)
- Reduced frontend long-task spikes in long streams

## 7. Quick Checklist

- [ ] Same prompt set and order
- [ ] Same model/provider and config
- [ ] Same machine and browser
- [ ] Warm-up run excluded
- [ ] Backend and frontend metrics both captured
- [ ] P50/P95 computed from same sample count
