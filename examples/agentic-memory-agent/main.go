// Example: Persistent Agent Memory
//
// Demonstrates how to combine PersistentConcurrentOrderedMap with the
// Graph to build an agent that:
//  1. Persists every memory to disk (survives restarts)
//  2. Searches memories by semantic similarity (vector index)
//  3. Builds a relationship graph between memories over time
//
// Run it twice to see persistence in action:
//
//	go run main.go         # populates the store
//	go run main.go         # loads from disk, data is there
package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/Dsouza10082/ConcurrentOrderedMap/graph"
	"github.com/Dsouza10082/ConcurrentOrderedMap/persistence"
)

// ─── Memory entry stored in the persistent map ───────────────────────────────

type MemoryEntry struct {
	ID        string
	Content   string
	Category  string // "episodic" | "semantic" | "procedural"
	CreatedAt time.Time
}

// ─── Persistent Memory Agent ─────────────────────────────────────────────────

type PersistentAgent struct {
	store   *persistence.PersistentConcurrentOrderedMap[string, MemoryEntry]
	graph   *graph.ConcurrentOrderedGraph[string, MemoryEntry]
	embedFn func(string) []float64
	count   int
}

func NewPersistentAgent(dataDir string, embedFn func(string) []float64) (*PersistentAgent, error) {
	opts := persistence.DefaultOptions()
	opts.VectorDims = 64 // use 1024 in production
	opts.SnapshotInterval = 2 * time.Minute
	opts.SyncOnWrite = false

	store, err := persistence.New[string, MemoryEntry](dataDir, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open store: %w", err)
	}

	a := &PersistentAgent{
		store:   store,
		graph:   graph.NewDirectedGraph[string, MemoryEntry](),
		embedFn: embedFn,
	}

	// Rebuild the in-memory graph from persisted data on startup
	a.rebuildGraph()
	return a, nil
}

// Remember persists a new memory and indexes it.
func (a *PersistentAgent) Remember(content, category string) string {
	a.count++
	id := fmt.Sprintf("mem-%04d-%d", a.count, time.Now().UnixMilli())
	entry := MemoryEntry{
		ID:        id,
		Content:   content,
		Category:  category,
		CreatedAt: time.Now(),
	}
	vec := a.embedFn(content)

	// Persist to disk
	_ = a.store.SetWithVector(id, entry, vec)

	// Index in the in-memory graph
	_ = a.graph.AddNode(id, entry, vec)

	// Auto-link to the most similar existing memory
	a.autoLink(id, vec)

	fmt.Printf("[REMEMBER] %s (%s): %q\n", id, category, content)
	return id
}

// Recall performs a semantic search over persisted memories.
func (a *PersistentAgent) Recall(query string, topK int) {
	vec := a.embedFn(query)
	results, err := a.store.VectorSearch(vec, topK, nil)
	if err != nil {
		fmt.Printf("[RECALL ERROR] %v\n", err)
		return
	}
	fmt.Printf("\n[RECALL] query=%q top-%d:\n", query, topK)
	for i, r := range results {
		entry, _ := a.store.Get(r.Key)
		fmt.Printf("  %d. score=%.4f [%s] %q\n", i+1, r.Score, entry.Category, entry.Content)
	}
}

// RecallFromContext retrieves memories reachable from an anchor node.
func (a *PersistentAgent) RecallFromContext(anchorID, query string, topK, hops int) {
	vec := a.embedFn(query)
	results, err := a.graph.VectorSearchFromNode(anchorID, vec, topK, hops, nil)
	if err != nil {
		fmt.Printf("[RECALL CTX ERROR] %v\n", err)
		return
	}
	fmt.Printf("\n[RECALL CTX] anchor=%s query=%q top-%d within %d hops:\n", anchorID, query, topK, hops)
	for i, r := range results {
		fmt.Printf("  %d. score=%.4f [%s] %q\n", i+1, r.Score, r.Node.Data.Category, r.Node.Data.Content)
	}
}

// Close flushes and closes the persistent store.
func (a *PersistentAgent) Close() error {
	return a.store.Close()
}

// Stats prints memory statistics.
func (a *PersistentAgent) Stats() {
	fmt.Printf("\n[STATS] Total memories: %d | Graph nodes: %d\n",
		a.store.Len(), a.graph.NodeCount())
	comps := a.graph.ConnectedComponents()
	fmt.Printf("[STATS] Graph components: %d\n", len(comps))
}

// rebuildGraph loads all persisted entries into the in-memory graph.
func (a *PersistentAgent) rebuildGraph() {
	pairs := a.store.Inner().GetOrderedV2()
	if len(pairs) == 0 {
		return
	}
	fmt.Printf("[STARTUP] Rebuilding graph from %d persisted memories...\n", len(pairs))
	for _, p := range pairs {
		entry := p.Value
		// Fetch vector from the vector index (we need to re-embed or store alongside)
		// For this example we re-embed on load; in production store vectors in the entry.
		vec := a.embedFn(entry.Content)
		_ = a.graph.AddNode(entry.ID, entry, vec)
		a.count++
	}
	// Rebuild auto-links
	nodes := a.graph.Nodes()
	for _, n := range nodes {
		a.autoLink(n.ID, n.Vector)
	}
	fmt.Printf("[STARTUP] Graph rebuilt.\n\n")
}

// autoLink connects a node to the most similar neighbour above threshold.
func (a *PersistentAgent) autoLink(newID string, vec []float64) {
	if a.graph.NodeCount() < 2 {
		return
	}
	results, err := a.graph.VectorSearchGlobal(vec, 3, nil)
	if err != nil {
		return
	}
	for _, r := range results {
		if r.Node.ID == newID || r.Score < 0.70 {
			continue
		}
		if !a.graph.HasEdge(r.Node.ID, newID) {
			_ = a.graph.AddEdge(r.Node.ID, newID, graph.EdgeData{
				Label:  "related",
				Weight: 1 - r.Score,
			})
		}
		break
	}
}

// ─── Mock embedding (replace with real embedding provider) ───────────────────

func mockEmbed(text string) []float64 {
	const dims = 64
	vec := make([]float64, dims)
	r := rand.New(rand.NewSource(hashStr(text)))
	for i := range vec {
		vec[i] = r.NormFloat64()
	}
	var norm float64
	for _, v := range vec {
		norm += v * v
	}
	norm = math.Sqrt(norm)
	for i := range vec {
		vec[i] /= norm
	}
	return vec
}

func hashStr(s string) int64 {
	var h int64 = 5381
	for _, c := range s {
		h = h*33 + int64(c)
	}
	return h
}

// ─── Main ─────────────────────────────────────────────────────────────────────

func main() {
	dataDir := filepath.Join(os.TempDir(), "agent-memory-demo")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("   Persistent Memory Agent with Vector Search")
	fmt.Printf("   Data directory: %s\n", dataDir)
	fmt.Println("═══════════════════════════════════════════════════════\n")

	agent, err := NewPersistentAgent(dataDir, mockEmbed)
	if err != nil {
		panic(err)
	}
	defer agent.Close()

	// ── Store episodic memories ───────────────────────────────────────────
	fmt.Println("── Storing Memories ──")
	mem1 := agent.Remember("Deployed new microservice to production at 14:30", "episodic")
	mem2 := agent.Remember("Received alert: database connection pool exhausted", "episodic")
	mem3 := agent.Remember("Database connections must never exceed 100 simultaneous", "semantic")
	mem4 := agent.Remember("To fix connection pool: increase pool size or add connection timeout", "procedural")
	mem5 := agent.Remember("User reported slow API responses after 14:30 deployment", "episodic")
	mem6 := agent.Remember("Root cause: new microservice opened connections without closing them", "semantic")
	_ = mem1
	_ = mem2
	_ = mem3
	_ = mem4
	_ = mem5
	_ = mem6

	fmt.Println()

	// ── Semantic search ───────────────────────────────────────────────────
	agent.Recall("database connection problem", 3)
	agent.Recall("deployment issue production", 3)

	// ── Context-anchored search ───────────────────────────────────────────
	agent.RecallFromContext(mem2, "how to fix database exhaustion", 3, 3)

	// ── Stats ─────────────────────────────────────────────────────────────
	agent.Stats()

	// ── Shortest path reasoning ───────────────────────────────────────────
	fmt.Println()
	result, err := agent.graph.ShortestPath(mem2, mem4)
	if err == nil {
		fmt.Printf("[REASONING] Path from alert to fix: %v (cost=%.2f)\n", result.Nodes, result.Distance)
	}

	fmt.Println("\n═══════════════════════════════════════════════════════")
	fmt.Println("  Run again to see memories loaded from disk!")
	fmt.Println("═══════════════════════════════════════════════════════")
}
