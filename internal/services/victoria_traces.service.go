package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/platformbuilds/mirador-core/internal/utils"
)

// GetOperations returns all operations for a specific service from VictoriaTraces
func (s *VictoriaTracesService) GetOperations(ctx context.Context, serviceName, tenantID string) ([]string, error) {
	// Multi-endpoint aggregation when multiple endpoints configured in this service
	if func() bool { s.mu.Lock(); defer s.mu.Unlock(); return len(s.endpoints) > 1 }() {
		return s.getOperationsMultiEndpoint(ctx, serviceName, tenantID)
	}

	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services/%s/operations", endpoint, serviceName)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if utils.IsUint32String(tenantID) {
		req.Header.Set("AccountID", tenantID)
	}

	resp, err := s.doRequestWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VictoriaTraces returned status %d", resp.StatusCode)
	}

	// Some VT builds return {"data":[...]} while others may return a bare array.
	// Read body once and try both shapes for robustness.
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	b = bytes.TrimSpace(b)
	var wrap struct {
		Data []string `json:"data"`
	}
	if json.Unmarshal(b, &wrap) == nil && wrap.Data != nil {
		operations := make([]string, len(wrap.Data))
		copy(operations, wrap.Data)
		return operations, nil
	}
	var ops []string
	if json.Unmarshal(b, &ops) == nil {
		return ops, nil
	}
	return nil, fmt.Errorf("unexpected operations response shape: %s", string(b))
}

// getOperationsMultiEndpoint aggregates operations from all configured endpoints in this service
func (s *VictoriaTracesService) getOperationsMultiEndpoint(ctx context.Context, serviceName, tenantID string) ([]string, error) {
	// Get endpoints safely
	s.mu.Lock()
	endpoints := make([]string, len(s.endpoints))
	copy(endpoints, s.endpoints)
	s.mu.Unlock()

	if len(endpoints) == 0 {
		return nil, errors.New("no VictoriaTraces endpoints configured")
	}

	type out struct {
		operations []string
		err        error
	}
	ch := make(chan out, len(endpoints))
	for _, endpoint := range endpoints {
		go func(ep string) {
			tempSvc := &VictoriaTracesService{
				name:      s.name,
				endpoints: []string{ep}, // Single endpoint
				timeout:   s.timeout,
				client:    s.client,
				logger:    s.logger,
				username:  s.username,
				password:  s.password,
				retries:   s.retries,
				backoffMS: s.backoffMS,
			}
			ops, e := tempSvc.getOperationsSingleEndpoint(ctx, serviceName, tenantID)
			ch <- out{ops, e}
		}(endpoint)
	}

	// Aggregate results - collect all unique operations
	opSet := make(map[string]struct{})
	for i := 0; i < len(endpoints); i++ {
		r := <-ch
		if r.err != nil {
			s.logger.Warn("operations from endpoint failed", "error", r.err)
			continue
		}
		for _, op := range r.operations {
			opSet[op] = struct{}{}
		}
	}

	operations := make([]string, 0, len(opSet))
	for op := range opSet {
		operations = append(operations, op)
	}
	return operations, nil
}

// getOperationsSingleEndpoint gets operations from a single endpoint (used by multi-endpoint aggregation)
func (s *VictoriaTracesService) getOperationsSingleEndpoint(ctx context.Context, serviceName, tenantID string) ([]string, error) {
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services/%s/operations", endpoint, serviceName)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

	if utils.IsUint32String(tenantID) {
		req.Header.Set("AccountID", tenantID)
	}

	resp, err := s.doRequestWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VictoriaTraces returned status %d", resp.StatusCode)
	}

	// Some VT builds return {"data":[...]} while others may return a bare array.
	// Read body once and try both shapes for robustness.
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	b = bytes.TrimSpace(b)
	var wrap struct {
		Data []string `json:"data"`
	}
	if json.Unmarshal(b, &wrap) == nil && wrap.Data != nil {
		operations := make([]string, len(wrap.Data))
		copy(operations, wrap.Data)
		return operations, nil
	}
	var ops []string
	if json.Unmarshal(b, &ops) == nil {
		return ops, nil
	}
	return nil, fmt.Errorf("unexpected operations response shape: %s", string(b))
}

// readErrBodyVT reads and returns error message from response body
func readErrBodyVT(r io.Reader) string {
	const max = 64 * 1024
	b, _ := io.ReadAll(io.LimitReader(r, max))
	s := strings.TrimSpace(string(b))
	if s == "" {
		return ""
	}
	var m map[string]any
	if json.Unmarshal(b, &m) == nil {
		if msg, ok := m["error"].(string); ok && msg != "" {
			return msg
		}
		if msg, ok := m["message"].(string); ok && msg != "" {
			return msg
		}
	}
	return s
}

// doRequestWithRetry sends an HTTP request and retries on 5xx or transport errors.
// It logs every retry attempt to stdout via s.logger so operators can see timeouts/errors.
func (s *VictoriaTracesService) doRequestWithRetry(
	ctx context.Context,
	req *http.Request,
) (*http.Response, error) {

	var lastErr error
	backoff := time.Duration(s.backoffMS) * time.Millisecond

	for attempt := 1; attempt <= s.retries; attempt++ {
		// Clone the request for each attempt
		reqCopy := req.Clone(ctx)
		if s.username != "" && reqCopy.Header.Get("Authorization") == "" {
			reqCopy.SetBasicAuth(s.username, s.password)
		}

		resp, err := s.client.Do(reqCopy)
		// transport error (timeout, connection refused, etc.)
		if err != nil {
			lastErr = err
			s.logger.Warn("VictoriaTraces request failed (transport)",
				"attempt", attempt, "method", req.Method, "url", req.URL.String(), "error", err)
		} else if resp.StatusCode >= 500 {
			// server error -> retry
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, readErrBodyVT(resp.Body))
			_ = resp.Body.Close()
			s.logger.Warn("VictoriaTraces 5xx response — retrying",
				"attempt", attempt, "method", req.Method, "url", req.URL.String(), "status", resp.StatusCode)
		} else {
			// success or non-retryable status
			return resp, nil
		}

		// no more retries?
		if attempt == s.retries || ctx.Err() != nil {
			break
		}

		// backoff (exponential)
		select {
		case <-time.After(backoff):
			backoff *= 2
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Final log with summary so it's visible in stdout
	s.logger.Error("VictoriaTraces request exhausted retries",
		"method", req.Method, "url", req.URL.String(), "retries", s.retries, "error", lastErr)
	return nil, lastErr
}
