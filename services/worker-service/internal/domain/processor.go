package domain

import "context"

type Processor interface {
	Process(ctx context.Context, msg *ProfileMessage) error
	Validate(msg *ProfileMessage) error
	Type() string
}

type ProcessingResult struct {
	Success bool
	Error   error
	Metrics map[string]float64
}
