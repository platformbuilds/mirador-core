package models

// StoreCorrelationRequest is the payload accepted by
// POST /api/v1/rca/store (RCAHandler.StoreCorrelation).
// Use the canonical types defined in models.go to avoid casts.
type StoreCorrelationRequest struct {
	CorrelationID string          `json:"correlationId" binding:"required"`
	IncidentID    string          `json:"incidentId"    binding:"required"`
	RootCause     string          `json:"rootCause"     binding:"required"`
	Confidence    float64         `json:"confidence"`
	RedAnchors    []*RedAnchor    `json:"redAnchors,omitempty"` // <-- matches CorrelationEvent.RedAnchors
	Timeline      []TimelineEvent `json:"timeline,omitempty"`   // <-- matches CorrelationEvent.Timeline
}
