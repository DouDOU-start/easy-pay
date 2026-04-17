// Package testsink is an in-memory ring buffer that captures inbound HTTP
// requests aimed at /test/notify/*. It lets developers point a merchant's
// notify_url at the gateway itself during local testing and inspect what
// the platform would have posted downstream. All state is lost on restart.
package testsink

import (
	"sync"
	"time"
)

type Record struct {
	ID         int64             `json:"id"`
	ReceivedAt time.Time         `json:"received_at"`
	Method     string            `json:"method"`
	Path       string            `json:"path"`
	Slot       string            `json:"slot"`
	RemoteIP   string            `json:"remote_ip"`
	Query      string            `json:"query"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
	BodySize   int               `json:"body_size"`
	Truncated  bool              `json:"truncated"`
}

type Sink struct {
	mu       sync.RWMutex
	records  []Record
	capacity int
	nextID   int64
}

func New(capacity int) *Sink {
	if capacity <= 0 {
		capacity = 200
	}
	return &Sink{capacity: capacity, records: make([]Record, 0, capacity)}
}

func (s *Sink) Push(r Record) Record {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	r.ID = s.nextID
	if r.ReceivedAt.IsZero() {
		r.ReceivedAt = time.Now()
	}
	if len(s.records) >= s.capacity {
		s.records = s.records[1:]
	}
	s.records = append(s.records, r)
	return r
}

// List returns a snapshot of records, newest first.
func (s *Sink) List() []Record {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Record, len(s.records))
	for i, r := range s.records {
		out[len(s.records)-1-i] = r
	}
	return out
}

func (s *Sink) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = s.records[:0]
}

func (s *Sink) Capacity() int { return s.capacity }
