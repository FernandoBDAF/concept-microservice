"""Budget → ntfy notifier (ADR-006.4, HANDOFF §3).

AWS Budgets publishes threshold breaches (50/80/100%) to an SNS topic; this
Lambda subscribes and re-POSTs each to an ntfy topic, mirroring the payload
mapping of scripts/obs/ntfy-relay/main.go (Title / Priority / Tags headers +
a plain-text body). Email stays the v0 channel — this path is gated in
Terraform on `ntfy_topic_url` (count 0 when empty), so it only exists when a
topic URL is configured.

Config (env):
  NTFY_TOPIC_URL   full ntfy topic URL to POST to (e.g.
                   https://ntfy.sh/coppice-lab-budget). Required; the Lambda
                   is not created without it.

Stdlib only (urllib) — matches the "lab plumbing, not a service" spirit of the
relay; no boto3 needed since the SNS event carries the message.
"""
import json
import os
import urllib.request


def _priority_and_tags(text):
    """Mirror ntfy-relay severity mapping, adapted for budget thresholds.

    100% / exceeded → urgent (rotating_light); 80% → high (warning);
    otherwise (50% / informational) → default (money_with_wings).
    """
    low = text.lower()
    if "100%" in low or "exceeded" in low or "forecast" in low and "100" in low:
        return "urgent", "rotating_light"
    if "80%" in low:
        return "high", "warning"
    return "default", "money_with_wings"


def _build(record):
    """Map one SNS record onto (title, body, priority, tags)."""
    sns = record.get("Sns", {})
    subject = sns.get("Subject") or "coppice-lab budget alert"
    message = sns.get("Message") or "(no budget message)"
    priority, tags = _priority_and_tags(subject + " " + message)
    title = subject
    return title, message, priority, tags


def _publish(url, title, body, priority, tags):
    req = urllib.request.Request(
        url,
        data=body.encode("utf-8"),
        method="POST",
        headers={"Title": title, "Priority": priority, "Tags": tags},
    )
    with urllib.request.urlopen(req, timeout=10) as resp:  # nosec — lab plumbing
        return resp.status


def handler(event, _context):
    url = os.environ.get("NTFY_TOPIC_URL", "")
    if not url:
        print(json.dumps({"msg": "no NTFY_TOPIC_URL; nothing to do"}))
        return {"published": 0, "reason": "no-topic-url"}

    published = 0
    failed = 0
    for record in event.get("Records", []):
        title, body, priority, tags = _build(record)
        try:
            status = _publish(url, title, body, priority, tags)
            published += 1
            print(json.dumps({"title": title, "priority": priority,
                              "status": status, "action": "published"}))
        except Exception as exc:  # non-2xx / network — SNS retries the delivery
            failed += 1
            print(json.dumps({"title": title, "action": "publish-failed",
                              "error": str(exc)}))

    if failed:
        # Raise so SNS retries per its delivery policy (mirrors the relay's
        # non-2xx-makes-Alertmanager-retry behavior).
        raise RuntimeError(f"{failed} ntfy publish(es) failed")
    return {"published": published}
