package utils

import (
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/models"
)

// QueryObjectPool provides object pooling for frequently allocated query structures
type QueryObjectPool struct {
	unifiedQueryPool sync.Pool
	correlationPool  sync.Pool
	resultPool       sync.Pool
}

// NewQueryObjectPool creates a new object pool for query structures
func NewQueryObjectPool() *QueryObjectPool {
	return &QueryObjectPool{
		unifiedQueryPool: sync.Pool{
			New: func() interface{} {
				return &models.UnifiedQuery{}
			},
		},
		correlationPool: sync.Pool{
			New: func() interface{} {
				return &models.CorrelationQuery{}
			},
		},
		resultPool: sync.Pool{
			New: func() interface{} {
				return &models.UnifiedResult{}
			},
		},
	}
}

// GetUnifiedQuery retrieves a UnifiedQuery from the pool
func (p *QueryObjectPool) GetUnifiedQuery() *models.UnifiedQuery {
	query := p.unifiedQueryPool.Get().(*models.UnifiedQuery)
	// Reset the object to clean state
	query.ID = ""
	query.Type = ""
	query.Query = ""
	query.TenantID = ""
	query.StartTime = nil
	query.EndTime = nil
	query.Timeout = ""
	query.Parameters = nil
	query.CorrelationOptions = nil
	query.CacheOptions = nil
	return query
}

// PutUnifiedQuery returns a UnifiedQuery to the pool
func (p *QueryObjectPool) PutUnifiedQuery(query *models.UnifiedQuery) {
	if query != nil {
		p.unifiedQueryPool.Put(query)
	}
}

// GetCorrelationQuery retrieves a CorrelationQuery from the pool
func (p *QueryObjectPool) GetCorrelationQuery() *models.CorrelationQuery {
	query := p.correlationPool.Get().(*models.CorrelationQuery)
	// Reset the object to clean state
	query.ID = ""
	query.RawQuery = ""
	query.Expressions = nil
	query.TimeWindow = nil
	query.Operator = ""
	return query
}

// PutCorrelationQuery returns a CorrelationQuery to the pool
func (p *QueryObjectPool) PutCorrelationQuery(query *models.CorrelationQuery) {
	if query != nil {
		p.correlationPool.Put(query)
	}
}

// GetUnifiedResult retrieves a UnifiedResult from the pool
func (p *QueryObjectPool) GetUnifiedResult() *models.UnifiedResult {
	result := p.resultPool.Get().(*models.UnifiedResult)
	// Reset the object to clean state
	result.QueryID = ""
	result.Type = ""
	result.Status = ""
	result.Data = nil
	result.Metadata = nil
	result.Correlations = nil
	result.ExecutionTime = 0
	result.Cached = false
	return result
}

// PutUnifiedResult returns a UnifiedResult to the pool
func (p *QueryObjectPool) PutUnifiedResult(result *models.UnifiedResult) {
	if result != nil {
		p.resultPool.Put(result)
	}
}

// QueryPoolManager manages multiple object pools with statistics
type QueryPoolManager struct {
	pool     *QueryObjectPool
	stats    PoolStats
	statsMux sync.RWMutex
}

// PoolStats contains statistics about pool usage
type PoolStats struct {
	UnifiedQueryGets    int64
	UnifiedQueryPuts    int64
	CorrelationGets     int64
	CorrelationPuts     int64
	ResultGets          int64
	ResultPuts          int64
	PoolHits            int64
	PoolMisses          int64
	LastReset           time.Time
}

// NewQueryPoolManager creates a new pool manager
func NewQueryPoolManager() *QueryPoolManager {
	return &QueryPoolManager{
		pool: NewQueryObjectPool(),
		stats: PoolStats{
			LastReset: time.Now(),
		},
	}
}

// GetUnifiedQuery gets a UnifiedQuery and updates stats
func (pm *QueryPoolManager) GetUnifiedQuery() *models.UnifiedQuery {
	pm.statsMux.Lock()
	pm.stats.UnifiedQueryGets++
	pm.statsMux.Unlock()
	return pm.pool.GetUnifiedQuery()
}

// PutUnifiedQuery puts a UnifiedQuery and updates stats
func (pm *QueryPoolManager) PutUnifiedQuery(query *models.UnifiedQuery) {
	pm.statsMux.Lock()
	pm.stats.UnifiedQueryPuts++
	pm.statsMux.Unlock()
	pm.pool.PutUnifiedQuery(query)
}

// GetCorrelationQuery gets a CorrelationQuery and updates stats
func (pm *QueryPoolManager) GetCorrelationQuery() *models.CorrelationQuery {
	pm.statsMux.Lock()
	pm.stats.CorrelationGets++
	pm.statsMux.Unlock()
	return pm.pool.GetCorrelationQuery()
}

// PutCorrelationQuery puts a CorrelationQuery and updates stats
func (pm *QueryPoolManager) PutCorrelationQuery(query *models.CorrelationQuery) {
	pm.statsMux.Lock()
	pm.stats.CorrelationPuts++
	pm.statsMux.Unlock()
	pm.pool.PutCorrelationQuery(query)
}

// GetUnifiedResult gets a UnifiedResult and updates stats
func (pm *QueryPoolManager) GetUnifiedResult() *models.UnifiedResult {
	pm.statsMux.Lock()
	pm.stats.ResultGets++
	pm.statsMux.Unlock()
	return pm.pool.GetUnifiedResult()
}

// PutUnifiedResult puts a UnifiedResult and updates stats
func (pm *QueryPoolManager) PutUnifiedResult(result *models.UnifiedResult) {
	pm.statsMux.Lock()
	pm.stats.ResultPuts++
	pm.statsMux.Unlock()
	pm.pool.PutUnifiedResult(result)
}

// GetStats returns current pool statistics
func (pm *QueryPoolManager) GetStats() PoolStats {
	pm.statsMux.RLock()
	defer pm.statsMux.RUnlock()
	return pm.stats
}

// ResetStats resets the pool statistics
func (pm *QueryPoolManager) ResetStats() {
	pm.statsMux.Lock()
	pm.stats = PoolStats{
		LastReset: time.Now(),
	}
	pm.statsMux.Unlock()
}