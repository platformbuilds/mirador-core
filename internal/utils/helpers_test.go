package utils

import (
	"testing"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
)

func TestGenerateIDs(t *testing.T) {
	if GenerateSessionID() == "" {
		t.Fatalf("empty session id")
	}
	if GenerateClientID() == "" {
		t.Fatalf("empty client id")
	}
}

func TestCountHelpers(t *testing.T) {
	data := map[string]interface{}{"result": []interface{}{1, 2, 3}}
	if CountSeries(data) != 3 {
		t.Fatalf("series count failed")
	}
	rangeData := map[string]interface{}{"result": []interface{}{
		map[string]interface{}{"values": []interface{}{1, 2}},
		map[string]interface{}{"values": []interface{}{3}},
	}}
	if CountDataPoints(rangeData) != 3 {
		t.Fatalf("datapoints failed")
	}
}

func TestFilterAndCalc(t *testing.T) {
	fr := []*models.SystemFracture{
		{Severity: "high", TimeToFracture: time.Second},
		{Severity: "low", TimeToFracture: 2 * time.Second},
		{Severity: "high", TimeToFracture: 3 * time.Second},
	}
	hi := FilterByRisk(fr, "high")
	if len(hi) != 2 {
		t.Fatalf("filter failed")
	}
	if CalculateAvgTimeToFailure(hi) != 2*time.Second {
		t.Fatalf("avg failed")
	}
}

func TestMisc(t *testing.T) {
	if CountAlertsBySeverity([]*models.Alert{{Severity: "s"}, {Severity: "x"}, {Severity: "s"}}, "s") != 2 {
		t.Fatalf("count alerts")
	}
	if !Contains([]string{"a", "b"}, "a") || Contains([]string{"a"}, "z") {
		t.Fatalf("contains")
	}
	names := GetStreamNames(map[string]bool{"m": true, "x": false})
	if len(names) != 1 || names[0] != "m" {
		t.Fatalf("streams")
	}
	if !IsUint32String("123") || IsUint32String("-1") || IsUint32String("abc") {
		t.Fatalf("uint32 parse")
	}
}
