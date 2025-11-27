package image

import (
	"context"
	"time"
)

// ImageRepository defines methods for temporary in-memory image storage
// with automatic resource cleanup via TTL.
type ImageRepository interface {
	// Save stores image bytes and returns a UUID identifier.
	// ttl defines how long the image should be kept in memory.
	Save(ctx context.Context, data []byte, ttl time.Duration) (string, error)
	// Get returns a copy of the image by id. The boolean indicates presence.
	Get(ctx context.Context, id string) ([]byte, bool)
	// Delete removes an image before TTL expiration.
	Delete(ctx context.Context, id string) error
}
