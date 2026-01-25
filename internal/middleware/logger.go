package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"dajtu/internal/logging"
)

type RequestLogger struct {
	traffic *TrafficStats
}

func NewRequestLogger(traffic *TrafficStats) *RequestLogger {
	return &RequestLogger{traffic: traffic}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.size += n
	return n, err
}

func (l *RequestLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(rec, r)

		duration := time.Since(start)
		if l.traffic != nil {
			l.traffic.Add(rec.size, time.Now())
		}
		ip := clientIP(r)
		logLine := fmt.Sprintf(
			"request method=%s path=%s status=%d bytes=%d ip=%s ua=%q dur_ms=%d",
			r.Method,
			r.URL.RequestURI(),
			rec.status,
			rec.size,
			ip,
			r.UserAgent(),
			duration.Milliseconds(),
		)
		if rec.status >= 400 {
			logging.Get(fmt.Sprintf("request_%d", rec.status)).Print(logLine)
			return
		}
		if isThumbRequest(r.URL.Path) {
			logging.Get("requests_thumb").Print(logLine)
			return
		}
		logging.Get("requests").Print(logLine)
	})
}

func isThumbRequest(path string) bool {
	if !strings.HasPrefix(path, "/i/") {
		return false
	}
	return strings.HasSuffix(path, "/thumb") || strings.HasSuffix(path, "/thumb.webp")
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if parsed := net.ParseIP(ip); parsed != nil {
				return parsed.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	if parsed := net.ParseIP(host); parsed != nil {
		return parsed.String()
	}
	return host
}
