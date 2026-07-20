#!/usr/bin/env python3
"""Unit tests for the scored-experiment runner's pure logic + a schema check
over every experiments/*.yaml. Runs WITHOUT a live stack (no network/subprocess).

  python3 scripts/experiments/test_run.py        # or: python3 -m unittest -q
"""
import sys
import unittest
from pathlib import Path

import yaml

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
sys.path.insert(0, str(HERE))

import run  # noqa: E402  (path injected above)

MISSING = run._MISSING


class TestParseDuration(unittest.TestCase):
    def test_units(self):
        self.assertEqual(run.parse_duration("300s"), 300)
        self.assertEqual(run.parse_duration("2m"), 120)
        self.assertEqual(run.parse_duration("1h"), 3600)
        self.assertEqual(run.parse_duration("500ms"), 0.5)
        self.assertEqual(run.parse_duration("30"), 30)
        self.assertEqual(run.parse_duration(45), 45.0)

    def test_bad(self):
        for bad in ("", "abc", "10x", "s"):
            with self.assertRaises(ValueError):
                run.parse_duration(bad)


class TestCompare(unittest.TestCase):
    def test_ops(self):
        self.assertTrue(run.compare(0, "==", 0))
        self.assertTrue(run.compare(0, "<=", 0))
        self.assertTrue(run.compare(3, ">=", 3))
        self.assertTrue(run.compare(5, ">", 0))
        self.assertTrue(run.compare(1, "!=", 0))
        self.assertFalse(run.compare(5, "==", 0))
        self.assertFalse(run.compare(0, ">", 0))
        # float epsilon
        self.assertTrue(run.compare(0.0, "==", 1e-12))

    def test_unknown_op(self):
        with self.assertRaises(ValueError):
            run.compare(1, "~=", 1)


class TestJqGet(unittest.TestCase):
    def test_paths(self):
        obj = {"status": "ok", "checks": {"redis": "down", "postgres": "ok"}}
        self.assertEqual(run.jq_get(obj, "status"), "ok")
        self.assertEqual(run.jq_get(obj, "checks.redis"), "down")
        self.assertIs(run.jq_get(obj, "checks.missing"), MISSING)
        self.assertIs(run.jq_get(obj, "nope.deep"), MISSING)


class TestPromql(unittest.TestCase):
    def _resp(self, val):
        return {"status": "success", "data": {"resultType": "vector",
                "result": [{"metric": {}, "value": [1700000000, str(val)]}]}}

    def test_sample(self):
        self.assertEqual(run.extract_promql_sample(self._resp(7)), 7.0)
        self.assertIsNone(run.extract_promql_sample(
            {"status": "success", "data": {"result": []}}))
        self.assertIsNone(run.extract_promql_sample({"status": "error"}))
        self.assertIsNone(run.extract_promql_sample({}))

    def test_evaluate(self):
        passed, actual, had = run.evaluate_promql(self._resp(0), "==", 0)
        self.assertTrue(passed and had and actual == 0.0)
        passed, actual, had = run.evaluate_promql(self._resp(20), ">=", 20)
        self.assertTrue(passed)
        # empty vector treated as 0.0 → passes ==0, fails >=20
        empty = {"status": "success", "data": {"result": []}}
        self.assertTrue(run.evaluate_promql(empty, "==", 0)[0])
        self.assertFalse(run.evaluate_promql(empty, ">=", 20)[0])
        self.assertFalse(run.evaluate_promql(empty, ">=", 20)[2])  # had_sample

    def test_impossible_threshold_fails(self):
        # EXP-45: an impossible threshold must fail, not rubber-stamp.
        self.assertFalse(run.evaluate_promql(self._resp(0), "<=", -1)[0])


class TestHttp(unittest.TestCase):
    def test_status_only(self):
        self.assertEqual(run.evaluate_http(200, 200, "", None, None), (True, "status 200"))
        self.assertFalse(run.evaluate_http(503, 200, "", None, None)[0])

    def test_json_path(self):
        body = '{"status":"degraded","checks":{"redis":"down"}}'
        self.assertTrue(run.evaluate_http(503, 503, body, "checks.redis", "down")[0])
        self.assertFalse(run.evaluate_http(503, 503, body, "checks.redis", "ok")[0])
        self.assertFalse(run.evaluate_http(503, 503, body, "checks.missing", "x")[0])
        self.assertFalse(run.evaluate_http(200, 200, "not json", "status", "ok")[0])


class TestValidateExperiment(unittest.TestCase):
    def _valid(self):
        return {
            "id": "exp-99", "title": "T", "needs": ["compose"],
            "steps": [{"run": "make x"}],
            "assertions": [{"type": "promql", "query": "up", "op": "==",
                            "value": 1, "timeout": "30s"}],
        }

    def test_ok(self):
        self.assertEqual(run.validate_experiment(self._valid(), "exp-99"), [])

    def test_empty_assertions_rejected(self):
        d = self._valid(); d["assertions"] = []
        self.assertTrue(any("falsifiable" in e for e in run.validate_experiment(d)))

    def test_id_mismatch(self):
        self.assertTrue(run.validate_experiment(self._valid(), "exp-01"))

    def test_bad_op_type_timeout(self):
        d = self._valid()
        d["assertions"][0]["op"] = "~="
        self.assertTrue(run.validate_experiment(d, "exp-99"))
        d = self._valid(); d["assertions"][0]["type"] = "bogus"
        self.assertTrue(run.validate_experiment(d, "exp-99"))
        d = self._valid(); d["assertions"][0]["timeout"] = "soon"
        self.assertTrue(run.validate_experiment(d, "exp-99"))

    def test_unknown_need(self):
        d = self._valid(); d["needs"] = ["k8s"]
        self.assertTrue(run.validate_experiment(d, "exp-99"))

    def test_http_json_pair(self):
        d = self._valid()
        d["assertions"] = [{"type": "http", "url": "http://x", "status": 200,
                            "json_path": "a", "timeout": "5s"}]  # missing json_equals
        self.assertTrue(run.validate_experiment(d, "exp-99"))


class TestAllExperimentYaml(unittest.TestCase):
    """Load + schema-check EVERY experiments/*.yaml (task B2/self-verify)."""

    def test_every_file_valid(self):
        files = sorted((ROOT / "experiments").glob("exp-*.yaml"))
        self.assertGreaterEqual(len(files), 12, "expected exp-01..exp-12")
        for path in files:
            with self.subTest(experiment=path.name):
                doc = yaml.safe_load(path.read_text())
                errors = run.validate_experiment(doc, expected_id=path.stem)
                self.assertEqual(errors, [], f"{path.name}: {errors}")
                self.assertEqual(doc["id"], path.stem, "id must match filename")
                self.assertTrue(doc.get("steps"), "steps must be non-empty")
                self.assertTrue(doc.get("assertions"), "assertions must be non-empty")

    def test_ids_are_unique(self):
        ids = [yaml.safe_load(p.read_text())["id"]
               for p in (ROOT / "experiments").glob("exp-*.yaml")]
        self.assertEqual(len(ids), len(set(ids)))


if __name__ == "__main__":
    unittest.main(verbosity=2)
