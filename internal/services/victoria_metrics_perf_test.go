package services

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// Performance Test Cases: benchmark ExecuteQuery hot path against a local server
func BenchmarkVictoriaMetrics_ExecuteQuery(b *testing.B) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/query", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"status":"success","data":{"result":[{"metric":{"__name__":"up"},"value":[1,2]}]}}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	ts := httptest.NewServer(mux)
	defer ts.Close()

	log := logger.New("error")
	svc := NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{ts.URL}, Timeout: 2000}, log)
	ctx := context.Background()
	req := &models.MetricsQLQueryRequest{Query: "up"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := svc.ExecuteQuery(ctx, req)
		if err != nil {
			b.Fatalf("err: %v", err)
		}
	}
}
