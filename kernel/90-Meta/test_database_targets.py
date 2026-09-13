"""Target declarations remain independent from optional schema repositories."""
import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest

from instance import validate_instance, dump_instance, _parse_minimal_yaml, InstanceError


def fixture():
    return {
        "cell": {"name": "Example", "purpose": "test"},
        "systems": [{"id": "alpha", "name": "Alpha"}],
        "sources": {"schema_repository": {"remote": "https://example.org/legacy.git", "note": "Legacy"}},
        "database_targets": [{"id": "alpha-prod", "system": "alpha", "environment": "prod",
            "instance": "project:region:instance", "database": "app", "schemas": ["public"],
            "repositories": [], "procedure": "Database access", "port_key": "alpha-prod"}],
    }


class DatabaseTargetsTests(unittest.TestCase):
    def test_zero_one_multiple_sources_roundtrip_and_legacy_preserved(self):
        for sources in ([], ["https://example.org/a.git"], ["https://example.org/a.git", "https://example.org/b.git"]):
            data = fixture()
            data["database_targets"][0]["repositories"] = sources
            validated = validate_instance(data)
            reread = validate_instance(_parse_minimal_yaml(dump_instance(validated)))
            self.assertEqual(validated, reread)
            self.assertEqual(sources, reread["database_targets"][0]["repositories"])
            self.assertEqual(data["sources"]["schema_repository"], reread["sources"]["schema_repository"])

    def test_legacy_without_targets(self):
        data = fixture()
        del data["database_targets"]
        self.assertEqual([], validate_instance(data)["database_targets"])

    def test_invalid_targets_fail_closed(self):
        for field, value in (("system", "unknown"), ("id", "Bad ID"), ("procedure", "../note"),
                             ("repositories", ["https://user:secret@example.org/a"]),
                             ("schemas", "public"), ("password", "secret")):
            data = fixture()
            data["database_targets"][0][field] = value
            with self.subTest(field=field), self.assertRaises(InstanceError):
                validate_instance(data)
        data = fixture()
        data["database_targets"].append(copy.deepcopy(data["database_targets"][0]))
        with self.assertRaises(InstanceError):
            validate_instance(data)

    def test_resolution_requires_procedure_but_not_repository(self):
        spec = importlib.util.spec_from_file_location("cell_config_targets", Path(__file__).with_name("cell-config.py"))
        module = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(module)
        data = validate_instance(fixture())
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            self.assertEqual("not_configured", module.database_target(root, data, "unknown")["status"])
            with self.assertRaises(InstanceError):
                module.database_target(root, data, "alpha-prod")
            folder = root / "60-Operacion"
            folder.mkdir()
            (folder / "Database access.md").write_text("---\ntipo: operacional\n---\n")
            result = module.database_target(root, data, "alpha-prod")
            self.assertEqual("configured", result["status"])
            self.assertEqual([], result["target"]["repositories"])


if __name__ == "__main__":
    unittest.main()
