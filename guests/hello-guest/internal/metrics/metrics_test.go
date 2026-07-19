package metrics

import (
	"strings"
	"testing"
)

func render(r *Registry) string {
	var b strings.Builder
	r.Write(&b)
	return b.String()
}

func TestCounterRendering(t *testing.T) {
	r := NewRegistry()
	c := r.Counter("hello_guest_web_requests_total", "Total HTTP requests served.")
	c.Inc()
	c.Add(2)

	got := render(r)
	want := "# HELP hello_guest_web_requests_total Total HTTP requests served.\n" +
		"# TYPE hello_guest_web_requests_total counter\n" +
		"hello_guest_web_requests_total 3\n"
	if got != want {
		t.Errorf("counter rendering:\n got: %q\nwant: %q", got, want)
	}
}

func TestGaugeFuncRendering(t *testing.T) {
	r := NewRegistry()
	r.GaugeFunc("hello_guest_web_uptime_seconds", "Seconds since process start.", func() float64 {
		return 42.5
	})

	got := render(r)
	want := "# HELP hello_guest_web_uptime_seconds Seconds since process start.\n" +
		"# TYPE hello_guest_web_uptime_seconds gauge\n" +
		"hello_guest_web_uptime_seconds 42.5\n"
	if got != want {
		t.Errorf("gauge rendering:\n got: %q\nwant: %q", got, want)
	}
}

func TestHistogramRendering(t *testing.T) {
	r := NewRegistry()
	// Exact binary fractions so the _sum renders without float noise.
	h := r.Histogram("hello_guest_job_duration_seconds", "Fake job duration.", []float64{0.5, 1})
	h.Observe(0.25) // -> le=0.5
	h.Observe(0.5)  // boundary: le is inclusive -> le=0.5
	h.Observe(0.75) // -> le=1
	h.Observe(2.0)  // -> +Inf only

	got := render(r)
	want := "# HELP hello_guest_job_duration_seconds Fake job duration.\n" +
		"# TYPE hello_guest_job_duration_seconds histogram\n" +
		"hello_guest_job_duration_seconds_bucket{le=\"0.5\"} 2\n" +
		"hello_guest_job_duration_seconds_bucket{le=\"1\"} 3\n" +
		"hello_guest_job_duration_seconds_bucket{le=\"+Inf\"} 4\n" +
		"hello_guest_job_duration_seconds_sum 3.5\n" +
		"hello_guest_job_duration_seconds_count 4\n"
	if got != want {
		t.Errorf("histogram rendering:\n got: %q\nwant: %q", got, want)
	}
}

func TestRegistrationOrderIsStable(t *testing.T) {
	r := NewRegistry()
	r.Counter("b_total", "second registered, first alphabetically? no — order is registration.")
	r.GaugeFunc("a_gauge", "registered after b_total.", func() float64 { return 0 })

	got := render(r)
	bIdx := strings.Index(got, "b_total")
	aIdx := strings.Index(got, "a_gauge")
	if bIdx == -1 || aIdx == -1 || bIdx > aIdx {
		t.Errorf("expected registration order (b_total before a_gauge), got:\n%s", got)
	}
}
