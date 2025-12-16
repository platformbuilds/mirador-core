package intent

import "testing"

func TestDetectIntent_AppHealth(t *testing.T) {
	cases := []struct {
		msg  string
		want CapabilityID
	}{
		{"How is checkout doing?", APP_HEALTH_OVERVIEW},
		{"health for payments", APP_HEALTH_OVERVIEW},
		{"is anything failing?", PERF_DETECT_FAILURES},
		{"run rca for the last hour", RCA_PERFORM},
		{"show kpi http_errors_total", KPI_SEARCH},
		{"tell me a joke", GENERAL_CHAT},
	}

	for _, c := range cases {
		ir, err := DetectIntent(c.msg)
		if err != nil {
			t.Fatalf("DetectIntent(%q) returned error: %v", c.msg, err)
		}
		if ir.CapabilityID != c.want {
			t.Fatalf("DetectIntent(%q) = %v; want %v", c.msg, ir.CapabilityID, c.want)
		}
	}
}
