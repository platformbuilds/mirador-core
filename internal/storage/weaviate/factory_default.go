package weaviate

import (
	"context"
	"encoding/json"
	"fmt"

	wv "github.com/weaviate/weaviate-go-client/v5/weaviate"

	"github.com/platformbuilds/mirador-core/internal/config"
	"github.com/platformbuilds/mirador-core/pkg/logger"
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
	// Prefer using the official v5 client for GraphQL requests so we centralize
	// SDK usage here. Keep variables support and decode into `out`.
	if o.client != nil {
		// Use Raw() to execute arbitrary GraphQL queries. The SDK decodes JSON
		// into the provided `out` value. Callers that relied on variables should
		// inline variables into the query string (existing code often does so).
		resp, err := o.client.GraphQL().Raw().WithQuery(query).Do(ctx)
		if err != nil {
			return err
		}
		// Marshal the generic response and unmarshal into out to preserve the
		// previous behavior of decoding into the provided out value.
		b, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if out != nil {
			if err := json.Unmarshal(b, out); err != nil {
				return err
			}
		}
		return nil
	}
	// Fallback to HTTP transport if client missing
	return o.httpT.GraphQL(ctx, query, variables, out)
}

func (o *officialTransport) DeleteObject(ctx context.Context, id string) error {
	return o.httpT.DeleteObject(ctx, id)
}

// NewTransportFromConfig returns the official transport (non-optional).
func NewTransportFromConfig(cfg config.WeaviateConfig, logger logger.Logger) (Transport, error) {
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
	// Set logger on the client for health checks
	if httpClient, ok := httpT.(*httpTransport); ok && httpClient.c != nil {
		httpClient.c.SetLogger(logger)
	}
	return &officialTransport{client: client, httpT: httpT}, nil
}

// Ready wraps transport.Ready for convenience.
func Ready(ctx context.Context, t Transport) error { return t.Ready(ctx) }
