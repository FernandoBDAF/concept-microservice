package metrics

import (
	"testing"
	"time"
)

func TestCounter(t *testing.T) {
	counter := NewCounter("test_counter", "Test counter", []string{"test"})

	// Test initial value
	if counter.Get() != 0 {
		t.Errorf("Expected initial value to be 0, got %f", counter.Get())
	}

	// Test Inc
	counter.Inc()
	if counter.Get() != 1 {
		t.Errorf("Expected value to be 1 after Inc, got %f", counter.Get())
	}

	// Test Add
	counter.Add(5)
	if counter.Get() != 6 {
		t.Errorf("Expected value to be 6 after Add(5), got %f", counter.Get())
	}

	// Test negative Add (should be ignored)
	counter.Add(-1)
	if counter.Get() != 6 {
		t.Errorf("Expected value to remain 6 after Add(-1), got %f", counter.Get())
	}
}

func TestGauge(t *testing.T) {
	gauge := NewGauge("test_gauge", "Test gauge", []string{"test"})

	// Test initial value
	if gauge.Get() != 0 {
		t.Errorf("Expected initial value to be 0, got %f", gauge.Get())
	}

	// Test Set
	gauge.Set(10)
	if gauge.Get() != 10 {
		t.Errorf("Expected value to be 10 after Set(10), got %f", gauge.Get())
	}

	// Test Inc
	gauge.Inc()
	if gauge.Get() != 11 {
		t.Errorf("Expected value to be 11 after Inc, got %f", gauge.Get())
	}

	// Test Dec
	gauge.Dec()
	if gauge.Get() != 10 {
		t.Errorf("Expected value to be 10 after Dec, got %f", gauge.Get())
	}

	// Test Add
	gauge.Add(5)
	if gauge.Get() != 15 {
		t.Errorf("Expected value to be 15 after Add(5), got %f", gauge.Get())
	}

	// Test Sub
	gauge.Sub(3)
	if gauge.Get() != 12 {
		t.Errorf("Expected value to be 12 after Sub(3), got %f", gauge.Get())
	}
}

func TestHistogram(t *testing.T) {
	buckets := []float64{1, 5, 10, 50, 100}
	histogram := NewHistogram("test_histogram", "Test histogram", []string{"test"}, buckets)

	// Test initial values
	stats := histogram.Get()
	if stats["count"] != 0 {
		t.Errorf("Expected initial count to be 0, got %f", stats["count"])
	}
	if stats["sum"] != 0 {
		t.Errorf("Expected initial sum to be 0, got %f", stats["sum"])
	}

	// Test Observe
	histogram.Observe(3)
	histogram.Observe(7)
	histogram.Observe(25)
	histogram.Observe(75)

	stats = histogram.Get()
	if stats["count"] != 4 {
		t.Errorf("Expected count to be 4, got %f", stats["count"])
	}
	if stats["sum"] != 110 {
		t.Errorf("Expected sum to be 110, got %f", stats["sum"])
	}
	if stats["avg"] != 27.5 {
		t.Errorf("Expected avg to be 27.5, got %f", stats["avg"])
	}
}

func TestTimer(t *testing.T) {
	timer := NewTimer("test_timer", "Test timer", []string{"test"})

	// Test initial values
	stats := timer.Get()
	if stats["count"] != 0 {
		t.Errorf("Expected initial count to be 0, got %f", stats["count"])
	}
	if stats["sum_seconds"] != 0 {
		t.Errorf("Expected initial sum to be 0, got %f", stats["sum_seconds"])
	}

	// Test Start/Stop
	timer.Start()
	time.Sleep(100 * time.Millisecond)
	timer.Stop()

	stats = timer.Get()
	if stats["count"] != 1 {
		t.Errorf("Expected count to be 1, got %f", stats["count"])
	}
	if stats["sum_seconds"] < 0.1 {
		t.Errorf("Expected sum to be at least 0.1, got %f", stats["sum_seconds"])
	}
	if stats["avg_seconds"] < 0.1 {
		t.Errorf("Expected avg to be at least 0.1, got %f", stats["avg_seconds"])
	}
}
