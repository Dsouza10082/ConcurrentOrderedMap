package graph

import (
	"errors"
	"fmt"
	"sync"

	com "github.com/Dsouza10082/ConcurrentOrderedMap/v2"
)

// ErrNodeNotFound is returned when an operation references a missing node.
var ErrNodeNotFound = errors.New("graph: node not found")

// ErrEdgeNotFound is returned when an operation references a missing edge.
var ErrEdgeNotFound = errors.New("graph: edge not found")

// ErrSelfLoop is returned when a caller tries to add a self-loop.
var ErrSelfLoop = errors.New("graph: self-loop not allowed")

// ErrDuplicateNode is returned when a node already exists.
var ErrDuplicateNode = errors.New("graph: node already exists")

// ConcurrentOrderedGraph is a thread-safe directed or undirected graph whose
// node-set is backed by a ConcurrentOrderedMap, preserving insertion order
// for deterministic iteration.
//
// Each node may hold an embedding vector ([]float64) enabling hybrid
// graph-traversal + semantic-search queries — the core primitive needed
// for agentic knowledge graphs and memory systems.
//
// Type parameters:
//   - K: node identifier type (must be comparable, e.g. string, int, uuid)
//   - V: arbitrary payload stored in each node
type ConcurrentOrderedGraph[K comparable, V any] struct {
	nodes    *com.ConcurrentOrderedMap[K, *GraphNode[K, V]]
	directed bool
	mu       sync.RWMutex // graph-level lock for structural mutations
}

// NewDirectedGraph creates an empty directed graph.
func NewDirectedGraph[K comparable, V any]() *ConcurrentOrderedGraph[K, V] {
	return &ConcurrentOrderedGraph[K, V]{
		nodes:    com.NewConcurrentOrderedMap[K, *GraphNode[K, V]](),
		directed: true,
	}
}

// NewUndirectedGraph creates an empty undirected graph.
// Every AddEdge call automatically creates the reverse edge.
func NewUndirectedGraph[K comparable, V any]() *ConcurrentOrderedGraph[K, V] {
	return &ConcurrentOrderedGraph[K, V]{
		nodes:    com.NewConcurrentOrderedMap[K, *GraphNode[K, V]](),
		directed: false,
	}
}

// IsDirected returns true if the graph is directed.
func (g *ConcurrentOrderedGraph[K, V]) IsDirected() bool { return g.directed }

// ─── Node operations ──────────────────────────────────────────────────────────

// AddNode inserts a new node. vec may be nil if no embedding is needed.
// Returns ErrDuplicateNode if the key already exists.
func (g *ConcurrentOrderedGraph[K, V]) AddNode(id K, data V, vec []float64) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.nodes.Exists(id) {
		return ErrDuplicateNode
	}
	g.nodes.Set(id, newNode(id, data, vec))
	return nil
}

// UpsertNode adds or fully replaces a node (payload + vector).
func (g *ConcurrentOrderedGraph[K, V]) UpsertNode(id K, data V, vec []float64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if existing, ok := g.nodes.Get(id); ok {
		existing.mu.Lock()
		existing.Data = data
		if len(vec) > 0 {
			v := make([]float64, len(vec))
			copy(v, vec)
			existing.Vector = v
		}
		existing.mu.Unlock()
		return
	}
	g.nodes.Set(id, newNode(id, data, vec))
}

// GetNode retrieves a node by ID.
func (g *ConcurrentOrderedGraph[K, V]) GetNode(id K) (*GraphNode[K, V], bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.nodes.Get(id)
}

// RemoveNode deletes a node and all edges that reference it.
func (g *ConcurrentOrderedGraph[K, V]) RemoveNode(id K) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if !g.nodes.Exists(id) {
		return ErrNodeNotFound
	}
	// Remove incoming edges from all other nodes
	pairs := g.nodes.GetOrderedV2()
	for _, p := range pairs {
		if p.Key != id {
			p.Value.removeEdge(id)
		}
	}
	g.nodes.Delete(id)
	return nil
}

// NodeCount returns the number of nodes.
func (g *ConcurrentOrderedGraph[K, V]) NodeCount() int {
	return g.nodes.Len()
}

// Nodes returns all nodes in insertion order.
func (g *ConcurrentOrderedGraph[K, V]) Nodes() []*GraphNode[K, V] {
	pairs := g.nodes.GetOrderedV2()
	ns := make([]*GraphNode[K, V], len(pairs))
	for i, p := range pairs {
		ns[i] = p.Value
	}
	return ns
}

// ─── Edge operations ──────────────────────────────────────────────────────────

// AddEdge adds a directed edge from → to.
// For undirected graphs the reverse edge is also added automatically.
// Returns ErrSelfLoop or ErrNodeNotFound as appropriate.
func (g *ConcurrentOrderedGraph[K, V]) AddEdge(from, to K, e EdgeData) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if from == to {
		return ErrSelfLoop
	}
	fNode, ok := g.nodes.Get(from)
	if !ok {
		return fmt.Errorf("%w: %v", ErrNodeNotFound, from)
	}
	tNode, ok := g.nodes.Get(to)
	if !ok {
		return fmt.Errorf("%w: %v", ErrNodeNotFound, to)
	}

	fNode.addEdge(to, e)
	if !g.directed {
		tNode.addEdge(from, e)
	}
	return nil
}

// RemoveEdge removes the edge from → to. For undirected graphs the reverse
// is also removed.
func (g *ConcurrentOrderedGraph[K, V]) RemoveEdge(from, to K) error {
	g.mu.RLock()
	fNode, ok := g.nodes.Get(from)
	g.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %v", ErrNodeNotFound, from)
	}
	fNode.removeEdge(to)

	if !g.directed {
		g.mu.RLock()
		tNode, ok := g.nodes.Get(to)
		g.mu.RUnlock()
		if ok {
			tNode.removeEdge(from)
		}
	}
	return nil
}

// GetNeighbors returns the outgoing neighbour IDs of a node in insertion order.
func (g *ConcurrentOrderedGraph[K, V]) GetNeighbors(id K) ([]K, error) {
	g.mu.RLock()
	node, ok := g.nodes.Get(id)
	g.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%w: %v", ErrNodeNotFound, id)
	}
	return node.Neighbors(), nil
}

// HasEdge reports whether an edge from → to exists.
func (g *ConcurrentOrderedGraph[K, V]) HasEdge(from, to K) bool {
	g.mu.RLock()
	node, ok := g.nodes.Get(from)
	g.mu.RUnlock()
	if !ok {
		return false
	}
	_, exists := node.Edge(to)
	return exists
}
