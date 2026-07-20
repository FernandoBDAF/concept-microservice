package queue

import "time"

type Config struct {
	// Connection Settings
	URL       string
	Heartbeat time.Duration
	Locale    string

	// Exchange and Queue Settings
	Exchange   string
	Queue      string
	RoutingKey string
	Durable    bool
	AutoDelete bool
	Exclusive  bool
	NoWait     bool

	// WorkerType names the owning worker (email/image/profile) for per-worker
	// metric names (<type>_retries_total, <type>_dlq_total). Empty falls back
	// to "worker".
	WorkerType string

	// Dead-letter topology metadata. Since ADR-008.4 the broker owns all
	// queue args (definitions.json) and services only verify passively, so
	// these fields are informational — retained for observability/config
	// symmetry with the api-service publisher, not used to declare anything.
	// DLX is always "<Exchange>.dlx", the retry exchange "<Exchange>.retry",
	// and the DLQ "<Queue>.dlq".
	MessageTTL    time.Duration // main-queue staleness TTL (broker-owned)
	DeadLetterTTL time.Duration // .dlq TTL (broker-owned)
	MaxRetries    int           // retry-tier count (broker-owned; == len(RetryTiers))

	// Consumer Settings
	PrefetchCount int
	PrefetchSize  int
	Global        bool

	// Publisher Settings
	Mandatory bool
	Immediate bool

	// Reconnect Settings
	RetryDelay time.Duration // initial backoff between reconnect attempts

	// Logging
	LogLevel string
}

func NewConfig() *Config {
	return &Config{
		Durable:       true,
		AutoDelete:    false,
		Exclusive:     false,
		NoWait:        false,
		PrefetchCount: DefaultPrefetchCount,
		MaxRetries:    DefaultMaxRetries,
		RetryDelay:    DefaultRetryDelay,
		Heartbeat:     DefaultHeartbeat,
		Locale:        DefaultLocale,
		LogLevel:      "info",
	}
}
