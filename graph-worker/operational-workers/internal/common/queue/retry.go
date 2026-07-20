package queue

// Retry-tier routing (ADR-008.1).
//
// The broker owns the wait-queues (deploy/rabbitmq/definitions.json):
//   <queue>.retry.{5s,30s,2m}, bound to <exchange>.retry on
//   <rk>.retry.<tier>, each dead-lettering back to the main exchange.
//
// Consumer contract (replaces nack-to-DLQ for retryable errors):
//  1. attempt = DeathCount(headers, queue) — how many times this message
//     already cycled through a wait-queue (x-death count for our retry queues).
//  2. tier, ok := NextRetryTier(attempt); if ok → publish the message body
//     unchanged to RetryExchange(exchange) with routing key
//     RetryRoutingKey(rk, tier), persistent, headers copied so x-death
//     survives, then ACK the delivery.
//  3. if !ok (tiers exhausted) → publish to DLXExchange(exchange) with rk
//     (poison path), then ACK. The DLQ is reached explicitly, never via a
//     broker nack-requeue.
// Unretryable errors (unmarshal/validation, see ErrUnretryable) skip straight
// to step 3.

import (
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RetryTiers in escalation order; must match the generator
// (scripts/rabbitmq/generate-definitions.py RETRY_TIERS) and the queue names
// in deploy/rabbitmq/definitions.json.
var RetryTiers = []string{"5s", "30s", "2m"}

// NextRetryTier maps how-many-times-already-retried to the next wait-queue
// suffix. attempt 0 → "5s", 1 → "30s", 2 → "2m", ≥3 → exhausted.
func NextRetryTier(attempt int) (string, bool) {
	if attempt < 0 || attempt >= len(RetryTiers) {
		return "", false
	}
	return RetryTiers[attempt], true
}

// RetryExchange is the exchange the consumer publishes retries to; the broker
// binds <queue>.retry.<tier> off it (definitions.json).
func RetryExchange(exchange string) string { return exchange + ".retry" }

// DLXExchange is the dead-letter exchange for the poison path.
func DLXExchange(exchange string) string { return exchange + ".dlx" }

// RetryRoutingKey routes a retry publish to the wait-queue for the given tier
// (e.g. "email.send" + "5s" → "email.send.retry.5s").
func RetryRoutingKey(rk, tier string) string { return rk + ".retry." + tier }

// DeathCount extracts the number of prior retry cycles from the x-death
// header. x-death is a []interface{} of amqp.Table entries (one per
// (queue,reason) pair the message has been dead-lettered from); this sums the
// "count" of entries whose "queue" is one of our <queue>.retry.* wait-queues.
// The work queue itself is excluded so a redelivery of an un-acked message
// (crash between process and ack — same headers, never dead-lettered) does not
// inflate the attempt count.
func DeathCount(headers map[string]any, queue string) int {
	raw, ok := headers["x-death"]
	if !ok || raw == nil {
		return 0
	}

	entries, ok := toSlice(raw)
	if !ok {
		return 0
	}

	prefix := queue + ".retry."
	total := 0
	for _, e := range entries {
		tbl, ok := toTable(e)
		if !ok {
			continue
		}
		q, _ := tbl["queue"].(string)
		if !strings.HasPrefix(q, prefix) {
			continue
		}
		total += toInt(tbl["count"])
	}
	return total
}

// toSlice normalises the x-death value, which amqp091-go decodes as
// []interface{} but which a hand-built fixture may express as []amqp.Table.
func toSlice(v any) ([]any, bool) {
	switch s := v.(type) {
	case []any:
		return s, true
	case []amqp.Table:
		out := make([]any, len(s))
		for i := range s {
			out[i] = s[i]
		}
		return out, true
	default:
		return nil, false
	}
}

// toTable accepts either amqp.Table (how amqp091-go decodes nested tables) or
// a plain map[string]interface{} (equivalent underlying type).
func toTable(v any) (amqp.Table, bool) {
	switch t := v.(type) {
	case amqp.Table:
		return t, true
	case map[string]any:
		return amqp.Table(t), true
	default:
		return nil, false
	}
}

// toInt coerces the many integer shapes an AMQP field value can arrive as
// (amqp091-go decodes x-death "count" as int64; fixtures/JSON may use others).
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int8:
		return int(n)
	case int16:
		return int(n)
	case int32:
		return int(n)
	case int64:
		return int(n)
	case uint:
		return int(n)
	case uint8:
		return int(n)
	case uint16:
		return int(n)
	case uint32:
		return int(n)
	case uint64:
		return int(n)
	case float32:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
