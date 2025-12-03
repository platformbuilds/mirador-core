package utils

import (
	"fmt"

	"github.com/alpkeskin/gotoon"
	"github.com/platformbuilds/mirador-core/internal/models"
)

// ConvertRCAToTOON converts an RCA response JSON structure to TOON format.
// TOON (Token Oriented Object Notation) is more LLM-friendly than JSON,
// providing better structural cues and reducing token count.
func ConvertRCAToTOON(rca *models.RCAResponse) (string, error) {
	if rca == nil {
		return "", fmt.Errorf("RCA response is nil")
	}

	// Convert to TOON using gotoon library
	toon, err := gotoon.Encode(rca, gotoon.WithIndent(2), gotoon.WithLengthMarker())
	if err != nil {
		return "", fmt.Errorf("TOON conversion failed: %w", err)
	}

	return toon, nil
}

// ConvertRCADataToTOON converts just the RCA data payload (excluding wrapper).
// This is useful when you want only the core RCA content without status/timestamp.
func ConvertRCADataToTOON(rcaData interface{}) (string, error) {
	if rcaData == nil {
		return "", fmt.Errorf("RCA data is nil")
	}

	toon, err := gotoon.Encode(rcaData, gotoon.WithIndent(2), gotoon.WithLengthMarker())
	if err != nil {
		return "", fmt.Errorf("TOON conversion failed: %w", err)
	}

	return toon, nil
}

// ValidateRCAResponse validates that the RCA response has required fields.
// This ensures schema checking before TOON conversion.
func ValidateRCAResponse(rca *models.RCAResponse) error {
	if rca == nil {
		return fmt.Errorf("RCA response is nil")
	}

	// Validate required fields
	if rca.Status == "" {
		return fmt.Errorf("missing required field: status")
	}
	if rca.Data == nil {
		return fmt.Errorf("missing required field: data")
	}

	return nil
}
