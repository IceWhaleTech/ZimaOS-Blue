#!/usr/bin/env python3
from __future__ import annotations

import json
import re
import subprocess
import sys
import time
import urllib.parse
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
AUDIT_CMD = ["node", "scripts/i18n-audit.mjs", "--json"]
OUTPUT_PATH = ROOT / "src" / "i18n" / "priority-translation-overrides.ts"
SPLIT_TOKEN = "\n@@ZIMA_SPLIT@@\n"
MAX_BATCH_CHARS = 3200
MAX_RETRIES = 5

GOOGLE_TARGETS = {
    "ca-ES": "ca",
    "cs-CZ": "cs",
    "da-DK": "da",
    "de-DE": "de",
    "el-GR": "el",
    "es-ES": "es",
    "fr-FR": "fr",
    "ga-IE": "ga",
    "hr-HR": "hr",
    "hu-HU": "hu",
    "it-IT": "it",
    "ja-JP": "ja",
    "ko-KR": "ko",
    "ml-IN": "ml",
    "nb-NO": "no",
    "nl-NL": "nl",
    "pl-PL": "pl",
    "pt-BR": "pt",
    "pt-PT": "pt-PT",
    "ro-RO": "ro",
    "ru-RU": "ru",
    "sk-SK": "sk",
    "sv-SE": "sv",
}

PLACEHOLDER_RE = re.compile(r"\{[^{}]+\}")
EMAIL_RE = re.compile(r"\b[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}\b")
MENTION_RE = re.compile(r"(?<![\w/])@[\w.-]+")

EN_GB_REPLACEMENTS = [
    (r"\bAnalyze\b", "Analyse"),
    (r"\banalyze\b", "analyse"),
    (r"\bAnalyzing\b", "Analysing"),
    (r"\banalyzing\b", "analysing"),
    (r"\bColor\b", "Colour"),
    (r"\bcolor\b", "colour"),
    (r"\bColors\b", "Colours"),
    (r"\bcolors\b", "colours"),
    (r"\bCustomize\b", "Customise"),
    (r"\bcustomize\b", "customise"),
    (r"\bCustomized\b", "Customised"),
    (r"\bcustomized\b", "customised"),
    (r"\bCustomizing\b", "Customising"),
    (r"\bcustomizing\b", "customising"),
    (r"\bBehavior\b", "Behaviour"),
    (r"\bbehavior\b", "behaviour"),
    (r"\bBehaviors\b", "Behaviours"),
    (r"\bbehaviors\b", "behaviours"),
    (r"\bCenter\b", "Centre"),
    (r"\bcenter\b", "centre"),
    (r"\bCenters\b", "Centres"),
    (r"\bcenters\b", "centres"),
    (r"\bFiber\b", "Fibre"),
    (r"\bfiber\b", "fibre"),
    (r"\bFavorite\b", "Favourite"),
    (r"\bfavorite\b", "favourite"),
    (r"\bFavorites\b", "Favourites"),
    (r"\bfavorites\b", "favourites"),
    (r"\bInitialize\b", "Initialise"),
    (r"\binitialize\b", "initialise"),
    (r"\bInitialized\b", "Initialised"),
    (r"\binitialized\b", "initialised"),
    (r"\bInitializing\b", "Initialising"),
    (r"\binitializing\b", "initialising"),
    (r"\bLicense\b", "Licence"),
    (r"\blicense\b", "licence"),
    (r"\bLicenses\b", "Licences"),
    (r"\blicenses\b", "licences"),
    (r"\bMeter\b", "Metre"),
    (r"\bmeter\b", "metre"),
    (r"\bOptimize\b", "Optimise"),
    (r"\boptimize\b", "optimise"),
    (r"\bOptimized\b", "Optimised"),
    (r"\boptimized\b", "optimised"),
    (r"\bOptimizing\b", "Optimising"),
    (r"\boptimizing\b", "optimising"),
    (r"\bOrganization\b", "Organisation"),
    (r"\borganization\b", "organisation"),
    (r"\bOrganizations\b", "Organisations"),
    (r"\borganizations\b", "organisations"),
]

KEEP_ENGLISH_KEYS = {
    "autoReply.regex",
    "channels.placeholderServerUrl",
    "channels.placeholderWebhookUrl",
    "chat.stats.speed",
    "claudecode.title",
    "common.id",
    "companion.anomalyGuide.types.xss.name",
    "connections.recv",
    "extensions.modal.url",
    "extensions.modal.urlPlaceholder",
    "ideDiscovery.envVar",
    "memory.proposalSourceKindUrl",
    "providerPool.apiFormatOptions.google",
    "providerPool.beta",
    "resultCard.labels.ms",
    "resultCard.titles.grep",
    "resultCard.titles.rg",
    "settings.externalAgents.eyebrow",
    "settings.tts.eta",
    "skillStore.modal.typeClawdhub",
    "skillStore.modal.url",
    "skillStore.modal.urlPlaceholder",
    "speech.asrModelInfo.macosNative.name",
    "speech.asrModelInfo.whisperLargeTurbo.name",
    "speech.convertTask.previewKind.pdf",
    "system.ram",
    "system.statusOk",
    "system.vram",
}

KEEP_ENGLISH_PATTERNS = [
    re.compile(r"^tenants\.settings\.timezones\.(london|shanghai)$"),
]


def run_node_json(script: str) -> object:
    output = subprocess.check_output(
        ["node", "--input-type=module", "-e", script],
        cwd=ROOT,
        text=True,
    )
    return json.loads(output)


def load_audit_report() -> dict:
    output = subprocess.check_output(AUDIT_CMD, cwd=ROOT, text=True)
    return json.loads(output)


def load_flattened_en_us() -> dict[str, str]:
    script = """
import fs from 'node:fs'
import path from 'node:path'

const filePath = path.join(process.cwd(), 'src', 'i18n', 'locales', 'en-US.ts')
const raw = fs.readFileSync(filePath, 'utf8')
const executable = raw.replace(/^\\s*export\\s+default\\s*/m, 'return ')
const localeValue = new Function(executable)()

function flatten(value, prefix = '', out = {}) {
  if (typeof value === 'string') {
    out[prefix] = value
    return out
  }
  if (!value || typeof value !== 'object' || Array.isArray(value)) return out
  for (const [key, nested] of Object.entries(value)) {
    const next = prefix ? `${prefix}.${key}` : key
    flatten(nested, next, out)
  }
  return out
}

console.log(JSON.stringify(flatten(localeValue)))
"""
    return run_node_json(script)


def load_existing_overrides() -> dict:
    if not OUTPUT_PATH.exists():
        return {}

    script = f"""
import fs from 'node:fs'
const raw = fs.readFileSync({json.dumps(str(OUTPUT_PATH))}, 'utf8')
const executable = raw.replace(/^\\s*export\\s+default\\s*/m, 'return ')
console.log(JSON.stringify(new Function(executable)()))
"""
    return run_node_json(script)


def flatten_nested(value: object, prefix: str = "", out: dict[str, str] | None = None) -> dict[str, str]:
    if out is None:
        out = {}
    if isinstance(value, str):
        out[prefix] = value
        return out
    if not isinstance(value, dict):
        return out
    for key, nested in value.items():
        next_prefix = f"{prefix}.{key}" if prefix else str(key)
        flatten_nested(nested, next_prefix, out)
    return out


def mask_placeholders(text: str) -> tuple[str, dict[str, str]]:
    replacements: dict[str, str] = {}

    def repl(match: re.Match[str]) -> str:
        token = f"__ZIMA_PH_{len(replacements)}__"
        replacements[token] = match.group(0)
        return token

    masked = PLACEHOLDER_RE.sub(repl, text)
    masked = EMAIL_RE.sub(repl, masked)
    masked = MENTION_RE.sub(repl, masked)
    return masked, replacements


def restore_placeholders(text: str, replacements: dict[str, str]) -> str:
    restored = text
    for token, original in replacements.items():
        restored = restored.replace(token, original)
    return restored


def apply_en_gb_rules(text: str) -> str:
    updated = text
    for pattern, replacement in EN_GB_REPLACEMENTS:
        updated = re.sub(pattern, replacement, updated)
    return updated


def should_keep_english(key: str) -> bool:
    return key in KEEP_ENGLISH_KEYS or any(pattern.match(key) for pattern in KEEP_ENGLISH_PATTERNS)


def translate_google(text: str, target: str) -> str:
    params = {
        "client": "gtx",
        "sl": "en",
        "tl": target,
        "dt": "t",
        "q": text,
    }
    url = "https://translate.googleapis.com/translate_a/single?" + urllib.parse.urlencode(params)
    request = urllib.request.Request(
        url,
        headers={
            "User-Agent": "Mozilla/5.0",
            "Accept-Language": "en-US,en;q=0.9",
        },
    )
    with urllib.request.urlopen(request, timeout=20) as response:
        payload = json.loads(response.read().decode())
    return "".join(part[0] for part in payload[0] if part and part[0])


def translate_batch(entries: list[tuple[str, str]], target: str) -> list[tuple[str, str]]:
    masked_entries: list[tuple[str, dict[str, str]]] = []
    texts: list[str] = []
    for _, source in entries:
        masked, replacements = mask_placeholders(source)
        masked_entries.append((masked, replacements))
        texts.append(masked)

    joined = SPLIT_TOKEN.join(texts)
    translated = translate_google(joined, target)
    parts = translated.split(SPLIT_TOKEN)

    if len(parts) != len(entries):
        raise ValueError(f"split mismatch: expected {len(entries)} parts, got {len(parts)}")

    restored = []
    for (key, _), part, (_, replacements) in zip(entries, parts, masked_entries):
        restored.append((key, restore_placeholders(part, replacements)))
    return restored


def translate_entries(entries: list[tuple[str, str]], target: str) -> dict[str, str]:
    translated: dict[str, str] = {}
    batches: list[list[tuple[str, str]]] = []
    current: list[tuple[str, str]] = []
    current_size = 0

    for key, value in entries:
        size = len(value) + len(SPLIT_TOKEN)
        if current and current_size + size > MAX_BATCH_CHARS:
            batches.append(current)
            current = []
            current_size = 0
        current.append((key, value))
        current_size += size

    if current:
        batches.append(current)

    for index, batch in enumerate(batches, start=1):
        for attempt in range(1, MAX_RETRIES + 1):
            try:
                for key, value in translate_batch(batch, target):
                    translated[key] = value
                print(
                    f"[translate] {target} batch {index}/{len(batches)} ok ({len(batch)} keys)",
                    file=sys.stderr,
                )
                break
            except Exception as error:  # noqa: BLE001
                if attempt == MAX_RETRIES:
                    if len(batch) == 1:
                        raise
                    midpoint = max(1, len(batch) // 2)
                    left = translate_entries(batch[:midpoint], target)
                    right = translate_entries(batch[midpoint:], target)
                    translated.update(left)
                    translated.update(right)
                    break

                sleep_seconds = attempt * 2
                print(
                    f"[translate] {target} batch {index}/{len(batches)} retry {attempt} after error: {error}",
                    file=sys.stderr,
                )
                time.sleep(sleep_seconds)

    return translated


def set_nested(target: dict, dotted_key: str, value: str) -> None:
    cursor = target
    parts = dotted_key.split(".")
    for part in parts[:-1]:
        cursor = cursor.setdefault(part, {})
    cursor[parts[-1]] = value


def normalize_keep_english_overrides(generated: dict, en_us: dict[str, str]) -> int:
    changed = 0
    for locale_overrides in generated.values():
        if not isinstance(locale_overrides, dict):
            continue
        locale_flat = flatten_nested(locale_overrides)
        for key, current in locale_flat.items():
            if not should_keep_english(key):
                continue
            source = en_us.get(key)
            if source is None or current == source:
                continue
            set_nested(locale_overrides, key, source)
            changed += 1
    return changed


def to_ts(value: object, indent: int = 0) -> str:
    spacer = "  " * indent
    if isinstance(value, dict):
        if not value:
            return "{}"
        lines = ["{"]
        items = list(value.items())
        for index, (key, nested) in enumerate(items):
            suffix = "," if index < len(items) - 1 else ""
            lines.append(
                f'{spacer}  {json.dumps(key, ensure_ascii=False)}: {to_ts(nested, indent + 1)}{suffix}'
            )
        lines.append(f"{spacer}}}")
        return "\n".join(lines)
    return json.dumps(value, ensure_ascii=False)


def write_output(generated: dict) -> None:
    header = "// Auto-generated translation overrides for runtime and audit coverage.\n"
    OUTPUT_PATH.write_text(header + "export default " + to_ts(generated) + "\n", encoding="utf-8")


def main() -> None:
    audit = load_audit_report()
    en_us = load_flattened_en_us()
    existing = load_existing_overrides()
    generated = existing if isinstance(existing, dict) else {}

    normalized = normalize_keep_english_overrides(generated, en_us)
    if normalized:
        print(f"[normalize] restored {normalized} technical keys to English", file=sys.stderr)
        write_output(generated)

    for locale in sorted(GOOGLE_TARGETS):
        locale_report = next((item for item in audit["locales"] if item["locale"] == locale), None)
        if not locale_report:
            continue

        keys = [key for key in locale_report["fallbackKeys"] if not should_keep_english(key)]
        if not keys:
            continue

        existing_flat = flatten_nested(generated.get(locale, {}))
        entries = [
            (key, en_us[key])
            for key in keys
            if key in en_us and (key not in existing_flat or existing_flat[key] == en_us[key])
        ]
        if not entries:
            print(f"[locale] {locale}: already complete in overrides", file=sys.stderr)
            continue

        print(f"[locale] {locale}: translating {len(entries)} keys", file=sys.stderr)

        translated_map = translate_entries(entries, GOOGLE_TARGETS[locale])
        locale_overrides = generated.setdefault(locale, {})
        for key, translated in translated_map.items():
            set_nested(locale_overrides, key, translated)
        write_output(generated)
        print(f"[locale] {locale}: saved", file=sys.stderr)

    en_gb_report = next((item for item in audit["locales"] if item["locale"] == "en-GB"), None)
    if en_gb_report:
        changed = 0
        locale_overrides = generated.setdefault("en-GB", {})
        for key in en_gb_report["fallbackKeys"]:
            source = en_us.get(key)
            if not source:
                continue
            localized = apply_en_gb_rules(source)
            if localized != source:
                set_nested(locale_overrides, key, localized)
                changed += 1
        print(f"[locale] en-GB: generated {changed} locale-specific overrides", file=sys.stderr)
        write_output(generated)

    print(f"[done] wrote {OUTPUT_PATH}", file=sys.stderr)


if __name__ == "__main__":
    main()
