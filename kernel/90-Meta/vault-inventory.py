#!/usr/bin/env python3
"""Read-only inventory of instance-scoped and explicitly approved repositories."""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import subprocess
import sys
from collections import Counter
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from instance import load_instance
from cell_scope import approved_cross_scope_repositories
from vault_frontmatter import read_frontmatter

CORE_APP_PREFIXES: tuple[str, ...] = ()
APPROVED_CROSS_APP_REPOSITORIES: set[str] = set()
STATUSES = (
    "current",
    "changed",
    "new",
    "acknowledged-no-node",
    "acknowledged-no-change",
    "acknowledged-review-rejected",
    "acknowledged-inspection-limited",
    "container",
    "archived",
    "renamed-or-transferred",
    "missing-candidate",
    "branch-ambiguous",
    "invalid-note",
)

SYNC_ACKNOWLEDGEMENTS = Path("90-Meta/.sync-acknowledgements.json")
ACKNOWLEDGEMENT_FIELDS = {
    "repository",
    "branch",
    "analyzed_sha",
    "decision",
    "analysis_date",
}
ACKNOWLEDGEMENT_DECISIONS = {
    "no-durable-node": "acknowledged-no-node",
    "no-documentation-change": "acknowledged-no-change",
    "review-rejected": "acknowledged-review-rejected",
    "inspection-limited": "acknowledged-inspection-limited",
}

CONTAINER_REPOSITORIES: dict[str, str] = {}
DEFAULT_ORG = ""
GITHUB_HOST = "github.com"
AUTH_PROBE_QUERY = """
query($org: String!) {
  viewer { login }
  organization(login: $org) { login }
}
"""
INVENTORY_QUERY = """
query($org: String!, $cursor: String) {
  organization(login: $org) {
    repositories(first: 100, after: $cursor) {
      nodes {
        name
        isArchived
        isFork
        url
        defaultBranchRef { name }
        main: ref(qualifiedName: "refs/heads/main") { name target { oid } }
        master: ref(qualifiedName: "refs/heads/master") { name target { oid } }
      }
      pageInfo { hasNextPage endCursor }
    }
  }
}
"""


def configure_scope(root: Path) -> None:
    """Derive repository identity from instance.yaml and the cell scope note."""
    instance = load_instance(root / "instance.yaml")
    sources = instance["sources"]
    global CORE_APP_PREFIXES, APPROVED_CROSS_APP_REPOSITORIES
    global CONTAINER_REPOSITORIES, DEFAULT_ORG
    CORE_APP_PREFIXES = tuple(
        f"{str(prefix).rstrip('-')}-"
        for prefix in sources["repo_prefixes"]
        if str(prefix).strip()
    )
    DEFAULT_ORG = str(sources["github_org"])

    remote = instance["vault"]["remote"].rstrip("/")
    container = remote.rsplit("/", 1)[-1].removesuffix(".git") if remote else ""
    CONTAINER_REPOSITORIES = (
        {container: "90-Meta/Repositorio del vault.md"} if container else {}
    )

    APPROVED_CROSS_APP_REPOSITORIES = approved_cross_scope_repositories(
        root,
        CORE_APP_PREFIXES,
        container,
    )


configure_scope(Path(__file__).resolve().parents[1])


def compact_graphql(query: str) -> str:
    """Make GraphQL safe to forward through native CLI wrappers."""
    return " ".join(query.split())


def is_tracked_repository(name: str) -> bool:
    return name.startswith(CORE_APP_PREFIXES) or name in APPROVED_CROSS_APP_REPOSITORIES


class OperationalError(RuntimeError):
    pass


@dataclass(frozen=True)
class GitHubContext:
    login: str
    source: str
    token: str


def run(
    command: list[str],
    allow_failure: bool = False,
    env: dict[str, str] | None = None,
) -> subprocess.CompletedProcess[str]:
    search_env = env if env is not None else os.environ
    executable = shutil.which(command[0], path=search_env.get("PATH"))
    resolved_command = [executable or command[0], *command[1:]]
    try:
        result = subprocess.run(resolved_command, text=True, capture_output=True, env=env, check=False)
    except FileNotFoundError as error:
        if command and command[0] == "gh":
            raise OperationalError("GitHub CLI 'gh' is required; install it and authenticate first") from error
        raise OperationalError(f"required command not found: {command[0]}") from error
    if result.returncode and not allow_failure:
        message = result.stderr.strip() or result.stdout.strip() or f"exit {result.returncode}"
        raise OperationalError(f"{' '.join(command[:3])}: {message}")
    return result


def token_env(token: str) -> dict[str, str]:
    env = os.environ.copy()
    env["GH_TOKEN"] = token
    env.pop("GITHUB_TOKEN", None)
    return env


def auth_store_env() -> dict[str, str]:
    env = os.environ.copy()
    env.pop("GH_TOKEN", None)
    env.pop("GITHUB_TOKEN", None)
    return env


def account_token(login: str) -> str:
    result = run(
        ["gh", "auth", "token", "--hostname", GITHUB_HOST, "--user", login],
        allow_failure=True,
        env=auth_store_env(),
    )
    token = result.stdout.strip()
    if result.returncode or not token:
        raise OperationalError(
            f"GitHub account {login!r} has no usable stored token; run gh auth login --hostname {GITHUB_HOST}"
        )
    return token


def probe_org_access(org: str, token: str) -> str | None:
    result = run(
        [
            "gh",
            "api",
            "graphql",
            "-f",
            f"query={compact_graphql(AUTH_PROBE_QUERY)}",
            "-f",
            f"org={org}",
        ],
        allow_failure=True,
        env=token_env(token),
    )
    if result.returncode:
        return None
    try:
        data = json.loads(result.stdout).get("data", {})
    except json.JSONDecodeError:
        return None
    organization = data.get("organization")
    viewer = data.get("viewer") or {}
    login = viewer.get("login")
    return str(login) if organization and login else None


def authenticated_accounts() -> list[dict[str, Any]]:
    result = run(
        ["gh", "auth", "status", "--hostname", GITHUB_HOST, "--json", "hosts"],
        env=auth_store_env(),
    )
    try:
        data = json.loads(result.stdout)
    except json.JSONDecodeError as error:
        raise OperationalError(f"gh auth status returned invalid JSON: {error}") from error
    accounts = data.get("hosts", {}).get(GITHUB_HOST, [])
    authenticated = [
        account
        for account in accounts
        if account.get("state") == "success" and isinstance(account.get("login"), str)
    ]
    if not authenticated:
        errors = sorted(
            {
                str(account["error"])
                for account in accounts
                if account.get("state") == "error" and account.get("error")
            }
        )
        if errors:
            raise OperationalError(f"could not validate GitHub accounts: {'; '.join(errors)}")
    return authenticated


def resolve_github_context(org: str, requested_user: str | None) -> GitHubContext:
    if requested_user:
        token = account_token(requested_user)
        login = probe_org_access(org, token)
        if not login:
            raise OperationalError(
                f"GitHub account {requested_user!r} cannot access {org}; verify permissions and SSO authorization"
            )
        return GitHubContext(login=login, source="explicit", token=token)

    ambient_token = os.environ.get("GH_TOKEN") or os.environ.get("GITHUB_TOKEN")
    if ambient_token:
        login = probe_org_access(org, ambient_token)
        if not login:
            raise OperationalError(
                f"the token from GH_TOKEN/GITHUB_TOKEN cannot access {org}; verify permissions and SSO authorization"
            )
        return GitHubContext(login=login, source="environment", token=ambient_token)

    accounts = authenticated_accounts()
    active = [account for account in accounts if account.get("active")]
    inactive = [account for account in accounts if not account.get("active")]
    accessible: list[GitHubContext] = []
    for account in active + inactive:
        login = str(account["login"])
        try:
            token = account_token(login)
        except OperationalError:
            continue
        verified_login = probe_org_access(org, token)
        if not verified_login:
            continue
        source = "active" if account.get("active") else "discovered"
        context = GitHubContext(login=verified_login, source=source, token=token)
        if source == "active":
            return context
        accessible.append(context)

    if len(accessible) == 1:
        return accessible[0]
    if len(accessible) > 1:
        candidates = ", ".join(sorted(context.login for context in accessible))
        raise OperationalError(
            f"multiple GitHub accounts can access {org}: {candidates}; choose one with --github-user"
        )
    raise OperationalError(
        f"no authenticated GitHub account can access {org}; run gh auth login and authorize SSO if required"
    )


def load_notes(root: Path) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    for path in sorted((root / "20-Repos").glob("*/*.md")):
        fields = read_frontmatter(path)
        aliases = fields.get("aliases", [])
        if isinstance(aliases, str):
            aliases = [aliases]
        full_name = next((str(value) for value in aliases if is_tracked_repository(str(value))), None)
        commit = fields.get("commit-analizado")
        branch = fields.get("rama-analizada")
        valid = bool(
            full_name
            and isinstance(commit, str)
            and re.fullmatch(r"[0-9a-f]{12}", commit)
            and branch in {"main", "master"}
        )
        records.append(
            {
                "note": path.stem,
                "path": path.relative_to(root).as_posix(),
                "repo": full_name or path.stem,
                "recorded_sha": commit if isinstance(commit, str) else None,
                "recorded_branch": branch if isinstance(branch, str) else None,
                "valid": valid,
            }
        )
    return records


def unique_json_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    result: dict[str, Any] = {}
    for key, value in pairs:
        if key in result:
            raise ValueError(f"duplicate JSON key: {key}")
        result[key] = value
    return result


def load_acknowledgements(root: Path) -> list[dict[str, str]]:
    path = root / SYNC_ACKNOWLEDGEMENTS
    if not path.exists():
        return []
    try:
        payload = json.loads(
            path.read_text(encoding="utf-8"),
            object_pairs_hook=unique_json_object,
        )
    except (OSError, UnicodeError, json.JSONDecodeError, ValueError) as error:
        raise OperationalError(f"invalid {SYNC_ACKNOWLEDGEMENTS}: {error}") from error
    if not isinstance(payload, dict) or set(payload) != {"version", "repositories"}:
        raise OperationalError(
            f"invalid {SYNC_ACKNOWLEDGEMENTS}: expected version and repositories"
        )
    if type(payload["version"]) is not int or payload["version"] != 1:
        raise OperationalError(f"invalid {SYNC_ACKNOWLEDGEMENTS}: unsupported version")
    raw_records = payload["repositories"]
    if not isinstance(raw_records, list):
        raise OperationalError(f"invalid {SYNC_ACKNOWLEDGEMENTS}: repositories must be a list")

    records: list[dict[str, str]] = []
    for index, raw in enumerate(raw_records):
        prefix = f"{SYNC_ACKNOWLEDGEMENTS} repositories[{index}]"
        if not isinstance(raw, dict) or set(raw) != ACKNOWLEDGEMENT_FIELDS:
            raise OperationalError(f"invalid {prefix}: fields do not match the contract")
        repository = raw["repository"]
        branch = raw["branch"]
        analyzed_sha = raw["analyzed_sha"]
        decision = raw["decision"]
        analysis_date = raw["analysis_date"]
        if not isinstance(repository, str) or not is_tracked_repository(repository):
            raise OperationalError(f"invalid {prefix}: repository is outside the tracked scope")
        if not isinstance(branch, str) or branch not in {"main", "master"}:
            raise OperationalError(f"invalid {prefix}: branch must be main or master")
        if not isinstance(analyzed_sha, str) or re.fullmatch(r"[0-9a-f]{12}", analyzed_sha) is None:
            raise OperationalError(f"invalid {prefix}: analyzed_sha must be 12 lowercase hexadecimal characters")
        if decision not in ACKNOWLEDGEMENT_DECISIONS:
            raise OperationalError(
                f"invalid {prefix}: decision is outside the synchronization contract"
            )
        if (
            not isinstance(analysis_date, str)
            or re.fullmatch(r"[0-9]{4}-[0-9]{2}-[0-9]{2}", analysis_date) is None
        ):
            raise OperationalError(f"invalid {prefix}: analysis_date must use YYYY-MM-DD")
        try:
            datetime.strptime(analysis_date, "%Y-%m-%d")
        except ValueError as error:
            raise OperationalError(f"invalid {prefix}: analysis_date must use YYYY-MM-DD") from error
        records.append(raw)

    repositories = [item["repository"] for item in records]
    if repositories != sorted(repositories):
        raise OperationalError(f"invalid {SYNC_ACKNOWLEDGEMENTS}: repositories are not sorted")
    if len(repositories) != len(set(repositories)):
        raise OperationalError(f"invalid {SYNC_ACKNOWLEDGEMENTS}: duplicate repository")
    return records


def load_org(org: str, github: GitHubContext) -> list[dict[str, Any]]:
    repos: list[dict[str, Any]] = []
    cursor: str | None = None
    while True:
        command = [
            "gh",
            "api",
            "graphql",
            "-f",
            f"query={compact_graphql(INVENTORY_QUERY)}",
            "-f",
            f"org={org}",
        ]
        if cursor is not None:
            command.extend(["-f", f"cursor={cursor}"])
        result = run(command, env=token_env(github.token))
        try:
            organization = json.loads(result.stdout).get("data", {}).get("organization")
        except json.JSONDecodeError as error:
            raise OperationalError(f"gh api graphql returned invalid JSON: {error}") from error
        if not organization:
            raise OperationalError(f"GitHub account {github.login!r} cannot read organization {org}")
        connection = organization.get("repositories", {})
        repos.extend(connection.get("nodes", []))
        page_info = connection.get("pageInfo", {})
        if not page_info.get("hasNextPage"):
            break
        cursor = page_info.get("endCursor")
        if not cursor:
            raise OperationalError("GitHub repository pagination returned no end cursor")

    records: list[dict[str, Any]] = []
    for repo in repos:
        if not is_tracked_repository(str(repo.get("name", ""))):
            continue
        selected = next(
            (
                (branch, repo.get(branch))
                for branch in ("main", "master")
                if repo.get(branch) and repo[branch].get("target", {}).get("oid")
            ),
            (None, None),
        )
        branch, reference = selected
        sha = reference.get("target", {}).get("oid", "")[:12] if reference else None
        records.append({**repo, "branch": branch, "sha": sha})
    return records


def resolve_missing(org: str, repo: str, github: GitHubContext) -> tuple[str, str | None]:
    result = run(
        ["gh", "repo", "view", f"{org}/{repo}", "--json", "nameWithOwner,url"],
        allow_failure=True,
        env=token_env(github.token),
    )
    if result.returncode:
        return "missing-candidate", None
    try:
        data = json.loads(result.stdout)
    except json.JSONDecodeError:
        return "missing-candidate", None
    expected = f"{org}/{repo}".lower()
    actual = str(data.get("nameWithOwner", "")).lower()
    if actual and actual != expected:
        return "renamed-or-transferred", data.get("nameWithOwner")
    return "missing-candidate", None


def build_inventory(root: Path, org: str, github: GitHubContext) -> list[dict[str, Any]]:
    notes = load_notes(root)
    acknowledgements = load_acknowledgements(root)
    org_repos = load_org(org, github)
    by_name = {repo["name"]: repo for repo in org_repos}
    note_by_repo = {
        note["repo"]: note
        for note in notes
        if is_tracked_repository(str(note["repo"]))
    }
    acknowledgement_by_repo = {
        item["repository"]: item for item in acknowledgements
    }
    invalid_overlap = sorted(
        repository
        for repository in set(note_by_repo).intersection(acknowledgement_by_repo)
        if (
            acknowledgement_by_repo[repository]["decision"] == "no-durable-node"
            or (
                acknowledgement_by_repo[repository]["decision"] != "no-documentation-change"
                and note_by_repo[repository]["recorded_branch"]
                == acknowledgement_by_repo[repository]["branch"]
                and note_by_repo[repository]["recorded_sha"]
                == acknowledgement_by_repo[repository]["analyzed_sha"]
            )
        )
    )
    if invalid_overlap:
        raise OperationalError(
            "repository has an incompatible vault note and sync acknowledgement: "
            + ", ".join(invalid_overlap)
        )

    missing_notes = sorted(
        repository
        for repository, acknowledgement in acknowledgement_by_repo.items()
        if acknowledgement["decision"] == "no-documentation-change"
        and repository not in note_by_repo
    )
    if missing_notes:
        raise OperationalError(
            "no-documentation-change acknowledgement requires an existing vault note: "
            + ", ".join(missing_notes)
        )

    records: list[dict[str, Any]] = []
    for repo in sorted(org_repos, key=lambda item: item["name"]):
        name = repo["name"]
        note = note_by_repo.get(name)
        acknowledgement = acknowledgement_by_repo.get(name)
        branch, sha = repo["branch"], repo["sha"]
        container_path = CONTAINER_REPOSITORIES.get(name)
        container_note = (
            Path(container_path).stem
            if container_path and (root / container_path).is_file()
            else None
        )
        if container_path and not container_note:
            status, details = "invalid-note", f"missing container note: {container_path}"
        elif note and not note["valid"]:
            status, details = "invalid-note", "missing/invalid alias, SHA or production branch"
        elif repo.get("isArchived"):
            status, details = "archived", "repository is archived"
        elif not branch:
            status, details = "branch-ambiguous", "no main or master branch"
        elif container_note:
            status = "container"
            details = "vault container documented in Meta; SHA tracking is intentionally not self-referential"
        elif acknowledgement and (
            acknowledgement["branch"] == branch
            and acknowledgement["analyzed_sha"] == sha
        ):
            status = ACKNOWLEDGEMENT_DECISIONS[acknowledgement["decision"]]
            details = {
                "no-durable-node": (
                    "complete tree accepted at this production-branch SHA; "
                    "no durable node selected"
                ),
                "no-documentation-change": (
                    "inspection accepted at this production-branch SHA; "
                    "no documentation change required; note baseline retained"
                ),
                "review-rejected": (
                    "inspection closed at this production-branch SHA; "
                    "the reviewed package was not accepted"
                ),
                "inspection-limited": (
                    "inspection closed at this production-branch SHA without "
                    "an accepted semantic package"
                ),
            }[acknowledgement["decision"]]
        elif not note:
            status, details = (
                "new",
                "no matching vault note; any prior synchronization cursor is stale",
            )
        elif note["recorded_sha"] == sha:
            status, details = "current", "recorded SHA matches production branch"
        else:
            status, details = "changed", "production branch SHA differs from note"
        records.append(
            {
                "note": container_note or (note["note"] if note else None),
                "repo": name,
                "status": status,
                "branch": branch,
                "recorded_sha": (
                    note["recorded_sha"]
                    if note and not container_path
                    else acknowledgement["analyzed_sha"]
                    if acknowledgement and not container_path
                    else None
                ),
                "remote_sha": sha,
                "details": details,
            }
        )

    for note in notes:
        if not note["valid"] and note["repo"] not in by_name:
            records.append(
                {
                    "note": note["note"],
                    "repo": note["repo"],
                    "status": "invalid-note",
                    "branch": note["recorded_branch"],
                    "recorded_sha": note["recorded_sha"],
                    "remote_sha": None,
                    "details": "missing/invalid alias, SHA or production branch",
                }
            )
        elif note["repo"] not in by_name:
            status, destination = resolve_missing(org, note["repo"], github)
            records.append(
                {
                    "note": note["note"],
                    "repo": note["repo"],
                    "status": status,
                    "branch": note["recorded_branch"],
                    "recorded_sha": note["recorded_sha"],
                    "remote_sha": None,
                    "details": destination or "repository absent from organization inventory",
                }
            )
    for acknowledgement in acknowledgements:
        repository = acknowledgement["repository"]
        if repository in by_name:
            continue
        status, destination = resolve_missing(org, repository, github)
        records.append(
            {
                "note": None,
                "repo": repository,
                "status": status,
                "branch": acknowledgement["branch"],
                "recorded_sha": acknowledgement["analyzed_sha"],
                "remote_sha": None,
                "details": destination or "repository absent from organization inventory",
            }
        )
    return sorted(records, key=lambda item: (item["repo"], item["note"] or ""))


def filter_records(records: list[dict[str, Any]], query: str | None) -> list[dict[str, Any]]:
    if not query:
        return records
    query = query.lower()
    selected = [
        item
        for item in records
        if query in {str(item.get("note") or "").lower(), str(item.get("repo") or "").lower()}
        or re.sub(r"^app\d{5}-", "", str(item.get("repo") or "").lower()) == query
    ]
    if not selected:
        raise OperationalError(f"--repo did not match a note or repository: {query}")
    return selected


def payload(org: str, records: list[dict[str, Any]], github: GitHubContext) -> dict[str, Any]:
    counts = Counter(item["status"] for item in records)
    return {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "org": org,
        "github": {
            "host": GITHUB_HOST,
            "login": github.login,
            "source": github.source,
        },
        "summary": {status: counts.get(status, 0) for status in STATUSES},
        "repos": records,
    }


def render_markdown(data: dict[str, Any]) -> str:
    lines = [
        "# Vault inventory",
        "",
        f"- Generated: {data['generated_at']}",
        f"- Organization: `{data['org']}`",
        f"- GitHub account: `{data['github']['login']}` ({data['github']['source']})",
        f"- Records: {len(data['repos'])}",
        "",
        "| Status | Count |",
        "|---|---:|",
    ]
    lines.extend(f"| `{status}` | {count} |" for status, count in data["summary"].items())
    lines.extend(
        [
            "",
            "| Repo | Note | Status | Branch | Recorded | Remote | Details |",
            "|---|---|---|---|---|---|---|",
        ]
    )
    for item in data["repos"]:
        values = [
            item["repo"],
            item["note"] or "—",
            item["status"],
            item["branch"] or "—",
            item["recorded_sha"] or "—",
            item["remote_sha"] or "—",
            item["details"],
        ]
        lines.append("| " + " | ".join(str(value).replace("|", "\\|") for value in values) + " |")
    return "\n".join(lines)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--org", help="Override sources.github_org for this invocation")
    parser.add_argument("--github-user", help="Use this stored gh account for the current command only")
    parser.add_argument("--repo")
    parser.add_argument("--format", choices=("markdown", "json"), default="markdown")
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1], help=argparse.SUPPRESS)
    return parser.parse_args()


def configure_output() -> None:
    for stream in (sys.stdout, sys.stderr):
        reconfigure = getattr(stream, "reconfigure", None)
        if reconfigure:
            reconfigure(errors="backslashreplace")


def main() -> int:
    configure_output()
    args = parse_args()
    configure_scope(args.root.resolve())
    org = args.org or DEFAULT_ORG
    if not org:
        print("ERROR: sources.github_org is required", file=sys.stderr)
        return 2
    try:
        github = resolve_github_context(org, args.github_user)
        records = filter_records(build_inventory(args.root.resolve(), org, github), args.repo)
    except OperationalError as error:
        print(f"ERROR: {error}", file=sys.stderr)
        return 2
    data = payload(org, records, github)
    print(json.dumps(data, indent=2) if args.format == "json" else render_markdown(data))
    return 0


if __name__ == "__main__":
    sys.exit(main())
