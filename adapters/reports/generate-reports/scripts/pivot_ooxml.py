#!/usr/bin/env python3
"""Inject native collapsed PivotTables into an XlsxWriter workbook."""
from __future__ import annotations

import posixpath
import shutil
import tempfile
import zipfile
from pathlib import Path, PurePosixPath
from typing import Optional
from xml.sax.saxutils import escape, quoteattr

from defusedxml import ElementTree as ET

MAIN = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
REL = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
PKG_REL = "http://schemas.openxmlformats.org/package/2006/relationships"
CHART = "http://schemas.openxmlformats.org/drawingml/2006/chart"


def parse_xml(content: bytes):
    """Parse one OOXML member while rejecting DTD and entity declarations."""
    return ET.fromstring(
        content,
        forbid_dtd=True,
        forbid_entities=True,
        forbid_external=True,
    )


def append_before_close(xml: str, local_name: str, fragment: str) -> str:
    for close in (f"</{local_name}>", f"</x:{local_name}>"):
        if close in xml:
            return xml.replace(close, fragment + close, 1)
    raise ValueError(f"missing closing tag {local_name}")


def col_name(number: int) -> str:
    result = ""
    while number:
        number, remainder = divmod(number - 1, 26)
        result = chr(65 + remainder) + result
    return result


def workbook_sheet_paths(files: dict[str, bytes]) -> dict[str, str]:
    workbook = parse_xml(files["xl/workbook.xml"])
    relationships = parse_xml(files["xl/_rels/workbook.xml.rels"])
    targets = {
        rel.attrib["Id"]: rel.attrib["Target"].lstrip("/")
        for rel in relationships
        if rel.attrib.get("Type", "").endswith("/worksheet")
    }
    result = {}
    for sheet in workbook.findall(f".//{{{MAIN}}}sheet"):
        target = targets[sheet.attrib[f"{{{REL}}}id"]]
        result[sheet.attrib["name"]] = target if target.startswith("xl/") else f"xl/{target}"
    return result


def related_part(source_path: str, target: str) -> str:
    if target.startswith("/"):
        return target.lstrip("/")
    return posixpath.normpath(posixpath.join(posixpath.dirname(source_path), target))


def relationships_path(part_path: str) -> str:
    part = PurePosixPath(part_path)
    return str(part.parent / "_rels" / f"{part.name}.rels")


def worksheet_drawing_path(files: dict[str, bytes], sheet_path: str) -> str:
    sheet_rels_path = relationships_path(sheet_path)
    sheet_rels = parse_xml(files[sheet_rels_path])
    drawing_relationship = next(
        relationship
        for relationship in sheet_rels
        if relationship.attrib.get("Type", "").endswith("/drawing")
    )
    return related_part(sheet_path, drawing_relationship.attrib["Target"])


def worksheet_chart_paths(files: dict[str, bytes], sheet_path: str) -> list[str]:
    drawing_path = worksheet_drawing_path(files, sheet_path)
    drawing = parse_xml(files[drawing_path])
    drawing_rels = parse_xml(files[relationships_path(drawing_path)])
    chart_targets = {
        relationship.attrib["Id"]: related_part(drawing_path, relationship.attrib["Target"])
        for relationship in drawing_rels
        if relationship.attrib.get("Type", "").endswith("/chart")
    }
    return [
        chart_targets[node.attrib[f"{{{REL}}}id"]]
        for node in drawing.findall(f".//{{{CHART}}}chart")
    ]


def link_pivot_chart(xml: str, pivot_source: str) -> str:
    if "<c:pivotSource>" in xml:
        raise ValueError("chart already has a pivot source")
    fragment = (
        f"<c:pivotSource><c:name>{escape(pivot_source)}</c:name>"
        '<c:fmtId val="0"/></c:pivotSource>'
    )
    if "<c:chart>" not in xml:
        raise ValueError("chartSpace has no chart element")
    return xml.replace("<c:chart>", fragment + "<c:chart>", 1)


def add_relationship(xml: Optional[str], identifier: str, relationship_type: str, target: str) -> str:
    fragment = (
        f'<Relationship Id={quoteattr(identifier)} Type={quoteattr(relationship_type)} '
        f'Target={quoteattr(target)}/>'
    )
    if xml is None:
        return f'<?xml version="1.0" encoding="UTF-8" standalone="yes"?><Relationships xmlns="{PKG_REL}">{fragment}</Relationships>'
    return append_before_close(xml, "Relationships", fragment)


def inject_pivots(
    workbook_path: Path,
    rows: list[dict[str, object]],
    years: list[int],
    pivot_row: int,
) -> None:
    with zipfile.ZipFile(workbook_path) as source:
        files = {name: source.read(name) for name in source.namelist()}

    months = sorted({str(row["event_month"]) for row in rows})
    countries = sorted({str(row["country"]) for row in rows})
    events = sorted({str(row["event_name"]) for row in rows})
    month_index = {value: index for index, value in enumerate(months)}
    country_index = {value: index for index, value in enumerate(countries)}
    event_index = {value: index for index, value in enumerate(events)}
    counts = [int(row["event_count"]) for row in rows]

    fields = [
        ("event_month", months),
        ("country", countries),
        ("event_name", events),
    ]
    cache_fields = "".join(
        f'<cacheField name={quoteattr(name)} numFmtId="0"><sharedItems containsString="1" count="{len(values)}">'
        + "".join(f'<s v={quoteattr(value)}/>' for value in values)
        + "</sharedItems></cacheField>"
        for name, values in fields
    )
    cache_fields += (
        '<cacheField name="event_count" numFmtId="0"><sharedItems containsNumber="1" '
        f'containsInteger="1" minValue="{min(counts)}" maxValue="{max(counts)}"/></cacheField>'
    )
    cache_definition = (
        '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
        f'<pivotCacheDefinition xmlns="{MAIN}" xmlns:r="{REL}" r:id="rId1" saveData="1" '
        f'refreshOnLoad="0" enableRefresh="1" createdVersion="8" refreshedVersion="8" '
        f'minRefreshableVersion="3" recordCount="{len(rows)}">'
        f'<cacheSource type="worksheet"><worksheetSource ref="A1:D{len(rows)+1}" sheet="Datos"/></cacheSource>'
        f'<cacheFields count="4">{cache_fields}</cacheFields></pivotCacheDefinition>'
    )
    cache_records = "".join(
        f'<r><x v="{month_index[str(row["event_month"])]}"/><x v="{country_index[str(row["country"])]}"/>'
        f'<x v="{event_index[str(row["event_name"])]}"/><n v="{int(row["event_count"])}"/></r>'
        for row in rows
    )
    additions: dict[str, bytes] = {
        "xl/pivotCache/pivotCacheDefinition1.xml": cache_definition.encode(),
        "xl/pivotCache/pivotCacheRecords1.xml": (
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            f'<pivotCacheRecords xmlns="{MAIN}" count="{len(rows)}">{cache_records}</pivotCacheRecords>'
        ).encode(),
        "xl/pivotCache/_rels/pivotCacheDefinition1.xml.rels": (
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            f'<Relationships xmlns="{PKG_REL}"><Relationship Id="rId1" Type="{REL}/pivotCacheRecords" '
            'Target="pivotCacheRecords1.xml"/></Relationships>'
        ).encode(),
    }

    workbook_xml = files["xl/workbook.xml"].decode()
    workbook_xml = append_before_close(
        workbook_xml,
        "workbook",
        f'<pivotCaches xmlns="{MAIN}"><pivotCache cacheId="1" r:id="RpivotCache1" xmlns:r="{REL}"/></pivotCaches>',
    )
    files["xl/workbook.xml"] = workbook_xml.encode()
    rels = files["xl/_rels/workbook.xml.rels"].decode()
    files["xl/_rels/workbook.xml.rels"] = add_relationship(
        rels,
        "RpivotCache1",
        f"{REL}/pivotCacheDefinition",
        "pivotCache/pivotCacheDefinition1.xml",
    ).encode()

    content_types = files["[Content_Types].xml"].decode()
    overrides = [
        ("/xl/pivotCache/pivotCacheDefinition1.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.pivotCacheDefinition+xml"),
        ("/xl/pivotCache/pivotCacheRecords1.xml", "application/vnd.openxmlformats-officedocument.spreadsheetml.pivotCacheRecords+xml"),
    ]
    sheet_paths = workbook_sheet_paths(files)
    for pivot_number, year in enumerate(years, 1):
        visible = [index for index, month in enumerate(months) if month.startswith(f"{year}-")]
        visible_set = set(visible)
        month_items = "".join(
            f'<item x="{index}"/>' if index in visible_set else f'<item h="1" x="{index}"/>'
            for index in range(len(months))
        ) + '<item t="default"/>'
        country_items = "".join(
            f'<item sd="0" x="{index}"/>' for index in range(len(countries))
        ) + '<item t="default"/>'
        event_items = "".join(f'<item x="{index}"/>' for index in range(len(events))) + '<item t="default"/>'
        row_items = "".join(f'<i><x v="{index}"/></i>' for index in range(len(countries))) + '<i t="grand"><x/></i>'
        col_items = "".join(f'<i><x v="{index}"/></i>' for index in visible) + '<i t="grand"><x/></i>'
        last_column = col_name(len(visible) + 2)
        last_row = pivot_row + len(countries) + 2
        pivot = (
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            f'<pivotTableDefinition xmlns="{MAIN}" name="PivotEventos{year}" cacheId="1" '
            'applyNumberFormats="0" applyBorderFormats="0" applyFontFormats="0" applyPatternFormats="0" '
            'applyAlignmentFormats="0" applyWidthHeightFormats="1" dataCaption="Valores" updatedVersion="8" '
            'minRefreshableVersion="3" useAutoFormatting="1" itemPrintTitles="1" createdVersion="8" '
            'indent="0" compact="0" compactData="0" showDrill="1">'
            f'<location ref="A{pivot_row}:{last_column}{last_row}" firstHeaderRow="1" firstDataRow="2" firstDataCol="2"/>'
            '<pivotFields count="4">'
            f'<pivotField axis="axisCol" compact="0" outline="0" showAll="0"><items count="{len(months)+1}">{month_items}</items></pivotField>'
            f'<pivotField axis="axisRow" compact="0" outline="0" showAll="0"><items count="{len(countries)+1}">{country_items}</items></pivotField>'
            f'<pivotField axis="axisRow" compact="0" outline="0" showAll="0"><items count="{len(events)+1}">{event_items}</items></pivotField>'
            '<pivotField dataField="1" compact="0" outline="0" showAll="0"/>'
            '</pivotFields><rowFields count="2"><field x="1"/><field x="2"/></rowFields>'
            f'<rowItems count="{len(countries)+1}">{row_items}</rowItems>'
            '<colFields count="1"><field x="0"/></colFields>'
            f'<colItems count="{len(visible)+1}">{col_items}</colItems>'
            '<dataFields count="1"><dataField name="Suma de event_count" fld="3" baseField="0" baseItem="0" numFmtId="0"/></dataFields>'
            '<pivotTableStyleInfo name="PivotStyleLight16" showRowHeaders="1" showColHeaders="1" showRowStripes="0" showColStripes="0" showLastColumn="1"/>'
            '</pivotTableDefinition>'
        )
        pivot_path = f"xl/pivotTables/pivotTable{pivot_number}.xml"
        additions[pivot_path] = pivot.encode()
        additions[f"xl/pivotTables/_rels/pivotTable{pivot_number}.xml.rels"] = (
            '<?xml version="1.0" encoding="UTF-8" standalone="yes"?>'
            f'<Relationships xmlns="{PKG_REL}"><Relationship Id="rId1" Type="{REL}/pivotCacheDefinition" '
            'Target="../pivotCache/pivotCacheDefinition1.xml"/></Relationships>'
        ).encode()
        overrides.append((f"/{pivot_path}", "application/vnd.openxmlformats-officedocument.spreadsheetml.pivotTable+xml"))

        sheet_path = sheet_paths[str(year)]
        sheet_xml = files[sheet_path].decode()
        sheet_xml = append_before_close(
            sheet_xml,
            "worksheet",
            f'<pivotTableParts xmlns="{MAIN}" count="1"><pivotTablePart r:id="RpivotTable{pivot_number}" xmlns:r="{REL}"/></pivotTableParts>',
        )
        files[sheet_path] = sheet_xml.encode()
        rel_path = str(PurePosixPath(sheet_path).parent / "_rels" / (PurePosixPath(sheet_path).name + ".rels"))
        sheet_rels = files.get(rel_path)
        files[rel_path] = add_relationship(
            sheet_rels.decode() if sheet_rels else None,
            f"RpivotTable{pivot_number}",
            f"{REL}/pivotTable",
            f"../pivotTables/pivotTable{pivot_number}.xml",
        ).encode()
        chart_paths = worksheet_chart_paths(files, sheet_path)
        if len(chart_paths) != 2:
            raise ValueError(f"expected two charts for {year}, found {len(chart_paths)}")
        for chart_path in chart_paths:
            files[chart_path] = link_pivot_chart(
                files[chart_path].decode(),
                f"{year}!PivotEventos{year}",
            ).encode()

    for part, content_type in overrides:
        content_types = append_before_close(
            content_types,
            "Types",
            f'<Override PartName={quoteattr(part)} ContentType={quoteattr(content_type)}/>',
        )
    files["[Content_Types].xml"] = content_types.encode()
    files.update(additions)

    with tempfile.NamedTemporaryFile(dir=workbook_path.parent, suffix=".xlsx", delete=False) as handle:
        temporary = Path(handle.name)
    try:
        with zipfile.ZipFile(temporary, "w", zipfile.ZIP_DEFLATED, compresslevel=6) as target:
            for name, data in files.items():
                info = zipfile.ZipInfo(name, date_time=(1980, 1, 1, 0, 0, 0))
                info.compress_type = zipfile.ZIP_DEFLATED
                info.create_system = 3
                info.external_attr = 0o600 << 16
                target.writestr(info, data, compress_type=zipfile.ZIP_DEFLATED, compresslevel=6)
        shutil.move(temporary, workbook_path)
    finally:
        temporary.unlink(missing_ok=True)
