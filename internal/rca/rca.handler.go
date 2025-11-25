package rca

import (
	"context"
	"net/http"

	"github.com/platformbuilds/mirador-core/internal/logging"
	corelogger "github.com/platformbuilds/mirador-core/pkg/logger"
)

// RCAHandler provides HTTP handlers for RCA endpoints.
type RCAHandler struct {
	service RCAService // delegate to service layer
	logger  logging.Logger
}

func NewRCAHandler(service RCAService, logger corelogger.Logger) *RCAHandler {
	// Accept the project's core logger at construction (keeps call-sites
	// unchanged) but store the internal facade so handler code depends only
	// on `internal/logging`.
	return &RCAHandler{service: service, logger: logging.FromCoreLogger(logger)}
}

// GetActiveCorrelations returns currently active correlations.
func (h *RCAHandler) GetActiveCorrelations(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// NOTE(HCB-007): Handler delegates to service layer per standard architecture pattern.
	result, err := h.service.ListActiveCorrelations(ctx)
	if err != nil {
		h.logger.Error("Failed to get active correlations", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if _, werr := w.Write([]byte("Internal error")); werr != nil {
			h.logger.Error("failed to write response", "error", werr)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write([]byte(result)); werr != nil {
		h.logger.Error("failed to write response", "error", werr)
	} // result should be JSON
}

// GetFailurePatterns returns detected failure patterns.
func (h *RCAHandler) GetFailurePatterns(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// NOTE(HCB-007): Handler delegates to service layer per standard architecture pattern.
	patterns, err := h.service.ListFailurePatterns(ctx)
	if err != nil {
		h.logger.Error("Failed to get failure patterns", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if _, werr := w.Write([]byte("Internal error")); werr != nil {
			h.logger.Error("failed to write response", "error", werr)
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, werr := w.Write([]byte(patterns)); werr != nil {
		h.logger.Error("failed to write response", "error", werr)
	} // patterns should be JSON
}

// StoreCorrelation stores a new correlation result.
func (h *RCAHandler) StoreCorrelation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// NOTE(HCB-007): Request body parsing happens in service layer for better testability.
	if err := h.service.StoreCorrelation(ctx, r.Body); err != nil {
		h.logger.Error("Failed to store correlation", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		if _, werr := w.Write([]byte("Internal error")); werr != nil {
			h.logger.Error("failed to write response", "error", werr)
		}
		return
	}
	w.WriteHeader(http.StatusCreated)
	if _, werr := w.Write([]byte("Correlation stored")); werr != nil {
		h.logger.Error("failed to write response", "error", werr)
	}
}

// RCAService defines the service interface for RCA operations.
type RCAService interface {
	ListActiveCorrelations(ctx context.Context) (string, error)
	ListFailurePatterns(ctx context.Context) (string, error)
	StoreCorrelation(ctx context.Context, body interface{}) error
}
