package weavstore

import (
	"reflect"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/models"
)

func TestThresholdsConversionRoundTrip(t *testing.T) {
	in := []models.Threshold{
		{Level: "warning", Operator: "gt", Value: 10.5, Description: "warn"},
		{Level: "critical", Operator: "gt", Value: 20.0, Description: "critical"},
	}

	props := thresholdsToProps(in)
	if props == nil || len(props) != len(in) {
		t.Fatalf("expected props length %d got %d", len(in), len(props))
	}

	out := propsToThresholds(props)
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("roundtrip mismatch:\nexpected: %+v\nactual:   %+v", in, out)
	}
}

func TestPropsToThresholdsHandlesInterfaceSlice(t *testing.T) {
	// Simulate the SDK decoding into []interface{} of map[string]interface{}
	raw := make([]interface{}, 2)
	raw[0] = map[string]interface{}{"severity": "warn", "operator": "lt", "value": 5.0, "message": "low"}
	raw[1] = map[string]interface{}{"severity": "crit", "operator": "gt", "value": 15.5, "message": "high"}

	out := propsToThresholds(raw)
	if len(out) != 2 {
		t.Fatalf("expected 2 thresholds, got %d", len(out))
	}
	if out[0].Level != "warn" || out[1].Level != "crit" {
		t.Fatalf("unexpected severity values: %+v", out)
	}
}
