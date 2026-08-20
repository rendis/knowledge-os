#!/usr/bin/env python3
"""Shared report resolution, period, normalization, and manifest helpers."""
from __future__ import annotations

import csv
import hashlib
import json
import re
from dataclasses import dataclass
from datetime import date, datetime, timezone
from pathlib import Path
from typing import Any, Optional

MONTH = re.compile(r"^(\d{4})-(0[1-9]|1[0-2])$")


class ReportError(Exception):
    def __init__(self, code: str, message: str) -> None:
        super().__init__(message)
        self.code = code


@dataclass(frozen=True)
class Period:
    start_month: str
    end_month: str

    @property
    def start_suffix(self) -> str:
        return self.start_month.replace("-", "") + "01"

    @property
    def end_suffix(self) -> str:
        return self.end_month_exclusive.replace("-", "") + "01"

    @property
    def end_month_exclusive(self) -> str:
        year, month = map(int, self.end_month.split("-"))
        month += 1
        if month == 13:
            year += 1
            month = 1
        return f"{year:04d}-{month:02d}"

    @property
    def last_closed_month(self) -> str:
        return self.end_month


def current_month() -> str:
    today = date.today()
    return f"{today.year:04d}-{today.month:02d}"


def resolve_period(start: Optional[str], end: Optional[str], period_mode: str = "any") -> Period:
    if period_mode not in {"any", "closed-months"}:
        raise ReportError("artifact-invalid", f"unsupported period mode: {period_mode}")
    if start is None or end is None:
        raise ReportError("invalid-period", "start-month and end-month are required")
    if not MONTH.fullmatch(start) or not MONTH.fullmatch(end) or start > end:
        raise ReportError("invalid-period", "expected start <= end in YYYY-MM")
    if period_mode == "closed-months" and end >= current_month():
        raise ReportError("invalid-period", "end-month must be a closed calendar month")
    return Period(start, end)


def recipes_root() -> Path:
    return Path(__file__).resolve().parent / "reports"


def list_recipes() -> list[dict[str, Any]]:
    result = []
    for path in sorted(recipes_root().glob("*/report.json")):
        recipe = json.loads(path.read_text(encoding="utf-8"))
        required = {
            "report_id",
            "contract_basename",
            "adapter",
            "renderer",
            "implementation_version",
            "project",
            "location",
            "source",
            "period_mode",
            "maximum_bytes_billed",
            "maximum_result_rows",
            "schema",
        }
        if not required.issubset(recipe):
            raise ReportError("artifact-invalid", f"incomplete recipe: {path}")
        if recipe.get("report_id") != path.parent.name:
            raise ReportError("artifact-invalid", f"recipe directory mismatch: {path}")
        recipe["recipe_dir"] = str(path.parent)
        result.append(recipe)
    return result


def load_recipe(report_id: str) -> dict[str, Any]:
    matches = [recipe for recipe in list_recipes() if recipe["report_id"] == report_id]
    if len(matches) != 1:
        raise ReportError("unsupported-report-id", report_id)
    return matches[0]


def unique_sorted(rows: list[dict[str, Any]], key: str) -> list[str]:
    return sorted({str(row[key]) for row in rows})


def read_rows(path: Path, expected_schema: list[str]) -> list[dict[str, Any]]:
    with path.open(newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle)
        if reader.fieldnames != expected_schema:
            raise ReportError("source-schema-invalid", f"expected {expected_schema}; got {reader.fieldnames}")
        rows = []
        for number, row in enumerate(reader, 2):
            if not MONTH.fullmatch(row["event_month"] or ""):
                raise ReportError("dataset-invalid", f"row {number}: invalid event_month")
            if not (row["country"] or "").strip() or not (row["event_name"] or "").strip():
                raise ReportError("dataset-invalid", f"row {number}: invalid dimension")
            try:
                count = int(row["event_count"])
            except (TypeError, ValueError) as error:
                raise ReportError("dataset-invalid", f"row {number}: invalid event_count") from error
            if count < 0:
                raise ReportError("dataset-invalid", f"row {number}: negative event_count")
            rows.append({**row, "event_count": count})
    keys = [(row["event_month"], row["country"], row["event_name"]) for row in rows]
    if keys != sorted(keys) or len(keys) != len(set(keys)):
        raise ReportError("dataset-invalid", "rows must be unique and sorted by grain")
    return rows


def sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def dataset_sha256(rows: list[dict[str, Any]]) -> str:
    payload = json.dumps(rows, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return hashlib.sha256(payload.encode("utf-8")).hexdigest()


def implementation_sha256(recipe_dir: Path) -> str:
    scripts = Path(__file__).resolve().parent
    paths = [
        recipe_dir / "report.json",
        recipe_dir / "query.sql",
        scripts / "requirements.txt",
        *(scripts / name for name in (
            "bigquery_adapter.py",
            "excel_renderer.py",
            "pivot_ooxml.py",
            "report_engine.py",
            "run_report.py",
            "validate_report.py",
        )),
    ]
    digest = hashlib.sha256()
    for item in sorted(paths, key=lambda path: path.name):
        digest.update(item.name.encode("utf-8"))
        digest.update(b"\0")
        digest.update(item.read_bytes())
        digest.update(b"\0")
    return digest.hexdigest()


def totals(rows: list[dict[str, Any]]) -> dict[str, Any]:
    by_month_country: dict[str, int] = {}
    for row in rows:
        key = f'{row["event_month"]}|{row["country"]}'
        by_month_country[key] = by_month_country.get(key, 0) + row["event_count"]
    return {
        "event_count": sum(row["event_count"] for row in rows),
        "by_month_country": dict(sorted(by_month_country.items())),
    }


def write_manifest(
    path: Path,
    recipe: dict[str, Any],
    period: Period,
    query_path: Path,
    workbook_path: Path,
    rows: list[dict[str, Any]],
    validation: dict[str, Any],
    bytes_processed: Optional[int],
) -> None:
    payload = {
        "manifest_version": 1,
        "report_id": recipe["report_id"],
        "implementation_version": recipe["implementation_version"],
        "period": {
            "start_month": period.start_month,
            "end_month": period.end_month,
            "end_month_exclusive": period.end_month_exclusive,
            "last_closed_month": period.last_closed_month,
        },
        "source": recipe["source"],
        "extracted_at_utc": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "dataset_sha256": dataset_sha256(rows),
        "recipe_sha256": sha256(Path(recipe["recipe_dir"]) / "report.json"),
        "query_sha256": sha256(query_path),
        "implementation_sha256": implementation_sha256(Path(recipe["recipe_dir"])),
        "row_count": len(rows),
        "totals": totals(rows),
        "bytes_processed": bytes_processed,
        "workbook_sha256": sha256(workbook_path),
        "validation": validation,
    }
    path.write_text(json.dumps(payload, ensure_ascii=False, sort_keys=True, indent=2) + "\n", encoding="utf-8")
