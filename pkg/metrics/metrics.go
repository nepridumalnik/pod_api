package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Registry stores counters for exposition and mirrors them to OTel counters.
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*atomic.Int64 // key = fullKey(name, labels)
	meter    metric.Meter
	otelCtrs map[string]metric.Int64Counter // base name -> instrument
}

func NewRegistry() *Registry {
	m := otel.GetMeterProvider().Meter("pod_api")
	return &Registry{
		counters: make(map[string]*atomic.Int64),
		meter:    m,
		otelCtrs: make(map[string]metric.Int64Counter),
	}
}

// fullKey makes deterministic key from name and labels map.
func fullKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(name)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}
	b.WriteByte('}')
	return b.String()
}

// Inc increases a named counter by n with labels.
// Also records the increment via OpenTelemetry counter instrument.
func (r *Registry) Inc(ctx context.Context, name string, labels map[string]string, n int64) {
	key := fullKey(name, labels)

	// local registry
	r.mu.RLock()
	c := r.counters[key]
	r.mu.RUnlock()
	if c == nil {
		r.mu.Lock()
		if c = r.counters[key]; c == nil {
			var v atomic.Int64
			r.counters[key] = &v
			c = &v
		}
		r.mu.Unlock()
	}
	c.Add(n)

	// OTel mirror
	r.mu.RLock()
	inst := r.otelCtrs[name]
	r.mu.RUnlock()
	if inst == nil {
		r.mu.Lock()
		if inst = r.otelCtrs[name]; inst == nil {
			ctr, _ := r.meter.Int64Counter(name)
			r.otelCtrs[name] = ctr
			inst = ctr
		}
		r.mu.Unlock()
	}
	if inst != nil {
		attrs := make([]attribute.KeyValue, 0, len(labels))
		for k, v := range labels {
			attrs = append(attrs, attribute.String(k, v))
		}
		inst.Add(ctx, n, metric.WithAttributes(attrs...))
	}
}

// Snapshot returns sorted text lines representing current counters.
func (r *Registry) SnapshotLines() []string {
	r.mu.RLock()
	keys := make([]string, 0, len(r.counters))
	for k := range r.counters {
		keys = append(keys, k)
	}
	r.mu.RUnlock()
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		r.mu.RLock()
		v := r.counters[k].Load()
		r.mu.RUnlock()
		lines = append(lines, fmt.Sprintf("%s %d", k, v))
	}
	return lines
}

// SnapshotJSON returns a map of counter->value for JSON rendering.
func (r *Registry) SnapshotJSON() map[string]int64 {
	out := make(map[string]int64)
	r.mu.RLock()
	for k, v := range r.counters {
		out[k] = v.Load()
	}
	r.mu.RUnlock()
	return out
}

// EchoHandlerText writes counters in simple text format.
func (r *Registry) EchoHandlerText(c echo.Context) error {
	lines := r.SnapshotLines()
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextPlainCharsetUTF8)
	for i := range lines {
		if _, err := c.Response().Write([]byte(lines[i] + "\n")); err != nil {
			return err
		}
	}
	return nil
}

// EchoHandlerJSON writes counters as JSON.
func (r *Registry) EchoHandlerJSON(c echo.Context) error {
	payload := r.SnapshotJSON()
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMEApplicationJSONCharsetUTF8)
	enc := json.NewEncoder(c.Response())
	return enc.Encode(payload)
}
