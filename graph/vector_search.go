package graph

import (
	"fmt"
	"sort"
)

// ─── Vector Search ───────────────────────────────────────────────────────────

// VectorSearchGlobal scans every node in the graph and returns the topK nodes
// whose Vector embedding is most similar (cosine similarity) to queryVec.
//
// filter is an optional predicate over the node payload; pass nil to include
// all nodes.
//
// Results are returned in descending similarity order.
func (g *ConcurrentOrderedGraph[K, V]) VectorSearchGlobal(
	queryVec []float64,
	topK int,
	filter func(V) bool,
) ([]ScoredNode[K, V], error) {
	if len(queryVec) == 0 {
		return nil, fmt.Errorf("graph: queryVec must be non-empty")
	}
	if topK <= 0 {
		return nil, fmt.Errorf("graph: topK must be > 0")
	}

	g.mu.RLock()
	pairs := g.nodes.GetOrderedV2()
	g.mu.RUnlock()

	results := make([]ScoredNode[K, V], 0, topK*2)
	for _, p := range pairs {
		n := p.Value
		if len(n.Vector) == 0 {
			continue
		}
		if filter != nil && !filter(n.Data) {
			continue
		}
		score := cosineSim(queryVec, n.Vector)
		results = append(results, ScoredNode[K, V]{Node: n, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// VectorSearchFromNode performs a bounded BFS starting at `start`, scoring
// each visited node by cosine similarity with queryVec, and returns the topK
// most similar nodes found within maxHops hops.
//
// This is the core primitive for agentic context retrieval:
// "given the current node (e.g. last tool call, last memory), what are the
// semantically closest related concepts reachable from here?"
//
// Parameters:
//   - start    — seed node for the BFS
//   - queryVec — embedding of the query (same dimensionality as node vectors)
//   - topK     — maximum results to return
//   - maxHops  — maximum BFS depth (use -1 for unlimited)
//   - filter   — optional payload predicate; nil = no filter
func (g *ConcurrentOrderedGraph[K, V]) VectorSearchFromNode(
	start K,
	queryVec []float64,
	topK int,
	maxHops int,
	filter func(V) bool,
) ([]ScoredNode[K, V], error) {
	if len(queryVec) == 0 {
		return nil, fmt.Errorf("graph: queryVec must be non-empty")
	}
	if topK <= 0 {
		return nil, fmt.Errorf("graph: topK must be > 0")
	}

	type qItem struct {
		id  K
		hop int
	}

	visited := make(map[K]bool)
	queue := []qItem{{start, 0}}
	visited[start] = true

	candidates := make([]ScoredNode[K, V], 0, topK*4)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		g.mu.RLock()
		node, ok := g.nodes.Get(cur.id)
		g.mu.RUnlock()
		if !ok {
			continue
		}

		// Score this node if it has a vector and passes the filter
		if len(node.Vector) > 0 && (filter == nil || filter(node.Data)) {
			score := cosineSim(queryVec, node.Vector)
			candidates = append(candidates, ScoredNode[K, V]{Node: node, Score: score})
		}

		// Expand neighbours unless depth limit reached
		if maxHops >= 0 && cur.hop >= maxHops {
			continue
		}
		for _, nb := range node.Neighbors() {
			if !visited[nb] {
				visited[nb] = true
				queue = append(queue, qItem{nb, cur.hop + 1})
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	return candidates, nil
}

// VectorSearchBidirectional expands the search in both directions on an
// undirected or directed graph by also following reverse edges. Useful for
// retrieving the broader context of a node (parents + children + siblings).
func (g *ConcurrentOrderedGraph[K, V]) VectorSearchBidirectional(
	start K,
	queryVec []float64,
	topK int,
	maxHops int,
	filter func(V) bool,
) ([]ScoredNode[K, V], error) {
	if len(queryVec) == 0 {
		return nil, fmt.Errorf("graph: queryVec must be non-empty")
	}

	// Build reverse adjacency on the fly for directed graphs
	g.mu.RLock()
	pairs := g.nodes.GetOrderedV2()
	reverseEdges := make(map[K][]K, len(pairs))
	for _, p := range pairs {
		for _, nb := range p.Value.Neighbors() {
			reverseEdges[nb] = append(reverseEdges[nb], p.Key)
		}
	}
	g.mu.RUnlock()

	type qItem struct {
		id  K
		hop int
	}

	visited := make(map[K]bool)
	queue := []qItem{{start, 0}}
	visited[start] = true
	candidates := make([]ScoredNode[K, V], 0, topK*4)

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		g.mu.RLock()
		node, ok := g.nodes.Get(cur.id)
		g.mu.RUnlock()
		if !ok {
			continue
		}

		if len(node.Vector) > 0 && (filter == nil || filter(node.Data)) {
			score := cosineSim(queryVec, node.Vector)
			candidates = append(candidates, ScoredNode[K, V]{Node: node, Score: score})
		}

		if maxHops >= 0 && cur.hop >= maxHops {
			continue
		}
		// Forward edges
		for _, nb := range node.Neighbors() {
			if !visited[nb] {
				visited[nb] = true
				queue = append(queue, qItem{nb, cur.hop + 1})
			}
		}
		// Reverse edges
		for _, rev := range reverseEdges[cur.id] {
			if !visited[rev] {
				visited[rev] = true
				queue = append(queue, qItem{rev, cur.hop + 1})
			}
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > topK {
		candidates = candidates[:topK]
	}
	return candidates, nil
}

// SemanticNeighbors is a convenience wrapper that returns the direct neighbours
// of `nodeID` ranked by semantic similarity to queryVec — no BFS, just the
// immediate 1-hop neighbourhood.
func (g *ConcurrentOrderedGraph[K, V]) SemanticNeighbors(
	nodeID K,
	queryVec []float64,
	topK int,
) ([]ScoredNode[K, V], error) {
	return g.VectorSearchFromNode(nodeID, queryVec, topK, 1, nil)
}
