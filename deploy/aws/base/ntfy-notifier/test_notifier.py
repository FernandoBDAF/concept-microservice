"""Offline unit tests for the budget→ntfy notifier — no AWS, no network.

Run: `python3 -m unittest` (from this dir) or `python3 -m unittest
deploy.aws.base.ntfy-notifier.test_notifier`. Covers the threshold→priority
mapping (the part that decides ntfy urgency) and SNS-record shaping.
"""
import unittest

import notifier


class TestPriorityMapping(unittest.TestCase):
    # AWS Budgets phrases even a 50% breach as "...has exceeded...", so the
    # mapping must key on the percentage, not the word "exceeded".
    def test_50pct_is_default(self):
        msg = "AWS Budget coppice-lab-monthly has exceeded threshold > 50.0% ($25.00)"
        self.assertEqual(
            notifier._priority_and_tags(msg), ("default", "money_with_wings"))

    def test_80pct_is_high(self):
        msg = "AWS Budget coppice-lab-monthly has exceeded threshold > 80.0% ($40.00)"
        self.assertEqual(
            notifier._priority_and_tags(msg), ("high", "warning"))

    def test_100pct_is_urgent(self):
        msg = "AWS Budget coppice-lab-monthly has exceeded threshold > 100.0% ($50.00)"
        self.assertEqual(
            notifier._priority_and_tags(msg), ("urgent", "rotating_light"))

    def test_bare_exceeded_not_urgent(self):
        # "exceeded" with no threshold percentage must not force urgent.
        self.assertEqual(notifier._priority_and_tags("budget exceeded")[0], "default")

    def test_100_wins_over_80_substring(self):
        # A message mentioning both must resolve to the higher band (urgent).
        msg = "threshold crossed 80.0% earlier; now > 100.0%"
        self.assertEqual(
            notifier._priority_and_tags(msg), ("urgent", "rotating_light"))


class TestBuild(unittest.TestCase):
    def test_maps_subject_and_message(self):
        record = {"Sns": {"Subject": "coppice-lab budget",
                          "Message": "has exceeded threshold > 80.0%"}}
        title, body, priority, tags = notifier._build(record)
        self.assertEqual(title, "coppice-lab budget")
        self.assertEqual(body, "has exceeded threshold > 80.0%")
        self.assertEqual((priority, tags), ("high", "warning"))

    def test_defaults_when_fields_missing(self):
        title, body, priority, _ = notifier._build({"Sns": {}})
        self.assertEqual(title, "coppice-lab budget alert")
        self.assertEqual(body, "(no budget message)")
        self.assertEqual(priority, "default")


if __name__ == "__main__":
    unittest.main()
