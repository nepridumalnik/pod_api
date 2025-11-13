package image

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type imageEntry struct {
	data  []byte
	timer *time.Timer
}

// MemoryRepository is an in-memory ImageRepository implementation.
type MemoryRepository struct {
	mu   sync.RWMutex
	data map[string]*imageEntry
}

// NewMemoryRepository creates an empty in-memory repository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{data: make(map[string]*imageEntry)}
}

// Save stores image bytes under a new UUID with TTL-based auto-deletion.
func (r *MemoryRepository) Save(ctx context.Context, b []byte, ttl time.Duration) (string, error) {
	if len(b) == 0 {
		return "", errors.New("empty image data")
	}

	id := uuid.NewString()

	// Make a copy of the data to avoid external modifications.
	copyBuf := make([]byte, len(b))
	copy(copyBuf, b)

	entry := &imageEntry{}
	// Create the timer first, but don't start until after storing.
	if ttl > 0 {
		entry.timer = time.AfterFunc(ttl, func() {
			// Background deletion; context not required.
			_ = r.Delete(context.Background(), id)
		})
	}
	entry.data = copyBuf

	r.mu.Lock()
	r.data[id] = entry
	r.mu.Unlock()

	return id, nil
}

// Get returns a copy of stored data by id without deleting it.
func (r *MemoryRepository) Get(ctx context.Context, id string) ([]byte, bool) {
	r.mu.RLock()
	e, ok := r.data[id]
	r.mu.RUnlock()
	if !ok || e == nil || len(e.data) == 0 {
		return nil, false
	}
	out := make([]byte, len(e.data))
	copy(out, e.data)
	return out, true
}

// Delete stops the TTL timer and removes the entry from memory.
func (r *MemoryRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	e, ok := r.data[id]
	if ok {
		delete(r.data, id)
	}
	r.mu.Unlock()

	if ok && e != nil && e.timer != nil {
		e.timer.Stop()
	}
	return nil
}
