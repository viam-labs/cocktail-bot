package bartender

import (
	"context"
	"errors"
	"fmt"
)

var errNotImplemented = errors.New("not implemented")

var knownVerbs = []string{
	"prepare_order",
	"get_queue",
}

func (b *bartender) DoCommand(_ context.Context, cmd map[string]any) (map[string]any, error) {
	for _, verb := range knownVerbs {
		if _, ok := cmd[verb]; ok {
			return nil, errNotImplemented
		}
	}
	return nil, fmt.Errorf("unknown command, supported: %v", knownVerbs)
}
