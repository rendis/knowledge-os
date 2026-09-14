#!/usr/bin/env python3
from __future__ import annotations

import json
import subprocess
import sys
import tempfile
import unittest
import zipfile
from pathlib import Path

from defusedxml.common import DefusedXmlException

SKILL_ROOT = Path(__file__).resolve().parents[1]
SCRIPTS = SKILL_ROOT / "scripts"
sys.path.insert(0, str(SCRIPTS))

from pivot_ooxml import parse_xml  # noqa: E402
from report_engine import ReportError  # noqa: E402
from validate_report import validate_workbook  # noqa: E402


class XmlSecurityTests(unittest.TestCase):
    def test_parser_accepts_standard_entities_and_utf16(self) -> None:
        escaped = parse_xml(b'<root value="A &amp; B"/>')
        self.assertEqual(escaped.attrib["value"], "A & B")
        utf16 = '<?xml version="1.0" encoding="UTF-16"?><root>ok</root>'.encode("utf-16")
        self.assertEqual(parse_xml(utf16).text, "ok")

    def test_parser_rejects_dtd_and_entity_declarations(self) -> None:
        hostile_documents = (
            b'<!DOCTYPE root [<!ENTITY item "expanded">]><root>&item;</root>',
            b'<!DOCTYPE root [<!ENTITY item SYSTEM "file:///etc/passwd">]><root>&item;</root>',
            b'<!DOCTYPE root SYSTEM "https://example.invalid/root.dtd"><root/>',
        )
        for document in hostile_documents:
            with self.subTest(document=document):
                with self.assertRaises(DefusedXmlException):
                    parse_xml(document)

    def test_validation_maps_hostile_and_malformed_xml_to_stable_error(self) -> None:
        documents = (
            b'<!DOCTYPE workbook [<!ENTITY item "expanded">]><workbook>&item;</workbook>',
            b"<workbook>",
        )
        for document in documents:
            with self.subTest(document=document), tempfile.TemporaryDirectory() as temp:
                workbook = Path(temp) / "hostile.xlsx"
                with zipfile.ZipFile(workbook, "w") as archive:
                    archive.writestr("xl/workbook.xml", document)
                with self.assertRaises(ReportError) as raised:
                    validate_workbook(workbook)
                self.assertEqual(raised.exception.code, "artifact-invalid")

    def test_legitimate_fixture_generation_and_validation_remain_compatible(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            fixture = root / "events.csv"
            fixture.write_text(
                "event_month,country,event_name,event_count\n"
                "2025-01,CL,created,2\n"
                "2025-02,CL,created,3\n",
                encoding="utf-8",
            )
            output = root / "output"
            result = subprocess.run(
                [
                    sys.executable,
                    "-B",
                    str(SCRIPTS / "run_report.py"),
                    "generate",
                    "--report-id",
                    "sample-monthly-events",
                    "--output-dir",
                    str(output),
                    "--start-month",
                    "2025-01",
                    "--end-month",
                    "2025-02",
                    "--input-csv",
                    str(fixture),
                ],
                text=True,
                capture_output=True,
                check=False,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["rows"], 2)
            self.assertTrue(Path(payload["workbook"]).is_file())


if __name__ == "__main__":
    unittest.main(verbosity=2)
