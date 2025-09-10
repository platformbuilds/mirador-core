package weaviate

import (
    "context"
    "net/http"
    "net/http/httptest"
    "testing"
    "time"
)

func TestReady_OK(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/v1/.well-known/ready" { w.WriteHeader(200); return }
        w.WriteHeader(404)
    }))
    defer ts.Close()
    c := &Client{BaseURL: ts.URL, HTTP: &http.Client{Timeout: 2 * time.Second}}
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    if err := c.Ready(ctx); err != nil { t.Fatalf("expected ready, got %v", err) }
}

func TestReady_Fail(t *testing.T) {
    ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path == "/v1/.well-known/ready" { w.WriteHeader(503); return }
        w.WriteHeader(404)
    }))
    defer ts.Close()
    c := &Client{BaseURL: ts.URL, HTTP: &http.Client{Timeout: 2 * time.Second}}
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    if err := c.Ready(ctx); err == nil { t.Fatalf("expected not ready error") }
}

