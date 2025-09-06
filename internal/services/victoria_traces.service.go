package services

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"

    "github.com/platformbuilds/mirador-core/internal/utils"
)

// GetOperations returns all operations for a specific service from VictoriaTraces
func (s *VictoriaTracesService) GetOperations(ctx context.Context, serviceName, tenantID string) ([]string, error) {
	endpoint := s.selectEndpoint()
	fullURL := fmt.Sprintf("%s/select/jaeger/api/services/%s/operations", endpoint, serviceName)

	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, err
	}

    if utils.IsUint32String(tenantID) {
        req.Header.Set("AccountID", tenantID)
    }

	resp, err := s.client.Do(req)
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
    if err != nil { return nil, err }
    b = bytes.TrimSpace(b)
    var wrap struct{ Data []string `json:"data"` }
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
