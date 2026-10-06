package ordersensor

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/rdk/data"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

func TestReadingsEmpty(t *testing.T) {
	s := &orderSensor{}
	_, err := s.Readings(context.Background(), nil)
	if !errors.Is(err, data.ErrNoCaptureToStore) {
		t.Fatalf("want ErrNoCaptureToStore, got %v", err)
	}
}

func TestPushAndRead(t *testing.T) {
	s := &orderSensor{}
	start := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(42 * time.Second)
	s.PushOrderReading(order.Reading{
		OrderID:   "o1",
		Drink:     "negroni",
		Status:    order.StatusSucceeded,
		StartedAt: start,
		EndedAt:   end,
	})
	got, err := s.Readings(context.Background(), nil)
	if err != nil {
		t.Fatalf("Readings err: %v", err)
	}
	if got["order_id"] != "o1" {
		t.Errorf("order_id: want o1, got %v", got["order_id"])
	}
	if got["drink"] != "negroni" {
		t.Errorf("drink: want negroni, got %v", got["drink"])
	}
	if got["status"] != "succeeded" {
		t.Errorf("status: want succeeded, got %v", got["status"])
	}
	if _, err := s.Readings(context.Background(), nil); !errors.Is(err, data.ErrNoCaptureToStore) {
		t.Fatalf("after pop want ErrNoCaptureToStore, got %v", err)
	}
}

func TestPushFailure(t *testing.T) {
	s := &orderSensor{}
	s.PushOrderReading(order.Reading{
		OrderID:    "o2",
		Drink:      "margarita",
		Status:     order.StatusFailed,
		ErrMessage: "pour timed out",
		FailedStep: "pour_gin",
		StartedAt:  time.Now(),
		EndedAt:    time.Now().Add(time.Second),
	})
	got, err := s.Readings(context.Background(), nil)
	if err != nil {
		t.Fatalf("Readings err: %v", err)
	}
	if got["status"] != "failed" {
		t.Errorf("status: want failed, got %v", got["status"])
	}
	if got["error_message"] != "pour timed out" {
		t.Errorf("error_message: want 'pour timed out', got %v", got["error_message"])
	}
	if got["failed_step"] != "pour_gin" {
		t.Errorf("failed_step: want pour_gin, got %v", got["failed_step"])
	}
}
