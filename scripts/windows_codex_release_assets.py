#!/usr/bin/env python3
from __future__ import annotations

import argparse
import shutil
from dataclasses import dataclass
from pathlib import Path
from typing import BinaryIO, Callable, Iterable, List, Optional
from urllib import request


DEFAULT_CODEX_RELEASE_BASE = "https://github.com/openai/codex/releases/latest/download"
WINDOWS_CODEX_ARCHES = ("amd64", "arm64")


@dataclass(frozen=True)
class WindowsCodexAsset:
    arch: str
    asset_name: str
    output_path: Path
    download_url: str
    source: str


def codex_asset_name_for_windows_arch(arch: str) -> str:
    normalized = str(arch or "").strip().lower()
    if normalized == "amd64":
        return "codex-x86_64-pc-windows-msvc.exe"
    if normalized == "arm64":
        return "codex-aarch64-pc-windows-msvc.exe"
    raise ValueError(f"unsupported Windows Codex arch: {arch}")


def build_download_url(base_url: str, asset_name: str) -> str:
    return f"{str(base_url or '').rstrip('/')}/{asset_name}"


def normalize_archs(archs: Iterable[str]) -> List[str]:
    normalized: List[str] = []
    seen = set()
    for arch in archs:
        value = str(arch or "").strip().lower()
        if not value:
            continue
        codex_asset_name_for_windows_arch(value)
        if value in seen:
            continue
        seen.add(value)
        normalized.append(value)
    if not normalized:
        return list(WINDOWS_CODEX_ARCHES)
    return normalized


def _default_urlopen(url: str, timeout: float) -> BinaryIO:
    req = request.Request(url, headers={"User-Agent": "zimaos-blue-windows-codex-assets"})
    return request.urlopen(req, timeout=timeout)


def _download_to_path(url: str, dest_path: Path, timeout_seconds: float, urlopen: Callable[[str, float], BinaryIO]) -> None:
    tmp_path = dest_path.with_suffix(dest_path.suffix + ".tmp")
    with urlopen(url, timeout_seconds) as response, tmp_path.open("wb") as fh:
        shutil.copyfileobj(response, fh)
    tmp_path.replace(dest_path)


def stage_windows_codex_assets(
    output_dir: Path,
    archs: Iterable[str] = WINDOWS_CODEX_ARCHES,
    source_dir: Optional[Path] = None,
    download_url_base: str = DEFAULT_CODEX_RELEASE_BASE,
    timeout_seconds: float = 300.0,
    urlopen: Callable[[str, float], BinaryIO] = _default_urlopen,
) -> List[WindowsCodexAsset]:
    normalized_archs = normalize_archs(archs)
    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    staged: List[WindowsCodexAsset] = []
    local_source_dir = Path(source_dir) if source_dir else None
    for arch in normalized_archs:
        asset_name = codex_asset_name_for_windows_arch(arch)
        output_path = output_dir / asset_name
        download_url = build_download_url(download_url_base, asset_name)

        if local_source_dir is not None:
            source_path = local_source_dir / asset_name
            if source_path.is_file():
                shutil.copy2(source_path, output_path)
                staged.append(
                    WindowsCodexAsset(
                        arch=arch,
                        asset_name=asset_name,
                        output_path=output_path,
                        download_url=download_url,
                        source="local",
                    )
                )
                continue

        _download_to_path(download_url, output_path, timeout_seconds, urlopen)
        staged.append(
            WindowsCodexAsset(
                arch=arch,
                asset_name=asset_name,
                output_path=output_path,
                download_url=download_url,
                source="download",
            )
        )

    return staged


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Stage Windows Codex sandbox release assets for Blue releases.")
    parser.add_argument("--output-dir", required=True, help="Directory where the Windows Codex assets will be staged.")
    parser.add_argument(
        "--source-dir",
        default="",
        help="Optional local directory containing prebuilt Windows Codex binaries. If an asset exists here, it is copied instead of downloaded.",
    )
    parser.add_argument(
        "--download-url-base",
        default=DEFAULT_CODEX_RELEASE_BASE,
        help="Base URL used to download missing Windows Codex assets.",
    )
    parser.add_argument(
        "--arch",
        dest="archs",
        action="append",
        default=[],
        help="Windows Codex architecture to stage. Repeat for multiple values. Defaults to amd64 and arm64.",
    )
    parser.add_argument(
        "--timeout-seconds",
        type=float,
        default=300.0,
        help="Per-asset download timeout in seconds.",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    staged = stage_windows_codex_assets(
        output_dir=Path(args.output_dir),
        archs=args.archs or WINDOWS_CODEX_ARCHES,
        source_dir=Path(args.source_dir) if str(args.source_dir or "").strip() else None,
        download_url_base=str(args.download_url_base or "").strip() or DEFAULT_CODEX_RELEASE_BASE,
        timeout_seconds=max(float(args.timeout_seconds), 1.0),
    )
    for item in staged:
        print(f"[{item.source}] {item.arch}: {item.output_path}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
