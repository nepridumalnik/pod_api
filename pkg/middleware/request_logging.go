package middleware

import (
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"pod_api/pkg/metrics"
)

// RequestLogger returns middleware that logs requests using zerolog
// and updates OpenTelemetry-backed counters.
func RequestLogger(reg *metrics.Registry) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			req := c.Request()
			rid := req.Header.Get("X-Request-ID")
			if rid == "" {
				rid = uuid.NewString()
				c.Response().Header().Set("X-Request-ID", rid)
			}

			// Attach request-scoped logger
			logger := log.With().
				Str("request_id", rid).
				Str("method", req.Method).
				Str("path", req.URL.Path).
				Str("remote_ip", c.RealIP()).
				Str("user_agent", req.UserAgent()).
				Logger()

			ctx := logger.WithContext(req.Context())
			c.SetRequest(req.WithContext(ctx))

			err := next(c)

			// Derive status
			status := c.Response().Status
			duration := time.Since(start)

			// Metrics: http requests total
			if reg != nil {
				reg.Inc(c.Request().Context(), "http_requests_total", map[string]string{
					"method": req.Method,
					"path":   req.URL.Path,
					"status": intToClass(status),
				}, 1)
			}

			// Log according to status
			if status >= 500 || err != nil {
				logger.Error().
					Err(err).
					Int("status", status).
					Dur("duration", duration).
					Msg("http request failed")
				if reg != nil {
					reg.Inc(c.Request().Context(), "http_requests_errors_total", map[string]string{
						"method": req.Method,
						"path":   req.URL.Path,
						"status": intToClass(status),
					}, 1)
				}
			} else {
				logger.Info().
					Int("status", status).
					Dur("duration", duration).
					Msg("http request served")
			}

			return err
		}
	}
}

func intToClass(code int) string {
	switch {
	case code >= 100 && code < 200:
		return "1xx"
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500 && code < 600:
		return "5xx"
	default:
		return "0"
	}
}
