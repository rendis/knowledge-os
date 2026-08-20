#!/usr/bin/env python3
"""Render a monthly event-count workbook with XlsxWriter."""
from __future__ import annotations

from collections import defaultdict
from datetime import datetime
from pathlib import Path
from typing import Any

import xlsxwriter

from pivot_ooxml import inject_pivots
from report_engine import unique_sorted

PIVOT_ROW = 20
SUMMARY_TABLE_ROW = 21
SERIES_COLORS = ("#2F73F6", "#F58220", "#8B5CF6", "#10B981", "#EF4444", "#F59E0B")


def series_color(index: int) -> str:
    return SERIES_COLORS[index % len(SERIES_COLORS)]


def aggregate(rows: list[dict[str, Any]]) -> tuple[list[str], list[str], dict[tuple[str, str], int]]:
    months = unique_sorted(rows, "event_month")
    countries = unique_sorted(rows, "country")
    values: dict[tuple[str, str], int] = defaultdict(int)
    for row in rows:
        values[(row["event_month"], row["country"])] += row["event_count"]
    return months, countries, values


def render_workbook(
    path: Path,
    rows: list[dict[str, Any]],
    title_period: str,
    report_id: str,
    period_start: str,
    period_end: str,
    dataset_hash: str,
) -> list[int]:
    months, countries, values = aggregate(rows)
    years = sorted({int(month[:4]) for month in months})
    workbook = xlsxwriter.Workbook(path)
    workbook.set_properties({
        "title": f"Monthly events {title_period}",
        "company": "cell",
        "created": datetime(2000, 1, 1),
    })
    workbook.set_custom_property("Report ID", report_id)
    workbook.set_custom_property("Period Start", period_start)
    workbook.set_custom_property("Period End", period_end)
    workbook.set_custom_property("Dataset SHA256", dataset_hash)
    title = workbook.add_format({"bold": True, "font_size": 20, "font_color": "#172033"})
    subtitle = workbook.add_format({"font_size": 10, "font_color": "#64748B"})
    header = workbook.add_format({"bold": True, "font_color": "#FFFFFF", "bg_color": "#172033", "align": "center", "border": 0})
    number = workbook.add_format({"num_format": "#,##0", "align": "right"})
    country_format = workbook.add_format({"bold": True, "bg_color": "#E7F5EF", "font_color": "#172033"})
    total_format = workbook.add_format({"bold": True, "bg_color": "#CDEDE0", "num_format": "#,##0", "top": 1})

    summary = workbook.add_worksheet("Resumen")
    summary.hide_gridlines(2)
    summary.set_column("A:A", 16)
    summary.set_column(1, len(months) + 1, 13)
    summary.write("A1", "Monthly events", title)
    summary.write("A2", f"Closed period: {title_period} · Each counted event is one row in the aggregate", subtitle)
    table_row = SUMMARY_TABLE_ROW
    summary.write(table_row, 0, "País", header)
    for index, month in enumerate(months, 1):
        summary.write(table_row, index, month, header)
    summary.write(table_row, len(months) + 1, "Total", header)
    for row_index, country in enumerate(countries, table_row + 1):
        summary.write(row_index, 0, country, country_format)
        for col_index, month in enumerate(months, 1):
            summary.write_number(row_index, col_index, values[(month, country)], number)
        summary.write_number(row_index, len(months) + 1, sum(values[(month, country)] for month in months), total_format)
    total_row = table_row + 1 + len(countries)
    summary.write(total_row, 0, "Total general", total_format)
    for col_index, month in enumerate(months, 1):
        summary.write_number(total_row, col_index, sum(values[(month, country)] for country in countries), total_format)
    summary.write_number(total_row, len(months) + 1, sum(row["event_count"] for row in rows), total_format)
    chart = workbook.add_chart({"type": "line"})
    for offset, country in enumerate(countries):
        row_index = table_row + 1 + offset
        chart.add_series({
            "name": ["Resumen", row_index, 0],
            "categories": ["Resumen", table_row, 1, table_row, len(months)],
            "values": ["Resumen", row_index, 1, row_index, len(months)],
            "line": {"color": series_color(offset), "width": 2.25},
        })
    chart.set_title({"name": "Evolución mensual por país", "name_font": {"size": 13}})
    chart.set_legend({"position": "bottom"})
    chart.set_y_axis({"num_format": "#,##0", "major_gridlines": {"visible": True, "line": {"color": "#D9DEE7"}}})
    chart.set_x_axis({"label_position": "low", "num_font": {"rotation": -45, "size": 8}})
    chart.set_chartarea({"border": {"none": True}})
    chart.set_plotarea({"border": {"none": True}})
    summary.insert_chart("A4", chart, {"x_scale": 2.05, "y_scale": 1.15})

    for year in years:
        sheet_name = str(year)
        sheet = workbook.add_worksheet(sheet_name)
        sheet.hide_gridlines(2)
        sheet.freeze_panes(PIVOT_ROW, 1)
        sheet.set_column("A:A", 24)
        sheet.set_column("B:N", 12)
        sheet.write("A1", f"Monthly events {year}", title)
        sheet.write("A2", "Los botones +/− de la tabla permiten expandir o contraer eventos", subtitle)
        year_months = [month for month in months if month.startswith(f"{year}-")]
        pr0 = PIVOT_ROW - 1
        sheet.write(pr0, 0, "Suma de event_count", header)
        sheet.write(pr0, 1, "event_month", header)
        sheet.write(pr0 + 1, 0, "País / Evento", header)
        for index, month in enumerate(year_months, 1):
            sheet.write(pr0 + 1, index, month, header)
        total_col = len(year_months) + 1
        sheet.write(pr0 + 1, total_col, "Total general", header)
        for index, country in enumerate(countries):
            row_index = pr0 + 2 + index
            sheet.write(row_index, 0, country, country_format)
            for col_index, month in enumerate(year_months, 1):
                sheet.write_number(row_index, col_index, values[(month, country)], number)
            sheet.write_number(row_index, total_col, sum(values[(month, country)] for month in year_months), total_format)
        grand_row = pr0 + 2 + len(countries)
        sheet.write(grand_row, 0, "Total general", total_format)
        for col_index, month in enumerate(year_months, 1):
            sheet.write_number(grand_row, col_index, sum(values[(month, country)] for country in countries), total_format)
        sheet.write_number(grand_row, total_col, sum(values[(month, country)] for month in year_months for country in countries), total_format)

        trend = workbook.add_chart({"type": "line"})
        for index, country in enumerate(countries):
            row_index = pr0 + 2 + index
            trend.add_series({
                "name": [sheet_name, row_index, 0],
                "categories": [sheet_name, pr0 + 1, 1, pr0 + 1, len(year_months)],
                "values": [sheet_name, row_index, 1, row_index, len(year_months)],
                "line": {"color": series_color(index), "width": 2.25},
            })
        trend.set_title({"name": "Evolución mensual", "name_font": {"size": 13}})
        trend.set_legend({"position": "bottom"})
        trend.set_y_axis({"num_format": "#,##0", "major_gridlines": {"visible": True, "line": {"color": "#D9DEE7"}}})
        trend.set_x_axis({"label_position": "low", "num_font": {"size": 8}})
        trend.set_chartarea({"border": {"none": True}})
        trend.set_plotarea({"border": {"none": True}})
        sheet.insert_chart("A4", trend, {"x_scale": 1.45, "y_scale": 1.0})

        distribution = workbook.add_chart({"type": "column"})
        distribution.add_series({
            "name": "Volumen anual",
            "categories": [sheet_name, pr0 + 2, 0, pr0 + 1 + len(countries), 0],
            "values": [sheet_name, pr0 + 2, total_col, pr0 + 1 + len(countries), total_col],
            "fill": {"color": "#2F73F6"},
            "border": {"none": True},
            "data_labels": {"value": True, "num_format": "#,##0"},
        })
        distribution.set_title({"name": "Volumen anual por país", "name_font": {"size": 13}})
        distribution.set_legend({"none": True})
        distribution.set_y_axis({"num_format": "#,##0", "major_gridlines": {"visible": True, "line": {"color": "#D9DEE7"}}})
        distribution.set_chartarea({"border": {"none": True}})
        distribution.set_plotarea({"border": {"none": True}})
        sheet.insert_chart("J4", distribution, {"x_scale": 1.0, "y_scale": 1.0})

    data = workbook.add_worksheet("Datos")
    data.freeze_panes(1, 0)
    data.set_column("A:A", 13)
    data.set_column("B:B", 12)
    data.set_column("C:C", 32)
    data.set_column("D:D", 14)
    for column, name in enumerate(("event_month", "country", "event_name", "event_count")):
        data.write(0, column, name, workbook.add_format({"bold": True}))
    for row_index, row in enumerate(rows, 1):
        data.write(row_index, 0, row["event_month"])
        data.write(row_index, 1, row["country"])
        data.write(row_index, 2, row["event_name"])
        data.write_number(row_index, 3, row["event_count"])
    workbook.close()
    inject_pivots(path, rows, years, PIVOT_ROW)
    return years
