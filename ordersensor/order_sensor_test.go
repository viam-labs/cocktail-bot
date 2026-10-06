package ordersensor

import (
	"context"
	"errors"
	"testing"
	"time"

	"go.viam.com/rdk/data"
	"go.viam.com/test"

	"github.com/viam-labs/cocktail-bot/bartender/order"
)

func TestReadingsEmpty(t *testing.T) {
	s := &orderSensor{}
	_, err := s.Readings(context.Background(), nil)
	test.That(t, errors.Is(err, data.ErrNoCaptureToStore), test.ShouldBeTrue)
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
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got["order_id"], test.ShouldEqual, "o1")
	test.That(t, got["drink"], test.ShouldEqual, "negroni")
	test.That(t, got["status"], test.ShouldEqual, "succeeded")

	_, err = s.Readings(context.Background(), nil)
	test.That(t, errors.Is(err, data.ErrNoCaptureToStore), test.ShouldBeTrue)
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
	test.That(t, err, test.ShouldBeNil)
	test.That(t, got["status"], test.ShouldEqual, "failed")
	test.That(t, got["error_message"], test.ShouldEqual, "pour timed out")
	test.That(t, got["failed_step"], test.ShouldEqual, "pour_gin")
}
