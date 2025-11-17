package rca

import (
	"testing"
)

func TestServiceGraph_AddEdge(t *testing.T) {
	graph := NewServiceGraph()

	edge := ServiceEdge{
		Source:       ServiceNode("api-gateway"),
		Target:       ServiceNode("tps"),
		RequestCount: 100,
		FailureCount: 5,
	}

	graph.AddEdge(edge)

	if graph.Size() != 2 {
		t.Errorf("Expected 2 nodes, got %d", graph.Size())
	}

	if graph.EdgeCount() != 1 {
		t.Errorf("Expected 1 edge, got %d", graph.EdgeCount())
	}
}

func TestServiceGraph_Neighbors(t *testing.T) {
	graph := NewServiceGraph()

	// Build: api-gateway -> tps -> cassandra
	//        api-gateway -> kafka

	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("kafka"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("tps"),
		Target: ServiceNode("cassandra"),
	})

	neighbors := graph.Neighbors(ServiceNode("api-gateway"))
	if len(neighbors) != 2 {
		t.Errorf("Expected 2 neighbors, got %d: %v", len(neighbors), neighbors)
	}

	// Check for expected neighbors
	found := make(map[ServiceNode]bool)
	for _, n := range neighbors {
		found[n] = true
	}
	if !found[ServiceNode("tps")] || !found[ServiceNode("kafka")] {
		t.Errorf("Expected neighbors tps and kafka, got: %v", neighbors)
	}

	// Check outgoing only
	downstream := graph.Downstream(ServiceNode("api-gateway"))
	if len(downstream) != 2 {
		t.Errorf("Expected 2 downstream, got %d", len(downstream))
	}

	upstream := graph.Upstream(ServiceNode("cassandra"))
	if len(upstream) != 1 || upstream[0] != ServiceNode("tps") {
		t.Errorf("Expected upstream=tps, got: %v", upstream)
	}
}

func TestServiceGraph_IsUpstream(t *testing.T) {
	graph := NewServiceGraph()

	// Build: api-gateway -> tps -> cassandra
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("tps"),
		Target: ServiceNode("cassandra"),
	})

	if !graph.IsUpstream(ServiceNode("api-gateway"), ServiceNode("tps")) {
		t.Error("Expected api-gateway -> tps to be upstream")
	}

	if !graph.IsUpstream(ServiceNode("api-gateway"), ServiceNode("cassandra")) {
		t.Error("Expected api-gateway -> cassandra to be upstream (transitive)")
	}

	if graph.IsUpstream(ServiceNode("cassandra"), ServiceNode("api-gateway")) {
		t.Error("Expected no path from cassandra to api-gateway")
	}

	if graph.IsUpstream(ServiceNode("api-gateway"), ServiceNode("api-gateway")) {
		t.Error("Expected IsUpstream to return false for same service")
	}
}

func TestServiceGraph_ShortestPath(t *testing.T) {
	graph := NewServiceGraph()

	// Build diamond: api-gateway -> [tps, kafka] -> cassandra
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("kafka"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("tps"),
		Target: ServiceNode("cassandra"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("kafka"),
		Target: ServiceNode("cassandra"),
	})

	// Test direct path.
	path, found := graph.ShortestPath(ServiceNode("api-gateway"), ServiceNode("tps"))
	if !found || len(path) != 2 {
		t.Errorf("Expected path [api-gateway, tps], got %v", path)
	}

	// Test multi-hop path.
	path, found = graph.ShortestPath(ServiceNode("api-gateway"), ServiceNode("cassandra"))
	if !found || len(path) != 3 {
		t.Errorf("Expected 3-node path, got %d nodes: %v", len(path), path)
	}

	// Test no path.
	path, found = graph.ShortestPath(ServiceNode("cassandra"), ServiceNode("api-gateway"))
	if found {
		t.Error("Expected no path from cassandra to api-gateway")
	}

	// Test same node.
	path, found = graph.ShortestPath(ServiceNode("api-gateway"), ServiceNode("api-gateway"))
	if !found || len(path) != 1 {
		t.Errorf("Expected [api-gateway], got %v", path)
	}
}

func TestServiceGraph_AllNodes(t *testing.T) {
	graph := NewServiceGraph()

	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})
	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("tps"),
		Target: ServiceNode("cassandra"),
	})

	nodes := graph.AllNodes()
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d: %v", len(nodes), nodes)
	}

	// Check they are sorted.
	for i := 1; i < len(nodes); i++ {
		if nodes[i] < nodes[i-1] {
			t.Error("Expected nodes to be sorted")
		}
	}
}

func TestServiceGraph_AllEdges(t *testing.T) {
	graph := NewServiceGraph()

	edges := []ServiceEdge{
		{Source: ServiceNode("api-gateway"), Target: ServiceNode("tps"), RequestCount: 100},
		{Source: ServiceNode("tps"), Target: ServiceNode("cassandra"), RequestCount: 50},
		{Source: ServiceNode("api-gateway"), Target: ServiceNode("kafka"), RequestCount: 30},
	}

	for _, e := range edges {
		graph.AddEdge(e)
	}

	allEdges := graph.AllEdges()
	if len(allEdges) != 3 {
		t.Errorf("Expected 3 edges, got %d", len(allEdges))
	}

	// Verify request counts are preserved.
	for _, e := range allEdges {
		if e.Source == ServiceNode("api-gateway") && e.Target == ServiceNode("tps") {
			if e.RequestCount != 100 {
				t.Errorf("Expected RequestCount=100, got %f", e.RequestCount)
			}
		}
	}
}

func TestServiceGraph_GetEdge(t *testing.T) {
	graph := NewServiceGraph()

	edge := ServiceEdge{
		Source:       ServiceNode("api-gateway"),
		Target:       ServiceNode("tps"),
		RequestCount: 100,
		ErrorRate:    0.05,
	}

	graph.AddEdge(edge)

	retrieved, ok := graph.GetEdge(ServiceNode("api-gateway"), ServiceNode("tps"))
	if !ok {
		t.Error("Expected to find edge")
	}

	if retrieved.RequestCount != 100 {
		t.Errorf("Expected RequestCount=100, got %f", retrieved.RequestCount)
	}

	if retrieved.ErrorRate != 0.05 {
		t.Errorf("Expected ErrorRate=0.05, got %f", retrieved.ErrorRate)
	}

	// Test non-existent edge.
	_, ok = graph.GetEdge(ServiceNode("api-gateway"), ServiceNode("nonexistent"))
	if ok {
		t.Error("Expected edge not to exist")
	}
}

func TestServiceGraph_Clear(t *testing.T) {
	graph := NewServiceGraph()

	graph.AddEdge(ServiceEdge{
		Source: ServiceNode("api-gateway"),
		Target: ServiceNode("tps"),
	})

	if graph.Size() != 2 {
		t.Errorf("Expected 2 nodes before clear, got %d", graph.Size())
	}

	graph.Clear()

	if graph.Size() != 0 {
		t.Errorf("Expected 0 nodes after clear, got %d", graph.Size())
	}

	if graph.EdgeCount() != 0 {
		t.Errorf("Expected 0 edges after clear, got %d", graph.EdgeCount())
	}
}

func TestServiceGraph_MultipleEdgesUpdate(t *testing.T) {
	graph := NewServiceGraph()

	// Add initial edge.
	graph.AddEdge(ServiceEdge{
		Source:       ServiceNode("api-gateway"),
		Target:       ServiceNode("tps"),
		RequestCount: 100,
	})

	// Update same edge with different values.
	graph.AddEdge(ServiceEdge{
		Source:       ServiceNode("api-gateway"),
		Target:       ServiceNode("tps"),
		RequestCount: 150,
		ErrorRate:    0.1,
	})

	edge, ok := graph.GetEdge(ServiceNode("api-gateway"), ServiceNode("tps"))
	if !ok {
		t.Error("Expected to find edge")
	}

	// The new edge should have replaced the old one.
	if edge.RequestCount != 150 {
		t.Errorf("Expected RequestCount=150 after update, got %f", edge.RequestCount)
	}

	if edge.ErrorRate != 0.1 {
		t.Errorf("Expected ErrorRate=0.1 after update, got %f", edge.ErrorRate)
	}
}

func TestServiceGraph_ComplexTopology(t *testing.T) {
	graph := NewServiceGraph()

	// Build financial transaction topology.
	// api-gateway -> [tps, keydb, kafka]
	// tps -> [cassandra, kafka]
	// kafka -> cassandra

	edges := []ServiceEdge{
		{Source: ServiceNode("api-gateway"), Target: ServiceNode("tps"), RequestCount: 1000, ErrorRate: 0.01},
		{Source: ServiceNode("api-gateway"), Target: ServiceNode("keydb"), RequestCount: 500, ErrorRate: 0.02},
		{Source: ServiceNode("api-gateway"), Target: ServiceNode("kafka"), RequestCount: 800, ErrorRate: 0.01},
		{Source: ServiceNode("tps"), Target: ServiceNode("cassandra"), RequestCount: 1500, ErrorRate: 0.005},
		{Source: ServiceNode("tps"), Target: ServiceNode("kafka"), RequestCount: 700, ErrorRate: 0.01},
		{Source: ServiceNode("kafka"), Target: ServiceNode("cassandra"), RequestCount: 1500, ErrorRate: 0.01},
	}

	for _, e := range edges {
		graph.AddEdge(e)
	}

	if graph.Size() != 5 {
		t.Errorf("Expected 5 nodes, got %d", graph.Size())
	}

	if graph.EdgeCount() != 6 {
		t.Errorf("Expected 6 edges, got %d", graph.EdgeCount())
	}

	// Check downstream of api-gateway.
	downstream := graph.Downstream(ServiceNode("api-gateway"))
	if len(downstream) != 3 {
		t.Errorf("Expected 3 downstream, got %d: %v", len(downstream), downstream)
	}

	// Check all paths to cassandra go through api-gateway eventually.
	if !graph.IsUpstream(ServiceNode("api-gateway"), ServiceNode("cassandra")) {
		t.Error("Expected api-gateway -> cassandra to be reachable")
	}

	// cassandra should have no downstream.
	downstream = graph.Downstream(ServiceNode("cassandra"))
	if len(downstream) != 0 {
		t.Errorf("Expected 0 downstream from cassandra, got %d", len(downstream))
	}
}
