package models

import (
	"fmt"
	"time"
)

// TimeWindowRequest is the canonical public API request for time-window-only
// endpoints. It is intentionally minimal and will be the long-term public
// contract for correlation/RCA HTTP endpoints.
type TimeWindowRequest struct {
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// ToTimeRange parses the StartTime and EndTime fields (RFC3339) and returns
// a models.TimeRange. Returns an error if parsing fails or if End <= Start.
func (t *TimeWindowRequest) ToTimeRange() (TimeRange, error) {
	if t == nil {
		return TimeRange{}, fmt.Errorf("time window request is nil")
	}
	if t.StartTime == "" || t.EndTime == "" {
		return TimeRange{}, fmt.Errorf("both startTime and endTime are required")
	}

	ts, err := time.Parse(time.RFC3339, t.StartTime)
	if err != nil {
		return TimeRange{}, fmt.Errorf("invalid startTime format: %w", err)
	}
	te, err := time.Parse(time.RFC3339, t.EndTime)
	if err != nil {
		return TimeRange{}, fmt.Errorf("invalid endTime format: %w", err)
	}
	if !ts.Before(te) {
		return TimeRange{}, fmt.Errorf("endTime must be after startTime")
	}

	return TimeRange{Start: ts, End: te}, nil
}
