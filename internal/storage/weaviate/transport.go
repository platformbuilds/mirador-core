package weaviate

import (
    "context"
)

// Transport is an abstraction over Weaviate access. It can be backed by the
// lightweight HTTP client or the official weaviate-go-client.
type Transport interface {
    // Ready should succeed when the server is ready to accept requests.
    Ready(ctx context.Context) error
    // EnsureClasses creates the provided class definitions if they don't exist.
    // Each element must be an object compatible with POST /v1/schema body.
    EnsureClasses(ctx context.Context, classDefs []map[string]any) error
    // PutObject upserts an object (by class + id) with the provided properties.
    PutObject(ctx context.Context, class, id string, props map[string]any) error
    // GraphQL executes a GraphQL POST /v1/graphql with variables, decoding into out.
    GraphQL(ctx context.Context, query string, variables map[string]any, out any) error
}

