package monitoring

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func Test_RecordWeaviateSchemaMissing_IncrementsCounter(t *testing.T) {
	// Reset is not straightforward; rely on a fresh test process for deterministic value.
	// Call the recorder and check the counter increments at least once.
	RecordWeaviateSchemaMissing("kpi_definition")

	// Validate the counter for the label
	v := testutil.ToFloat64(weaviateSchemaMissingTotal.WithLabelValues("kpi_definition"))
	if v < 1.0 {
		t.Fatalf("expected weaviate schema missing counter >= 1; got %f", v)
	}
}
