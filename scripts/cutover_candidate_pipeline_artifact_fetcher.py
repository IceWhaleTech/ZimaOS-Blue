#!/usr/bin/env python3
from __future__ import annotations

import argparse
import io
import json
import logging
import os
import sys
import zipfile
from datetime import datetime, timezone
from pathlib import Path, PurePosixPath
from typing import Any, Dict, List, Optional
from urllib import error, parse, request


LOG = logging.getLogger("cutover-candidate-pipeline-artifact-fetcher")


class GitHubAPIError(RuntimeError):
    pass


class GitHubActionsClient:
    def __init__(self, repo: str, api_base_url: str = "https://api.github.com", token: str = ""):
        self.repo = normalize_repo(repo)
        self.api_base_url = normalize_api_base_url(api_base_url)
        self.token = str(token or "").strip()
        self.repo_path = encode_repo_path(self.repo)

    def list_workflow_runs(self, workflow: str, branch: str, per_page: int, page: int) -> Dict[str, Any]:
        query = {
            "status": "completed",
            "per_page": str(max(1, min(per_page, 100))),
            "page": str(max(page, 1)),
        }
        if branch:
            query["branch"] = branch
        encoded_query = parse.urlencode(query)
        workflow_path = parse.quote(str(workflow or "").strip(), safe="")
        return self._json_request("GET", f"/repos/{self.repo_path}/actions/workflows/{workflow_path}/runs?{encoded_query}")

    def list_run_artifacts(self, run_id: str) -> Dict[str, Any]:
        return self._json_request("GET", f"/repos/{self.repo_path}/actions/runs/{run_id}/artifacts?per_page=100")

    def download_artifact_zip(self, artifact_id: str) -> bytes:
        return self._raw_request("GET", f"/repos/{self.repo_path}/actions/artifacts/{artifact_id}/zip")

    def _json_request(self, method: str, path: str) -> Dict[str, Any]:
        raw = self._raw_request(method, path)
        if not raw.strip():
            return {}
        try:
            return json.loads(raw.decode("utf-8", errors="replace"))
        except json.JSONDecodeError as exc:
            raise GitHubAPIError(f"{method} {path} returned non-JSON body") from exc

    def _raw_request(self, method: str, path: str) -> bytes:
        headers = {
            "Accept": "application/vnd.github+json",
            "X-GitHub-Api-Version": "2022-11-28",
            "User-Agent": "cutover-candidate-pipeline-artifact-fetcher",
        }
        if self.token:
            headers["Authorization"] = f"Bearer {self.token}"
        req = request.Request(f"{self.api_base_url}{path}", method=method, headers=headers)
        try:
            with request.urlopen(req, timeout=30.0) as resp:
                return resp.read()
        except error.HTTPError as exc:
            body = exc.read().decode("utf-8", errors="replace")
            raise GitHubAPIError(f"{method} {path} failed: HTTP {exc.code}: {body}") from exc
        except error.URLError as exc:
            raise GitHubAPIError(f"{method} {path} failed: {exc}") from exc


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Download cutover candidate pipeline report artifacts from prior GitHub Actions workflow runs.")
    parser.add_argument("--repo", default=os.environ.get("GITHUB_REPOSITORY", ""), help="GitHub repository in owner/repo form.")
    parser.add_argument("--github-token", default=os.environ.get("GITHUB_TOKEN", ""), help="Optional GitHub token used for Actions API access.")
    parser.add_argument("--api-base-url", default=os.environ.get("GITHUB_API_URL", "https://api.github.com"), help="GitHub API base URL.")
    parser.add_argument("--workflow", default="cutover-candidate-pipeline.yml", help="Workflow file name or workflow id used to list historical runs.")
    parser.add_argument("--branch", default=os.environ.get("GITHUB_REF_NAME", ""), help="Optional branch filter.")
    parser.add_argument("--artifact-name", default="cutover-candidate-pipeline-report", help="Artifact name to download from matching workflow runs.")
    parser.add_argument("--exclude-run-id", default=os.environ.get("GITHUB_RUN_ID", ""), help="Optional workflow run id to exclude from history fetches.")
    parser.add_argument("--limit-runs", type=int, default=10, help="Maximum number of completed workflow runs to inspect after exclusions.")
    parser.add_argument("--output-dir", default="docs/reports/cutover_candidate_pipeline_history_artifacts", help="Directory where fetched artifacts will be extracted.")
    parser.add_argument("--output-json", default="docs/reports/cutover_candidate_pipeline_history_artifacts/manifest.json", help="Path for the structured fetch manifest.")
    parser.add_argument("--verbose", "-v", action="store_true", help="Enable verbose logging.")
    return parser.parse_args()


def normalize_repo(raw: str) -> str:
    repo = str(raw or "").strip().strip("/")
    if not repo or "/" not in repo:
        raise ValueError("repo must be in owner/repo form")
    owner, name = repo.split("/", 1)
    owner = owner.strip()
    name = name.strip()
    if not owner or not name:
        raise ValueError("repo must be in owner/repo form")
    return f"{owner}/{name}"


def normalize_api_base_url(raw: str) -> str:
    base = str(raw or "").strip().rstrip("/")
    if not base:
        return "https://api.github.com"
    return base


def encode_repo_path(repo: str) -> str:
    owner, name = repo.split("/", 1)
    return "/".join(parse.quote(part, safe="") for part in (owner, name))


def iter_workflow_runs(
    client: GitHubActionsClient,
    workflow: str,
    branch: str,
    exclude_run_id: str,
    limit_runs: int,
) -> List[Dict[str, Any]]:
    target_count = max(limit_runs, 0)
    if target_count == 0:
        return []

    runs: List[Dict[str, Any]] = []
    page = 1
    per_page = min(max(target_count, 1), 100)
    excluded = str(exclude_run_id or "").strip()

    while len(runs) < target_count:
        payload = client.list_workflow_runs(workflow=workflow, branch=branch, per_page=per_page, page=page)
        workflow_runs = payload.get("workflow_runs") if isinstance(payload, dict) else None
        if not isinstance(workflow_runs, list) or not workflow_runs:
            break
        for workflow_run in workflow_runs:
            if not isinstance(workflow_run, dict):
                continue
            run_id = str(workflow_run.get("id") or "").strip()
            if excluded and run_id == excluded:
                continue
            runs.append(workflow_run)
            if len(runs) >= target_count:
                break
        if len(workflow_runs) < per_page:
            break
        page += 1

    return runs[:target_count]


def sanitize_zip_member(name: str) -> Optional[Path]:
    pure_path = PurePosixPath(str(name or ""))
    if not pure_path.name:
        return None
    if pure_path.is_absolute():
        return None
    if any(part in ("", ".", "..") for part in pure_path.parts):
        return None
    return Path(*pure_path.parts)


def extract_artifact_zip(raw_zip: bytes, destination: Path) -> List[str]:
    extracted: List[str] = []
    destination.mkdir(parents=True, exist_ok=True)
    with zipfile.ZipFile(io.BytesIO(raw_zip)) as archive:
        for member in archive.infolist():
            if member.is_dir():
                continue
            relative_path = sanitize_zip_member(member.filename)
            if relative_path is None:
                LOG.warning("skipping unsafe artifact member %s", member.filename)
                continue
            target_path = destination / relative_path
            target_path.parent.mkdir(parents=True, exist_ok=True)
            target_path.write_bytes(archive.read(member))
            extracted.append(str(relative_path))
    extracted.sort()
    return extracted


def is_cutover_candidate_pipeline_report_payload(payload: Any) -> bool:
    if not isinstance(payload, dict):
        return False
    if not isinstance(payload.get("status"), str):
        return False
    if not isinstance(payload.get("ready"), bool):
        return False
    return isinstance(payload.get("steps"), list)


def stage_pipeline_report_files(output_dir: Path, destination: Path, extracted_files: List[str], run_id: str, artifact_id: str) -> List[str]:
    staged: List[str] = []
    report_dir = output_dir / "pipeline_reports"
    report_dir.mkdir(parents=True, exist_ok=True)
    for relative_name in extracted_files:
        if not relative_name.endswith(".json"):
            continue
        source_path = destination / relative_name
        try:
            payload = json.loads(source_path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        if not is_cutover_candidate_pipeline_report_payload(payload):
            continue
        target_name = f"run-{run_id}-artifact-{artifact_id}-{Path(relative_name).name}"
        target_path = report_dir / target_name
        target_path.write_bytes(source_path.read_bytes())
        staged.append(str(target_path.relative_to(output_dir)))
    staged.sort()
    return staged


def fetch_cutover_candidate_pipeline_history(client: GitHubActionsClient, args: argparse.Namespace) -> Dict[str, Any]:
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)

    workflow_runs = iter_workflow_runs(
        client=client,
        workflow=str(args.workflow or "").strip(),
        branch=str(args.branch or "").strip(),
        exclude_run_id=str(args.exclude_run_id or "").strip(),
        limit_runs=int(args.limit_runs),
    )

    downloaded: List[Dict[str, Any]] = []
    artifact_name = str(args.artifact_name or "").strip()
    for workflow_run in workflow_runs:
        run_id = str(workflow_run.get("id") or "").strip()
        if not run_id:
            continue
        payload = client.list_run_artifacts(run_id)
        artifacts = payload.get("artifacts") if isinstance(payload, dict) else None
        if not isinstance(artifacts, list):
            continue
        for artifact in artifacts:
            if not isinstance(artifact, dict):
                continue
            if str(artifact.get("name") or "").strip() != artifact_name:
                continue
            if bool(artifact.get("expired")):
                continue
            artifact_id = str(artifact.get("id") or "").strip()
            if not artifact_id:
                continue
            artifact_zip = client.download_artifact_zip(artifact_id)
            destination = output_dir / f"run-{run_id}-artifact-{artifact_id}"
            extracted_files = extract_artifact_zip(artifact_zip, destination)
            pipeline_report_files = stage_pipeline_report_files(output_dir, destination, extracted_files, run_id, artifact_id)
            downloaded.append(
                {
                    "run_id": run_id,
                    "run_number": workflow_run.get("run_number"),
                    "run_attempt": workflow_run.get("run_attempt"),
                    "head_branch": workflow_run.get("head_branch"),
                    "display_title": workflow_run.get("display_title"),
                    "created_at": workflow_run.get("created_at"),
                    "updated_at": workflow_run.get("updated_at"),
                    "artifact_id": artifact_id,
                    "artifact_name": artifact.get("name"),
                    "size_in_bytes": artifact.get("size_in_bytes"),
                    "destination": str(destination),
                    "extracted_files": extracted_files,
                    "pipeline_report_files": pipeline_report_files,
                }
            )

    return {
        "generated_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "repo": client.repo,
        "workflow": str(args.workflow or "").strip(),
        "branch": str(args.branch or "").strip(),
        "artifact_name": artifact_name,
        "exclude_run_id": str(args.exclude_run_id or "").strip(),
        "limit_runs": int(args.limit_runs),
        "considered_run_count": len(workflow_runs),
        "downloaded_artifact_count": len(downloaded),
        "downloaded_report_count": sum(len(entry["pipeline_report_files"]) for entry in downloaded),
        "artifacts": downloaded,
    }


def write_manifest(manifest: Dict[str, Any], output_json: Path) -> None:
    output_json.parent.mkdir(parents=True, exist_ok=True)
    output_json.write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def main() -> int:
    args = parse_args()
    logging.basicConfig(level=logging.DEBUG if args.verbose else logging.INFO, format="%(levelname)s %(message)s")

    try:
        client = GitHubActionsClient(repo=args.repo, api_base_url=args.api_base_url, token=args.github_token)
        manifest = fetch_cutover_candidate_pipeline_history(client, args)
    except Exception as exc:
        LOG.error("cutover candidate pipeline artifact fetch failed: %s", exc)
        return 1

    write_manifest(manifest, Path(args.output_json))
    LOG.info(
        "downloaded %d cutover candidate pipeline artifacts from %d workflow runs",
        manifest.get("downloaded_artifact_count", 0),
        manifest.get("considered_run_count", 0),
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
