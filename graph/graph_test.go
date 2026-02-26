package graph

import (
	"testing"
)

func makeVec(seed float64, dims int) []float64 {
	v := make([]float64, dims)
	for i := range v {
		v[i] = seed + float64(i)*0.01
	}
	return v
}

// ─── Node operations ─────────────────────────────────────────────────────────

func TestAddAndGetNode(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	if err := g.AddNode("a", "hello", nil); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	node, ok := g.GetNode("a")
	if !ok {
		t.Fatal("GetNode: expected node to exist")
	}
	if node.Data != "hello" {
		t.Fatalf("expected data %q, got %q", "hello", node.Data)
	}
}

func TestDuplicateNodeReturnsError(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("a", "first", nil)
	err := g.AddNode("a", "second", nil)
	if err != ErrDuplicateNode {
		t.Fatalf("expected ErrDuplicateNode, got %v", err)
	}
}

func TestUpsertNode(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	g.UpsertNode("a", "v1", nil)
	g.UpsertNode("a", "v2", nil)
	node, _ := g.GetNode("a")
	if node.Data != "v2" {
		t.Fatalf("expected upserted value v2, got %q", node.Data)
	}
}

func TestRemoveNode(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("a", "A", nil)
	_ = g.AddNode("b", "B", nil)
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	if err := g.RemoveNode("b"); err != nil {
		t.Fatalf("RemoveNode: %v", err)
	}
	if g.NodeCount() != 1 {
		t.Fatalf("expected 1 node after removal, got %d", g.NodeCount())
	}
	// Edge should also be gone
	if g.HasEdge("a", "b") {
		t.Fatal("edge to removed node should be gone")
	}
}

func TestRemoveNonExistentNode(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	if err := g.RemoveNode("ghost"); err != ErrNodeNotFound {
		t.Fatalf("expected ErrNodeNotFound, got %v", err)
	}
}

// ─── Edge operations ─────────────────────────────────────────────────────────

func TestAddAndHasEdge(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("a", "A", nil)
	_ = g.AddNode("b", "B", nil)
	_ = g.AddEdge("a", "b", EdgeData{Label: "link", Weight: 2.5})
	if !g.HasEdge("a", "b") {
		t.Fatal("expected edge a→b to exist")
	}
	if g.HasEdge("b", "a") {
		t.Fatal("directed graph: reverse edge should not exist")
	}
}

func TestUndirectedEdge(t *testing.T) {
	g := NewUndirectedGraph[string, string]()
	_ = g.AddNode("a", "A", nil)
	_ = g.AddNode("b", "B", nil)
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	if !g.HasEdge("a", "b") || !g.HasEdge("b", "a") {
		t.Fatal("undirected: both directions should exist")
	}
}

func TestSelfLoopRejected(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("a", "A", nil)
	if err := g.AddEdge("a", "a", EdgeData{}); err != ErrSelfLoop {
		t.Fatalf("expected ErrSelfLoop, got %v", err)
	}
}

func TestRemoveEdge(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("a", "A", nil)
	_ = g.AddNode("b", "B", nil)
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	_ = g.RemoveEdge("a", "b")
	if g.HasEdge("a", "b") {
		t.Fatal("edge should be removed")
	}
}

func TestGetNeighbors(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("a", "A", nil)
	_ = g.AddNode("b", "B", nil)
	_ = g.AddNode("c", "C", nil)
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	_ = g.AddEdge("a", "c", EdgeData{Weight: 1})
	nb, err := g.GetNeighbors("a")
	if err != nil {
		t.Fatal(err)
	}
	if len(nb) != 2 {
		t.Fatalf("expected 2 neighbours, got %d", len(nb))
	}
}

// ─── Nodes() order ────────────────────────────────────────────────────────────

func TestInsertionOrder(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	keys := []string{"z", "a", "m", "b"}
	for _, k := range keys {
		_ = g.AddNode(k, k, nil)
	}
	nodes := g.Nodes()
	for i, n := range nodes {
		if n.ID != keys[i] {
			t.Fatalf("position %d: expected %q, got %q", i, keys[i], n.ID)
		}
	}
}

// ─── BFS / DFS ────────────────────────────────────────────────────────────────

func TestBFS(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	for _, k := range []string{"a", "b", "c", "d"} {
		_ = g.AddNode(k, k, nil)
	}
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	_ = g.AddEdge("a", "c", EdgeData{Weight: 1})
	_ = g.AddEdge("b", "d", EdgeData{Weight: 1})

	visited := map[string]bool{}
	_ = g.BFS("a", func(id string, _ string) bool {
		visited[id] = true
		return true
	})
	for _, k := range []string{"a", "b", "c", "d"} {
		if !visited[k] {
			t.Fatalf("BFS missed node %q", k)
		}
	}
}

func TestBFSEarlyStop(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	for _, k := range []string{"a", "b", "c"} {
		_ = g.AddNode(k, k, nil)
	}
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	_ = g.AddEdge("b", "c", EdgeData{Weight: 1})

	count := 0
	_ = g.BFS("a", func(_ string, _ string) bool {
		count++
		return count < 2 // stop after 2 visits
	})
	if count != 2 {
		t.Fatalf("expected early stop at 2, got %d", count)
	}
}

func TestBFSWithDepth(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	for _, k := range []string{"a", "b", "c", "d"} {
		_ = g.AddNode(k, k, nil)
	}
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	_ = g.AddEdge("b", "c", EdgeData{Weight: 1})
	_ = g.AddEdge("c", "d", EdgeData{Weight: 1})

	visited := map[string]bool{}
	_ = g.BFSWithDepth("a", 2, func(id string, _ string, depth int) bool {
		visited[id] = true
		return true
	})
	if visited["d"] {
		t.Fatal("depth=2 BFS should not reach node d (depth 3)")
	}
	if !visited["c"] {
		t.Fatal("depth=2 BFS should reach node c")
	}
}

func TestDFS(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	for _, k := range []string{"a", "b", "c"} {
		_ = g.AddNode(k, k, nil)
	}
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	_ = g.AddEdge("b", "c", EdgeData{Weight: 1})

	visited := map[string]bool{}
	_ = g.DFS("a", func(id string, _ string) bool {
		visited[id] = true
		return true
	})
	for _, k := range []string{"a", "b", "c"} {
		if !visited[k] {
			t.Fatalf("DFS missed node %q", k)
		}
	}
}

// ─── Connected components ─────────────────────────────────────────────────────

func TestConnectedComponents(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	// Component 1: a-b
	_ = g.AddNode("a", "a", nil)
	_ = g.AddNode("b", "b", nil)
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	// Component 2: c (isolated)
	_ = g.AddNode("c", "c", nil)

	comps := g.ConnectedComponents()
	if len(comps) != 2 {
		t.Fatalf("expected 2 components, got %d", len(comps))
	}
}

// ─── Vector search ────────────────────────────────────────────────────────────

func TestVectorSearchGlobal(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	query := makeVec(1.0, 8)
	_ = g.AddNode("near", "near", makeVec(1.0, 8))
	_ = g.AddNode("far", "far", makeVec(9.0, 8))

	results, err := g.VectorSearchGlobal(query, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}
	if results[0].Node.ID != "near" {
		t.Fatalf("expected nearest node to be 'near', got %q", results[0].Node.ID)
	}
}

func TestVectorSearchFromNode(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("root", "root", makeVec(1.0, 8))
	_ = g.AddNode("child1", "child1", makeVec(1.1, 8))
	_ = g.AddNode("child2", "child2", makeVec(5.0, 8))
	_ = g.AddNode("orphan", "orphan", makeVec(1.0, 8)) // not connected
	_ = g.AddEdge("root", "child1", EdgeData{Weight: 1})
	_ = g.AddEdge("root", "child2", EdgeData{Weight: 1})

	results, err := g.VectorSearchFromNode("root", makeVec(1.0, 8), 5, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, r := range results {
		ids[r.Node.ID] = true
	}
	if ids["orphan"] {
		t.Fatal("orphan node should not appear in bounded search from root")
	}
}

func TestVectorSearchFilter(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("keep", "keep", makeVec(1.0, 8))
	_ = g.AddNode("skip", "skip", makeVec(1.0, 8))

	results, err := g.VectorSearchGlobal(makeVec(1.0, 8), 5, func(v string) bool {
		return v == "keep"
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results {
		if r.Node.ID == "skip" {
			t.Fatal("filtered node 'skip' should not appear in results")
		}
	}
}

// ─── Dijkstra ─────────────────────────────────────────────────────────────────

func TestShortestPath(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	for _, k := range []string{"a", "b", "c", "d"} {
		_ = g.AddNode(k, k, nil)
	}
	_ = g.AddEdge("a", "b", EdgeData{Weight: 1})
	_ = g.AddEdge("b", "c", EdgeData{Weight: 1})
	_ = g.AddEdge("a", "d", EdgeData{Weight: 10})
	_ = g.AddEdge("d", "c", EdgeData{Weight: 1})

	result, err := g.ShortestPath("a", "c")
	if err != nil {
		t.Fatal(err)
	}
	// Shortest: a→b→c (cost 2) vs a→d→c (cost 11)
	if result.Distance != 2.0 {
		t.Fatalf("expected distance 2.0, got %.1f", result.Distance)
	}
	if len(result.Nodes) != 3 {
		t.Fatalf("expected path length 3, got %d: %v", len(result.Nodes), result.Nodes)
	}
}

func TestShortestPathNoRoute(t *testing.T) {
	g := NewDirectedGraph[string, string]()
	_ = g.AddNode("a", "a", nil)
	_ = g.AddNode("b", "b", nil)
	// No edge between them
	_, err := g.ShortestPath("a", "b")
	if err == nil {
		t.Fatal("expected error for disconnected nodes")
	}
}

// ─── Concurrency smoke test ────────────────────────────────────────────────────

func TestConcurrentAddNodes(t *testing.T) {
	g := NewDirectedGraph[int, string]()
	done := make(chan struct{})
	for i := 0; i < 100; i++ {
		go func(id int) {
			_ = g.AddNode(id, "v", nil)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 100; i++ {
		<-done
	}
	if g.NodeCount() != 100 {
		t.Fatalf("expected 100 nodes, got %d", g.NodeCount())
	}
}
