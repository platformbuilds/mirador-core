package handlers

import (
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

type SessionHandler struct {
	logger logger.Logger
}

func NewSessionHandler(_ interface{}, l logger.Logger) *SessionHandler {
	// Session management has been removed from Mirador Core.
	// Handler exists only as a no-op placeholder to avoid breaking tests and existing server wiring.
	return &SessionHandler{logger: l}
}
