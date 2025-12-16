package weaviate

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	wv "github.com/weaviate/weaviate-go-client/v5/weaviate"
	"go.uber.org/zap"

	"github.com/platformbuilds/mirador-core/internal/mira/session"
)

// Store implements session.SessionStore using Weaviate as the backend.
// This allows persistent, distributed session storage for MIRA conversations.
type Store struct {
	client *wv.Client
	logger *zap.Logger
	ttl    time.Duration
	class  string // Weaviate class name for sessions
}

// New creates a new Weaviate-backed session store.
// Parameters:
//   - client: Weaviate client (v5)
//   - logger: Zap logger for debugging
//   - ttl: Time-to-live for sessions; cleanup is responsibility of background job (Phase-2)
//   - className: Weaviate class name to use (e.g., "MIRASession")
func New(client *wv.Client, logger *zap.Logger, ttl time.Duration, className string) (*Store, error) {
	if client == nil {
		return nil, fmt.Errorf("weaviate client is nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is nil")
	}
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	if className == "" {
		className = "MIRASession"
	}

	store := &Store{
		client: client,
		logger: logger,
		ttl:    ttl,
		class:  className,
	}

	// Phase-2: Initialize Weaviate schema for session class (create if not exists)
	// This is deferred to Phase-2 as it requires schema management integration.
	// NOTE: AT-009 - Schema initialization for persistent sessions

	return store, nil
}

// Get retrieves a session by ID from Weaviate.
// Returns (SessionData{}, false) if not found or expired.
func (s *Store) Get(id string) (session.SessionData, bool) {
	// Phase-1 stub: always return false (no persistent storage yet)
	// Phase-2: Query Weaviate with filter: id == sessionID AND updatedAt > (now - ttl)
	// NOTE: AT-010 - Implement Weaviate query for session retrieval
	return session.SessionData{}, false
}

// Set stores or updates a session in Weaviate.
// Converts SessionData to JSON and stores in Weaviate with current timestamp.
func (s *Store) Set(id string, sd *session.SessionData) {
	if sd == nil {
		return
	}

	// Phase-1 stub: no-op
	// Phase-2: Serialize SessionData to JSON and upsert into Weaviate
	// Fields to store: id, scope, lastHealth, lastFailures, lastRCA, pendingMutations, updatedAt, ttl
	// NOTE: AT-010 - Implement Weaviate upsert for session storage

	s.logger.Debug("Set called for session",
		zap.String("id", id),
		zap.String("scope", sd.Scope),
		zap.Time("updatedAt", sd.UpdatedAt),
	)
}

// Reset deletes a session from Weaviate.
func (s *Store) Reset(id string) {
	// Phase-1 stub: no-op
	// Phase-2: Delete from Weaviate where id == sessionID
	// NOTE: AT-010 - Implement Weaviate delete for session cleanup

	s.logger.Debug("Reset called for session", zap.String("id", id))
}

// Ensure creates or retrieves a session, initializing if needed.
// In Weaviate implementation, ensures TTL is set on retrieval.
func (s *Store) Ensure(id string) session.SessionData {
	// Phase-1 stub: always return a new SessionData
	// Phase-2: Query Weaviate; if found and not expired, return it; otherwise create new
	// NOTE: AT-010 - Implement Ensure with Weaviate fallback logic

	return session.SessionData{
		UpdatedAt: time.Now(),
	}
}

// CleanupExpired removes all expired sessions from Weaviate.
// This is a background operation for Phase-2 (deferred).
// NOTE: AT-009 - Implement background cleanup job
func (s *Store) CleanupExpired(ctx context.Context) (int, error) {
	// Phase-2 stub: return 0, nil (no cleanup yet)
	// Phase-2: Delete from Weaviate where (now - updatedAt) > ttl
	return 0, nil
}

// sessionRecord is the internal representation for Weaviate storage.
// This struct is used to serialize/deserialize session data to/from Weaviate.
type sessionRecord struct {
	ID               string                   `json:"id"`
	Scope            string                   `json:"scope"`
	LastHealth       map[string]interface{}   `json:"lastHealth,omitempty"`
	LastFailures     []map[string]interface{} `json:"lastFailures,omitempty"`
	LastRCA          map[string]interface{}   `json:"lastRca,omitempty"`
	PendingMutations []map[string]interface{} `json:"pendingMutations,omitempty"`
	UpdatedAt        time.Time                `json:"updatedAt"`
	TTLSeconds       int64                    `json:"ttlSeconds"`
	ExpiresAt        time.Time                `json:"expiresAt"`
}

// toSessionRecord converts a SessionData to a sessionRecord for Weaviate storage.
func (s *Store) toSessionRecord(id string, sd *session.SessionData) *sessionRecord {
	expiresAt := sd.UpdatedAt.Add(s.ttl)
	return &sessionRecord{
		ID:               id,
		Scope:            sd.Scope,
		LastHealth:       sd.LastHealth,
		LastFailures:     sd.LastFailures,
		LastRCA:          sd.LastRCA,
		PendingMutations: sd.PendingMutations,
		UpdatedAt:        sd.UpdatedAt,
		TTLSeconds:       int64(s.ttl.Seconds()),
		ExpiresAt:        expiresAt,
	}
}

// fromSessionRecord converts a sessionRecord (from Weaviate) to a SessionData.
func (s *Store) fromSessionRecord(rec *sessionRecord) session.SessionData {
	return session.SessionData{
		Scope:            rec.Scope,
		LastHealth:       rec.LastHealth,
		LastFailures:     rec.LastFailures,
		LastRCA:          rec.LastRCA,
		PendingMutations: rec.PendingMutations,
		UpdatedAt:        rec.UpdatedAt,
	}
}

// isExpired checks if a record is expired.
func (s *Store) isExpired(rec *sessionRecord) bool {
	return time.Now().After(rec.ExpiresAt)
}

// Helper to marshal SessionData to JSON string for storage.
func marshalSessionData(sd *session.SessionData) (string, error) {
	b, err := json.Marshal(sd)
	if err != nil {
		return "", fmt.Errorf("marshal session data: %w", err)
	}
	return string(b), nil
}

// Helper to unmarshal JSON string to SessionData.
func unmarshalSessionData(jsonStr string) (*session.SessionData, error) {
	var sd session.SessionData
	if err := json.Unmarshal([]byte(jsonStr), &sd); err != nil {
		return nil, fmt.Errorf("unmarshal session data: %w", err)
	}
	return &sd, nil
}
