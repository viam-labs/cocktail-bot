package order

import (
	"time"

	"github.com/google/uuid"
)

type Order struct {
	ID          string    `json:"id"`
	Drink       string    `json:"drink"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
	RawStep     string    `json:"raw_step"`
	CompletedAt time.Time `json:"completed_at"`
}

func NewOrder(drink string) Order {
	return Order{
		ID:         uuid.New().String(),
		Drink:      drink,
		EnqueuedAt: time.Now(),
	}
}
