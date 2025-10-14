package atl

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"bot-jual/internal/metrics"

	"log/slog"
)

// WebhookEvent contains metadata and payload from Atlantic webhook.
type WebhookEvent struct {
	Type       string
	Headers    map[string]string
	Payload    json.RawMessage
	ReceivedAt time.Time
}

// WebhookProcessor defines handler interface for Atlantic events.
type WebhookProcessor interface {
	HandleAtlanticEvent(ctx context.Context, event WebhookEvent) error
}

// WebhookHandler verifies Atlantic webhook signature and forwards events.
type WebhookHandler struct {
	logger      *slog.Logger
	metrics     *metrics.Metrics
	usernameMD5 string
	passwordMD5 string
	processor   WebhookProcessor
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(logger *slog.Logger, metrics *metrics.Metrics, usernameMD5, passwordMD5 string, processor WebhookProcessor) *WebhookHandler {
	return &WebhookHandler{
		logger:      logger.With("component", "atlantic_webhook"),
		metrics:     metrics,
		usernameMD5: strings.ToLower(usernameMD5),
		passwordMD5: strings.ToLower(passwordMD5),
		processor:   processor,
	}
}

// ServeHTTP satisfies http.Handler.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.validateAuth(r); err != nil {
		h.metrics.Errors.WithLabelValues("atlantic_webhook_auth").Inc()
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.metrics.Errors.WithLabelValues("atlantic_webhook").Inc()
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	eventType := detectEventType(r.Header, body)
	headers := map[string]string{}
	for key, vals := range r.Header {
		if len(vals) > 0 {
			headers[key] = vals[0]
		}
	}

	event := WebhookEvent{
		Type:       eventType,
		Headers:    headers,
		Payload:    body,
		ReceivedAt: time.Now(),
	}

	if h.processor != nil {
		if err := h.processor.HandleAtlanticEvent(r.Context(), event); err != nil {
			h.logger.Error("failed processing webhook", "error", err, "event", eventType)
			h.metrics.Errors.WithLabelValues("atlantic_webhook_process").Inc()
			http.Error(w, "failed to process", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (h *WebhookHandler) validateAuth(r *http.Request) error {
	username, password, ok := r.BasicAuth()
	if !ok {
		if h.validateSignatureHeader(r) {
			return nil
		}
		return fmt.Errorf("missing basic auth")
	}

	if md5Hex(username) != h.usernameMD5 {
		return fmt.Errorf("invalid username hash")
	}
	if md5Hex(password) != h.passwordMD5 {
		return fmt.Errorf("invalid password hash")
	}
	return nil
}

func (h *WebhookHandler) validateSignatureHeader(r *http.Request) bool {
	signature := strings.TrimSpace(r.Header.Get("X-Atl-Signature"))
	if signature == "" {
		signature = strings.TrimSpace(r.Header.Get("X-Atlantic-Signature"))
	}
	if signature == "" {
		signature = strings.TrimSpace(r.Header.Get("X-Signature"))
	}
	if signature == "" {
		return false
	}
	signature = strings.ToLower(signature)
	return signature == h.usernameMD5 || signature == h.passwordMD5
}

func md5Hex(val string) string {
	sum := md5.Sum([]byte(val))
	return strings.ToLower(hex.EncodeToString(sum[:]))
}

func detectEventType(header http.Header, body []byte) string {
	for _, key := range []string{"X-Atlantic-Event", "X-Event-Type", "X-Event"} {
		if val := header.Get(key); val != "" {
			return val
		}
	}

	var generic struct {
		Type      string `json:"type"`
		Event     string `json:"event"`
		EventType string `json:"event_type"`
	}
	if err := json.Unmarshal(body, &generic); err == nil {
		if generic.EventType != "" {
			return generic.EventType
		}
		if generic.Type != "" {
			return generic.Type
		}
		if generic.Event != "" {
			return generic.Event
		}
	}
	return "unknown"
}
