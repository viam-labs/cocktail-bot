package order

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Snapshot is everything the queue needs to pick up where it left off after a restart.
type Snapshot struct {
	Pending        []Order `json:"pending"`
	Current        *Order  `json:"current,omitempty"`
	AwaitingPickup *Order  `json:"awaiting_pickup,omitempty"`
	Recent         []Order `json:"recent"`
}

type Store interface {
	Load() (Snapshot, error)
	Save(Snapshot) error
}

// storedOrder carries the payload, which Order hides from API responses.
type storedOrder struct {
	Order
	Payload json.RawMessage `json:"payload,omitempty"`
}

type storedSnapshot struct {
	Pending        []storedOrder `json:"pending"`
	Current        *storedOrder  `json:"current,omitempty"`
	AwaitingPickup *storedOrder  `json:"awaiting_pickup,omitempty"`
	Recent         []storedOrder `json:"recent"`
}

func toStored(o Order) storedOrder { return storedOrder{Order: o, Payload: o.Payload} }

func fromStored(s storedOrder) Order {
	o := s.Order
	o.Payload = s.Payload
	return o
}

func storeList(in []Order) []storedOrder {
	out := make([]storedOrder, len(in))
	for i, o := range in {
		out[i] = toStored(o)
	}
	return out
}

func loadList(in []storedOrder) []Order {
	out := make([]Order, len(in))
	for i, o := range in {
		out[i] = fromStored(o)
	}
	return out
}

func storePtr(o *Order) *storedOrder {
	if o == nil {
		return nil
	}
	s := toStored(*o)
	return &s
}

func loadPtr(s *storedOrder) *Order {
	if s == nil {
		return nil
	}
	o := fromStored(*s)
	return &o
}

// FileStore keeps the queue in a JSON file, replaced atomically on every save.
type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (f *FileStore) Load() (Snapshot, error) {
	raw, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return Snapshot{}, nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("read %s: %w", f.path, err)
	}
	var s storedSnapshot
	if err := json.Unmarshal(raw, &s); err != nil {
		return Snapshot{}, fmt.Errorf("parse %s: %w", f.path, err)
	}
	return Snapshot{
		Pending:        loadList(s.Pending),
		Current:        loadPtr(s.Current),
		AwaitingPickup: loadPtr(s.AwaitingPickup),
		Recent:         loadList(s.Recent),
	}, nil
}

func (f *FileStore) Save(snap Snapshot) error {
	raw, err := json.Marshal(storedSnapshot{
		Pending:        storeList(snap.Pending),
		Current:        storePtr(snap.Current),
		AwaitingPickup: storePtr(snap.AwaitingPickup),
		Recent:         storeList(snap.Recent),
	})
	if err != nil {
		return fmt.Errorf("marshal queue: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(f.path), 0o755); err != nil {
		return fmt.Errorf("create %s: %w", filepath.Dir(f.path), err)
	}
	tmp := f.path + ".tmp"
	file, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", tmp, err)
	}
	_, werr := file.Write(raw)
	// Sync before rename so a power cut leaves either the old queue or the new one, never an empty file.
	serr := file.Sync()
	cerr := file.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmp, f.path, err)
	}
	return nil
}
