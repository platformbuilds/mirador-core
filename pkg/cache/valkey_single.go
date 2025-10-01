package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/go-redis/redis/v8"
    "github.com/platformbuilds/mirador-core/internal/models"
    "github.com/platformbuilds/mirador-core/internal/monitoring"
    "github.com/platformbuilds/mirador-core/pkg/logger"
)

// valkeySingleImpl implements ValkeyCluster against a single-node Valkey/Redis instance.
type valkeySingleImpl struct {
    client *redis.Client
    logger logger.Logger
    ttl    time.Duration
}

func NewValkeySingle(addr string, db int, password string, defaultTTL time.Duration) (ValkeyCluster, error) {
    client := redis.NewClient(&redis.Options{
        Addr:         addr,
        Password:     password,
        DB:           db,
        ReadTimeout:  5 * time.Second,
        WriteTimeout: 5 * time.Second,
        PoolSize:     10,
        MinIdleConns: 5,
    })

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := client.Ping(ctx).Err(); err != nil {
        return nil, fmt.Errorf("failed to connect to Valkey single-node: %w", err)
    }

    return &valkeySingleImpl{
        client: client,
        logger: logger.New("info"),
        ttl:    defaultTTL,
    }, nil
}

func (v *valkeySingleImpl) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := v.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		monitoring.RecordCacheOperation("get", "miss")
		return nil, fmt.Errorf("key not found: %s", key)
	}
	
	if err != nil {
		monitoring.RecordCacheOperation("get", "error")
		return nil, err
	}
	
	monitoring.RecordCacheOperation("get", "hit")
	return b, nil
}

func (v *valkeySingleImpl) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
    var data []byte
    switch x := value.(type) {
    case []byte:
        data = x
    case string:
        data = []byte(x)
    default:
        j, err := json.Marshal(x)
        if err != nil {
            monitoring.RecordCacheOperation("set", "error")
            return fmt.Errorf("marshal value for key %s: %w", key, err)
        }
        data = j
    }
    if ttl <= 0 {
        ttl = v.ttl
    }
    err := v.client.Set(ctx, key, data, ttl).Err()
    if err != nil {
        monitoring.RecordCacheOperation("set", "error")
        return err
    }
    monitoring.RecordCacheOperation("set", "success")
    return nil
}

func (v *valkeySingleImpl) Delete(ctx context.Context, key string) error {
    err := v.client.Del(ctx, key).Err()
    if err != nil {
        monitoring.RecordCacheOperation("delete", "error")
        return err
    }
    monitoring.RecordCacheOperation("delete", "success")
    return nil
}

func (v *valkeySingleImpl) SetSession(ctx context.Context, session *models.UserSession) error {
    session.LastActivity = time.Now()
    key := fmt.Sprintf("session:%s", session.ID)
    if err := v.Set(ctx, key, session, 24*time.Hour); err != nil {
        monitoring.RecordCacheOperation("set_session", "error")
        return err
    }
    tenantKey := fmt.Sprintf("tenant_sessions:%s", session.TenantID)
    err := v.client.SAdd(ctx, tenantKey, session.ID).Err()
    if err != nil {
        monitoring.RecordCacheOperation("set_session", "error")
        return err
    }
    monitoring.RecordCacheOperation("set_session", "success")
    return nil
}

func (v *valkeySingleImpl) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
    key := fmt.Sprintf("session:%s", sessionID)
    data, err := v.Get(ctx, key)
    if err != nil {
        monitoring.RecordCacheOperation("get_session", "miss")
        return nil, err
    }
    var session models.UserSession
    if err := json.Unmarshal(data, &session); err != nil {
        monitoring.RecordCacheOperation("get_session", "error")
        return nil, fmt.Errorf("failed to unmarshal session: %w", err)
    }
    monitoring.RecordCacheOperation("get_session", "hit")
    return &session, nil
}

func (v *valkeySingleImpl) InvalidateSession(ctx context.Context, sessionID string) error {
    sess, err := v.GetSession(ctx, sessionID)
    if err == nil && sess != nil {
        tenantKey := fmt.Sprintf("tenant_sessions:%s", sess.TenantID)
        _ = v.client.SRem(ctx, tenantKey, sessionID).Err()
    }
    err = v.Delete(ctx, fmt.Sprintf("session:%s", sessionID))
    if err != nil {
        monitoring.RecordCacheOperation("invalidate_session", "error")
        return err
    }
    monitoring.RecordCacheOperation("invalidate_session", "success")
    return nil
}

func (v *valkeySingleImpl) GetActiveSessions(ctx context.Context, tenantID string) ([]*models.UserSession, error) {
    tenantKey := fmt.Sprintf("tenant_sessions:%s", tenantID)
    sessionIDs, err := v.client.SMembers(ctx, tenantKey).Result()
    if err != nil {
        return nil, err
    }
    sessions := make([]*models.UserSession, 0, len(sessionIDs))
    for _, sessionID := range sessionIDs {
        if session, err := v.GetSession(ctx, sessionID); err == nil {
            sessions = append(sessions, session)
        } else {
            _ = v.client.SRem(ctx, tenantKey, sessionID).Err()
        }
    }
    return sessions, nil
}

func (v *valkeySingleImpl) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
    key := fmt.Sprintf("query_cache:%s", queryHash)
    return v.Set(ctx, key, result, ttl)
}

func (v *valkeySingleImpl) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
    key := fmt.Sprintf("query_cache:%s", queryHash)
    return v.Get(ctx, key)
}

// HealthCheck pings the Valkey single-node instance.
func (v *valkeySingleImpl) HealthCheck(ctx context.Context) error {
    if ctx == nil {
        c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        ctx = c
    }
    return v.client.Ping(ctx).Err()
}
