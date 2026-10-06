package bartender

type Config struct{}

func (c *Config) Validate(_ string) ([]string, []string, error) {
	return nil, nil, nil
}
