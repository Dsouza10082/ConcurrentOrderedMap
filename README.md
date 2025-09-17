
<img width="1536" height="1024" alt="gopher_doing_multiple_things_strong" src="https://github.com/user-attachments/assets/9cc7a8a4-9932-4bc6-8caf-fbf4e30b5866" />

# ConcurrentOrderedMap
ConcurrentOrderedMap provides a thread-safe map implementation that preserves insertion order. It combines the safety of synchronized access with predictable iteration order, making it suitable for concurrent environments where key ordering matters

# Install
- go get github.com/Dsouza10082/ConcurrentOrderedMap@v1.1.0

# Use cases:
- Concurrent access patterns with deterministic iteration requirements
- Scenarios requiring both thread-safety and insertion order guarantees
- Replacing sync.Map when ordered traversal is needed
- Singleton for database connections
- String similarity search (direct from the dependency) for machine learning
- Vector search using BGE-M3 and text-embedding-3-large embedding types (1024 dimensions) for machine learning
- Agents creation with zero dependency 
- Objects Pool
 
# Thread-safety:
All operations are safe for concurrent use by multiple goroutines.

# ConcurrentOrderedMap API Documentation

## Methods

### GetOrderedV2

Returns a slice of `OrderedPair[K, V]` containing the map's key-value pairs in insertion order.

**Returns:**
- A slice of `OrderedPair[K, V]` containing the key-value pairs in insertion order

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
map.Set("b", 2)
map.Set("c", 3)
pairs := map.GetOrderedV2()
fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
```

**Note:**
- The returned slice is a copy of the map's data and is safe for concurrent use
- The map is locked for reading during the operation, ensuring thread-safety
- The order of the pairs is guaranteed to be the same as the order of insertion

---

## Constructors

### NewConcurrentOrderedMap

Creates a new ConcurrentOrderedMap with the given key and value types. It initializes the map with empty data and ensures thread-safety for concurrent operations.

**Returns:**
- A pointer to a new `ConcurrentOrderedMap[K, V]` instance

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
map.Set("b", 2)
map.Set("c", 3)
pairs := map.GetOrderedV2()
fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
```

---

### NewConcurrentOrderedMapWithCapacity

Creates a new ConcurrentOrderedMap with the given key and value types and initial capacity. It initializes the map with empty data and ensures thread-safety for concurrent operations.

**Returns:**
- A pointer to a new `ConcurrentOrderedMap[K, V]` instance

**Example:**
```go
map := NewConcurrentOrderedMapWithCapacity[string, int](10)
map.Set("a", 1)
map.Set("b", 2)
map.Set("c", 3)
pairs := map.GetOrderedV2()
fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
```

---

## Query Methods

### Exists

Checks if a key exists in the map.

**Returns:**
- `true` if the key exists, `false` otherwise

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
exists := map.Exists("a")
fmt.Println(exists) // Output: true
```

**Note:**
- The map is locked for reading during the operation, ensuring thread-safety
- The operation is thread-safe and can be used concurrently by multiple goroutines

---

### Get

Retrieves a value from the map by key.

**Returns:**
- The value associated with the key, and a boolean indicating if the key exists
- The boolean is `true` if the key exists, `false` otherwise

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
value, exists := map.Get("a")
fmt.Println(value, exists) // Output: 1 true
```

**Note:**
- The map is locked for reading during the operation, ensuring thread-safety
- The operation is thread-safe and can be used concurrently by multiple goroutines

---

## Modification Methods

### Set

Adds a new key-value pair to the map.

**Returns:**
- The map itself

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
map.Set("b", 2)
map.Set("c", 3)
fmt.Println(map) // Output: {a: 1, b: 2, c: 3}
```

**Note:**
- The map is locked for writing during the operation, ensuring thread-safety
- The operation is thread-safe and can be used concurrently by multiple goroutines

---

### Update

Updates a value in the map by key using a function.

**Returns:**
- An error if the update fails

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
err := map.Update("a", func(v *int) error {
    *v = 2
    return nil
})
fmt.Println(err) // Output: nil
```

**Note:**
- The map is locked for writing during the operation, ensuring thread-safety
- The operation is thread-safe and can be used concurrently by multiple goroutines

---

### UpdateWithPointer

Updates a value in the map by key using a pointer function.

**Returns:**
- An error if the update fails

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
err := map.UpdateWithPointer("a", func(v *int) error {
    *v = 2
    return nil
})
fmt.Println(err) // Output: nil
```

**Note:**
- The map is locked for writing during the operation, ensuring thread-safety
- The operation is thread-safe and can be used concurrently by multiple goroutines

---

### Delete

Removes a key-value pair from the map.

**Returns:**
- The map itself

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
map.Set("b", 2)
map.Set("c", 3)
map.Delete("b")
fmt.Println(map) // Output: {a: 1, c: 3}
```

**Note:**
- The map is locked for writing during the operation, ensuring thread-safety
- The operation is thread-safe and can be used concurrently by multiple goroutines

---

## Utility Methods

### Len

Returns the number of key-value pairs in the map.

**Returns:**
- The number of key-value pairs in the map

**Example:**
```go
map := NewConcurrentOrderedMap[string, int]()
map.Set("a", 1)
map.Set("b", 2)
map.Set("c", 3)
fmt.Println(map.Len()) // Output: 3
```

**Note:**
- The map is locked for reading during the operation, ensuring thread-safety
- The operation is thread-safe and can be used concurrently by multiple goroutines



# Author: Dsouza
# License: MIT

---

# Similarity & Search Extensions

This library now includes a rich set of **string similarity** and **vector similarity** order/filter helpers that work on top of `ConcurrentOrderedMap`. These helpers keep the core guarantees (thread-safety + deterministic/stable ordering) while letting you rank results by how close they are to a target query.

The features are provided as **non-breaking** additions—use only what you need.

## Quick Summary

**String-based (for `V == string`):**

- Levenshtein
  - `OrderedByLevenshteinDistance(target string, caseInsensitive bool)`
  - `OrderedByLevenshteinDistanceFiltered(target string, caseInsensitive bool, filter string)`
- Damerau–Levenshtein (OSA; adjacent transpositions)
  - `OrderedByDamerauLevenshteinDistance(target string, caseInsensitive bool)`
  - `OrderedByDamerauLevenshteinDistanceFiltered(target string, caseInsensitive bool, filter string)`
- Jaro–Winkler (similarity in [0,1], higher is better)
  - `OrderedByJaroWinklerSimilarity(target string, caseInsensitive bool)`
  - `OrderedByJaroWinklerSimilarityFiltered(target string, caseInsensitive bool, filter string)`
- Cosine (TF–IDF) over tokens
  - `OrderedByCosineTFIDF(target string, caseInsensitive bool)`
  - `OrderedByCosineTFIDFFiltered(target string, caseInsensitive bool, filter string)`
- Soundex (phonetic similarity)
  - `OrderedBySoundex(target string, caseInsensitive bool)`
  - `OrderedBySoundexFiltered(target string, caseInsensitive bool, filter string)`
- N-grams with Jaccard (default n=3; score in [0,1])
  - `OrderedByJaccardNGrams(target string, caseInsensitive bool)`
  - `OrderedByJaccardNGramsFiltered(target string, caseInsensitive bool, filter string)`

**Blended text similarity:**

- Combine the above metrics for stronger ranking:
  - `OrderedByCombinedSimilarity(target string, caseInsensitive bool)`
  - `OrderedByCombinedSimilarityFiltered(target string, caseInsensitive bool, filter string)`

**Vector + Text blended (supports 1024-dim embeddings like BGE-M3 and OpenAI `text-embedding-3-large` 1024):**

- Highly configurable blend between **vector cosine** and all **string metrics**:
  - `OrderedByVectorCombinedSimilarity(target string, caseInsensitive bool, queryVec []float64, dims int, extract VectorTextExtractor[V], weights *VectorBlendWeights)`
  - `OrderedByVectorCombinedSimilarityFiltered(target string, caseInsensitive bool, filter string, queryVec []float64, dims int, extract VectorTextExtractor[V], weights *VectorBlendWeights)`
- Pure vector-only ordering (helpful for baseline comparisons):
  - `OrderedByVectorOnlyFiltered(caseInsensitive bool, filter string, queryVec []float64, dims int, extract VectorTextExtractor[V])`

### Notes
- All *string* similarity functions require **`V` to be `string`**.
- The **filtered** variants include only entries whose value’s **text contains** the `filter` substring (respects `caseInsensitive`) *before* ranking.
- Ordering is **stable**: ties preserve insertion order.

---

## Usage — String Similarities

```go
package main

import (
    "fmt"
    co "github.com/Dsouza10082/ConcurrentOrderedMap"
)

func main() {
    m := co.NewConcurrentOrderedMap[string, string]()
    m.Set("a", "developer")
    m.Set("b", "deliver")
    m.Set("c", "delta")
    m.Set("d", "programmer")

    // Levenshtein (ascending distance -> implemented as descending similarity)
    pairs, _ := m.OrderedByLevenshteinDistance("developer", true)
    fmt.Println("Levenshtein:", pairs)

    // Filter to keep only values containing "dev"
    pairsF, _ := m.OrderedByLevenshteinDistanceFiltered("developer", true, "dev")
    fmt.Println("Levenshtein + filter(\"dev\"):", pairsF)

    // Jaro–Winkler
    jw, _ := m.OrderedByJaroWinklerSimilarity("developer", true)
    fmt.Println("Jaro–Winkler:", jw)

    // Cosine TF–IDF (token based)
    tfidf, _ := m.OrderedByCosineTFIDF("python developer", true)
    fmt.Println("TF–IDF:", tfidf)

    // Soundex (phonetic)
    sdx, _ := m.OrderedBySoundex("Robert", true)
    fmt.Println("Soundex:", sdx)

    // Jaccard n-grams (default n=3)
    jac, _ := m.OrderedByJaccardNGrams("developer", true)
    fmt.Println("Jaccard trigrams:", jac)

    // Combined blend
    cmb, _ := m.OrderedByCombinedSimilarity("developer python", true)
    fmt.Println("Combined:", cmb)
}
```

---

## Usage — Vector + Text Blend (1024-dim)

**Typical value shape:** `map[string]interface{}{"text": string, "vector": []float64}`

```go
package main

import (
    "fmt"
    "math/rand"
    co "github.com/Dsouza10082/ConcurrentOrderedMap"
)

func main() {
    dims := 1024
    // Example query embedding (replace with your real 1024-dim vector)
    queryVec := make([]float64, dims)
    for i := range queryVec { queryVec[i] = rand.Float64() }

    m := co.NewConcurrentOrderedMap[float64, map[string]interface{}]()
    m.Set(1.0, map[string]interface{}{"text": "python developer", "vector": queryVec})
    m.Set(2.0, map[string]interface{}{"text": "developer", "vector": randomUnit(dims)})
    m.Set(3.0, map[string]interface{}{"text": "unrelated text", "vector": randomUnit(dims)})

    w := co.DefaultVectorBlendWeights() // tune if needed (e.g., w.WVec = 4, w.WCos = 2)

    // Blend vectors + all string similarities (no substring filter)
    pairs, err := m.OrderedByVectorCombinedSimilarity(
        "developer python",
        true,
        queryVec,
        dims,
        co.MapExtractor, // reads "text" and "vector" from value
        &w,
    )
    if err != nil { panic(err) }
    fmt.Println("Vector+Text:", pairs)

    // With substring filter over TEXT (case-insensitive)
    pairsF, _ := m.OrderedByVectorCombinedSimilarityFiltered(
        "developer python",
        true,
        "dev",
        queryVec,
        dims,
        co.MapExtractor,
        &w,
    )
    fmt.Println("Vector+Text filtered:", pairsF)

    // Vector-only baseline with filter
    vo, _ := m.OrderedByVectorOnlyFiltered(true, "dev", queryVec, dims, co.MapExtractor)
    fmt.Println("Vector-only filtered:", vo)
}

// randomUnit is pseudo; use your actual embeddings in production.
func randomUnit(d int) []float64 {
    v := make([]float64, d)
    var s float64
    for i := 0; i < d; i++ {
        v[i] = rand.NormFloat64()
        s += v[i] * v[i]
    }
    n := 1.0
    if s > 0 { n = 1.0 / (sqrt(s)) } // inline sqrt if you prefer; or math.Sqrt
    for i := range v { v[i] *= n }
    return v
}
```

**Extractor helpers**

If your values are structs, create a custom extractor:

```go
type Item struct {
    ID     int
    Name   string
    Vector []float64
}

func ItemExtractor(v Item) (string, []float64, bool) {
    if len(v.Vector) == 0 { return "", nil, false }
    return v.Name, v.Vector, true
}
```

For `map[string]interface{}` values, use the ready-made:

```go
// MapExtractor(value) -> (text, vector, ok)
co.MapExtractor
```

**Weights**

```go
w := co.DefaultVectorBlendWeights()
w.WVec = 4.0  // emphasize vectors
w.WCos = 2.0  // emphasize token coverage
// tweak WLev, WDL, WJW, WJac, WSdx as needed
```

---

## API Reference (New Methods)

### Levenshtein
- `OrderedByLevenshteinDistance(target string, caseInsensitive bool)`
- `OrderedByLevenshteinDistanceFiltered(target string, caseInsensitive bool, filter string)`

### Damerau–Levenshtein (OSA)
- `OrderedByDamerauLevenshteinDistance(target string, caseInsensitive bool)`
- `OrderedByDamerauLevenshteinDistanceFiltered(target string, caseInsensitive bool, filter string)`

### Jaro–Winkler
- `OrderedByJaroWinklerSimilarity(target string, caseInsensitive bool)`
- `OrderedByJaroWinklerSimilarityFiltered(target string, caseInsensitive bool, filter string)`

### Cosine TF–IDF
- `OrderedByCosineTFIDF(target string, caseInsensitive bool)`
- `OrderedByCosineTFIDFFiltered(target string, caseInsensitive bool, filter string)`

### Soundex
- `OrderedBySoundex(target string, caseInsensitive bool)`
- `OrderedBySoundexFiltered(target string, caseInsensitive bool, filter string)`

### Jaccard N-grams (n=3 by default)
- `OrderedByJaccardNGrams(target string, caseInsensitive bool)`
- `OrderedByJaccardNGramsFiltered(target string, caseInsensitive bool, filter string)`

### Combined (text metrics blend)
- `OrderedByCombinedSimilarity(target string, caseInsensitive bool)`
- `OrderedByCombinedSimilarityFiltered(target string, caseInsensitive bool, filter string)`

### Vector + Text Combined
- `OrderedByVectorCombinedSimilarity(target string, caseInsensitive bool, queryVec []float64, dims int, extract VectorTextExtractor[V], weights *VectorBlendWeights)`
- `OrderedByVectorCombinedSimilarityFiltered(target string, caseInsensitive bool, filter string, queryVec []float64, dims int, extract VectorTextExtractor[V], weights *VectorBlendWeights)`
- `OrderedByVectorOnlyFiltered(caseInsensitive bool, filter string, queryVec []float64, dims int, extract VectorTextExtractor[V])`

---

## Running Tests & Benchmarks

### Requirements
- Go **1.20+** (generics supported; 1.18+ works, 1.20+ recommended).

### Run unit tests
```bash
go test ./...
```

### Benchmarks — string similarities
Use env var `SIM_BENCH_ITEMS` to control dataset size (default: 10000).

```bash
# default size
go test -bench=Filtered_ -benchmem ./...

# larger dataset
SIM_BENCH_ITEMS=200000 go test -bench=Filtered_ -benchmem ./...
```

### Benchmarks — vector similarities (1024-d)
Use env var `SIM_BENCH_VEC_ITEMS` to control dataset size (default: 10000).

```bash
# default vector size
go test -bench=Vector -benchmem ./...

# larger dataset (warning: heavy)
SIM_BENCH_VEC_ITEMS=200000 go test -bench=Vector -benchmem ./...
```

> Tip: Start with 10k–50k locally and scale up. The **Combined** and **Vector+Text** variants do more work per item and will be the slowest but most accurate.

