package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/platformbuilds/miradorstack/internal/models"
	"github.com/platformbuilds/miradorstack/pkg/logger"
)

type ValkeyCluster interface {
	// General caching
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Delete(ctx context.Context, key string) error

	// Session management (as shown in diagram)
	GetSession(ctx context.Context, sessionID string) (*models.UserSession, error)
	SetSession(ctx context.Context, session *models.UserSession) error
	InvalidateSession(ctx context.Context, sessionID string) error
	GetActiveSessions(ctx context.Context, tenantID string) ([]*models.UserSession, error)

	// Query result caching for faster fetch
	CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error
	GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error)
}

type valkeyClusterImpl struct {
	client *redis.ClusterClient
	logger logger.Logger
	ttl    time.Duration
}

func NewValkeyCluster(nodes []string, defaultTTL time.Duration) (ValkeyCluster, error) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:        nodes,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	// Test connection to Valkey cluster
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Valkey cluster: %w", err)
	}

	return &valkeyClusterImpl{
		client: client,
		logger: logger.New("info"),
		ttl:    defaultTTL,
	}, nil
}

/* ---------------------------- generic cache ---------------------------- */

func (v *valkeyClusterImpl) Get(ctx context.Context, key string) ([]byte, error) {
	b, err := v.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, fmt.Errorf("key not found: %s", key)
	}
	return b, err
}

func (v *valkeyClusterImpl) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	var data []byte
	switch x := value.(type) {
	case []byte:
		data = x
	case string:
		data = []byte(x)
	default:
		j, err := json.Marshal(x)
		if err != nil {
			return fmt.Errorf("marshal value for key %s: %w", key, err)
		}
		data = j
	}
	if ttl <= 0 {
		ttl = v.ttl
	}
	return v.client.Set(ctx, key, data, ttl).Err()
}

func (v *valkeyClusterImpl) Delete(ctx context.Context, key string) error {
	return v.client.Del(ctx, key).Err()
}

/* --------------------------- session management --------------------------- */

func (v *valkeyClusterImpl) SetSession(ctx context.Context, session *models.UserSession) error {
	session.LastActivity = time.Now()
	key := fmt.Sprintf("session:%s", session.ID)

	// Store session with 24h TTL (override default)
	if err := v.Set(ctx, key, session, 24*time.Hour); err != nil {
		return err
	}

	// Add to tenant active sessions set
	tenantKey := fmt.Sprintf("tenant_sessions:%s", session.TenantID)
	return v.client.SAdd(ctx, tenantKey, session.ID).Err()
}

func (v *valkeyClusterImpl) GetSession(ctx context.Context, sessionID string) (*models.UserSession, error) {
	key := fmt.Sprintf("session:%s", sessionID)
	data, err := v.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var session models.UserSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	return &session, nil
}

func (v *valkeyClusterImpl) InvalidateSession(ctx context.Context, sessionID string) error {
	// Best-effort remove from tenant set as well
	sess, err := v.GetSession(ctx, sessionID)
	if err == nil && sess != nil {
		tenantKey := fmt.Sprintf("tenant_sessions:%s", sess.TenantID)
		_ = v.client.SRem(ctx, tenantKey, sessionID).Err()
	}
	return v.Delete(ctx, fmt.Sprintf("session:%s", sessionID))
}

func (v *valkeyClusterImpl) GetActiveSessions(ctx context.Context, tenantID string) ([]*models.UserSession, error) {
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
			// Clean up stale references
			_ = v.client.SRem(ctx, tenantKey, sessionID).Err()
		}
	}
	return sessions, nil
}

/* --------------------------- query result cache --------------------------- */

func (v *valkeyClusterImpl) CacheQueryResult(ctx context.Context, queryHash string, result interface{}, ttl time.Duration) error {
	key := fmt.Sprintf("query_cache:%s", queryHash)
	return v.Set(ctx, key, result, ttl)
}

func (v *valkeyClusterImpl) GetCachedQueryResult(ctx context.Context, queryHash string) ([]byte, error) {
	key := fmt.Sprintf("query_cache:%s", queryHash)
	return v.Get(ctx, key)
}
