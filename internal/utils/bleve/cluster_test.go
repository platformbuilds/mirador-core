package bleve

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/platformbuilds/mirador-core/pkg/logger"
)

func TestClusterCoordinator_StartStop(t *testing.T) {
	coordinator := NewClusterCoordinator("node-1", &mockMetadataStore{}, logger.New("test"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Start coordinator
	err := coordinator.Start(ctx)
	require.NoError(t, err)

	// Manually trigger leader election since it runs on a timer
	coordinator.PerformLeaderElection(ctx)

	// Check initial status - should be leader since it's the only node
	status := coordinator.GetClusterStatus()
	assert.Equal(t, "node-1", status["leader_id"])
	assert.True(t, status["is_leader"].(bool))
	assert.Equal(t, 1, status["member_count"])

	// Stop coordinator
	err = coordinator.Stop()
	assert.NoError(t, err)
}

func TestClusterCoordinator_Heartbeat(t *testing.T) {
	coordinator := NewClusterCoordinator("node-1", &mockMetadataStore{}, logger.New("test"))

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := coordinator.Start(ctx)
	require.NoError(t, err)

	// Wait for a heartbeat
	time.Sleep(100 * time.Millisecond)

	status := coordinator.GetClusterStatus()
	members := status["members"].([]map[string]interface{})
	require.Len(t, members, 1)

	member := members[0]
	assert.Equal(t, "node-1", member["node_id"])
	assert.True(t, member["is_active"].(bool))
	assert.Equal(t, "active", member["status"])

	coordinator.Stop()
}

func TestClusterCoordinator_NodeFailure(t *testing.T) {
	coordinator := NewClusterCoordinator("node-1", &mockMetadataStore{}, logger.New("test"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := coordinator.Start(ctx)
	require.NoError(t, err)

	// Simulate adding another node that's already failed
	coordinator.mu.Lock()
	coordinator.members["node-2"] = &ClusterMember{
		NodeID:   "node-2",
		Address:  "localhost:8081",
		LastSeen: time.Now().Add(-5 * time.Minute), // Old timestamp (failed)
		IsActive: false,
		Status:   "failed",
	}
	coordinator.mu.Unlock()

	status := coordinator.GetClusterStatus()
	members := status["members"].([]map[string]interface{})
	assert.Len(t, members, 2)

	// Find node-2
	var node2 map[string]interface{}
	for _, member := range members {
		if member["node_id"] == "node-2" {
			node2 = member
			break
		}
	}

	require.NotNil(t, node2)
	assert.False(t, node2["is_active"].(bool))
	assert.Equal(t, "failed", node2["status"])

	coordinator.Stop()
}

func TestClusterCoordinator_GetClusterStatus(t *testing.T) {
	coordinator := NewClusterCoordinator("node-1", &mockMetadataStore{}, logger.New("test"))

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := coordinator.Start(ctx)
	require.NoError(t, err)

	// Trigger leader election
	coordinator.PerformLeaderElection(ctx)

	status := coordinator.GetClusterStatus()

	assert.Contains(t, status, "leader_id")
	assert.Contains(t, status, "is_leader")
	assert.Contains(t, status, "member_count")
	assert.Contains(t, status, "members")

	assert.Equal(t, 1, status["member_count"])
	assert.True(t, status["is_leader"].(bool))

	coordinator.Stop()
}
