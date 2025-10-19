package weaviate

import (
	"context"
	"fmt"

	"github.com/platformbuilds/mirador-core/internal/config"
	wv "github.com/weaviate/weaviate-go-client/v5/weaviate"
)

type officialTransport struct {
	client *wv.Client
	httpT  Transport
}

func (o *officialTransport) GetSchema(ctx context.Context, out any) error {
	// Delegate to lightweight HTTP transport for schema operations
	return o.httpT.GetSchema(ctx, out)
}

func (o *officialTransport) Ready(ctx context.Context) error { return o.httpT.Ready(ctx) }

func (o *officialTransport) EnsureClasses(ctx context.Context, classDefs []map[string]any) error {
	// Use lightweight HTTP transport to create raw class definitions
	return o.httpT.EnsureClasses(ctx, classDefs)
}

func (o *officialTransport) PutObject(ctx context.Context, class, id string, props map[string]any) error {
	// Use lightweight HTTP transport for generic upsert semantics
	return o.httpT.PutObject(ctx, class, id, props)
}

func (o *officialTransport) GraphQL(ctx context.Context, query string, variables map[string]any, out any) error {
	// Delegate to lightweight HTTP transport for maximum compatibility
	return o.httpT.GraphQL(ctx, query, variables, out)
}

func (o *officialTransport) DeleteObject(ctx context.Context, id string) error {
	return o.httpT.DeleteObject(ctx, id)
}

// NewTransportFromConfig returns the official transport (non-optional).
func NewTransportFromConfig(cfg config.WeaviateConfig) (Transport, error) {
	hostPort := cfg.Host
	if cfg.Port != 0 {
		hostPort = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}
	conf := wv.Config{Scheme: cfg.Scheme, Host: hostPort}
	client, err := wv.NewClient(conf)
	if err != nil {
		return nil, err
	}
	// Build an HTTP transport using the same config for raw operations
	httpT := NewHTTPTransport(New(cfg))
	return &officialTransport{client: client, httpT: httpT}, nil
}

// Ready wraps transport.Ready for convenience.
func Ready(ctx context.Context, t Transport) error { return t.Ready(ctx) }
