#!/usr/bin/env python3
import argparse
import json
import statistics
import subprocess
import time
from dataclasses import dataclass
from typing import Any, Dict, Optional, Tuple


@dataclass
class Trial:
    idx: int
    ok: bool
    timed_out: bool
    wall_ms: int
    error: str
    verification_passed: bool
    target_hit: bool
    payload: Optional[Dict[str, Any]]


def parse_last_json(stdout: str) -> Optional[Dict[str, Any]]:
    # bluecli prints cards + a final JSON (often pretty-printed across multiple lines).
    # Extract the last JSON object we can decode from the full stdout buffer.
    decoder = json.JSONDecoder()
    last_obj: Optional[Dict[str, Any]] = None
    idx = 0
    while True:
        start = stdout.find("{", idx)
        if start < 0:
            break
        try:
            obj, end = decoder.raw_decode(stdout[start:])
            if isinstance(obj, dict):
                last_obj = obj
            idx = start + max(end, 1)
        except Exception:
            idx = start + 1
    return last_obj


def truthy(v: Any) -> bool:
    if isinstance(v, bool):
        return v
    if isinstance(v, str):
        return v.strip().lower() in ("1", "true", "yes", "ok")
    if isinstance(v, (int, float)):
        return v != 0
    return False


def extract_status(payload: Optional[Dict[str, Any]]) -> Tuple[bool, bool, str, bool, bool]:
    if not payload:
        return False, False, "no_json_payload", False, False
    data = payload.get("data") or {}
    err = ""
    if isinstance(data, dict):
        err = str(data.get("error") or "")
    verification_passed = truthy(data.get("verification_passed"))
    target_hit = truthy(data.get("target_hit"))
    ok = (err.strip() == "") and (verification_passed or target_hit)
    return ok, False, err.strip(), verification_passed, target_hit


def percentile(values, p: float) -> float:
    if not values:
        return 0.0
    values = sorted(values)
    k = (len(values) - 1) * p
    f = int(k)
    c = min(f + 1, len(values) - 1)
    if f == c:
        return float(values[f])
    d0 = values[f] * (c - k)
    d1 = values[c] * (k - f)
    return float(d0 + d1)


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--bluecli", default="./server/bin/.bluecli")
    ap.add_argument("--n", type=int, default=3)
    ap.add_argument("--timeout-s", type=float, default=45.0)
    ap.add_argument("--task-timeout-ms", type=int, default=60000)
    ap.add_argument("--dev", action="store_true", help="use --dev profile for both client and service")
    ap.add_argument("--prefer-visual", action="store_true", help="prefer visual conversation locate path (OCR+click) over keyboard search")
    ap.add_argument("--app-name", default="飞书")
    ap.add_argument("--window-id", default="", help="host window id; avoids app/window ambiguity (recommended)")
    ap.add_argument("--conversation", default="test_group")
    ap.add_argument("--value", default="你们好，我是 blue 发的（bench）")
    ap.add_argument("--submit", action="store_true", help="actually send; default drafts only")
    ap.add_argument("--no-intercept", action="store_true", default=True)
    args = ap.parse_args()

    window_id = (args.window_id or "").strip()
    if window_id == "":
        # Resolve a stable window_id once (not counted in trial latency).
        focus_cmd = [args.bluecli]
        if args.dev:
            focus_cmd.append("--dev")
        focus_cmd += [
            "--no-intercept",
            "computer_use",
            "action=focus",
            f"app_name={args.app_name}",
            "--json",
        ]
        try:
            proc = subprocess.run(
                focus_cmd,
                capture_output=True,
                text=True,
                timeout=min(15.0, args.timeout_s),
            )
            payload = parse_last_json(proc.stdout or "")
            data = (payload or {}).get("data") or {}
            if isinstance(data, dict):
                window_id = str(data.get("window_id") or "").strip()
        except Exception:
            window_id = ""

    trials = []
    for i in range(1, args.n + 1):
        # Avoid spaces: bluecli's loose key=value parsing may re-join argv and split on spaces.
        value = f"{args.value}#{i}"
        cmd = [args.bluecli]
        if args.dev:
            cmd.append("--dev")
        cmd += [
            "--no-intercept",
            "computer_use",
            "action=message",
        ]
        if args.prefer_visual:
            cmd.append("prefer_visual=true")
        cmd += [
            # Always pass app_name when available to enable app-profile heuristics (visual fast path, etc.),
            # while still using window_id for deterministic window targeting when provided.
            *( [f"window_id={window_id}"] if window_id else [] ),
            f"app_name={args.app_name}",
            f'conversation={args.conversation}',
            f'value={value}',
            f"submit={'true' if args.submit else 'false'}",
            "--json",
        ]
        start = time.time()
        try:
            proc = subprocess.run(
                cmd,
                capture_output=True,
                text=True,
                timeout=args.timeout_s,
            )
            wall_ms = int((time.time() - start) * 1000)
            payload = parse_last_json(proc.stdout or "")
            ok, timed_out, err, verification_passed, target_hit = extract_status(payload)
            if proc.returncode != 0 and err == "":
                err = (proc.stderr or "").strip() or f"exit_{proc.returncode}"
            trials.append(
                Trial(
                    idx=i,
                    ok=ok,
                    timed_out=timed_out,
                    wall_ms=wall_ms,
                    error=err,
                    verification_passed=verification_passed,
                    target_hit=target_hit,
                    payload=payload,
                )
            )
        except subprocess.TimeoutExpired:
            wall_ms = int((time.time() - start) * 1000)
            trials.append(
                Trial(
                    idx=i,
                    ok=False,
                    timed_out=True,
                    wall_ms=wall_ms,
                    error="timeout",
                    verification_passed=False,
                    target_hit=False,
                    payload=None,
                )
            )

    ok_count = sum(1 for t in trials if t.ok)
    timeout_count = sum(1 for t in trials if t.timed_out)
    wall = [t.wall_ms for t in trials]
    ok_wall = [t.wall_ms for t in trials if t.ok]

    print(json.dumps(
        {
            "n": args.n,
            "ok": ok_count,
            "success_rate": ok_count / args.n if args.n else 0,
            "timeouts": timeout_count,
            "window_id": window_id,
            "wall_ms": {
                "mean": int(statistics.mean(wall)) if wall else 0,
                "p50": int(percentile(wall, 0.5)),
                "p90": int(percentile(wall, 0.9)),
                "min": min(wall) if wall else 0,
                "max": max(wall) if wall else 0,
            },
            "ok_wall_ms": {
                "mean": int(statistics.mean(ok_wall)) if ok_wall else 0,
                "p50": int(percentile(ok_wall, 0.5)) if ok_wall else 0,
                "p90": int(percentile(ok_wall, 0.9)) if ok_wall else 0,
            },
            "trials": [
                {
                    "i": t.idx,
                    "ok": t.ok,
                    "timed_out": t.timed_out,
                    "wall_ms": t.wall_ms,
                    "error": t.error,
                    "verification_passed": t.verification_passed,
                    "target_hit": t.target_hit,
                }
                for t in trials
            ],
        },
        ensure_ascii=False,
        indent=2,
    ))


if __name__ == "__main__":
    main()
