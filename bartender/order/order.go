package order

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID          string    `json:"id"`
	Drink       string    `json:"drink"`
	Double      bool      `json:"double"`
	Quantity    int       `json:"quantity"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
	StartedAt   time.Time `json:"started_at"`
	RawStep     string    `json:"raw_step"`
	CompletedAt time.Time `json:"completed_at"`
	Error       string    `json:"error,omitempty"`

	// Payload is what the executor needs to make the drink; it never leaves the module.
	Payload any `json:"-"`
}

func NewOrder(drink string) Order {
	return Order{
		ID:         uuid.New().String(),
		Drink:      drink,
		Quantity:   1,
		EnqueuedAt: time.Now(),
	}
}
