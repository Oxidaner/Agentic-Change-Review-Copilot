package review

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("review not found")

// Store defines the persistence contract used by the review service.
//
// The interface is intentionally small so the service can switch between in-memory
// and PostgreSQL implementations without changing the business flow.
type Store interface {
	Save(record Record) error
	Get(reviewID string) (Record, error)
	FindByDedupeKey(dedupeKey string) (Record, error)
	List() []Record
}

// MemoryStore is the lightweight implementation used for tests and local runs
// when no database is configured.
type MemoryStore struct {
	mu          sync.RWMutex
	records     map[string]Record
	dedupeIndex map[string]string
}

// NewMemoryStore initializes an empty in-memory review store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records:     make(map[string]Record),
		dedupeIndex: make(map[string]string),
	}
}

// Save upserts the full materialized record.
//
// The service always persists the whole record snapshot, so the in-memory store
// can stay simple and avoid partial update semantics.
func (s *MemoryStore) Save(record Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.Review.ReviewID] = record
	if record.Review.DedupeKey != "" {
		s.dedupeIndex[record.Review.DedupeKey] = record.Review.ReviewID
	}
	return nil
}

// Get loads a review record by its review identifier.
func (s *MemoryStore) Get(reviewID string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[reviewID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}

// FindByDedupeKey returns an existing record for idempotent create behavior.
func (s *MemoryStore) FindByDedupeKey(dedupeKey string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	reviewID, ok := s.dedupeIndex[dedupeKey]
	if !ok {
		return Record{}, ErrNotFound
	}
	record, ok := s.records[reviewID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}

// List returns a snapshot of all persisted records for simple metric aggregation.
func (s *MemoryStore) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		out = append(out, record)
	}
	return out
}
