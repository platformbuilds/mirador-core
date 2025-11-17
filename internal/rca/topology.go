package rca

// ================================
// topology.go - Service dependency graph abstraction
// ================================
// ServiceGraph captures directed edges from servicegraph connector metrics
// and provides query helpers for topology analysis.

import (
	"fmt"
	"sort"
	"sync"
)

// ServiceNode represents a service in the topology.
type ServiceNode string

// ServiceEdge represents a directed dependency from client to server.
// Aggregates statistics from servicegraph metrics.
type ServiceEdge struct {
	// Source and target service names.
	Source ServiceNode
	Target ServiceNode

	// Request statistics.
	RequestCount float64
	FailureCount float64
	RequestRate  float64 // requests per second (derived from counts and time window)
	FailureRate  float64 // failures per second
	ErrorRate    float64 // fraction of requests that failed (0.0..1.0)

	// Latency statistics (in milliseconds).
	LatencyAvgMs float64
	LatencyP50Ms float64
	LatencyP95Ms float64
	LatencyP99Ms float64

	// Additional attributes for future RCA use.
	Attributes map[string]interface{}
}

// ServiceGraph is a directed graph of service dependencies.
type ServiceGraph struct {
	// Map of (source, target) -> ServiceEdge.
	edges map[serviceEdgeKey]ServiceEdge

	// Map of service name -> list of outgoing edges.
	outgoing map[ServiceNode][]ServiceEdge

	// Map of service name -> list of incoming edges.
	incoming map[ServiceNode][]ServiceEdge

	// All nodes (services) observed.
	nodes map[ServiceNode]bool

	mu sync.RWMutex
}

type serviceEdgeKey struct {
	source ServiceNode
	target ServiceNode
}

// NewServiceGraph creates an empty service graph.
func NewServiceGraph() *ServiceGraph {
	return &ServiceGraph{
		edges:    make(map[serviceEdgeKey]ServiceEdge),
		outgoing: make(map[ServiceNode][]ServiceEdge),
		incoming: make(map[ServiceNode][]ServiceEdge),
		nodes:    make(map[ServiceNode]bool),
	}
}

// AddEdge adds or updates a directed edge in the graph.
func (sg *ServiceGraph) AddEdge(edge ServiceEdge) {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	key := serviceEdgeKey{source: edge.Source, target: edge.Target}

	// Store or update the edge.
	sg.edges[key] = edge

	// Record both nodes.
	sg.nodes[edge.Source] = true
	sg.nodes[edge.Target] = true

	// Rebuild outgoing/incoming maps (simple approach for clarity).
	sg.rebuildIndexes()
}

// rebuildIndexes rebuilds outgoing/incoming adjacency lists.
// Must be called with the lock held.
func (sg *ServiceGraph) rebuildIndexes() {
	sg.outgoing = make(map[ServiceNode][]ServiceEdge)
	sg.incoming = make(map[ServiceNode][]ServiceEdge)

	for _, edge := range sg.edges {
		sg.outgoing[edge.Source] = append(sg.outgoing[edge.Source], edge)
		sg.incoming[edge.Target] = append(sg.incoming[edge.Target], edge)
	}

	// Sort for deterministic results.
	for _, edges := range sg.outgoing {
		sort.Slice(edges, func(i, j int) bool {
			return edges[i].Target < edges[j].Target
		})
	}
	for _, edges := range sg.incoming {
		sort.Slice(edges, func(i, j int) bool {
			return edges[i].Source < edges[j].Source
		})
	}
}

// GetEdge returns the edge between source and target if it exists.
func (sg *ServiceGraph) GetEdge(source, target ServiceNode) (ServiceEdge, bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	key := serviceEdgeKey{source: source, target: target}
	edge, ok := sg.edges[key]
	return edge, ok
}

// Neighbors returns all direct neighbors (adjacent services).
// For directional analysis, use Downstream/Upstream.
func (sg *ServiceGraph) Neighbors(service ServiceNode) []ServiceNode {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	var neighbors []ServiceNode
	seen := make(map[ServiceNode]bool)

	// Outgoing neighbors (services this one calls).
	for _, edge := range sg.outgoing[service] {
		if !seen[edge.Target] {
			neighbors = append(neighbors, edge.Target)
			seen[edge.Target] = true
		}
	}

	// Incoming neighbors (services that call this one).
	for _, edge := range sg.incoming[service] {
		if !seen[edge.Source] {
			neighbors = append(neighbors, edge.Source)
			seen[edge.Source] = true
		}
	}

	sort.Slice(neighbors, func(i, j int) bool {
		return neighbors[i] < neighbors[j]
	})

	return neighbors
}

// Downstream returns services directly called by the given service.
func (sg *ServiceGraph) Downstream(service ServiceNode) []ServiceNode {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	var downstream []ServiceNode
	for _, edge := range sg.outgoing[service] {
		downstream = append(downstream, edge.Target)
	}
	sort.Slice(downstream, func(i, j int) bool {
		return downstream[i] < downstream[j]
	})
	return downstream
}

// Upstream returns services that call the given service.
func (sg *ServiceGraph) Upstream(service ServiceNode) []ServiceNode {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	var upstream []ServiceNode
	for _, edge := range sg.incoming[service] {
		upstream = append(upstream, edge.Source)
	}
	sort.Slice(upstream, func(i, j int) bool {
		return upstream[i] < upstream[j]
	})
	return upstream
}

// IsUpstream returns true if there is a directed path from `from` to `to`.
// Returns false if from == to.
func (sg *ServiceGraph) IsUpstream(from, to ServiceNode) bool {
	if from == to {
		return false
	}

	visited := make(map[ServiceNode]bool)
	return sg.hasPath(from, to, visited)
}

// hasPath implements depth-first search for path detection.
// Must be called with the read lock held.
func (sg *ServiceGraph) hasPath(current, target ServiceNode, visited map[ServiceNode]bool) bool {
	if current == target {
		return true
	}
	if visited[current] {
		return false
	}
	visited[current] = true

	for _, neighbor := range sg.outgoing[current] {
		if sg.hasPath(neighbor.Target, target, visited) {
			return true
		}
	}

	return false
}

// ShortestPath computes the shortest path from source to target using BFS.
// Returns (path, found). If not found, path is nil and found is false.
func (sg *ServiceGraph) ShortestPath(source, target ServiceNode) ([]ServiceNode, bool) {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	if source == target {
		return []ServiceNode{source}, true
	}

	// BFS.
	queue := []ServiceNode{source}
	visited := make(map[ServiceNode]bool)
	parent := make(map[ServiceNode]ServiceNode)

	visited[source] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, neighbor := range sg.outgoing[current] {
			if visited[neighbor.Target] {
				continue
			}
			visited[neighbor.Target] = true
			parent[neighbor.Target] = current

			if neighbor.Target == target {
				// Reconstruct path.
				path := []ServiceNode{target}
				node := target
				for node != source {
					node = parent[node]
					path = append(path, node)
				}
				// Reverse path.
				for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
					path[i], path[j] = path[j], path[i]
				}
				return path, true
			}

			queue = append(queue, neighbor.Target)
		}
	}

	return nil, false
}

// AllNodes returns all services in the graph.
func (sg *ServiceGraph) AllNodes() []ServiceNode {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	var nodes []ServiceNode
	for node := range sg.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i] < nodes[j]
	})
	return nodes
}

// AllEdges returns all edges in the graph.
func (sg *ServiceGraph) AllEdges() []ServiceEdge {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	var edges []ServiceEdge
	for _, edge := range sg.edges {
		edges = append(edges, edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		return edges[i].Target < edges[j].Target
	})
	return edges
}

// Size returns the number of nodes in the graph.
func (sg *ServiceGraph) Size() int {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return len(sg.nodes)
}

// EdgeCount returns the number of edges in the graph.
func (sg *ServiceGraph) EdgeCount() int {
	sg.mu.RLock()
	defer sg.mu.RUnlock()
	return len(sg.edges)
}

// Clear removes all nodes and edges from the graph.
func (sg *ServiceGraph) Clear() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	sg.edges = make(map[serviceEdgeKey]ServiceEdge)
	sg.outgoing = make(map[ServiceNode][]ServiceEdge)
	sg.incoming = make(map[ServiceNode][]ServiceEdge)
	sg.nodes = make(map[ServiceNode]bool)
}

// String returns a string representation of the graph (for debugging).
func (sg *ServiceGraph) String() string {
	sg.mu.RLock()
	defer sg.mu.RUnlock()

	s := fmt.Sprintf("ServiceGraph(nodes=%d, edges=%d)\n", len(sg.nodes), len(sg.edges))
	for _, edge := range sg.AllEdges() {
		s += fmt.Sprintf("  %s -> %s (req_rate=%.2f, err_rate=%.2f)\n",
			edge.Source, edge.Target, edge.RequestRate, edge.ErrorRate)
	}
	return s
}
