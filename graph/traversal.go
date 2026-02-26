package graph

import (
	"container/heap"
	"fmt"
)

// ─── BFS ─────────────────────────────────────────────────────────────────────

// BFS performs a breadth-first traversal starting from `start`.
// The visitor function receives (nodeID, nodeData) for every reachable node.
// Return false from the visitor to stop traversal early.
func (g *ConcurrentOrderedGraph[K, V]) BFS(start K, visit func(K, V) bool) error {
	g.mu.RLock()
	_, ok := g.nodes.Get(start)
	g.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %v", ErrNodeNotFound, start)
	}

	visited := make(map[K]bool)
	queue := []K{start}
	visited[start] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		g.mu.RLock()
		node, _ := g.nodes.Get(cur)
		g.mu.RUnlock()

		if !visit(cur, node.Data) {
			return nil
		}

		for _, nb := range node.Neighbors() {
			if !visited[nb] {
				visited[nb] = true
				queue = append(queue, nb)
			}
		}
	}
	return nil
}

// BFSWithDepth is BFS with hop-depth limit. Useful for bounded graph exploration
// in agentic scenarios where you want "context within N hops".
func (g *ConcurrentOrderedGraph[K, V]) BFSWithDepth(
	start K,
	maxDepth int,
	visit func(id K, data V, depth int) bool,
) error {
	g.mu.RLock()
	_, ok := g.nodes.Get(start)
	g.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %v", ErrNodeNotFound, start)
	}

	type item struct {
		id    K
		depth int
	}
	visited := make(map[K]bool)
	queue := []item{{start, 0}}
	visited[start] = true

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		g.mu.RLock()
		node, _ := g.nodes.Get(cur.id)
		g.mu.RUnlock()

		if !visit(cur.id, node.Data, cur.depth) {
			return nil
		}

		if cur.depth >= maxDepth {
			continue
		}

		for _, nb := range node.Neighbors() {
			if !visited[nb] {
				visited[nb] = true
				queue = append(queue, item{nb, cur.depth + 1})
			}
		}
	}
	return nil
}

// ─── DFS ─────────────────────────────────────────────────────────────────────

// DFS performs a depth-first traversal starting from `start`.
// The visitor function receives (nodeID, nodeData).
// Return false to stop early.
func (g *ConcurrentOrderedGraph[K, V]) DFS(start K, visit func(K, V) bool) error {
	g.mu.RLock()
	_, ok := g.nodes.Get(start)
	g.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %v", ErrNodeNotFound, start)
	}

	visited := make(map[K]bool)
	var dfs func(id K) bool
	dfs = func(id K) bool {
		if visited[id] {
			return true
		}
		visited[id] = true

		g.mu.RLock()
		node, _ := g.nodes.Get(id)
		g.mu.RUnlock()

		if !visit(id, node.Data) {
			return false
		}
		for _, nb := range node.Neighbors() {
			if !dfs(nb) {
				return false
			}
		}
		return true
	}
	dfs(start)
	return nil
}

// ─── Dijkstra ────────────────────────────────────────────────────────────────

// dijkstraItem is a priority-queue entry.
type dijkstraItem[K comparable] struct {
	id   K
	dist float64
	idx  int
}

type dijkstraPQ[K comparable] []*dijkstraItem[K]

func (pq dijkstraPQ[K]) Len() int            { return len(pq) }
func (pq dijkstraPQ[K]) Less(i, j int) bool  { return pq[i].dist < pq[j].dist }
func (pq dijkstraPQ[K]) Swap(i, j int)       { pq[i], pq[j] = pq[j], pq[i]; pq[i].idx = i; pq[j].idx = j }
func (pq *dijkstraPQ[K]) Push(x any)         { item := x.(*dijkstraItem[K]); item.idx = len(*pq); *pq = append(*pq, item) }
func (pq *dijkstraPQ[K]) Pop() any           { old := *pq; n := len(old); item := old[n-1]; *pq = old[:n-1]; return item }

// ShortestPath finds the shortest (lowest-weight) path between `from` and `to`
// using Dijkstra's algorithm. Edge weights must be non-negative.
// Returns PathResult with the ordered node IDs and total distance.
func (g *ConcurrentOrderedGraph[K, V]) ShortestPath(from, to K) (PathResult[K], error) {
	g.mu.RLock()
	_, okF := g.nodes.Get(from)
	_, okT := g.nodes.Get(to)
	g.mu.RUnlock()

	if !okF {
		return PathResult[K]{}, fmt.Errorf("%w: %v", ErrNodeNotFound, from)
	}
	if !okT {
		return PathResult[K]{}, fmt.Errorf("%w: %v", ErrNodeNotFound, to)
	}

	const inf = 1e18
	dist := map[K]float64{from: 0}
	prev := make(map[K]K)
	inPrev := make(map[K]bool) // track which nodes have a predecessor

	pq := &dijkstraPQ[K]{{id: from, dist: 0}}
	heap.Init(pq)

	for pq.Len() > 0 {
		cur := heap.Pop(pq).(*dijkstraItem[K])

		if cur.dist > dist[cur.id] {
			continue // stale entry
		}
		if cur.id == to {
			break
		}

		g.mu.RLock()
		node, _ := g.nodes.Get(cur.id)
		g.mu.RUnlock()

		for _, nb := range node.Neighbors() {
			edge, _ := node.Edge(nb)
			nd := cur.dist + edge.Weight
			if d, ok := dist[nb]; !ok || nd < d {
				dist[nb] = nd
				prev[nb] = cur.id
				inPrev[nb] = true
				heap.Push(pq, &dijkstraItem[K]{id: nb, dist: nd})
			}
		}
	}

	if _, reached := dist[to]; !reached {
		return PathResult[K]{}, fmt.Errorf("graph: no path from %v to %v", from, to)
	}

	// Reconstruct path
	var path []K
	for cur := to; cur != from || len(path) == 0; {
		path = append([]K{cur}, path...)
		if !inPrev[cur] {
			break
		}
		cur = prev[cur]
	}
	// Ensure 'from' is included
	if len(path) == 0 || path[0] != from {
		path = append([]K{from}, path...)
	}

	return PathResult[K]{Nodes: path, Distance: dist[to]}, nil
}

// AllReachable returns every node reachable from `start` (BFS order).
func (g *ConcurrentOrderedGraph[K, V]) AllReachable(start K) ([]K, error) {
	var result []K
	err := g.BFS(start, func(id K, _ V) bool {
		result = append(result, id)
		return true
	})
	return result, err
}

// ConnectedComponents returns groups of mutually-reachable node IDs.
// Useful for detecting isolated sub-graphs in an agent's knowledge base.
func (g *ConcurrentOrderedGraph[K, V]) ConnectedComponents() [][]K {
	g.mu.RLock()
	pairs := g.nodes.GetOrderedV2()
	g.mu.RUnlock()

	visited := make(map[K]bool)
	var components [][]K

	for _, p := range pairs {
		if visited[p.Key] {
			continue
		}
		var component []K
		_ = g.BFS(p.Key, func(id K, _ V) bool {
			visited[id] = true
			component = append(component, id)
			return true
		})
		components = append(components, component)
	}
	return components
}
