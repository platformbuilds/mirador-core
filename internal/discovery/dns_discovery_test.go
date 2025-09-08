package discovery

import "testing"

func TestResolveEndpoints_EmptyService(t *testing.T) {
    eps := resolveEndpoints(DNSConfig{Service: "", Port: 0, Scheme: "http"})
    if len(eps) != 0 { t.Fatalf("expected no endpoints for empty service") }
}

