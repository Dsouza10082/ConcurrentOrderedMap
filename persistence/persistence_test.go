package persistence

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

type testItem struct {
	Name  string
	Score float64
}

func tempDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("com_test_%d", os.Getpid()))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func testOpts() Options {
	opts := DefaultOptions()
	opts.SnapshotInterval = 0 // disable background goroutine in tests
	opts.MaxWALEntries = 0
	opts.VectorDims = 0
	return opts
}

func makeTestVec(seed float64) []float64 {
	dims := 8
	v := make([]float64, dims)
	for i := range v {
		v[i] = seed + float64(i)*0.1
	}
	var n float64
	for _, x := range v {
		n += x * x
	}
	n = math.Sqrt(n)
	for i := range v {
		v[i] /= n
	}
	return v
}

// ─── Basic Set / Get / Delete ─────────────────────────────────────────────────

func TestSetAndGet(t *testing.T) {
	store, err := New[string, testItem](tempDir(t), testOpts())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	item := testItem{"alpha", 0.9}
	if err := store.Set("k1", item); err != nil {
		t.Fatal(err)
	}
	got, ok := store.Get("k1")
	if !ok {
		t.Fatal("expected key k1 to exist")
	}
	if got.Name != item.Name {
		t.Fatalf("expected %q, got %q", item.Name, got.Name)
	}
}

func TestExists(t *testing.T) {
	store, _ := New[string, testItem](tempDir(t), testOpts())
	defer store.Close()

	_ = store.Set("x", testItem{"x", 1})
	if !store.Exists("x") {
		t.Fatal("key x should exist")
	}
	if store.Exists("y") {
		t.Fatal("key y should not exist")
	}
}

func TestDelete(t *testing.T) {
	store, _ := New[string, testItem](tempDir(t), testOpts())
	defer store.Close()

	_ = store.Set("k", testItem{"k", 1})
	_ = store.Delete("k")
	if store.Exists("k") {
		t.Fatal("key should be deleted")
	}
	if store.Len() != 0 {
		t.Fatalf("expected Len=0, got %d", store.Len())
	}
}

func TestLen(t *testing.T) {
	store, _ := New[string, testItem](tempDir(t), testOpts())
	defer store.Close()

	for i := 0; i < 5; i++ {
		_ = store.Set(fmt.Sprintf("k%d", i), testItem{fmt.Sprintf("v%d", i), float64(i)})
	}
	if store.Len() != 5 {
		t.Fatalf("expected Len=5, got %d", store.Len())
	}
}

// ─── WAL recovery ─────────────────────────────────────────────────────────────

func TestRecoveryAfterReopen(t *testing.T) {
	dir := tempDir(t)

	// First open: write data
	store1, err := New[string, testItem](dir, testOpts())
	if err != nil {
		t.Fatal(err)
	}
	_ = store1.Set("a", testItem{"alpha", 1.0})
	_ = store1.Set("b", testItem{"beta", 2.0})
	_ = store1.Delete("a")
	if err := store1.Sync(); err != nil {
		t.Fatal(err)
	}
	_ = store1.Close()

	// Second open: should recover b, not a
	store2, err := New[string, testItem](dir, testOpts())
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	if store2.Exists("a") {
		t.Fatal("deleted key 'a' should not be recovered")
	}
	got, ok := store2.Get("b")
	if !ok {
		t.Fatal("key 'b' should be recovered from WAL")
	}
	if got.Name != "beta" {
		t.Fatalf("expected 'beta', got %q", got.Name)
	}
}

// ─── Snapshot ─────────────────────────────────────────────────────────────────

func TestSnapshotAndRecovery(t *testing.T) {
	dir := tempDir(t)
	store1, _ := New[string, testItem](dir, testOpts())

	for i := 0; i < 10; i++ {
		_ = store1.Set(fmt.Sprintf("k%d", i), testItem{fmt.Sprintf("v%d", i), float64(i)})
	}
	if err := store1.Snapshot(); err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	_ = store1.Close()

	// Reopen after snapshot: WAL is empty, data comes from snapshot
	store2, err := New[string, testItem](dir, testOpts())
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()

	if store2.Len() != 10 {
		t.Fatalf("expected 10 items after snapshot recovery, got %d", store2.Len())
	}
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("k%d", i)
		if !store2.Exists(key) {
			t.Fatalf("key %q missing after snapshot recovery", key)
		}
	}
}

// ─── Vector index ─────────────────────────────────────────────────────────────

func TestSetWithVectorAndSearch(t *testing.T) {
	opts := testOpts()
	opts.VectorDims = 8

	store, err := New[string, testItem](tempDir(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	_ = store.SetWithVector("near", testItem{"near", 1.0}, makeTestVec(1.0))
	_ = store.SetWithVector("far", testItem{"far", 0.1}, makeTestVec(9.0))

	results, err := store.VectorSearch(makeTestVec(1.0), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least one search result")
	}
	if results[0].Key != "near" {
		t.Fatalf("expected 'near' as top result, got %q", results[0].Key)
	}
}

func TestVectorSearchWithFilter(t *testing.T) {
	opts := testOpts()
	opts.VectorDims = 8

	store, _ := New[string, testItem](tempDir(t), opts)
	defer store.Close()

	_ = store.SetWithVector("a", testItem{"a", 1}, makeTestVec(1.0))
	_ = store.SetWithVector("b", testItem{"b", 1}, makeTestVec(1.0))

	results, _ := store.VectorSearch(makeTestVec(1.0), 5, func(key string) bool {
		return key == "a"
	})
	for _, r := range results {
		if r.Key == "b" {
			t.Fatal("filtered key 'b' should not appear in results")
		}
	}
}

func TestVectorSearchWithoutDimsReturnsError(t *testing.T) {
	store, _ := New[string, testItem](tempDir(t), testOpts()) // VectorDims = 0
	defer store.Close()

	_, err := store.VectorSearch(makeTestVec(1.0), 5, nil)
	if err == nil {
		t.Fatal("expected error when VectorDims not configured")
	}
}

// ─── FlatVectorIndex standalone ───────────────────────────────────────────────

func TestFlatIndexUpsertAndSearch(t *testing.T) {
	idx := NewFlatVectorIndex[string](8)

	_ = idx.Upsert("doc1", makeTestVec(1.0))
	_ = idx.Upsert("doc2", makeTestVec(9.0))

	results, err := idx.Search(makeTestVec(1.0), 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Key != "doc1" {
		t.Fatalf("expected doc1 as nearest, got %q", results[0].Key)
	}
}

func TestFlatIndexDelete(t *testing.T) {
	idx := NewFlatVectorIndex[string](8)
	_ = idx.Upsert("a", makeTestVec(1.0))
	_ = idx.Upsert("b", makeTestVec(1.0))
	idx.Delete("a")

	if idx.Len() != 1 {
		t.Fatalf("expected 1 entry after delete, got %d", idx.Len())
	}
	results, _ := idx.Search(makeTestVec(1.0), 5, nil)
	for _, r := range results {
		if r.Key == "a" {
			t.Fatal("deleted key 'a' should not appear in search results")
		}
	}
}

func TestFlatIndexWrongDims(t *testing.T) {
	idx := NewFlatVectorIndex[string](8)
	err := idx.Upsert("x", makeTestVec(1.0)[:4]) // wrong dims
	if err == nil {
		t.Fatal("expected error for wrong dims")
	}
}

func TestFlatIndexUpsertReplace(t *testing.T) {
	idx := NewFlatVectorIndex[string](8)
	_ = idx.Upsert("k", makeTestVec(1.0))
	_ = idx.Upsert("k", makeTestVec(9.0)) // replace

	if idx.Len() != 1 {
		t.Fatalf("upsert should not grow index, expected 1 got %d", idx.Len())
	}
}

// ─── Concurrency smoke ────────────────────────────────────────────────────────

func TestConcurrentWrites(t *testing.T) {
	store, _ := New[string, testItem](tempDir(t), testOpts())
	defer store.Close()

	done := make(chan struct{}, 50)
	for i := 0; i < 50; i++ {
		go func(n int) {
			_ = store.Set(fmt.Sprintf("k%d", n), testItem{fmt.Sprintf("v%d", n), float64(n)})
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
	if store.Len() != 50 {
		t.Fatalf("expected 50 entries, got %d", store.Len())
	}
}
