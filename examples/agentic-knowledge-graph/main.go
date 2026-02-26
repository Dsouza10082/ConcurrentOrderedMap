// Example: Autonomous Agent with Knowledge Graph Memory
//
// This example demonstrates an Agent that:
//  1. Stores its observations and tool-call results as nodes in a knowledge graph
//  2. Connects related observations with typed edges
//  3. Uses vector search to retrieve semantically relevant context before each action
//  4. Navigates the graph (BFS/Dijkstra) to find reasoning chains
//
// This is the foundational pattern for memory-augmented autonomous agents.
// In production you would replace mockEmbed() with a real embedding call
// (e.g., BGE-M3 via Ollama or OpenAI text-embedding-3-large).
package main

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/Dsouza10082/ConcurrentOrderedMap/graph"
)

// ─── Domain types ─────────────────────────────────────────────────────────────

type NodeType string

const (
	TypeObservation NodeType = "observation"
	TypeAction      NodeType = "action"
	TypeToolResult  NodeType = "tool_result"
	TypeGoal        NodeType = "goal"
	TypeFact        NodeType = "fact"
)

// AgentMemoryNode is the payload stored in each graph node.
type AgentMemoryNode struct {
	Type      NodeType
	Content   string
	Source    string    // which tool / observation produced this
	Timestamp time.Time
	Metadata  map[string]any
}

// ─── Agent ────────────────────────────────────────────────────────────────────

// MemoryAgent is an autonomous agent backed by a knowledge graph.
type MemoryAgent struct {
	memory    *graph.ConcurrentOrderedGraph[string, AgentMemoryNode]
	nodeCount int
	embedFn   func(text string) []float64
}

// NewMemoryAgent creates an agent with an embed function.
// Replace embedFn with your real embedding provider.
func NewMemoryAgent(embedFn func(string) []float64) *MemoryAgent {
	return &MemoryAgent{
		memory:  graph.NewDirectedGraph[string, AgentMemoryNode](),
		embedFn: embedFn,
	}
}

// newID generates a simple sequential node ID.
func (a *MemoryAgent) newID(prefix string) string {
	a.nodeCount++
	return fmt.Sprintf("%s-%04d", prefix, a.nodeCount)
}

// Observe records an external observation into the knowledge graph.
// It automatically connects the new node to the most semantically related
// existing node (if any).
func (a *MemoryAgent) Observe(content, source string, meta map[string]any) string {
	id := a.newID("obs")
	node := AgentMemoryNode{
		Type:      TypeObservation,
		Content:   content,
		Source:    source,
		Timestamp: time.Now(),
		Metadata:  meta,
	}
	vec := a.embedFn(content)
	_ = a.memory.AddNode(id, node, vec)

	// Auto-connect to most related existing observation
	a.autoLink(id, vec, 0.75)
	fmt.Printf("[OBSERVE] %s: %q\n", id, content)
	return id
}

// Act records an action the agent took and links it to its triggering context.
func (a *MemoryAgent) Act(description, triggerNodeID string) string {
	id := a.newID("act")
	node := AgentMemoryNode{
		Type:      TypeAction,
		Content:   description,
		Timestamp: time.Now(),
	}
	vec := a.embedFn(description)
	_ = a.memory.AddNode(id, node, vec)

	if triggerNodeID != "" {
		_ = a.memory.AddEdge(triggerNodeID, id, graph.EdgeData{
			Label:  "triggered",
			Weight: 1.0,
		})
	}
	fmt.Printf("[ACT]     %s: %q\n", id, description)
	return id
}

// StoreFact persists a derived fact and connects it to its sources.
func (a *MemoryAgent) StoreFact(fact string, sourceIDs []string) string {
	id := a.newID("fact")
	node := AgentMemoryNode{
		Type:      TypeFact,
		Content:   fact,
		Timestamp: time.Now(),
	}
	vec := a.embedFn(fact)
	_ = a.memory.AddNode(id, node, vec)

	for _, src := range sourceIDs {
		_ = a.memory.AddEdge(src, id, graph.EdgeData{
			Label:  "supports",
			Weight: 1.0,
		})
	}
	fmt.Printf("[FACT]    %s: %q\n", id, fact)
	return id
}

// QueryContext retrieves the topK most semantically relevant memories
// reachable within maxHops from the given anchor node.
// This is what an agent calls before generating its next action.
func (a *MemoryAgent) QueryContext(query, anchorID string, topK, maxHops int) []graph.ScoredNode[string, AgentMemoryNode] {
	queryVec := a.embedFn(query)
	results, err := a.memory.VectorSearchFromNode(anchorID, queryVec, topK, maxHops, nil)
	if err != nil {
		// Fall back to global search if anchor not found
		results, _ = a.memory.VectorSearchGlobal(queryVec, topK, nil)
	}
	return results
}

// QueryGlobal does a full-graph semantic search (no anchor).
func (a *MemoryAgent) QueryGlobal(query string, topK int) []graph.ScoredNode[string, AgentMemoryNode] {
	queryVec := a.embedFn(query)
	results, _ := a.memory.VectorSearchGlobal(queryVec, topK, nil)
	return results
}

// ReasoningChain finds the shortest path between two memories, returning the
// chain of nodes that connects them — useful for explaining why two concepts
// are related in the agent's memory.
func (a *MemoryAgent) ReasoningChain(fromID, toID string) ([]string, float64) {
	result, err := a.memory.ShortestPath(fromID, toID)
	if err != nil {
		return nil, 0
	}
	return result.Nodes, result.Distance
}

// Explore returns all nodes reachable from `startID` within `hops` hops, BFS-ordered.
func (a *MemoryAgent) Explore(startID string, hops int) []AgentMemoryNode {
	var nodes []AgentMemoryNode
	_ = a.memory.BFSWithDepth(startID, hops, func(id string, data AgentMemoryNode, depth int) bool {
		nodes = append(nodes, data)
		return true
	})
	return nodes
}

// autoLink connects a new node to the most similar existing node if the
// similarity exceeds threshold.
func (a *MemoryAgent) autoLink(newID string, vec []float64, threshold float64) {
	if a.memory.NodeCount() < 2 {
		return
	}
	results, err := a.memory.VectorSearchGlobal(vec, 2, func(n AgentMemoryNode) bool {
		return true // include all
	})
	if err != nil || len(results) == 0 {
		return
	}
	// results[0] is the new node itself (score=1.0); skip it
	for _, r := range results {
		if r.Node.ID == newID {
			continue
		}
		if r.Score >= threshold {
			_ = a.memory.AddEdge(r.Node.ID, newID, graph.EdgeData{
				Label:  "related",
				Weight: 1 - r.Score, // lower weight = more similar = shorter path
			})
		}
		break
	}
}

// ─── Mock embedding (replace with real BGE-M3 / Ollama) ─────────────────────

// mockEmbed returns a deterministic pseudo-embedding based on the text content.
// In production wire this to:
//   - Ollama: POST http://localhost:11434/api/embed
//   - OpenAI: client.CreateEmbedding("text-embedding-3-large")
//   - BGE-M3: your Orus API embedding endpoint
func mockEmbed(text string) []float64 {
	const dims = 64 // use 1024 in production
	vec := make([]float64, dims)
	r := rand.New(rand.NewSource(hashStr(text)))
	for i := range vec {
		vec[i] = r.NormFloat64()
	}
	// normalise
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
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("   Autonomous Agent with ConcurrentOrderedGraph Memory")
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println()

	agent := NewMemoryAgent(mockEmbed)

	// ── Phase 1: The agent observes its environment ───────────────────────
	fmt.Println("── Phase 1: Observations ──")
	obs1 := agent.Observe("User asked: 'What is the capital of France?'", "user_input", nil)
	obs2 := agent.Observe("Wikipedia tool returned: Paris is the capital of France", "wikipedia_tool", nil)
	obs3 := agent.Observe("User asked: 'What is the population of Paris?'", "user_input", nil)
	obs4 := agent.Observe("Web search returned: Paris population is approximately 2.1 million", "web_search_tool", nil)
	obs5 := agent.Observe("User asked: 'Recommend a good restaurant in Paris near the Eiffel Tower'", "user_input", nil)
	obs6 := agent.Observe("Maps API returned: Le Jules Verne restaurant, inside Eiffel Tower, Michelin starred", "maps_tool", nil)

	fmt.Println()

	// ── Phase 2: The agent takes actions ─────────────────────────────────
	fmt.Println("── Phase 2: Actions ──")
	act1 := agent.Act("Called Wikipedia API for France capital", obs1)
	act2 := agent.Act("Called web search for Paris population", obs3)
	act3 := agent.Act("Called Google Maps API for Eiffel Tower restaurants", obs5)
	_ = act1
	_ = act2
	_ = act3

	fmt.Println()

	// ── Phase 3: Agent derives facts ─────────────────────────────────────
	fmt.Println("── Phase 3: Derived Facts ──")
	fact1 := agent.StoreFact("Paris is the capital of France with ~2.1M inhabitants", []string{obs2, obs4})
	fact2 := agent.StoreFact("Le Jules Verne is a top dining option near Eiffel Tower", []string{obs6})
	_ = fact1
	_ = fact2

	fmt.Println()

	// ── Phase 4: Context retrieval before next action ─────────────────────
	fmt.Println("── Phase 4: Context Retrieval ──")
	fmt.Println("Query: 'Paris tourism and restaurants'")
	fmt.Println("Anchor: last user message (obs5), maxHops=3, topK=4")
	fmt.Println()

	ctx := agent.QueryContext("Paris tourism and restaurants", obs5, 4, 3)
	for i, r := range ctx {
		fmt.Printf("  [%d] score=%.4f type=%-12s content=%q\n",
			i+1, r.Score, r.Node.Data.Type, r.Node.Data.Content)
	}

	fmt.Println()

	// ── Phase 5: Global semantic search ──────────────────────────────────
	fmt.Println("── Phase 5: Global Semantic Search ──")
	fmt.Println("Query: 'capital city information'")
	global := agent.QueryGlobal("capital city information", 3)
	for i, r := range global {
		fmt.Printf("  [%d] score=%.4f id=%-10s %q\n",
			i+1, r.Score, r.Node.ID, r.Node.Data.Content)
	}

	fmt.Println()

	// ── Phase 6: Reasoning chain ──────────────────────────────────────────
	fmt.Println("── Phase 6: Reasoning Chain (Dijkstra) ──")
	fmt.Printf("Path from %s (initial user question) → %s (derived fact)\n", obs1, fact1)
	chain, dist := agent.ReasoningChain(obs1, fact1)
	fmt.Printf("  Chain: %v\n  Distance: %.2f\n", chain, dist)

	fmt.Println()

	// ── Phase 7: BFS exploration ──────────────────────────────────────────
	fmt.Println("── Phase 7: BFS Memory Exploration from obs1, depth=2 ──")
	nearby := agent.Explore(obs1, 2)
	for _, n := range nearby {
		fmt.Printf("  [%s] %q\n", n.Type, n.Content)
	}

	fmt.Println()

	// ── Phase 8: Connected components (detect isolated memories) ─────────
	fmt.Println("── Phase 8: Memory Graph Structure ──")
	comps := agent.memory.ConnectedComponents()
	fmt.Printf("  Total nodes: %d\n", agent.memory.NodeCount())
	fmt.Printf("  Connected components: %d\n", len(comps))
	for i, c := range comps {
		fmt.Printf("  Component %d: %v\n", i+1, c)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════")
	fmt.Println("  Done. This graph is the agent's persistent world model.")
	fmt.Println("═══════════════════════════════════════════════════════")
}