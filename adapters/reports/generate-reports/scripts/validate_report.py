#!/usr/bin/env python3
"""Validate workbook structure, native pivots, and manifest integrity."""
from __future__ import annotations

import json
import re
import zipfile
from pathlib import Path
from typing import Any, Optional

from defusedxml import ElementTree as ET
from defusedxml.common import DefusedXmlException

from pivot_ooxml import parse_xml, workbook_sheet_paths, worksheet_chart_paths, worksheet_drawing_path
from report_engine import ReportError, sha256

MAIN = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
CHART = "http://schemas.openxmlformats.org/drawingml/2006/chart"
DRAWING = "http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing"
CUSTOM = "http://schemas.openxmlformats.org/officeDocument/2006/custom-properties"


def workbook_sheets(files: dict[str, bytes]) -> list[str]:
    root = parse_xml(files["xl/workbook.xml"])
    return [sheet.attrib["name"] for sheet in root.findall(f".//{{{MAIN}}}sheet")]


def validate_workbook(path: Path, expected_rows: Optional[int] = None) -> dict[str, Any]:
    try:
        with zipfile.ZipFile(path) as archive:
            names = set(archive.namelist())
            files = {name: archive.read(name) for name in names}
            sheets = workbook_sheets(files)
            years = [name for name in sheets if re.fullmatch(r"\d{4}", name)]
            if sheets != ["Resumen", *years, "Datos"] or not years:
                raise ReportError("artifact-invalid", f"unexpected sheets: {sheets}")
            if "docProps/custom.xml" not in names:
                raise ReportError("artifact-invalid", "workbook provenance properties missing")
            custom = parse_xml(files["docProps/custom.xml"])
            properties = {
                item.attrib["name"]: next(iter(item)).text or ""
                for item in custom.findall(f"{{{CUSTOM}}}property")
            }
            required_properties = {"Report ID", "Period Start", "Period End", "Dataset SHA256"}
            if set(properties) != required_properties or not re.fullmatch(r"[0-9a-f]{64}", properties["Dataset SHA256"]):
                raise ReportError("artifact-invalid", "invalid workbook provenance properties")
            pivot_tables = sorted(name for name in names if re.fullmatch(r"xl/pivotTables/pivotTable\d+\.xml", name))
            if len(pivot_tables) != len(years):
                raise ReportError("artifact-invalid", "one native pivot per year is required")
            if "xl/pivotCache/pivotCacheDefinition1.xml" not in names or "xl/pivotCache/pivotCacheRecords1.xml" not in names:
                raise ReportError("artifact-invalid", "shared native pivot cache missing")
            cache = files["xl/pivotCache/pivotCacheDefinition1.xml"].decode()
            records = files["xl/pivotCache/pivotCacheRecords1.xml"].decode()
            record_match = re.search(r'recordCount="(\d+)"', cache)
            record_count = int(record_match.group(1)) if record_match else -1
            if expected_rows is not None and record_count != expected_rows:
                raise ReportError("artifact-invalid", "pivot cache row count mismatch")
            if f'count="{record_count}"' not in records:
                raise ReportError("artifact-invalid", "pivot cache records mismatch")
            for year, pivot_path in zip(years, pivot_tables):
                pivot = files[pivot_path].decode()
                if f'name="PivotEventos{year}"' not in pivot or 'showDrill="1"' not in pivot:
                    raise ReportError("artifact-invalid", f"invalid pivot for {year}")
                if 'axis="axisRow"' not in pivot or '<rowFields count="2">' not in pivot or 'sd="0"' not in pivot:
                    raise ReportError("artifact-invalid", f"pivot hierarchy not collapsed for {year}")
            if any(name.startswith("xl/tables/") for name in names):
                raise ReportError("artifact-invalid", "Excel Tables are not allowed")
            chart_count = len([name for name in names if name.startswith("xl/charts/chart") and name.endswith(".xml")])
            if chart_count != 1 + 2 * len(years):
                raise ReportError("artifact-invalid", "unexpected chart count")
            sheet_paths = workbook_sheet_paths(files)
            summary = parse_xml(files[sheet_paths["Resumen"]])
            summary_unfrozen = summary.find(f".//{{{MAIN}}}pane") is None
            if not summary_unfrozen:
                raise ReportError("artifact-invalid", "Resumen must not contain frozen panes")
            if summary.find(f'.//{{{MAIN}}}c[@r="A22"]') is None:
                raise ReportError("artifact-invalid", "Resumen table must start at A22")
            summary_drawing = parse_xml(files[worksheet_drawing_path(files, sheet_paths["Resumen"])])
            chart_end_rows = [
                int(node.text)
                for node in summary_drawing.findall(f".//{{{DRAWING}}}to/{{{DRAWING}}}row")
                if node.text is not None
            ]
            summary_chart_clearance = bool(chart_end_rows) and max(chart_end_rows) < 21
            if not summary_chart_clearance:
                raise ReportError("artifact-invalid", "Resumen chart overlaps its table")
            summary_charts = worksheet_chart_paths(files, sheet_paths["Resumen"])
            if any(parse_xml(files[chart_path]).find(f"{{{CHART}}}pivotSource") is not None for chart_path in summary_charts):
                raise ReportError("artifact-invalid", "Resumen chart must remain a regular chart")
            pivot_chart_count = 0
            for year in years:
                chart_paths = worksheet_chart_paths(files, sheet_paths[year])
                if len(chart_paths) != 2:
                    raise ReportError("artifact-invalid", f"two PivotCharts are required for {year}")
                expected_source = f"{year}!PivotEventos{year}"
                for chart_path in chart_paths:
                    chart = parse_xml(files[chart_path])
                    pivot_source = chart.find(f"{{{CHART}}}pivotSource")
                    source_name = pivot_source.find(f"{{{CHART}}}name") if pivot_source is not None else None
                    format_id = pivot_source.find(f"{{{CHART}}}fmtId") if pivot_source is not None else None
                    if (
                        source_name is None
                        or source_name.text != expected_source
                        or format_id is None
                        or format_id.attrib.get("val") != "0"
                    ):
                        raise ReportError("artifact-invalid", f"invalid PivotChart source for {year}")
                    pivot_chart_count += 1
            return {
                "valid": True,
                "report_id": properties["Report ID"],
                "period_start_month": properties["Period Start"],
                "period_end_month": properties["Period End"],
                "dataset_sha256": properties["Dataset SHA256"],
                "sheets": sheets,
                "years": [int(year) for year in years],
                "native_pivots": len(pivot_tables),
                "pivot_charts": pivot_chart_count,
                "pivot_cache_records": record_count,
                "charts": chart_count,
                "excel_tables": 0,
                "summary_unfrozen": summary_unfrozen,
                "summary_chart_clearance": summary_chart_clearance,
            }
    except (KeyError, zipfile.BadZipFile, ET.ParseError, DefusedXmlException) as error:
        raise ReportError("artifact-invalid", str(error)) from error


def validate_manifest(workbook: Path, manifest: Path) -> dict[str, Any]:
    payload = json.loads(manifest.read_text(encoding="utf-8"))
    if payload.get("manifest_version") != 1:
        raise ReportError("artifact-invalid", "unsupported manifest version")
    if not isinstance(payload.get("implementation_version"), int) or payload["implementation_version"] < 1:
        raise ReportError("artifact-invalid", "invalid implementation version")
    for field in ("dataset_sha256", "recipe_sha256", "query_sha256", "implementation_sha256", "workbook_sha256"):
        if not re.fullmatch(r"[0-9a-f]{64}", str(payload.get(field, ""))):
            raise ReportError("artifact-invalid", f"invalid {field}")
    if not re.fullmatch(r"\d{4}-\d{2}-\d{2}T.+Z", str(payload.get("extracted_at_utc", ""))):
        raise ReportError("artifact-invalid", "invalid extracted_at_utc")
    if payload.get("workbook_sha256") != sha256(workbook):
        raise ReportError("artifact-invalid", "workbook hash mismatch")
    validation = validate_workbook(workbook, payload.get("row_count"))
    period = payload.get("period", {})
    if (
        payload.get("report_id") != validation["report_id"]
        or payload.get("dataset_sha256") != validation["dataset_sha256"]
        or period.get("start_month") != validation["period_start_month"]
        or period.get("end_month") != validation["period_end_month"]
    ):
        raise ReportError("artifact-invalid", "manifest provenance mismatch")
    if payload.get("validation") != validation:
        raise ReportError("artifact-invalid", "manifest validation mismatch")
    return validation
