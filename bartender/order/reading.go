package order

import "time"

type Status string

const (
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

type Reading struct {
	OrderID    string
	Drink      string
	Status     Status
	ErrMessage string
	FailedStep string
	StartedAt  time.Time
	EndedAt    time.Time
}
