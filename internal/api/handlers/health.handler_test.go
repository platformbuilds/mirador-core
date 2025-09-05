package handlers

import (
    "context"
    "net"
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/gin-gonic/gin"
    "github.com/platformbuilds/miradorstack/internal/grpc/clients"
    alertpb "github.com/platformbuilds/miradorstack/internal/grpc/proto/alert"
    "github.com/platformbuilds/miradorstack/internal/config"
    "github.com/platformbuilds/miradorstack/internal/services"
    "github.com/platformbuilds/miradorstack/internal/models"
    "github.com/platformbuilds/miradorstack/pkg/logger"
    "github.com/stretchr/testify/assert"
    "google.golang.org/grpc"
)

// ---- Fake Predict/RCA clients ---------------------------------------------

type okPredict struct{}

func (okPredict) AnalyzeFractures(ctx context.Context, _ *models.FractureAnalysisRequest) (*models.FractureAnalysisResponse, error) {
    return &models.FractureAnalysisResponse{}, nil
}
func (okPredict) GetActiveModels(ctx context.Context, _ *models.ActiveModelsRequest) (*models.ActiveModelsResponse, error) {
    return &models.ActiveModelsResponse{}, nil
}
func (okPredict) HealthCheck() error { return nil }

type okRCA struct{}

func (okRCA) InvestigateIncident(ctx context.Context, _ *models.RCAInvestigationRequest) (*models.CorrelationResult, error) {
    return &models.CorrelationResult{}, nil
}
func (okRCA) HealthCheck() error { return nil }

// ---- gRPC Alert server stub ------------------------------------------------

type alertServer struct{
    alertpb.UnimplementedAlertEngineServiceServer
}

func (s *alertServer) GetHealth(ctx context.Context, _ *alertpb.GetHealthRequest) (*alertpb.GetHealthResponse, error) {
    return &alertpb.GetHealthResponse{Status: "ok"}, nil
}

func startAlertGRPC(t *testing.T) (addr string, shutdown func()) {
    t.Helper()
    lis, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil {
        t.Fatalf("listen: %v", err)
    }
    s := grpc.NewServer()
    alertpb.RegisterAlertEngineServiceServer(s, &alertServer{})
    go s.Serve(lis)
    return lis.Addr().String(), func() { s.Stop(); _ = lis.Close() }
}

func newOKHTTPServer() *httptest.Server {
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/health" {
            w.WriteHeader(http.StatusOK)
            _, _ = w.Write([]byte("ok"))
            return
        }
        http.NotFound(w, r)
    }))
}

func TestHealthHandler_ReadinessCheck_AllHealthy(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // Start fake upstreams
    m := newOKHTTPServer(); defer m.Close()
    l := newOKHTTPServer(); defer l.Close()
    tr := newOKHTTPServer(); defer tr.Close()

    addr, stop := startAlertGRPC(t); defer stop()

    // Build services
    metricsSvc := services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{m.URL}, Timeout: 500}, logger.New("error"))
    logsSvc := services.NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{l.URL}, Timeout: 500}, logger.New("error"))
    tracesSvc := services.NewVictoriaTracesService(config.VictoriaTracesConfig{Endpoints: []string{tr.URL}, Timeout: 500}, logger.New("error"))
    vm := &services.VictoriaMetricsServices{Metrics: metricsSvc, Logs: logsSvc, Traces: tracesSvc}

    // Build gRPC clients bundle
    alertClient, err := clients.NewAlertEngineClient(addr, logger.New("error"))
    if err != nil { t.Fatalf("alert client: %v", err) }
    gc := &clients.GRPCClients{
        PredictEngine: okPredict{},
        RCAEngine:     okRCA{},
        AlertEngine:   alertClient,
        // logger omitted
    }

    h := NewHealthHandler(gc, vm, logger.New("error"))

    // Exercise
    req := httptest.NewRequest(http.MethodGet, "/ready", nil)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    h.ReadinessCheck(c)

    // Assert
    assert.Equal(t, http.StatusOK, w.Code)
}

func TestHealthHandler_ReadinessCheck_UnhealthyWhenMetricsFails(t *testing.T) {
    gin.SetMode(gin.TestMode)

    // Metrics server returns 500 on /health
    m := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/health" { w.WriteHeader(http.StatusInternalServerError); return }
        http.NotFound(w, r)
    }))
    defer m.Close()

    // Logs/Traces healthy
    l := newOKHTTPServer(); defer l.Close()
    tr := newOKHTTPServer(); defer tr.Close()

    addr, stop := startAlertGRPC(t); defer stop()

    metricsSvc := services.NewVictoriaMetricsService(config.VictoriaMetricsConfig{Endpoints: []string{m.URL}, Timeout: 500}, logger.New("error"))
    logsSvc := services.NewVictoriaLogsService(config.VictoriaLogsConfig{Endpoints: []string{l.URL}, Timeout: 500}, logger.New("error"))
    tracesSvc := services.NewVictoriaTracesService(config.VictoriaTracesConfig{Endpoints: []string{tr.URL}, Timeout: 500}, logger.New("error"))
    vm := &services.VictoriaMetricsServices{Metrics: metricsSvc, Logs: logsSvc, Traces: tracesSvc}

    alertClient, err := clients.NewAlertEngineClient(addr, logger.New("error"))
    if err != nil { t.Fatalf("alert client: %v", err) }
    gc := &clients.GRPCClients{PredictEngine: okPredict{}, RCAEngine: okRCA{}, AlertEngine: alertClient}

    h := NewHealthHandler(gc, vm, logger.New("error"))

    req := httptest.NewRequest(http.MethodGet, "/ready", nil)
    w := httptest.NewRecorder()
    c, _ := gin.CreateTestContext(w)
    c.Request = req
    h.ReadinessCheck(c)

    // Expect 503 due to metrics unhealthy
    if w.Code != http.StatusServiceUnavailable { t.Fatalf("expected 503, got %d", w.Code) }
}
