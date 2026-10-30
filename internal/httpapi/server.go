// Package httpapi serves the read API from stored data.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/czarist/api_brasileirao/internal/domain"
	"golang.org/x/time/rate"
)

type contextKey int

const requestIDKey contextKey = 1

// Options configures cross-cutting HTTP behavior.
type Options struct {
	CORSOrigins []string
	RateRPS     float64
	RateBurst   int
	TrustProxy  bool
	Log         *slog.Logger
}

// Handler is the public HTTP server.
type Handler struct {
	reader  Reader
	options Options
	mux     *http.ServeMux
	limits  *limitStore
}

// New registers the public routes.
func New(reader Reader, options Options) *Handler {
	if options.Log == nil {
		options.Log = slog.Default()
	}
	if options.RateRPS <= 0 {
		options.RateRPS = 10
	}
	if options.RateBurst < 1 {
		options.RateBurst = 1
	}
	h := &Handler{
		reader:  reader,
		options: options,
		mux:     http.NewServeMux(),
		limits:  newLimitStore(rate.Limit(options.RateRPS), options.RateBurst),
	}
	h.mux.HandleFunc("GET /{$}", h.index)
	h.mux.HandleFunc("GET /health/live", h.live)
	h.mux.HandleFunc("GET /health/ready", h.ready)
	h.mux.HandleFunc("GET /openapi.yaml", h.openAPI)
	h.mux.HandleFunc("GET /docs", h.docs)
	h.mux.HandleFunc("GET /docs/assets/{file}", h.asset)
	h.mux.HandleFunc("GET /api/v1/competitions", h.competitions)
	h.mux.HandleFunc("GET /api/v1/competitions/{code}", h.competition)
	h.mux.HandleFunc("GET /api/v1/competitions/{code}/seasons", h.seasons)
	h.mux.HandleFunc("GET /api/v1/competitions/{code}/seasons/{year}", h.season)
	h.mux.HandleFunc("GET /api/v1/competitions/{code}/seasons/{year}/teams", h.teams)
	h.mux.HandleFunc("GET /api/v1/competitions/{code}/seasons/{year}/standings", h.standings)
	h.mux.HandleFunc("GET /api/v1/competitions/{code}/seasons/{year}/rounds", h.rounds)
	h.mux.HandleFunc("GET /api/v1/competitions/{code}/seasons/{year}/matches", h.matches)
	h.mux.HandleFunc("GET /api/v1/matches/{id}", h.match)
	h.mux.HandleFunc("/api/", h.unknown)
	return h
}

// ServeHTTP adds the request id, CORS, recovery, and the API rate limit.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	id := requestID(r.Header.Get("X-Request-Id"))
	w.Header().Set("X-Request-Id", id)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	h.writeCORS(w, r)
	ctx := context.WithValue(r.Context(), requestIDKey, id)
	r = r.WithContext(ctx)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	rec := &statusWriter{ResponseWriter: w, status: http.StatusOK}
	defer func() {
		if recovered := recover(); recovered != nil {
			h.options.Log.Error("request panic", "request_id", id, "panic", recovered)
			writeError(rec, http.StatusInternalServerError, "internal", "internal error", id)
		}
		h.options.Log.Info("request", "request_id", id, "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration_ms", time.Since(started).Milliseconds())
	}()
	if strings.HasPrefix(r.URL.Path, "/api/") && !h.limits.allow(clientIP(r, h.options.TrustProxy)) {
		rec.Header().Set("Retry-After", "1")
		writeError(rec, http.StatusTooManyRequests, "rate_limited", "too many requests", id)
		return
	}
	h.mux.ServeHTTP(rec, r)
}

func (h *Handler) writeCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	allow := ""
	for _, item := range h.options.CORSOrigins {
		if item == "*" {
			if origin != "" {
				allow = origin
			} else {
				allow = "*"
			}
			break
		}
		if origin != "" && strings.EqualFold(item, origin) {
			allow = origin
			break
		}
	}
	if allow != "" {
		w.Header().Set("Access-Control-Allow-Origin", allow)
		w.Header().Set("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, X-Request-Id")
}

func requestID(header string) string {
	header = strings.TrimSpace(header)
	if header != "" && len(header) <= 64 && validToken(header) {
		return header
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "request"
	}
	return hex.EncodeToString(buf)
}

func validToken(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}

func clientIP(r *http.Request, trustProxy bool) string {
	if trustProxy {
		forwarded := r.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			part := strings.TrimSpace(strings.Split(forwarded, ",")[0])
			if ip := net.ParseIP(part); ip != nil {
				return ip.String()
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type limitStore struct {
	mu    sync.Mutex
	items map[string]*rate.Limiter
	limit rate.Limit
	burst int
}

func newLimitStore(limit rate.Limit, burst int) *limitStore {
	return &limitStore{items: map[string]*rate.Limiter{}, limit: limit, burst: burst}
}

func (s *limitStore) allow(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) > 10000 {
		s.items = map[string]*rate.Limiter{}
	}
	limiter, ok := s.items[key]
	if !ok {
		limiter = rate.NewLimiter(s.limit, s.burst)
		s.items[key] = limiter
	}
	return limiter.Allow()
}

type statusWriter struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (w *statusWriter) WriteHeader(status int) {
	if w.wrote {
		return
	}
	w.status = status
	w.wrote = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(body []byte) (int, error) {
	if !w.wrote {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message, requestID string) {
	writeJSON(w, status, errorDTO{Code: code, Message: message, RequestID: requestID})
}

func requestIDFrom(r *http.Request) string {
	value, _ := r.Context().Value(requestIDKey).(string)
	if value == "" {
		return "request"
	}
	return value
}

func (h *Handler) writeStoredError(w http.ResponseWriter, r *http.Request, err error, notFound string) {
	id := requestIDFrom(r)
	var invalid *domain.ValidationError
	switch {
	case errors.As(err, &invalid):
		writeError(w, http.StatusBadRequest, "invalid", invalid.Message, id)
	case errors.Is(err, domain.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", notFound, id)
	case errors.Is(err, domain.ErrInvalid):
		writeError(w, http.StatusBadRequest, "invalid", "invalid request", id)
	default:
		h.options.Log.Error("database request failed", "request_id", id, "error", err.Error())
		writeError(w, http.StatusServiceUnavailable, "unavailable", "database is unavailable", id)
	}
}
