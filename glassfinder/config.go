package glassfinder

import (
	"errors"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/services/vision"
)

var (
	defaultLabels        = []string{"wine glass", "cup"}
	defaultMinConfidence = 0.5
)

type Config struct {
	CameraName    string   `json:"camera_name"`
	DetectorName  string   `json:"detector_name"`
	Labels        []string `json:"labels,omitempty"`
	MinConfidence float64  `json:"min_confidence,omitempty"`
}

func (c *Config) Validate(_ string) ([]string, []string, error) {
	if c.CameraName == "" {
		return nil, nil, errors.New("camera_name is required")
	}
	if c.DetectorName == "" {
		return nil, nil, errors.New("detector_name is required")
	}
	if c.MinConfidence < 0 || c.MinConfidence > 1 {
		return nil, nil, errors.New("min_confidence must be in [0, 1]")
	}
	return []string{
		camera.Named(c.CameraName).String(),
		vision.Named(c.DetectorName).String(),
	}, nil, nil
}

func (c *Config) labels() []string {
	if len(c.Labels) == 0 {
		return defaultLabels
	}
	return c.Labels
}

func (c *Config) minConfidence() float64 {
	if c.MinConfidence == 0 {
		return defaultMinConfidence
	}
	return c.MinConfidence
}
