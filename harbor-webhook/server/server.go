package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/lmhinnel/kubecon/harbor-webhook/harbor"
	"github.com/lmhinnel/kubecon/harbor-webhook/signer"
)

// Config holds HTTP server configuration.
type Config struct {
	Port          string
	WebhookSecret string
}

// Server is the HTTP server.
type Server struct {
	cfg    Config
	signer *signer.Signer
	mux    *http.ServeMux
}

// New creates a Server with routes registered.
func New(cfg Config, s *signer.Signer) *Server {
	srv := &Server{cfg: cfg, signer: s}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /webhook", srv.handleWebhook)
	mux.HandleFunc("GET /healthz", srv.handleHealthz)
	srv.mux = mux
	return srv
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	addr := fmt.Sprintf(":%s", s.cfg.Port)
	slog.Info("http server listening", "addr", addr)
	return http.ListenAndServe(addr, s.mux)
}

// ── Handlers ──────────────────────────────────────────────────────────────────

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	if s.cfg.WebhookSecret != "" {
		if r.Header.Get("X-Harbor-Webhook-Token") != s.cfg.WebhookSecret {
			slog.Warn("invalid webhook token", "remote", r.RemoteAddr)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	var hook harbor.Webhook
	if err := json.Unmarshal(body, &hook); err != nil {
		slog.Error("json decode failed", "err", err)
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	// ── Guard 1: only handle scan completion events ───────────────────────────
	if hook.Type != "SCANNING_COMPLETED" {
		slog.Debug("ignoring event", "type", hook.Type)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// ── Guard 2: loop prevention — drop events triggered by our own robot ─────
	// When cosign pushes the .sig tag, Harbor may re-scan it and fire another
	// SCANNING_COMPLETED. The operator field will be our robot account name.
	// Dropping these events here breaks the infinite loop.
	if s.signer.IsOperatorSkipped(hook.Operator) {
		slog.Info("dropping event from skip-listed operator (loop prevention)",
			"operator", hook.Operator,
			"type", hook.Type,
		)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Acknowledge immediately — cosign sign can take several seconds.
	go s.processScanEvent(r.Context(), hook)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	fmt.Fprintln(w, `{"status":"accepted"}`)
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"status":"ok","time":"%s"}`, time.Now().UTC().Format(time.RFC3339))
}

// ── Event processing ──────────────────────────────────────────────────────────

func (s *Server) processScanEvent(ctx context.Context, hook harbor.Webhook) {
	for _, resource := range hook.EventData.Resources {
		for _, overview := range resource.ScanOverview {
			s.handleResource(ctx, hook.Operator, resource, overview)
		}
	}
}

func (s *Server) handleResource(
	ctx context.Context,
	operator string,
	resource harbor.Resource,
	overview harbor.ScanOverview,
) {
	log := slog.With(
		"image", resource.ResourceURL,
		"digest", resource.Digest,
		"tag", resource.Tag,
		"operator", operator,
		"severity", overview.Severity,
		"scan_status", overview.ScanStatus,
		"total_vulns", overview.Summary.Total,
		"fixable", overview.Summary.Fixable,
		"scanner", overview.Scanner.Name,
		"scanner_version", overview.Scanner.Version,
	)

	log.Info("evaluating scan result")

	// ── Guard 3: scan must have succeeded ─────────────────────────────────────
	if overview.ScanStatus != "Success" {
		log.Warn("scan did not succeed — skipping", "status", overview.ScanStatus)
		return
	}

	// ── Guard 4: severity must be in allow-list ────────────────────────────────
	if !s.signer.IsSeverityAllowed(overview.Severity) {
		log.Warn("severity not allowed — image will NOT be signed",
			"severity", overview.Severity,
			"allowed", s.signer.AllowedSeverities(),
		)
		return
	}

	// Always sign by digest — never by mutable tag.
	imageRef := fmt.Sprintf("%s@%s", resource.ResourceURL, resource.Digest)

	if err := s.signer.SignImage(ctx, imageRef); err != nil {
		log.Error("signing failed", "ref", imageRef, "err", err)
		return
	}

	log.Info("image signed and pushed", "ref", imageRef)
}
