"""Offline unit tests for the reaper — no AWS, boto3 stubbed via sys.modules.

Run: `python3 -m unittest` (from this dir) or `python3 -m unittest
deploy.aws.base.reaper.test_reaper`. Covers ARN parsing and the dry-run /
delete dispatch — the parts that decide the blast radius.
"""
import json
import io
import sys
import types
import unittest
from contextlib import redirect_stdout

import reaper


class FakeClient:
    def __init__(self, calls, name):
        self._calls = calls
        self._name = name

    def get_paginator(self, _op):
        return self

    def paginate(self, **_kw):
        return [{"ResourceTagMappingList": self._calls["_resources"]}]

    def __getattr__(self, method):
        def _record(**kwargs):
            self._calls.setdefault(self._name, []).append((method, kwargs))
        return _record


class FakeSession:
    def __init__(self, resources):
        self.calls = {"_resources": resources}

    def client(self, name):
        return FakeClient(self.calls, name)


def _run(resources, dry_run):
    """Invoke the handler with boto3 stubbed to return `resources`."""
    sess = FakeSession(resources)
    fake_boto3 = types.ModuleType("boto3")
    fake_boto3.Session = lambda *a, **k: sess
    sys.modules["boto3"] = fake_boto3
    import os
    os.environ["DRY_RUN"] = "true" if dry_run else "false"
    buf = io.StringIO()
    with redirect_stdout(buf):
        result = reaper.handler({}, None)
    logs = [json.loads(line) for line in buf.getvalue().splitlines() if line.strip()]
    return result, logs, sess.calls


def _res(arn, ttl, stack="session"):
    tags = [{"Key": "stack", "Value": stack}]
    if ttl is not None:
        tags.append({"Key": "ttl", "Value": str(ttl)})
    return {"ResourceARN": arn, "Tags": tags}


class TestParseArn(unittest.TestCase):
    def test_ec2_instance(self):
        self.assertEqual(
            reaper.parse_arn("arn:aws:ec2:us-east-1:123:instance/i-abc"),
            ("ec2", "instance", "i-abc"),
        )

    def test_rds_colon(self):
        self.assertEqual(
            reaper.parse_arn("arn:aws:rds:us-east-1:123:db:mydb"),
            ("rds", "db", "mydb"),
        )

    def test_eks_nodegroup_nested(self):
        self.assertEqual(
            reaper.parse_arn("arn:aws:eks:us-east-1:123:nodegroup/cl/ng/uuid"),
            ("eks", "nodegroup", "cl/ng/uuid"),
        )

    def test_elbv2(self):
        svc, rtype, rid = reaper.parse_arn(
            "arn:aws:elasticloadbalancing:us-east-1:123:loadbalancer/app/lb/id")
        self.assertEqual((svc, rtype), ("elasticloadbalancing", "loadbalancer"))


class TestHandler(unittest.TestCase):
    def test_dry_run_deletes_nothing(self):
        past = 100  # long expired
        result, logs, calls = _run([_res("arn:aws:ec2:r:1:instance/i-1", past)], True)
        self.assertTrue(result["dry_run"])
        self.assertEqual(result["reaped"], 0)
        self.assertEqual(result["considered"], 1)
        self.assertEqual(calls.get("ec2"), None)  # no delete calls
        self.assertTrue(any(l.get("action") == "would-delete" for l in logs))

    def test_expired_gets_deleted(self):
        result, _, calls = _run([_res("arn:aws:ec2:r:1:instance/i-1", 100)], False)
        self.assertEqual(result["reaped"], 1)
        self.assertEqual(calls["ec2"], [("terminate_instances", {"InstanceIds": ["i-1"]})])

    def test_base_stack_never_touched(self):
        result, logs, calls = _run(
            [_res("arn:aws:ec2:r:1:instance/i-1", 100, stack="base")], False)
        self.assertEqual(result["reaped"], 0)
        self.assertNotIn("ec2", calls)
        self.assertTrue(any(l.get("reason") == "stack=base" for l in logs))

    def test_not_expired_skipped(self):
        result, _, calls = _run(
            [_res("arn:aws:ec2:r:1:instance/i-1", 9999999999)], False)
        self.assertEqual(result["reaped"], 0)
        self.assertNotIn("ec2", calls)

    def test_rds_delete_args(self):
        result, _, calls = _run([_res("arn:aws:rds:r:1:db:mydb", 100)], False)
        self.assertEqual(result["reaped"], 1)
        method, kwargs = calls["rds"][0]
        self.assertEqual(method, "delete_db_instance")
        self.assertEqual(kwargs["DBInstanceIdentifier"], "mydb")
        self.assertTrue(kwargs["SkipFinalSnapshot"])

    def test_elbv2_uses_full_arn(self):
        arn = "arn:aws:elasticloadbalancing:r:1:loadbalancer/app/lb/id"
        result, _, calls = _run([_res(arn, 100)], False)
        method, kwargs = calls["elbv2"][0]  # boto3 client name, not ARN service
        self.assertEqual(method, "delete_load_balancer")
        self.assertEqual(kwargs["LoadBalancerArn"], arn)

    def test_eks_nodegroup_args(self):
        arn = "arn:aws:eks:r:1:nodegroup/mycluster/myng/uuid"
        _run([_res(arn, 100)], False)


if __name__ == "__main__":
    unittest.main()
