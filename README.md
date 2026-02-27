
<img width="1536" height="1024" alt="gopher223" src="https://github.com/user-attachments/assets/f4a5e730-d172-451a-aca0-efcfb6350b5c" />

# ConcurrentOrderedMap v2.0.0

[![Go Reference](https://pkg.go.dev/badge/github.com/Dsouza10082/ConcurrentOrderedMap.svg)](https://pkg.go.dev/github.com/Dsouza10082/ConcurrentOrderedMap)
[![MIT License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

ConcurrentOrderedMap provides a thread-safe map implementation that preserves insertion order. It combines the safety of synchronized access with predictable iteration order, making it suitable for concurrent environments where key ordering matters.

---

## Install

```bash
go get github.com/Dsouza10082/ConcurrentOrderedMap@v2.0.0
```

---

## Use cases

- Concurrent access patterns with deterministic iteration requirements
- Scenarios requiring both thread-safety and insertion order guarantees
- Replacing `sync.Map` when ordered traversal is needed
- Singleton for database connections
- String similarity search (direct from the dependency) for machine learning
- Vector search using BGE-M3 and `text-embedding-3-large` embedding types (1024 dimensions)
- **Knowledge graphs for autonomous agents and agentic memory systems** *(new)*
- **Persistent durable storage with semantic vector search** *(new)*
- Agents creation with zero dependency
- Objects Pool

---

## Contents

- [ConcurrentOrderedMap API](#concurrentorderedmap-api-documentation)
- [Similarity & Search Extensions](#similarity--search-extensions)
- [Graph — Knowledge Graph & Agent Memory](#graph--knowledge-graph--agent-memory) *(new)*
- [Persistence — Durable Storage with Vector Search](#persistence--durable-storage-with-vector-search) *(new)*
- [Agentic Examples](#agentic-examples) *(new)*
- [Step-by-step: Adding Graph & Persistence to the Library](#step-by-step-adding-graph--persistence-to-the-library) *(new)*
- [Running Tests & Benchmarks](#running-tests--benchmarks)

---

## ConcurrentOrderedMap API Documentation

### Methods

#### GetOrderedV2

Returns a slice of `OrderedPair[K, V]` containing the map's key-value pairs in insertion order.

```go
m := NewConcurrentOrderedMap[string, int]()
m.Set("a", 1)
m.Set("b", 2)
pairs := m.GetOrderedV2()
fmt.Println(pairs) // [{a 1} {b 2}]
```

### Constructors

#### NewConcurrentOrderedMap

```go
m := NewConcurrentOrderedMap[string, int]()
```

#### NewConcurrentOrderedMapWithCapacity

```go
m := NewConcurrentOrderedMapWithCapacity[string, int](10)
```

### Query Methods

#### Exists

```go
exists := m.Exists("a") // true/false
```

#### Get

```go
value, exists := m.Get("a")
```

### Modification Methods

#### Set

```go
m.Set("a", 1)
```

#### Update / UpdateWithPointer

```go
m.Update("a", func(v *int) error { *v = 2; return nil })
```

#### Delete

```go
m.Delete("b")
```

### Utility Methods

#### Len

```go
fmt.Println(m.Len()) // 3
```

---

## Similarity & Search Extensions

### String-based (for `V == string`)

| Method | Algorithm |
|---|---|
| `OrderedByLevenshteinDistance` | Edit distance |
| `OrderedByDamerauLevenshteinDistance` | Edit distance + transpositions |
| `OrderedByJaroWinklerSimilarity` | Jaro-Winkler (0-1) |
| `OrderedByCosineTFIDF` | Token cosine (TF-IDF) |
| `OrderedBySoundex` | Phonetic similarity |
| `OrderedByJaccardNGrams` | Jaccard trigrams |
| `OrderedByCombinedSimilarity` | Blend of all above |

All methods have a `...Filtered(target, caseInsensitive, filter)` variant.

### Vector + Text Blend (1024-dim BGE-M3 / OpenAI)

```go
w := co.DefaultVectorBlendWeights()
pairs, err := m.OrderedByVectorCombinedSimilarity(
    "developer python", true, queryVec, 1024, co.MapExtractor, &w,
)
```

---

## Graph — Knowledge Graph & Agent Memory

The `graph` package provides a thread-safe, ordered directed/undirected graph built on top of `ConcurrentOrderedMap`. Each node can carry an arbitrary payload and an optional embedding vector, enabling **graph traversal combined with semantic vector search**.

This is the core primitive needed for:
- Autonomous agent memory systems
- RAG pipelines with relational context
- Knowledge graphs for reasoning chains
- Multi-step tool-call history with semantic retrieval

### Quick start

```go
import "github.com/Dsouza10082/ConcurrentOrderedMap/graph"

type Memory struct {
    Content   string
    Category  string
}

g := graph.NewDirectedGraph[string, Memory]()

// Add nodes with embedding vectors
g.AddNode("obs-001", Memory{"User asked about Paris", "observation"}, embed("User asked about Paris"))
g.AddNode("obs-002", Memory{"Paris is the capital of France", "fact"}, embed("Paris capital France"))
g.AddNode("act-001", Memory{"Called Wikipedia API", "action"}, embed("Wikipedia API call"))

// Connect related nodes
g.AddEdge("obs-001", "act-001", graph.EdgeData{Label: "triggered", Weight: 1.0})
g.AddEdge("act-001", "obs-002", graph.EdgeData{Label: "produced",  Weight: 1.0})
```

### Vector Search from a Node (context-aware retrieval)

```go
// "What do I know about France, starting from this observation?"
results, err := g.VectorSearchFromNode(
    "obs-001",          // anchor node (start BFS here)
    queryVec,           // embedding of the query
    5,                  // topK results
    3,                  // maxHops (BFS depth limit)
    nil,                // optional filter func(Memory) bool
)
for _, r := range results {
    fmt.Printf("score=%.4f  %q\n", r.Score, r.Node.Data.Content)
}
```

### Global Vector Search

```go
results, _ := g.VectorSearchGlobal(queryVec, 5, func(m Memory) bool {
    return m.Category == "fact" // only search facts
})
```

### Bidirectional Semantic Search

```go
// Expands both forward and backward edges — retrieves parents, children, and siblings
results, _ := g.VectorSearchBidirectional("obs-001", queryVec, 5, 2, nil)
```

### Shortest Path (Dijkstra)

```go
// Find the reasoning chain between two memories
result, err := g.ShortestPath("obs-001", "obs-002")
fmt.Println(result.Nodes)    // ["obs-001", "act-001", "obs-002"]
fmt.Println(result.Distance) // 2.0
```

### BFS / DFS Traversal

```go
// BFS with depth limit — ideal for "context within N hops"
g.BFSWithDepth("obs-001", 2, func(id string, data Memory, depth int) bool {
    fmt.Printf("depth=%d  %s: %q\n", depth, id, data.Content)
    return true // return false to stop early
})

// Standard DFS
g.DFS("obs-001", func(id string, data Memory) bool {
    fmt.Println(id, data.Content)
    return true
})
```

### Connected Components

```go
// Detect isolated sub-graphs (useful for finding orphan memories)
components := g.ConnectedComponents()
fmt.Printf("%d connected components\n", len(components))
```

### Full Graph API

```go
// Nodes
g.AddNode(id, data, vec)
g.UpsertNode(id, data, vec)   // add or replace
g.GetNode(id)                 // (*GraphNode, bool)
g.RemoveNode(id)
g.NodeCount()
g.Nodes()                     // all nodes in insertion order

// Edges
g.AddEdge(from, to, EdgeData{Weight, Label, Metadata})
g.RemoveEdge(from, to)
g.GetNeighbors(id)            // []K in insertion order
g.HasEdge(from, to)

// Traversal
g.BFS(start, visitor)
g.BFSWithDepth(start, maxDepth, visitor)
g.DFS(start, visitor)
g.ShortestPath(from, to)      // PathResult{Nodes, Distance}
g.AllReachable(start)         // []K
g.ConnectedComponents()       // [][]K

// Vector Search
g.VectorSearchGlobal(queryVec, topK, filter)
g.VectorSearchFromNode(start, queryVec, topK, maxHops, filter)
g.VectorSearchBidirectional(start, queryVec, topK, maxHops, filter)
g.SemanticNeighbors(nodeID, queryVec, topK)  // 1-hop only
```

---

## Persistence — Durable Storage with Vector Search

The `persistence` package wraps `ConcurrentOrderedMap` with crash-safe durability using a **Write-Ahead Log (WAL)** + periodic **binary snapshots**. It also includes a `FlatVectorIndex` for in-memory semantic search over persisted values.

Zero external dependencies — 100% Go stdlib (`os`, `encoding/gob`, `bufio`, `sync`).

### How it works

```
Write path:   Set(key, value) → WAL append (O(1)) → in-memory map update
              Snapshot()      → gob encode all entries → atomic file rename → WAL truncate

Read path:    Open(dir) → load latest snapshot → replay WAL entries after snapshot seq
              Get(key)  → O(1) from in-memory map
              Search()  → cosine similarity over FlatVectorIndex (in-memory, O(n))
```

### Quick start

```go
import "github.com/Dsouza10082/ConcurrentOrderedMap/persistence"

opts := persistence.DefaultOptions()
opts.VectorDims = 1024 // set to your embedding dimensions; 0 = no vector index

store, err := persistence.New[string, MyStruct]("/data/my-agent", opts)
if err != nil { panic(err) }
defer store.Close() // flushes + writes final snapshot

// Store with optional embedding vector
store.SetWithVector("key1", MyStruct{...}, embeddingVector)

// Plain store (no vector)
store.Set("key2", MyStruct{...})

// Retrieve
val, ok := store.Get("key1")

// Semantic search over persisted vectors
results, err := store.VectorSearch(queryVec, 5, nil)
for _, r := range results {
    val, _ := store.Get(r.Key)
    fmt.Printf("score=%.4f  %+v\n", r.Score, val)
}

// Manual snapshot (also happens automatically per SnapshotInterval)
store.Snapshot()
```

### Options

```go
type Options struct {
    SnapshotInterval time.Duration // auto-snapshot period (0 = disable)
    MaxWALEntries    int           // compact after N entries (0 = disable)
    SyncOnWrite      bool          // fdatasync on every write (slower, safer)
    VectorDims       int           // embedding dims; 0 = no vector index
}

// Defaults: 5min snapshot, 100k WAL entries, no fsync, no vector index
opts := persistence.DefaultOptions()
```

### FlatVectorIndex (standalone)

You can also use the vector index directly, independent of the persistent map:

```go
idx := persistence.NewFlatVectorIndex[string](1024)

idx.Upsert("doc-001", vector1)
idx.Upsert("doc-002", vector2)
idx.Delete("doc-001")

results, _ := idx.Search(queryVec, 5, func(key string) bool {
    return strings.HasPrefix(key, "doc-")
})
```

---

## Agentic Examples

### Example 1 — Autonomous Agent with Knowledge Graph Memory

Located at `examples/agentic-knowledge-graph/main.go`

Demonstrates a `MemoryAgent` that:
- Stores observations, actions, and derived facts as graph nodes
- Auto-links related nodes by semantic similarity
- Retrieves relevant context before each action (`VectorSearchFromNode`)
- Traces reasoning chains between memories (`ShortestPath`)
- Detects isolated memory clusters (`ConnectedComponents`)

```bash
cd examples/agentic-knowledge-graph
go run main.go
```

**Key pattern:**

```go
agent := NewMemoryAgent(myEmbedFn)

// Agent observes environment
obs1 := agent.Observe("User asked: What is the capital of France?", "user_input", nil)
obs2 := agent.Observe("Wikipedia: Paris is the capital", "wiki_tool", nil)

// Agent acts
agent.Act("Called Wikipedia API", obs1)

// Agent derives a fact
agent.StoreFact("Paris is the capital of France", []string{obs1, obs2})

// Before next action: retrieve context within 3 hops of current node
ctx := agent.QueryContext("France capital city", obs1, topK=5, maxHops=3)
```

### Example 2 — Persistent Memory Agent

Located at `examples/agentic-memory-agent/main.go`

Demonstrates persistence + graph working together:
- Every memory is persisted to disk via WAL
- On restart, the agent loads all memories and rebuilds the graph
- Vector search works across both in-memory graph and persisted store

```bash
cd examples/agentic-memory-agent
go run main.go   # first run: stores memories
go run main.go   # second run: loads from disk, shows they're still there
```

### Example 3 — Basic Persistence

Located at `examples/persistence-basic/main.go`

Minimal example showing `Set`, `SetWithVector`, `VectorSearch`, and `Snapshot`.

```bash
cd examples/persistence-basic
go run main.go
```

### Connecting to a real embedding provider

Replace the `mockEmbed` function in examples with a real API call:

```go
// Ollama (local, BGE-M3)
func embed(text string) []float64 {
    body := `{"model":"bge-m3","input":"` + text + `"}`
    resp, _ := http.Post("http://localhost:11434/api/embed", "application/json", strings.NewReader(body))
    var result struct { Embeddings [][]float64 }
    json.NewDecoder(resp.Body).Decode(&result)
    return result.Embeddings[0]
}

// OpenAI text-embedding-3-large (1024 dims)
func embed(text string) []float64 {
    client := openai.NewClient(os.Getenv("OPENAI_API_KEY"))
    resp, _ := client.CreateEmbedding(ctx, &openai.EmbeddingRequest{
        Model: "text-embedding-3-large",
        Input: []string{text},
        Dimensions: 1024,
    })
    return resp.Data[0].Embedding
}
```

## Running Tests & Benchmarks

### Requirements

- Go **1.20+** (generics; 1.18+ works, 1.20+ recommended)

### Run all tests

```bash
go test ./...
```

### Benchmarks — string similarities

```bash
# default dataset size (10 000 items)
go test -bench=Filtered_ -benchmem ./...

# larger dataset
SIM_BENCH_ITEMS=200000 go test -bench=Filtered_ -benchmem ./...
```

### Benchmarks — vector similarities (1024-d)

```bash
go test -bench=Vector -benchmem ./...

SIM_BENCH_VEC_ITEMS=200000 go test -bench=Vector -benchmem ./...
```

---

## Author: Dsouza

## License: MIT
