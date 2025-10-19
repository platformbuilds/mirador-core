package models

import (
	"strconv"
	"strings"
	"time"
)

// FlexibleTime unmarshals either RFC3339/RFC3339Nano strings
// or epoch values in seconds, milliseconds, or microseconds.
type FlexibleTime struct{ time.Time }

func (ft *FlexibleTime) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" || s == "" {
		ft.Time = time.Time{}
		return nil
	}
	// String input
	if len(s) > 0 && (s[0] == '"' && s[len(s)-1] == '"') {
		val := strings.Trim(s, "\"")
		// Try RFC3339 / RFC3339Nano
		if t, err := time.Parse(time.RFC3339Nano, val); err == nil {
			ft.Time = t
			return nil
		}
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			ft.Time = t
			return nil
		}
		// Try numeric inside string
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			ft.Time = fromFlexibleEpoch(n)
			return nil
		}
		// Fallback: empty
		ft.Time = time.Time{}
		return nil
	}
	// Numeric input
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		ft.Time = fromFlexibleEpoch(n)
		return nil
	}
	// Unknown shape
	ft.Time = time.Time{}
	return nil
}

func fromFlexibleEpoch(n int64) time.Time {
	switch {
	case n >= 1_000_000_000_000_000: // >= 1e15: microseconds
		return time.UnixMicro(n)
	case n >= 1_000_000_000_000: // >= 1e12: milliseconds
		return time.UnixMilli(n)
	case n >= 1_000_000_000: // >= 1e9: seconds
		return time.Unix(n, 0)
	default:
		// treat as seconds
		return time.Unix(n, 0)
	}
}

func (ft FlexibleTime) IsZero() bool      { return ft.Time.IsZero() }
func (ft FlexibleTime) AsTime() time.Time { return ft.Time }
