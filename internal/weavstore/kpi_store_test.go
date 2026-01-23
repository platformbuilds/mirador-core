package weavstore

import (
	"reflect"
	"testing"
)

func TestThresholdsConversionRoundTrip(t *testing.T) {
	in := []Threshold{
		{Level: "warning", Operator: "gt", Value: 10.5, Description: "warn"},
		{Level: "critical", Operator: "gt", Value: 20.0, Description: "critical"},
	}

	// thresholdsToProps now returns a JSON string
	jsonStr := thresholdsToProps(in)
	if jsonStr == "" {
		t.Fatal("expected non-empty JSON string")
	}

	// propsToThresholds expects a string (JSON)
	out := propsToThresholds(jsonStr)
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
