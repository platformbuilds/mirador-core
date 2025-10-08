package bleve

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/platformbuilds/mirador-core/internal/monitoring"
	"github.com/platformbuilds/mirador-core/pkg/logger"
)

// ClusterCoordinator manages coordination between distributed Bleve nodes
type ClusterCoordinator struct {
	nodeID            string
	metadata          MetadataStore
	logger            logger.Logger
	heartbeatInterval time.Duration
	shutdownCh        chan struct{}
	wg                sync.WaitGroup
	mu                sync.RWMutex
	isLeader          bool
	leaderID          string
	members           map[string]*ClusterMember
	stopped           bool
}

// ClusterMember represents a member of the Bleve cluster
type ClusterMember struct {
	NodeID     string    `json:"node_id"`
	Address    string    `json:"address"`
	LastSeen   time.Time `json:"last_seen"`
	IsActive   bool      `json:"is_active"`
	ShardCount int       `json:"shard_count"`
	Status     string    `json:"status"` // active, joining, leaving
}

// ClusterEvent represents events in the cluster
type ClusterEvent struct {
	Type      string      `json:"type"` // node_joined, node_left, leader_elected, etc.
	NodeID    string      `json:"node_id"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

// NewClusterCoordinator creates a new cluster coordinator
func NewClusterCoordinator(nodeID string, metadata MetadataStore, logger logger.Logger) *ClusterCoordinator {
	return &ClusterCoordinator{
		nodeID:            nodeID,
		metadata:          metadata,
		logger:            logger,
		heartbeatInterval: 30 * time.Second, // 30 seconds
		shutdownCh:        make(chan struct{}),
		members:           make(map[string]*ClusterMember),
	}
}

// Start begins the cluster coordination process
func (cc *ClusterCoordinator) Start(ctx context.Context) error {
	cc.logger.Info("Starting cluster coordinator", "nodeID", cc.nodeID)

	// Register this node
	if err := cc.registerNode(ctx); err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	// Start heartbeat
	cc.wg.Add(1)
	go cc.heartbeatLoop(ctx)

	// Start leader election
	cc.wg.Add(1)
	go cc.leaderElectionLoop(ctx)

	// Start cluster monitoring
	cc.wg.Add(1)
	go cc.monitorCluster(ctx)

	cc.logger.Info("Cluster coordinator started successfully")
	return nil
}

// Stop stops the cluster coordination
func (cc *ClusterCoordinator) Stop() error {
	cc.mu.Lock()
	if cc.stopped {
		cc.mu.Unlock()
		return nil
	}
	cc.stopped = true
	cc.mu.Unlock()

	cc.logger.Info("Stopping cluster coordinator")

	close(cc.shutdownCh)
	cc.wg.Wait()

	// Unregister this node
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := cc.unregisterNode(ctx); err != nil {
		cc.logger.Error("Failed to unregister node", "error", err)
	}

	cc.logger.Info("Cluster coordinator stopped")
	return nil
}

// registerNode registers this node with the cluster
func (cc *ClusterCoordinator) registerNode(ctx context.Context) error {
	member := &ClusterMember{
		NodeID:     cc.nodeID,
		Address:    "localhost:8080", // TODO: Make configurable
		LastSeen:   time.Now(),
		IsActive:   true,
		ShardCount: 0,
		Status:     "active",
	}

	cc.mu.Lock()
	cc.members[cc.nodeID] = member
	nodeCount := len(cc.members)
	cc.mu.Unlock()

	// Update cluster state
	clusterState := &ClusterState{
		Nodes:       cc.getNodeMap(),
		LastUpdated: time.Now(),
	}

	if err := cc.metadata.UpdateClusterState(ctx, clusterState); err != nil {
		return err
	}

	// Record cluster node count
	monitoring.RecordBleveClusterNodes(nodeCount)
	return nil
}

// unregisterNode removes this node from the cluster
func (cc *ClusterCoordinator) unregisterNode(ctx context.Context) error {
	cc.mu.Lock()
	delete(cc.members, cc.nodeID)
	nodeCount := len(cc.members)
	cc.mu.Unlock()

	// Update cluster state
	clusterState := &ClusterState{
		Nodes:       cc.getNodeMap(),
		LastUpdated: time.Now(),
	}

	if err := cc.metadata.UpdateClusterState(ctx, clusterState); err != nil {
		return err
	}

	// Record cluster node count
	monitoring.RecordBleveClusterNodes(nodeCount)
	return nil
}

// heartbeatLoop sends periodic heartbeats to maintain cluster membership
func (cc *ClusterCoordinator) heartbeatLoop(ctx context.Context) {
	defer cc.wg.Done()

	ticker := time.NewTicker(cc.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.shutdownCh:
			return
		case <-ticker.C:
			if err := cc.sendHeartbeat(ctx); err != nil {
				cc.logger.Warn("Failed to send heartbeat", "error", err)
			}
		}
	}
}

// sendHeartbeat updates the node's last seen time
func (cc *ClusterCoordinator) sendHeartbeat(ctx context.Context) error {
	cc.mu.Lock()
	if member, exists := cc.members[cc.nodeID]; exists {
		member.LastSeen = time.Now()
	}
	cc.mu.Unlock()

	// Update cluster state
	clusterState := &ClusterState{
		Nodes:       cc.getNodeMap(),
		LastUpdated: time.Now(),
	}

	return cc.metadata.UpdateClusterState(ctx, clusterState)
}

// leaderElectionLoop handles leader election
func (cc *ClusterCoordinator) leaderElectionLoop(ctx context.Context) {
	defer cc.wg.Done()

	ticker := time.NewTicker(60 * time.Second) // Check every 60 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.shutdownCh:
			return
		case <-ticker.C:
			if err := cc.PerformLeaderElection(ctx); err != nil {
				cc.logger.Warn("Leader election failed", "error", err)
			}
		}
	}
}

// PerformLeaderElection elects a leader based on node IDs
func (cc *ClusterCoordinator) PerformLeaderElection(ctx context.Context) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if len(cc.members) == 0 {
		return nil
	}

	// Simple leader election: lowest node ID becomes leader
	var leaderID string
	for nodeID := range cc.members {
		if leaderID == "" || nodeID < leaderID {
			leaderID = nodeID
		}
	}

	wasLeader := cc.isLeader
	cc.isLeader = (leaderID == cc.nodeID)
	cc.leaderID = leaderID

	if cc.isLeader && !wasLeader {
		cc.logger.Info("Became cluster leader", "nodeID", cc.nodeID)
		monitoring.RecordBleveLeadershipChange()
	} else if !cc.isLeader && wasLeader {
		cc.logger.Info("No longer cluster leader", "newLeader", leaderID)
	}

	// Record cluster node count
	monitoring.RecordBleveClusterNodes(len(cc.members))

	return nil
}

// monitorCluster monitors cluster health and handles node failures
func (cc *ClusterCoordinator) monitorCluster(ctx context.Context) {
	defer cc.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-cc.shutdownCh:
			return
		case <-ticker.C:
			if err := cc.checkClusterHealth(ctx); err != nil {
				cc.logger.Warn("Cluster health check failed", "error", err)
			}
		}
	}
}

// checkClusterHealth checks for failed nodes and updates cluster state
func (cc *ClusterCoordinator) checkClusterHealth(ctx context.Context) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	now := time.Now()
	failedNodes := make([]string, 0)

	for nodeID, member := range cc.members {
		// Mark nodes as failed if not seen for more than 2 heartbeats
		if now.Sub(member.LastSeen) > 2*cc.heartbeatInterval {
			if member.IsActive {
				cc.logger.Warn("Node marked as failed", "nodeID", nodeID, "lastSeen", member.LastSeen)
				member.IsActive = false
				member.Status = "failed"
				failedNodes = append(failedNodes, nodeID)
			}
		}
	}

	if len(failedNodes) > 0 {
		// Update cluster state
		clusterState := &ClusterState{
			Nodes:       cc.getNodeMap(),
			LastUpdated: time.Now(),
		}

		if err := cc.metadata.UpdateClusterState(ctx, clusterState); err != nil {
			return err
		}

		// Trigger rebalancing if leader
		if cc.isLeader {
			go cc.triggerRebalancing(ctx, failedNodes)
		}
	}

	return nil
}

// triggerRebalancing initiates rebalancing when nodes fail
func (cc *ClusterCoordinator) triggerRebalancing(ctx context.Context, failedNodes []string) {
	cc.logger.Info("Triggering rebalancing due to failed nodes", "failedNodes", failedNodes)

	// Acquire distributed lock for rebalancing
	lockKey := "bleve:rebalancing:lock"
	locked, err := cc.metadata.AcquireLock(ctx, lockKey, 5*time.Minute)
	if err != nil {
		cc.logger.Error("Failed to acquire rebalancing lock", "error", err)
		return
	}
	if !locked {
		cc.logger.Info("Rebalancing already in progress, skipping")
		return
	}
	defer func() {
		if err := cc.metadata.ReleaseLock(ctx, lockKey); err != nil {
			cc.logger.Warn("Failed to release rebalancing lock", "error", err)
		}
	}()

	// Get all index metadata to find shards assigned to failed nodes
	indices, err := cc.metadata.ListIndices(ctx)
	if err != nil {
		cc.logger.Error("Failed to list indices for rebalancing", "error", err)
		return
	}

	shardsToRebalance := make(map[string][]string) // index_name -> []shard_ids

	for _, indexName := range indices {
		metadata, err := cc.metadata.GetIndexMetadata(ctx, indexName)
		if err != nil {
			cc.logger.Warn("Failed to get metadata for index", "indexName", indexName, "error", err)
			continue
		}

		// Find shards assigned to failed nodes
		for shardID, nodeID := range metadata.ShardNodes {
			for _, failedNode := range failedNodes {
				if nodeID == failedNode {
					shardsToRebalance[indexName] = append(shardsToRebalance[indexName], shardID)
				}
			}
		}
	}

	totalShardsToRebalance := 0
	for _, shards := range shardsToRebalance {
		totalShardsToRebalance += len(shards)
	}

	if totalShardsToRebalance == 0 {
		cc.logger.Info("No shards need rebalancing")
		return
	}

	// Redistribute shards to active nodes
	if err := cc.redistributeShards(ctx, shardsToRebalance); err != nil {
		cc.logger.Error("Failed to redistribute shards", "error", err)
		return
	}

	cc.logger.Info("Rebalancing completed", "redistributedShards", totalShardsToRebalance)
	monitoring.RecordBleveRebalancingOperation(totalShardsToRebalance, true)
}

// GetClusterStatus returns the current cluster status
func (cc *ClusterCoordinator) GetClusterStatus() map[string]interface{} {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	members := make([]map[string]interface{}, 0, len(cc.members))
	for _, member := range cc.members {
		members = append(members, map[string]interface{}{
			"node_id":     member.NodeID,
			"address":     member.Address,
			"last_seen":   member.LastSeen,
			"is_active":   member.IsActive,
			"shard_count": member.ShardCount,
			"status":      member.Status,
		})
	}

	return map[string]interface{}{
		"leader_id":    cc.leaderID,
		"is_leader":    cc.isLeader,
		"member_count": len(cc.members),
		"members":      members,
	}
}

// IsLeader returns whether this node is the cluster leader
func (cc *ClusterCoordinator) IsLeader() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.isLeader
}

// GetLeaderID returns the current leader node ID
func (cc *ClusterCoordinator) GetLeaderID() string {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.leaderID
}

// redistributeShards redistributes shards from failed nodes to active nodes
func (cc *ClusterCoordinator) redistributeShards(ctx context.Context, shardsToRebalance map[string][]string) error {
	// Get active nodes
	activeNodes := make([]string, 0)
	cc.mu.RLock()
	for nodeID, member := range cc.members {
		if member.IsActive && member.Status == "active" {
			activeNodes = append(activeNodes, nodeID)
		}
	}
	cc.mu.RUnlock()

	if len(activeNodes) == 0 {
		return fmt.Errorf("no active nodes available for rebalancing")
	}

	// Redistribute shards for each index
	for indexName, shardIDs := range shardsToRebalance {
		if err := cc.redistributeIndexShards(ctx, indexName, shardIDs, activeNodes); err != nil {
			cc.logger.Error("Failed to redistribute shards for index", "indexName", indexName, "error", err)
			continue
		}
	}

	return nil
}

// redistributeIndexShards redistributes shards for a specific index
func (cc *ClusterCoordinator) redistributeIndexShards(ctx context.Context, indexName string, shardIDs []string, activeNodes []string) error {
	// Get current index metadata
	metadata, err := cc.metadata.GetIndexMetadata(ctx, indexName)
	if err != nil {
		return fmt.Errorf("failed to get metadata for index %s: %w", indexName, err)
	}

	// Redistribute each shard to an active node using round-robin
	for i, shardID := range shardIDs {
		targetNodeID := activeNodes[i%len(activeNodes)]
		metadata.ShardNodes[shardID] = targetNodeID
		cc.logger.Info("Redistributed shard", "shardID", shardID, "fromNode", "failed", "toNode", targetNodeID)
	}

	// Update metadata
	metadata.LastUpdated = time.Now()
	metadata.Status = "active" // Reset status after rebalancing

	if err := cc.metadata.StoreIndexMetadata(ctx, metadata); err != nil {
		return fmt.Errorf("failed to update metadata for index %s: %w", indexName, err)
	}

	return nil
}

// getNodeMap converts members map to the format expected by ClusterState
func (cc *ClusterCoordinator) getNodeMap() map[string]NodeInfo {
	nodeMap := make(map[string]NodeInfo)
	for nodeID, member := range cc.members {
		status := member.Status
		if !member.IsActive {
			status = "inactive"
		}

		nodeMap[nodeID] = NodeInfo{
			NodeID:     member.NodeID,
			Address:    member.Address,
			Status:     status,
			LastSeen:   member.LastSeen,
			ShardCount: member.ShardCount,
		}
	}
	return nodeMap
}
