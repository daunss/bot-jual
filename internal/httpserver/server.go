package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"bot-jual/internal/atl"
	"bot-jual/internal/cache"
	"bot-jual/internal/metrics"
	"bot-jual/internal/nlu"
	"bot-jual/internal/repo"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handlers groups optional HTTP handlers to mount.
type Handlers struct {
	AtlanticWebhook http.Handler
}

// Dependencies exposes core dependencies to handlers that need them.
type Dependencies struct {
	Repository repo.Repository
	Redis      *cache.Redis
	NLU        *nlu.Client
	Atlantic   *atl.Client
}

// Server wraps an http.Server with predefined routes.
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
	metrics    *metrics.Metrics
	handlers   Handlers
	deps       Dependencies
	basePath   string
}

// New creates a new HTTP server listening on addr with health and metrics endpoints.
func New(addr string, logger *slog.Logger, metricRegistry *metrics.Metrics, handlers Handlers, basePath string) *Server {
	server := &Server{
		logger:   logger.With("component", "http"),
		metrics:  metricRegistry,
		handlers: handlers,
		basePath: normaliseBasePath(basePath),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/admin/reload-price-cache", server.handleReloadPriceCache)

	if handlers.AtlanticWebhook != nil {
		mux.Handle("/webhook/atlantic", handlers.AtlanticWebhook)
	}

	handler := mountWithBasePath(server.basePath, mux)

	server.httpServer = &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if server.basePath != "" {
		server.logger.Info("http server configured with base path", "base_path", server.basePath)
	}

	return server
}

// SetDependencies makes dependencies accessible to handlers.
func (s *Server) SetDependencies(deps Dependencies) {
	s.deps = deps
}

// Start begins listening for incoming HTTP requests.
func (s *Server) Start() error {
	s.logger.Info("http server listening", "addr", s.httpServer.Addr)
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http server listen: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down http server")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) handleReloadPriceCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.deps.Atlantic == nil {
		http.Error(w, "atlantic client unavailable", http.StatusServiceUnavailable)
		return
	}

	productType := r.URL.Query().Get("type")
	items, err := s.deps.Atlantic.PriceList(r.Context(), productType, true)
	if err != nil {
		s.logger.Error("failed reloading price list", "error", err, "type", productType)
		http.Error(w, "failed reloading price list", http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]any{
		"status": "ok",
		"type":   productType,
		"count":  len(items),
	})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "failed to encode json", http.StatusInternalServerError)
	}
}

func mountWithBasePath(basePath string, handler http.Handler) http.Handler {
	if basePath == "" {
		return handler
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, basePath) {
			http.NotFound(w, r)
			return
		}
		if len(r.URL.Path) > len(basePath) && r.URL.Path[len(basePath)] != '/' {
			http.NotFound(w, r)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, basePath)
		if trimmed == "" {
			trimmed = "/"
		}
		r.URL.Path = trimmed
		if r.URL.RawPath != "" {
			rawTrimmed := strings.TrimPrefix(r.URL.RawPath, basePath)
			if rawTrimmed == "" {
				rawTrimmed = "/"
			}
			r.URL.RawPath = rawTrimmed
		}
		handler.ServeHTTP(w, r)
	})
}

func normaliseBasePath(base string) string {
	base = strings.TrimSpace(base)
	if base == "" || base == "/" {
		return ""
	}
	if !strings.HasPrefix(base, "/") {
		base = "/" + base
	}
	return strings.TrimSuffix(base, "/")
}
