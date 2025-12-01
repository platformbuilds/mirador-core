# API Debug Summary: unified/failures/get Endpoint

## Issue Found ✅

**Problem**: The `GET /api/v1/unified/failures/get?failure_id=<id>` endpoint was always returning 404 even when the failure record existed in Weaviate.

**Root Cause**: The handler was treating the `failure_id` query parameter (human-readable ID like `"fintrans-simulator-api-gatewaycall-tps-20251201-121215"`) as if it were a UUID when searching in Weaviate. 

The actual Weaviate object was stored with a UUID-based object ID, causing the lookup to fail.

---

## Solution Implemented ✅

### Two New Methods Added to `WeaviateFailureStore`:

1. **`GetFailureByID(ctx, failureID string) (*FailureRecord, error)`**
   - Searches by human-readable FailureID
   - Scans all FailureRecord objects and matches by `failureId` property
   - Returns the complete failure record

2. **`DeleteFailureByID(ctx, failureID string) error`**
   - Deletes by human-readable FailureID
   - Finds object with matching `failureId` property
   - Deletes the Weaviate object

### Two Handlers Updated:

1. **`HandleGetFailureDetail()` in unified_query.go**
   - Changed from: `failureStore.GetFailure(failureID)` ❌
   - Changed to: `failureStore.GetFailureByID(failureID)` ✅

2. **`HandleDeleteFailure()` in unified_query.go**
   - Changed from: `failureStore.DeleteFailure(failureID)` ❌
   - Changed to: `failureStore.DeleteFailureByID(failureID)` ✅

---

## Data Model Clarification

```go
type FailureRecord struct {
    FailureUUID string  // UUID v5 hash (used as Weaviate object ID) - PRIMARY KEY
    FailureID   string  // Human-readable ID (stored as property) - SEARCHABLE
    // ...
}
```

- **FailureUUID**: `"a1b2c3d4-e5f6-5a7b-8c9d-0e1f2a3b4c5d"` (UUID format)
- **FailureID**: `"fintrans-simulator-api-gatewaycall-tps-20251201-121215"` (readable format)

The API endpoints accept and expect the **FailureID** (human-readable), but the old code was treating it as a UUID.

---

## Files Changed

| File | Changes | Type |
|------|---------|------|
| `internal/weavstore/failures_store.go` | +85 lines (GetFailureByID) +50 lines (DeleteFailureByID) | New Methods |
| `internal/api/handlers/unified_query.go` | Updated HandleGetFailureDetail() and HandleDeleteFailure() | Handler Updates |

---

## Build Status

✅ **Code compiles successfully** - No errors

```
go build ./cmd/server - SUCCESS
```

---

## Endpoints Fixed

| Endpoint | Before | After |
|----------|--------|-------|
| `GET /api/v1/unified/failures/get?failure_id=<id>` | ❌ Always 404 | ✅ Returns failure details |
| `DELETE /api/v1/unified/failures/delete?failure_id=<id>` | ❌ Always fails | ✅ Deletes correctly |

---

## Next Actions

1. ✅ Test the fixed endpoint with a real failure ID
2. ✅ Verify DELETE endpoint works
3. ✅ Run the full test suite: `make localdev-test-all-api`
4. ⏳ Code review and merge to main

---

## Key Insight

The issue was a simple but critical parameter type mismatch:
- API contract expects: **FailureID** (human-readable string)
- Code was using it as: **FailureUUID** (UUID hash)
- Result: Object lookup failed because object IDs didn't match

The fix maintains backward compatibility while enabling correct failure retrieval.
