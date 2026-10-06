package order

import (
	"encoding/json"
	"errors"
	"fmt"
)

type Request struct {
	Drink string `json:"drink"`
}

func DecodeRequest(raw any) (Request, error) {
	var req Request
	m, ok := raw.(map[string]any)
	if !ok {
		return req, fmt.Errorf("prepare_order value must be an object with a %q key", "drink")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return req, fmt.Errorf("prepare_order: %w", err)
	}
	if err := json.Unmarshal(b, &req); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return req, fmt.Errorf("prepare_order field %q must be a %s, got %s", typeErr.Field, typeErr.Type, typeErr.Value)
		}
		return req, fmt.Errorf("prepare_order: %w", err)
	}
	if req.Drink == "" {
		return req, fmt.Errorf("prepare_order field %q is required", "drink")
	}
	return req, nil
}
