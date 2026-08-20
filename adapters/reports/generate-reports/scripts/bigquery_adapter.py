#!/usr/bin/env python3
"""Bounded BigQuery adapter for bundled report queries."""
from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Any

from report_engine import Period, ReportError


def base_command(recipe: dict[str, Any]) -> list[str]:
    return [
        "bq",
        f'--project_id={recipe["project"]}',
        f'--location={recipe["location"]}',
        "--quiet",
    ]


def parameters(period: Period) -> list[str]:
    return [
        f"--parameter=start_suffix:STRING:{period.start_suffix}",
        f"--parameter=end_suffix:STRING:{period.end_suffix}",
    ]


def run(command: list[str], query: str) -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(command, input=query, text=True, capture_output=True, check=False)
    except FileNotFoundError as error:
        raise ReportError("renderer-unavailable", "bq executable not found") from error


def dry_run(recipe: dict[str, Any], period: Period, query_path: Path) -> int:
    query = query_path.read_text(encoding="utf-8")
    command = base_command(recipe) + [
        "--format=prettyjson",
        "query",
        "--dry_run",
        "--use_legacy_sql=false",
        *parameters(period),
    ]
    result = run(command, query)
    if result.returncode:
        raise ReportError("dataset-invalid", result.stderr.strip() or "BigQuery dry-run failed")
    payload = json.loads(result.stdout)
    processed = int(payload.get("statistics", {}).get("totalBytesProcessed", 0))
    if processed > int(recipe["maximum_bytes_billed"]):
        raise ReportError(
            "query-cap-exceeded",
            f'{processed} > {recipe["maximum_bytes_billed"]}',
        )
    return processed


def extract(recipe: dict[str, Any], period: Period, query_path: Path, csv_path: Path) -> None:
    query = query_path.read_text(encoding="utf-8")
    command = base_command(recipe) + [
        "--format=csv",
        "query",
        "--use_legacy_sql=false",
        f'--maximum_bytes_billed={recipe["maximum_bytes_billed"]}',
        f'--max_rows={recipe["maximum_result_rows"]}',
        *parameters(period),
    ]
    result = run(command, query)
    if result.returncode:
        raise ReportError("dataset-invalid", result.stderr.strip() or "BigQuery query failed")
    csv_path.write_text(result.stdout, encoding="utf-8")
