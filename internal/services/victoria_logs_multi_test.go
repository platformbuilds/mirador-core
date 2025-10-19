package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/internal/models"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// fakeVL returns a server that responds to /select/logsql/query with a wrapped JSON format
func fakeVL(t *testing.T, fields []string, rows [][]any) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/select/logsql/query", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := struct {
			Status string   `json:"status"`
			Fields []string `json:"fields"`
			Data   [][]any  `json:"data"`
		}{Status: "success", Fields: fields, Data: rows}
		_ = json.NewEncoder(w).Encode(resp)
	})
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	return httptest.NewServer(mux)
}

func TestLogs_Aggregated_ExecuteQuery_ConcatsAndUnions(t *testing.T) {
	a := fakeVL(t, []string{"ts", "msg", "service"}, [][]any{{1, "a", "svc-a"}})
	defer a.Close()
	b := fakeVL(t, []string{"ts", "msg", "host"}, [][]any{{2, "b", "host-1"}})
	defer b.Close()

	log := logger.New("error")
	parent := NewVictoriaLogsService(config.VictoriaLogsConfig{Name: "parent"}, log)
	childA := NewVictoriaLogsService(config.VictoriaLogsConfig{Name: "fin_logs", Endpoints: []string{a.URL}, Timeout: 2000}, log)
	childB := NewVictoriaLogsService(config.VictoriaLogsConfig{Name: "os_logs", Endpoints: []string{b.URL}, Timeout: 2000}, log)
	parent.SetChildren([]*VictoriaLogsService{childA, childB})

	res, err := parent.ExecuteQuery(context.Background(), &models.LogsQLQueryRequest{Query: "*", Limit: 10})
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	if len(res.Logs) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(res.Logs))
	}
	// union of fields should include service and host
	has := func(k string) bool {
		for _, f := range res.Fields {
			if f == k {
				return true
			}
		}
		return false
	}
	if !has("service") || !has("host") {
		t.Fatalf("expected union fields include service and host: %v", res.Fields)
	}
}

func TestLogs_Aggregated_GetFields_Union(t *testing.T) {
	a := fakeVL(t, []string{"a", "b"}, [][]any{{1, 2}})
	defer a.Close()
	b := fakeVL(t, []string{"b", "c"}, [][]any{{3, 4}})
	defer b.Close()
	log := logger.New("error")
	parent := NewVictoriaLogsService(config.VictoriaLogsConfig{Name: "parent"}, log)
	parent.SetChildren([]*VictoriaLogsService{
		NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{a.URL}, Timeout: 2000}, log),
		NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{b.URL}, Timeout: 2000}, log),
	})
	fields, err := parent.GetFields(context.Background(), "")
	if err != nil {
		t.Fatalf("GetFields: %v", err)
	}
	if len(fields) != 3 {
		t.Fatalf("expected union size 3, got %d (%v)", len(fields), fields)
	}
}

func TestLogs_Aggregated_GetStreams_Union(t *testing.T) {
	a := fakeVL(t, []string{"service"}, [][]any{{1, "a"}})
	defer a.Close()
	b := fakeVL(t, []string{"host"}, [][]any{{2, "h"}})
	defer b.Close()
	log := logger.New("error")
	parent := NewVictoriaLogsService(config.VictoriaLogsConfig{Name: "parent"}, log)
	parent.SetChildren([]*VictoriaLogsService{
		NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{a.URL}, Timeout: 2000}, log),
		NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{b.URL}, Timeout: 2000}, log),
	})
	streams, err := parent.GetStreams(context.Background(), "", 10)
	if err != nil {
		t.Fatalf("GetStreams: %v", err)
	}
	if len(streams) < 2 {
		t.Fatalf("expected at least 2 stream labels, got %d (%v)", len(streams), streams)
	}
}
