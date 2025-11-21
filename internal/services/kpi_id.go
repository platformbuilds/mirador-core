package services

import (
	"fmt"
	"strings"

	"github.com/gofrs/uuid/v5"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// KPIIDNamespace is the constant namespace UUID used to generate deterministic KPI IDs.
// This should remain constant for all servers so that the same canonical key always
// yields the same UUID. If parsing fails, KPIIDNamespace will be zero value.
var KPIIDNamespace = func() uuid.UUID {
	u, err := uuid.FromString("f47ac10b-58cc-4372-a567-0e02b2c3d479")
	if err != nil {
		return uuid.UUID{}
	}
	return u
}()

// GenerateDeterministicKPIID builds a canonical key for the given KPI definition
// and returns a deterministic UUID derived from that key.
func GenerateDeterministicKPIID(k *models.KPIDefinition) (string, error) {
	if k == nil {
		return "", fmt.Errorf("kpi definition is nil")
	}

	// Normalize helper
	norm := func(s string) string {
		return strings.ToLower(strings.TrimSpace(s))
	}

	var canonical string
	if norm(k.Source) != "" && norm(k.SourceID) != "" {
		canonical = fmt.Sprintf("KPIDefinition|source=%s|sourceId=%s", norm(k.Source), norm(k.SourceID))
	} else if norm(k.Namespace) != "" {
		canonical = fmt.Sprintf("KPIDefinition|namespace=%s|name=%s", norm(k.Namespace), norm(k.Name))
	} else {
		canonical = fmt.Sprintf("KPIDefinition|name=%s", norm(k.Name))
	}

	// Use UUID v5 (SHA-1) deterministic generation
	id := uuid.NewV5(KPIIDNamespace, canonical)
	return id.String(), nil
}
