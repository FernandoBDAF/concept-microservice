package queue

import "time"

const (
	// Exchange and Queue Names
	ExchangeName = "profile-tasks"
	QueueName    = "profile-processing"
	RoutingKey   = "profile.task"

	// Default Values
	DefaultPrefetchCount = 1
	DefaultMaxRetries    = 3
	DefaultRetryDelay    = 5 * time.Second

	// Connection Settings
	DefaultHeartbeat = 10 * time.Second
	DefaultLocale    = "en_US"
)
