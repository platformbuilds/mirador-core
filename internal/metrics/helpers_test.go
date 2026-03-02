package metrics

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Test errors
var (
	errConnectionRefused = errors.New("connection refused")
	errLoadFailed        = errors.New("load failed")
	errTimeout           = errors.New("timeout")
	errConnectionLost    = errors.New("connection lost")
)

func TestRecordMariaDBStats(t *testing.T) {
	stats := sql.DBStats{
		OpenConnections: 10,
		InUse:           5,
		Idle:            5,
	}

	RecordMariaDBStats(stats)

	// Verify metrics were set (we can't easily read gauge values,
	// but we ensure no panic occurred)
	assert.NotPanics(t, func() {
		RecordMariaDBStats(stats)
	})
}

func TestRecordMariaDBQuery(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordMariaDBQuery("select", 100*time.Millisecond, nil)
		})
	})

	t.Run("error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordMariaDBQuery("insert", 50*time.Millisecond, errConnectionRefused)
		})
	})
}

func TestRecordMariaDBPing(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordMariaDBPing(5 * time.Millisecond)
	})
}

func TestRecordBootstrapOperation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordBootstrapOperation("kpi_load", 2*time.Second, 100, nil)
		})
	})

	t.Run("error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordBootstrapOperation("datasource_load", time.Second, 0, errLoadFailed)
		})
	})
}

func TestRecordKPISyncRun(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordKPISyncRun(5*time.Second, ResultSuccess)
		})
	})

	t.Run("error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordKPISyncRun(time.Second, ResultError)
		})
	})

	t.Run("skipped", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordKPISyncRun(0, "skipped")
		})
	})
}

func TestRecordKPISyncItems(t *testing.T) {
	assert.NotPanics(t, func() {
		RecordKPISyncItems(100, 10, 5, 2)
	})
}

func TestRecordWeaviateOperation(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordWeaviateOperation("create", 100*time.Millisecond, nil)
		})
	})

	t.Run("error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordWeaviateOperation("query", 500*time.Millisecond, errTimeout)
		})
	})
}

func TestRecordCacheOperation(t *testing.T) {
	t.Run("hit", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordCacheOperation("get", 5*time.Millisecond, true, nil)
		})
	})

	t.Run("miss", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordCacheOperation("get", 3*time.Millisecond, false, nil)
		})
	})

	t.Run("error", func(t *testing.T) {
		assert.NotPanics(t, func() {
			RecordCacheOperation("set", 10*time.Millisecond, false, errConnectionLost)
		})
	})
}
