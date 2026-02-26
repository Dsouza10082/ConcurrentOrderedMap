package persistence

import (
	"fmt"
	"math"
	"sort"
	"sync"
)

// VectorResult is a single hit from a vector search.
type VectorResult[K comparable] struct {
	Key   K
	Score float64
}

// FlatVectorIndex is an in-memory brute-force cosine-similarity index.
//
// Suitable for up to ~500 000 vectors. For larger corpora consider a
// hierarchical index (HNSW) as a drop-in replacement behind the same
// interface.
//
// All operations are safe for concurrent use.
type FlatVectorIndex[K comparable] struct {
	mu      sync.RWMutex
	dims    int
	entries []flatEntry[K]
	keyIdx  map[K]int // key → position in entries slice (for O(1) updates)
}

type flatEntry[K comparable] struct {
	key    K
	vector []float64 // unit-normalised for fast cosine
}

// NewFlatVectorIndex creates an empty index that expects vectors of `dims` dimensions.
func NewFlatVectorIndex[K comparable](dims int) *FlatVectorIndex[K] {
	return &FlatVectorIndex[K]{
		dims:   dims,
		keyIdx: make(map[K]int),
	}
}

// Upsert adds or replaces a vector for key. The vector is normalised
// internally so raw embeddings (e.g. BGE-M3 outputs) can be passed directly.
func (idx *FlatVectorIndex[K]) Upsert(key K, vec []float64) error {
	if len(vec) != idx.dims {
		return fmt.Errorf("vector index: expected %d dims, got %d", idx.dims, len(vec))
	}
	norm := normalise(vec)

	idx.mu.Lock()
	defer idx.mu.Unlock()

	if pos, ok := idx.keyIdx[key]; ok {
		idx.entries[pos].vector = norm
		return nil
	}
	idx.keyIdx[key] = len(idx.entries)
	idx.entries = append(idx.entries, flatEntry[K]{key: key, vector: norm})
	return nil
}

// Delete removes a key from the index.
func (idx *FlatVectorIndex[K]) Delete(key K) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	pos, ok := idx.keyIdx[key]
	if !ok {
		return
	}
	last := len(idx.entries) - 1
	if pos != last {
		// Swap with last to avoid O(n) slice shift
		idx.entries[pos] = idx.entries[last]
		idx.keyIdx[idx.entries[pos].key] = pos
	}
	idx.entries = idx.entries[:last]
	delete(idx.keyIdx, key)
}

// Search returns the topK nearest neighbours of queryVec by cosine similarity.
// filter is an optional key-predicate; pass nil to search all entries.
func (idx *FlatVectorIndex[K]) Search(
	queryVec []float64,
	topK int,
	filter func(K) bool,
) ([]VectorResult[K], error) {
	if len(queryVec) != idx.dims {
		return nil, fmt.Errorf("vector index: query has %d dims, index expects %d", len(queryVec), idx.dims)
	}
	q := normalise(queryVec)

	idx.mu.RLock()
	entries := idx.entries // read-only snapshot
	idx.mu.RUnlock()

	results := make([]VectorResult[K], 0, topK*2)
	for _, e := range entries {
		if filter != nil && !filter(e.key) {
			continue
		}
		score := dot(q, e.vector)
		results = append(results, VectorResult[K]{Key: e.key, Score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if len(results) > topK {
		results = results[:topK]
	}
	return results, nil
}

// Len returns the number of vectors in the index.
func (idx *FlatVectorIndex[K]) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

// ─── math helpers ─────────────────────────────────────────────────────────────

func dot(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

func normalise(v []float64) []float64 {
	var n float64
	for _, x := range v {
		n += x * x
	}
	if n == 0 {
		out := make([]float64, len(v))
		return out
	}
	inv := 1.0 / math.Sqrt(n)
	out := make([]float64, len(v))
	for i, x := range v {
		out[i] = x * inv
	}
	return out
}
