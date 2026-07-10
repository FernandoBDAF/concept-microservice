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

	// Consumer Settings
	PrefetchCount int
	PrefetchSize  int
	Global        bool

	// Publisher Settings
	Mandatory bool
	Immediate bool

	// Retry Settings
	MaxRetries int
	RetryDelay time.Duration

	// Logging
	LogLevel string
}

func NewConfig() *Config {
	return &Config{
		Exchange:      ExchangeName,
		Queue:         QueueName,
		RoutingKey:    RoutingKey,
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
