package review

import (
	"errors"
	"sync"
)

var ErrNotFound = errors.New("review not found")

type Store interface {
	Save(record Record) error
	Get(reviewID string) (Record, error)
	FindByDedupeKey(dedupeKey string) (Record, error)
	List() []Record
}

type MemoryStore struct {
	mu          sync.RWMutex
	records     map[string]Record
	dedupeIndex map[string]string
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		records:     make(map[string]Record),
		dedupeIndex: make(map[string]string),
	}
}

func (s *MemoryStore) Save(record Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[record.Review.ReviewID] = record
	if record.Review.DedupeKey != "" {
		s.dedupeIndex[record.Review.DedupeKey] = record.Review.ReviewID
	}
	return nil
}

func (s *MemoryStore) Get(reviewID string) (Record, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, ok := s.records[reviewID]
	if !ok {
		return Record{}, ErrNotFound
	}
	return record, nil
}

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

func (s *MemoryStore) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Record, 0, len(s.records))
	for _, record := range s.records {
		out = append(out, record)
	}
	return out
}
