package testflow

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("task not found")
var ErrConflict = errors.New("conflict")

// Store defines the persistence contract used by the testflow service.
type Store interface {
	Save(record Record) error
	Get(taskID string) (Record, error)
	List() []Record
	FindByDedupeKey(dedupeKey string) (Record, error)
}

// MemoryStore is the in-memory implementation used for tests and local runs
// without PostgreSQL.
type MemoryStore struct {
	mu          sync.RWMutex
	records     map[string]Record
	dedupeIndex map[string]string
}

// NewMemoryStore initializes an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records:     make(map[string]Record),
		dedupeIndex: make(map[string]string),
	}
}

// Save upserts the full task aggregate and maintains the dedupe index.
func (s *MemoryStore) Save(record Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.records[record.Task.TaskID] = record
	if record.Task.DedupeKey != "" {
		s.dedupeIndex[record.Task.DedupeKey] = record.Task.TaskID
	}
	return nil
}

// Get loads a task aggregate by task ID.
func (s *MemoryStore) Get(taskID string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[taskID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}

// List returns all records for metrics aggregation and administrative reads.
func (s *MemoryStore) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		out = append(out, record)
	}
	return out
}

// FindByDedupeKey supports idempotent task creation.
func (s *MemoryStore) FindByDedupeKey(dedupeKey string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	taskID, ok := s.dedupeIndex[dedupeKey]
	if !ok {
		return Record{}, ErrNotFound
	}
	record, ok := s.records[taskID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}
