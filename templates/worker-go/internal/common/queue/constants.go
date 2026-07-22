package queue

import "time"

// Generic fallback defaults. Every worker binary overrides the topology
// fields (Exchange/Queue/RoutingKey/Prefetch/TTL/DeadLetterTTL/MaxRetries)
// explicitly per graph-worker/shared/contracts/ROUTING_KEYS.md — these
// constants only backstop NewConfig() and are not queue-specific.
const (
	DefaultPrefetchCount = 1
	DefaultMaxRetries    = 3
	DefaultRetryDelay    = 1 * time.Second

	// Connection Settings
	DefaultHeartbeat = 10 * time.Second
	DefaultLocale    = "en_US"
)
