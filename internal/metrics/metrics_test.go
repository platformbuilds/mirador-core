package metrics

import "testing"

func TestMetrics_Increment(t *testing.T) {
    HTTPRequestsTotal.WithLabelValues("GET", "/health", "200", "t").Inc()
}

