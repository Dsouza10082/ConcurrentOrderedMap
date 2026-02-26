// Package graph provides a thread-safe, ordered directed/undirected graph
// built on top of ConcurrentOrderedMap. Each node can carry an arbitrary
// payload and an optional embedding vector, enabling graph traversal
// combined with semantic vector search — ideal for knowledge graphs,
// agentic memory systems, and RAG pipelines.
package graph

import (
	"fmt"
	"math"
	"sync"

	com "github.com/Dsouza10082/ConcurrentOrderedMap"
)

// EdgeData holds the metadata associated with a directed connection between
// two nodes. Weight is used by Dijkstra; Label and Metadata are free-form.
type EdgeData struct {
	Weight   float64
	Label    string
	Metadata map[string]any
}

// GraphNode is a single vertex in the graph.
// It stores the caller's payload (Data), an optional embedding Vector, and
// its adjacency list as a ConcurrentOrderedMap so that neighbours are
// themselves thread-safe and ordered by insertion time.
type GraphNode[K comparable, V any] struct {
	ID     K
	Data   V
	Vector []float64 // optional; nil means "no embedding"

	mu    sync.RWMutex
	edges *com.ConcurrentOrderedMap[K, EdgeData]
}

// newNode allocates a GraphNode. vec may be nil.
func newNode[K comparable, V any](id K, data V, vec []float64) *GraphNode[K, V] {
	var v []float64
	if len(vec) > 0 {
		v = make([]float64, len(vec))
		copy(v, vec)
	}
	return &GraphNode[K, V]{
		ID:     id,
		Data:   data,
		Vector: v,
		edges:  com.NewConcurrentOrderedMap[K, EdgeData](),
	}
}

// addEdge adds or replaces an outgoing edge.
func (n *GraphNode[K, V]) addEdge(to K, e EdgeData) {
	n.edges.Set(to, e)
}

// removeEdge removes an outgoing edge (no-op if absent).
func (n *GraphNode[K, V]) removeEdge(to K) {
	n.edges.Delete(to)
}

// Neighbors returns all outgoing neighbour keys in insertion order.
func (n *GraphNode[K, V]) Neighbors() []K {
	pairs := n.edges.GetOrderedV2()
	ks := make([]K, len(pairs))
	for i, p := range pairs {
		ks[i] = p.Key
	}
	return ks
}

// Edge returns the EdgeData for a specific neighbour, if it exists.
func (n *GraphNode[K, V]) Edge(to K) (EdgeData, bool) {
	return n.edges.Get(to)
}

// Degree returns the number of outgoing edges.
func (n *GraphNode[K, V]) Degree() int {
	return n.edges.Len()
}

// UpdateData mutates the payload under a write lock.
func (n *GraphNode[K, V]) UpdateData(fn func(*V) error) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	return fn(&n.Data)
}

// UpdateVector replaces the embedding under a write lock.
func (n *GraphNode[K, V]) UpdateVector(vec []float64) {
	n.mu.Lock()
	defer n.mu.Unlock()
	v := make([]float64, len(vec))
	copy(v, vec)
	n.Vector = v
}

// ScoredNode is a query result: a node together with its similarity score.
type ScoredNode[K comparable, V any] struct {
	Node  *GraphNode[K, V]
	Score float64
}

// String implements fmt.Stringer for debug output.
func (s ScoredNode[K, V]) String() string {
	return fmt.Sprintf("ScoredNode{ID:%v Score:%.4f}", s.Node.ID, s.Score)
}

// PathResult holds a shortest-path result from Dijkstra.
type PathResult[K comparable] struct {
	Nodes    []K
	Distance float64
}

// cosineSim computes the cosine similarity between two equal-length vectors.
// Returns 0 if either vector has zero norm or lengths differ.
func cosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
