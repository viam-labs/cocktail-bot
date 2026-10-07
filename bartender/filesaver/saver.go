package filesaver

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.viam.com/rdk/logging"
	"golang.org/x/sync/errgroup"
)

const (
	bufferSize = 1000
	numWorkers = 4
	drainGrace = 5 * time.Second
)

type SaveFunc func(path string) error

type job struct {
	orderID  string
	filename string
	save     SaveFunc
}

// A nil *Saver is valid — every method is a no-op, so callers don't nil-check.
type Saver struct {
	baseDir string
	ch      chan job
	ctx     context.Context
	cancel  context.CancelFunc
	g       *errgroup.Group
	logger  logging.Logger
}

// New returns nil if baseDir is empty, so an unset config disables saving cleanly.
func New(baseDir string, logger logging.Logger) *Saver {
	if baseDir == "" {
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &Saver{
		baseDir: baseDir,
		ch:      make(chan job, bufferSize),
		ctx:     ctx,
		cancel:  cancel,
		g:       new(errgroup.Group),
		logger:  logger,
	}
	for i := 0; i < numWorkers; i++ {
		s.g.Go(s.run)
	}
	return s
}

func (s *Saver) run() error {
	for {
		select {
		case <-s.ctx.Done():
			return nil
		case j, ok := <-s.ch:
			if !ok {
				return nil
			}
			s.write(j)
		}
	}
}

// tag=<orderID> format is the Viam Data Manager capture layout — synced files
// land with the orderID as a filterable tag on the data page.
func (s *Saver) pathFor(j job, now time.Time) string {
	stamp := now.Format("20060102_150405.000")
	return filepath.Join(s.baseDir, "tag="+j.orderID, fmt.Sprintf("%s_%s", stamp, j.filename))
}

func (s *Saver) write(j job) {
	path := s.pathFor(j, time.Now())
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		s.logger.Warnf("filesaver: mkdir %s: %v", filepath.Dir(path), err)
		return
	}
	if err := j.save(path); err != nil {
		s.logger.Warnf("filesaver: write %s: %v", path, err)
		return
	}
}

// Drops the job silently if ctx is done, the saver is shutting down, or orderID is "".
func (s *Saver) SaveAsync(ctx context.Context, orderID, filename string, save SaveFunc) {
	if s == nil || orderID == "" {
		return
	}
	if ctx.Err() != nil {
		return
	}
	if used := len(s.ch); used > int(0.8*float64(bufferSize)) {
		s.logger.Warnf("filesaver: buffer %d/%d (80%%+ full)", used, bufferSize)
	}
	select {
	case s.ch <- job{orderID: orderID, filename: filename, save: save}:
	case <-ctx.Done():
	case <-s.ctx.Done():
	}
}

func (s *Saver) Close() error {
	if s == nil {
		return nil
	}
	deadline := time.Now().Add(drainGrace)
	for time.Now().Before(deadline) {
		if len(s.ch) == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	s.cancel()
	return s.g.Wait()
}
