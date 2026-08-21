#!/usr/bin/env python3
"""List, generate, or validate registered report recipes."""
from __future__ import annotations

import argparse
import json
import os
import shutil
import sys
import tempfile
from pathlib import Path

from bigquery_adapter import dry_run, extract
from report_engine import (
    ReportError,
    list_recipes,
    load_recipe,
    read_rows,
    resolve_period,
    dataset_sha256,
    write_manifest,
)
from validate_report import validate_manifest, validate_workbook


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    subparsers.add_parser("list")
    generate = subparsers.add_parser("generate")
    generate.add_argument("--report-id", required=True)
    generate.add_argument("--output-dir", type=Path, required=True)
    generate.add_argument("--start-month")
    generate.add_argument("--end-month")
    generate.add_argument("--input-csv", type=Path)
    validate = subparsers.add_parser("validate")
    validate.add_argument("--workbook", type=Path, required=True)
    validate.add_argument("--manifest", type=Path, required=True)
    return parser.parse_args()


def generate(args: argparse.Namespace) -> dict[str, object]:
    from excel_renderer import render_workbook

    recipe = load_recipe(args.report_id)
    if recipe["renderer"] != "monthly-event-excel":
        raise ReportError("renderer-unavailable", str(recipe["renderer"]))
    period = resolve_period(args.start_month, args.end_month, recipe.get("period_mode", "any"))
    recipe_dir = Path(recipe["recipe_dir"])
    query_path = recipe_dir / "query.sql"
    args.output_dir.mkdir(parents=True, exist_ok=True)
    base = f'{recipe["report_id"]}_{period.start_month}_a_{period.last_closed_month}'
    workbook_path = args.output_dir / f"{base}.xlsx"
    manifest_path = args.output_dir / f"{base}.manifest.json"
    if workbook_path.exists() or manifest_path.exists():
        raise ReportError("artifact-invalid", "refusing to overwrite an existing artifact package")
    with tempfile.TemporaryDirectory(prefix=".knowledge-os-report-", dir=args.output_dir) as temp:
        temporary = Path(temp)
        csv_path = temporary / "dataset.csv"
        bytes_processed = None
        if args.input_csv:
            shutil.copyfile(args.input_csv, csv_path)
        else:
            bytes_processed = dry_run(recipe, period, query_path)
            extract(recipe, period, query_path, csv_path)
        rows = read_rows(csv_path, recipe["schema"])
        if not rows:
            raise ReportError("dataset-invalid", "query returned no rows")
        if len(rows) >= int(recipe["maximum_result_rows"]):
            raise ReportError("dataset-invalid", "result row cap reached; completeness is unknown")
        if rows[0]["event_month"] < period.start_month or rows[-1]["event_month"] >= period.end_month_exclusive:
            raise ReportError("dataset-invalid", "dataset is outside the requested period")
        temporary_workbook = temporary / "report.xlsx"
        temporary_manifest = temporary / "report.manifest.json"
        snapshot_hash = dataset_sha256(rows)
        render_workbook(
            temporary_workbook,
            rows,
            f"{period.start_month} a {period.last_closed_month}",
            recipe["report_id"],
            period.start_month,
            period.end_month,
            snapshot_hash,
        )
        validation = validate_workbook(temporary_workbook, len(rows))
        write_manifest(
            temporary_manifest,
            recipe,
            period,
            query_path,
            temporary_workbook,
            rows,
            validation,
            bytes_processed,
        )
        validate_manifest(temporary_workbook, temporary_manifest)
        publish_package(temporary_workbook, temporary_manifest, workbook_path, manifest_path)
    return {
        "report_id": recipe["report_id"],
        "workbook": str(workbook_path),
        "manifest": str(manifest_path),
        "rows": len(rows),
        "years": validation["years"],
        "bytes_processed": bytes_processed,
    }


def publish_package(
    temporary_workbook: Path,
    temporary_manifest: Path,
    workbook_path: Path,
    manifest_path: Path,
) -> None:
    try:
        os.replace(temporary_workbook, workbook_path)
        os.replace(temporary_manifest, manifest_path)
    except OSError as error:
        workbook_path.unlink(missing_ok=True)
        manifest_path.unlink(missing_ok=True)
        raise ReportError("artifact-invalid", "failed to publish artifact package") from error


def main() -> int:
    args = parse_args()
    try:
        if args.command == "list":
            result = [
                {"report_id": item["report_id"], "contract_basename": item["contract_basename"]}
                for item in list_recipes()
            ]
        elif args.command == "generate":
            result = generate(args)
        else:
            result = validate_manifest(args.workbook, args.manifest)
        print(json.dumps(result, ensure_ascii=False, sort_keys=True, indent=2))
        return 0
    except ReportError as error:
        print(json.dumps({"error": error.code, "message": str(error)}, ensure_ascii=False))
        return 2


if __name__ == "__main__":
    sys.exit(main())
