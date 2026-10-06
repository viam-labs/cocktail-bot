package bartender

import (
	"go.viam.com/rdk/components/sensor"
)

type Config struct {
	OrderSensorName string `json:"order_sensor_name,omitempty"`
}

func (c *Config) Validate(_ string) ([]string, []string, error) {
	var optional []string
	if c.OrderSensorName != "" {
		optional = append(optional, sensor.Named(c.OrderSensorName).String())
	}
	return nil, optional, nil
}
