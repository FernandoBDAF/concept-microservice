package queue

import (
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// deathEntry builds one x-death table entry modeled on a real RabbitMQ capture:
// fields queue, reason, count, exchange, routing-keys, time — using the
// amqp091-go types the library actually decodes headers into (amqp.Table,
// []interface{}, int64, time.Time).
func deathEntry(queue, reason, exchange, rk string, count int64) amqp.Table {
	return amqp.Table{
		"count":        count,
		"reason":       reason,
		"queue":        queue,
		"time":         time.Now(),
		"exchange":     exchange,
		"routing-keys": []interface{}{rk},
	}
}

func TestDeathCount(t *testing.T) {
	const q = "email-processing"

	tests := []struct {
		name    string
		headers map[string]any
		want    int
	}{
		{"no headers", map[string]any{}, 0},
		{"nil x-death", map[string]any{"x-death": nil}, 0},
		{
			name: "single 5s cycle",
			headers: map[string]any{"x-death": []interface{}{
				deathEntry("email-processing.retry.5s", "expired", "email-tasks.retry", "email.send.retry.5s", 1),
			}},
			want: 1,
		},
		{
			name: "5s then 30s cycles (distinct queues, summed)",
			headers: map[string]any{"x-death": []interface{}{
				deathEntry("email-processing.retry.30s", "expired", "email-tasks.retry", "email.send.retry.30s", 1),
				deathEntry("email-processing.retry.5s", "expired", "email-tasks.retry", "email.send.retry.5s", 1),
			}},
			want: 2,
		},
		{
			name: "work-queue staleness expiry is NOT counted",
			headers: map[string]any{"x-death": []interface{}{
				deathEntry("email-processing.retry.5s", "expired", "email-tasks.retry", "email.send.retry.5s", 1),
				// staleness TTL on the work queue itself (reason expired) must be excluded
				deathEntry("email-processing", "expired", "email-tasks", "email.send", 3),
			}},
			want: 1,
		},
		{
			name: "recurring same tier accrues via the count field",
			headers: map[string]any{"x-death": []interface{}{
				deathEntry("email-processing.retry.2m", "expired", "email-tasks.retry", "email.send.retry.2m", 2),
			}},
			want: 2,
		},
		{
			name: "another worker's retry queue does not match this queue's prefix",
			headers: map[string]any{"x-death": []interface{}{
				deathEntry("image-processing.retry.5s", "expired", "image-tasks.retry", "image.process.retry.5s", 5),
			}},
			want: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DeathCount(tt.headers, q); got != tt.want {
				t.Errorf("DeathCount() = %d, want %d", got, tt.want)
			}
		})
	}
}

// TestDeathCount_TypeVariants covers the value shapes DeathCount must tolerate:
// x-death as []amqp.Table vs []interface{}, entries as amqp.Table vs plain map,
// and count as int32/int rather than only int64.
func TestDeathCount_TypeVariants(t *testing.T) {
	headers := map[string]any{"x-death": []amqp.Table{
		{"queue": "image-processing.retry.5s", "count": int32(1)},
	}}
	if got := DeathCount(headers, "image-processing"); got != 1 {
		t.Errorf("[]amqp.Table/int32 = %d, want 1", got)
	}

	headers2 := map[string]any{"x-death": []interface{}{
		map[string]any{"queue": "image-processing.retry.30s", "count": 2},
	}}
	if got := DeathCount(headers2, "image-processing"); got != 2 {
		t.Errorf("map/int = %d, want 2", got)
	}
}

func TestNextRetryTier(t *testing.T) {
	cases := []struct {
		attempt  int
		wantTier string
		wantOK   bool
	}{
		{0, "5s", true},
		{1, "30s", true},
		{2, "2m", true},
		{3, "", false}, // exhausted → DLQ
		{4, "", false},
		{-1, "", false},
	}
	for _, c := range cases {
		if tier, ok := NextRetryTier(c.attempt); tier != c.wantTier || ok != c.wantOK {
			t.Errorf("NextRetryTier(%d) = (%q,%v), want (%q,%v)", c.attempt, tier, ok, c.wantTier, c.wantOK)
		}
	}
}

func TestRoutingHelpers(t *testing.T) {
	if got := RetryExchange("email-tasks"); got != "email-tasks.retry" {
		t.Errorf("RetryExchange = %q, want email-tasks.retry", got)
	}
	if got := DLXExchange("email-tasks"); got != "email-tasks.dlx" {
		t.Errorf("DLXExchange = %q, want email-tasks.dlx", got)
	}
	if got := RetryRoutingKey("email.send", "5s"); got != "email.send.retry.5s" {
		t.Errorf("RetryRoutingKey = %q, want email.send.retry.5s", got)
	}
}

func TestCopyHeaders_NoAlias(t *testing.T) {
	orig := amqp.Table{
		"x-death":     []interface{}{amqp.Table{"queue": "q", "count": int64(1)}},
		"traceparent": "abc",
	}
	cp := copyHeaders(orig)
	if len(cp) != len(orig) {
		t.Fatalf("copyHeaders len = %d, want %d", len(cp), len(orig))
	}
	cp["new"] = "x"
	if _, aliased := orig["new"]; aliased {
		t.Error("copyHeaders must not alias the original table")
	}
}
