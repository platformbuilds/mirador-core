# UUID to Human-Readable Name Resolution - Implementation Summary

**Date:** 25 November 2025  
**Status:** ✅ Implemented and Tested

## Problem Statement

The RCA API endpoints (`POST /api/v1/unified/rca`) were returning KPI UUIDs instead of human-readable names in the following JSON fields:

- `impact.impactService`
- `impact.metricName`
- `rootCause.service`
- `rootCause.component`
- `chains[].steps[].service`
- `chains[].steps[].component`
- `chains[].impactPath[]`

This made the API response difficult to read and understand without additional lookups.

## Root Cause Analysis

The issue originated in the **Correlation Engine** where `RedAnchor` objects are populated:

1. **Correlation Engine** (`internal/services/correlation_engine.go:386-397`)
   - Discovers KPIs from the KPI registry
   - Directly assigns KPI UUIDs to `RedAnchor.Service` and `RedAnchor.Metric` fields
   - No name resolution occurs at this stage

2. **RCA Engine** (`internal/rca/engine.go`)
   - Consumes `RedAnchors` from correlation results
   - Propagates UUIDs through `IncidentContext` and `RCAStep` objects
   - Works exclusively with UUIDs for internal consistency

3. **API Handler** (`internal/api/handlers/rca.handler.go`)
   - Previously had `enrichKPIMetadata()` that added metadata fields
   - Did NOT replace the UUID values in the DTO fields
   - UUIDs were serialized directly to JSON

## Solution Implemented

### Approach: Option B - Replace UUIDs at DTO Layer

We chose to replace UUIDs with names at the presentation layer (DTO conversion) while preserving UUIDs in separate fields for reference. This approach:

- ✅ Keeps internal engine logic using UUIDs (stable identifiers)
- ✅ Presents human-readable names in the API response
- ✅ Preserves original UUIDs in dedicated fields for traceability
- ✅ Maintains backward compatibility for clients that need UUIDs

### Changes Made

#### 1. Updated DTO Models (`internal/models/rca.go`)

**`IncidentContextDTO`** - Added UUID preservation fields:
```go
type IncidentContextDTO struct {
    ID            string  `json:"id"`
    ImpactService string  `json:"impactService"`           // Now contains name
    MetricName    string  `json:"metricName"`              // Now contains name
    ImpactServiceUUID string `json:"impactServiceUuid,omitempty"` // ← NEW: Original UUID
    MetricNameUUID    string `json:"metricNameUuid,omitempty"`    // ← NEW: Original UUID
    TimeStartStr  string  `json:"timeStart"`
    TimeEndStr    string  `json:"timeEnd"`
    ImpactSummary string  `json:"impactSummary"`
    Severity      float64 `json:"severity"`
}
```

**`RCAStepDTO`** - Added UUID preservation fields:
```go
type RCAStepDTO struct {
    WhyIndex  int    `json:"whyIndex"`
    Service   string `json:"service"`                    // Now contains name
    Component string `json:"component"`                  // Now contains name
    ServiceUUID   string `json:"serviceUuid,omitempty"`   // ← NEW: Original UUID
    ComponentUUID string `json:"componentUuid,omitempty"` // ← NEW: Original UUID
    KPIName    string `json:"kpiName,omitempty"`
    KPIFormula string `json:"kpiFormula,omitempty"`
    // ... rest of fields
}
```

#### 2. Implemented UUID Resolution (`internal/api/handlers/rca.handler.go`)

**New Helper Function** - `resolveUUIDToName()`:
```go
// resolveUUIDToName attempts to resolve a UUID to a KPI name.
// If the value is a valid KPI UUID, it replaces *value with the KPI name
// and stores the original UUID in *uuidField.
func (h *RCAHandler) resolveUUIDToName(value *string, uuidField *string) {
    if value == nil || *value == "" || h.kpiRepo == nil {
        return
    }

    kpi, err := h.kpiRepo.GetKPI(context.Background(), *value)
    if err != nil || kpi == nil {
        return // Not a KPI UUID - keep original value
    }

    // Found a KPI - replace value with name and store UUID
    if uuidField != nil {
        *uuidField = *value
    }
    *value = kpi.Name
}
```

**Updated `convertIncidentContext()`**:
- Calls `resolveUUIDToName()` for `ImpactService` and `MetricName`
- Populates UUID fields with original values

**Updated `convertRCAChain()`**:
- Resolves UUIDs in `ImpactPath` array to names

**Updated `enrichKPIMetadata()`**:
- Now replaces Service/Component UUIDs with names
- Populates ServiceUUID/ComponentUUID fields
- Handles both Service and Component resolution

#### 3. Added Comprehensive Test (`internal/api/handlers/rca_uuid_resolution_test.go`)

Created `TestHandleComputeRCA_UUIDResolution` that verifies:
- ✅ ImpactService UUID → Name resolution
- ✅ MetricName UUID → Name resolution
- ✅ Step Service UUID → Name resolution
- ✅ Step Component UUID → Name resolution
- ✅ ImpactPath UUID → Name resolution
- ✅ UUID preservation in dedicated fields
- ✅ Non-UUID values remain unchanged

## API Response Changes

### Before (UUIDs)
```json
{
  "impact": {
    "impactService": "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
    "metricName": "864c82d3-e941-5020-9dbc-99b4dcb0318d"
  },
  "chains": [{
    "steps": [{
      "service": "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
      "component": "864c82d3-e941-5020-9dbc-99b4dcb0318d"
    }],
    "impactPath": [
      "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
      "864c82d3-e941-5020-9dbc-99b4dcb0318d"
    ]
  }]
}
```

### After (Human-Readable Names + UUID Preservation)
```json
{
  "impact": {
    "impactService": "Transaction Success Rate",
    "metricName": "Error Rate Percentage",
    "impactServiceUuid": "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
    "metricNameUuid": "864c82d3-e941-5020-9dbc-99b4dcb0318d"
  },
  "chains": [{
    "steps": [{
      "service": "Transaction Success Rate",
      "component": "Error Rate Percentage",
      "serviceUuid": "47c0c489-efc2-5d0d-a9d0-2b1a96eeb7a3",
      "componentUuid": "864c82d3-e941-5020-9dbc-99b4dcb0318d",
      "kpiName": "Transaction Success Rate",
      "kpiFormula": "sum(rate(transactions_total{status=\"success\"}[5m]))"
    }],
    "impactPath": [
      "Transaction Success Rate",
      "Error Rate Percentage"
    ]
  }]
}

## Correlation API (/api/v1/unified/correlation)

The Correlation API previously returned KPI UUIDs in several fields (for
example `CauseCandidate.KPI`, `RedAnchor.Service`, and entries in
`AffectedServices`). This made correlation responses harder to read at a
glance.

### Approach: Option A - Present names, keep UUIDs

To match the RCA presentation change and improve readability, we implemented
a name-first approach for the correlation response payload while preserving
the original UUIDs for traceability in additional optional fields.

### Changes Made

- `internal/models/models.go` — `CauseCandidate` now includes `kpiUuid` and
  `kpiFormula` (optional), and `KPI` is populated with the human-readable
  name when the KPI repo provides a definition.
- `internal/services/correlation_engine.go` — the Correlation engine will try
  to resolve KPI IDs using the KPI repo during response construction; when a
  KPI definition is found the engine writes the human name into `KPI` and
  preserves the original UUID in `kpiUuid`.
- Handler/DTO conversion and OpenAPI schemas were updated so that consumer
  documentation and examples show `kpi`, `kpiUuid`, and `kpiFormula` together.

### Tests & Examples

- Added/updated unit tests for correlation flows that assert name resolution
  and UUID preservation under `internal/services` and
  `internal/api/handlers`.
- Added example responses and OpenAPI schema entries to reflect the new
  correlation response format.

### Fallback behaviour

- When the KPI repo isn't available or lookup fails the Correlation API will
  continue to return the original UUIDs (unchanged) and omit the
  `kpiUuid`/`kpiFormula` fields, preserving backwards compatibility.
```

## Benefits

1. **Improved Readability**: API responses now show human-readable KPI names
2. **Traceability**: Original UUIDs preserved in dedicated fields for debugging
3. **Backward Compatibility**: Clients can still access UUIDs if needed
4. **Performance**: Resolution happens only at presentation layer, not during analysis
5. **Flexibility**: Non-KPI values (hardcoded strings like "infrastructure", "process") remain unchanged

## Testing

All tests pass:
```bash
✓ TestHandleComputeRCA_ValidRequest
✓ TestHandleComputeRCA_MissingImpactService
✓ TestHandleComputeRCA_InvalidTimeFormat
✓ TestHandleComputeRCA_TimeOrderError
✓ TestHandleComputeRCA_RCAEngineFails
✓ TestHandleComputeRCA_StrictRejectsExtraFields
✓ TestHandleComputeRCA_TimeWindowOnly_Valid
✓ TestHandleComputeRCA_TimeWindowOnly_InvalidFormat
✓ TestHandleComputeRCA_UUIDResolution  ← NEW TEST
```

## Files Modified

1. `internal/models/rca.go` - Added UUID preservation fields to DTOs
2. `internal/api/handlers/rca.handler.go` - Implemented UUID resolution logic
3. `internal/api/handlers/rca_uuid_resolution_test.go` - Added comprehensive test

## Compliance

This implementation follows the guidelines in `AGENTS.md`:
- ✅ No changes to core engine logic (RCA/Correlation engines untouched)
- ✅ Changes confined to presentation layer (DTOs and handlers)
- ✅ Comprehensive test coverage
- ✅ Maintains API contract for `/api/v1/unified/rca`
- ✅ No hardcoded strings in engines (per §3.6)

## Future Considerations

1. **Caching**: Consider caching KPI lookups to reduce repository calls
2. **Batch Resolution**: Optimize multiple UUID resolutions with batch queries
3. **Fallback Names**: Consider generating friendly fallbacks for failed lookups
4. **Client Libraries**: Update client SDKs to utilize new UUID fields
