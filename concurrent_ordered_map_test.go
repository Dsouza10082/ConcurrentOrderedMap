package concurrentmap

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStruct for testing ordered operations
type TestStruct struct {
	ID    int
	Name  string
	Score float64
}

func TestBasicOperations(t *testing.T) {
	m := NewConcurrentOrderedMap[string, int]()

	// Test Set and Get
	m.Set("one", 1)
	m.Set("two", 2)
	m.Set("three", 3)

	if val, exists := m.Get("two"); !exists || val != 2 {
		t.Errorf("Expected 2, got %v", val)
	}

	// Test Exists
	if !m.Exists("one") {
		t.Error("Key 'one' should exist")
	}

	if m.Exists("four") {
		t.Error("Key 'four' should not exist")
	}

	// Test Len
	if m.Len() != 3 {
		t.Errorf("Expected length 3, got %d", m.Len())
	}

	// ADD THIS LINE - Test Delete operation
	m.Delete("two")

	if m.Len() != 2 {
		t.Errorf("Expected length 2 after deletion, got %d", m.Len())
	}
}

func TestOrder(t *testing.T) {
	m := NewConcurrentOrderedMap[int, string]()

	// Insert in specific order
	for i := 0; i < 5; i++ {
		m.Set(i, fmt.Sprintf("value-%d", i))
	}

	// Check order is maintained
	keys := m.Keys()
	for i, key := range keys {
		if key != i {
			t.Errorf("Expected key %d at position %d, got %d", i, i, key)
		}
	}

	// Check ordered pairs
	pairs := m.GetOrdered()
	for i, pair := range pairs {
		if pair.Key != i {
			t.Errorf("Expected key %d, got %d", i, pair.Key)
		}
		expected := fmt.Sprintf("value-%d", i)
		if pair.Value != expected {
			t.Errorf("Expected value %s, got %s", expected, pair.Value)
		}
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewConcurrentOrderedMap[int, string]()
	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			m.Set(id, fmt.Sprintf("value-%d", id))
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			time.Sleep(time.Microsecond) // Small delay to mix reads with writes
			m.Get(id)
		}(i)
	}

	// Concurrent updates
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			m.Update(id, func(v *string) error {
				*v = fmt.Sprintf("updated-%d", id)
				return nil
			})
		}(i)
	}

	wg.Wait()

	// Verify map integrity
	if m.Len() != 100 {
		t.Errorf("Expected 100 items, got %d", m.Len())
	}
}

func TestUpdateBehavior(t *testing.T) {
	m := NewConcurrentOrderedMap[string, TestStruct]()
	
	// Test update on non-existent key
	err := m.Update("missing", func(v *TestStruct) error {
		v.ID = 100
		return nil
	})
	
	fmt.Printf("Error: %v\n", err)
	
	// Check if key was created
	if val, exists := m.Get("missing"); exists {
		fmt.Printf("Key was created with value: %+v\n", val)
	} else {
		fmt.Println("Key was not created")
	}
}

func TestGenericUpdate(t *testing.T) {
	m := NewConcurrentOrderedMap[string, int]()
	m.Set("counter", 10)

	// Test DirectUpdate
	err := m.UpdateGeneric("counter", DirectUpdate[int]{NewValue: 20})
	if err != nil {
		t.Errorf("DirectUpdate failed: %v", err)
	}

	val, _ := m.Get("counter")
	if val != 20 {
		t.Errorf("Expected 20, got %d", val)
	}

	// Test FunctionUpdate
	err = m.UpdateGeneric("counter", FunctionUpdate[int]{
		UpdateFunc: func(v *int) error {
			*v += 5
			return nil
		},
	})

	if err != nil {
		t.Errorf("FunctionUpdate failed: %v", err)
	}

	val, _ = m.Get("counter")
	if val != 25 {
		t.Errorf("Expected 25, got %d", val)
	}
}

func TestOrderedByField(t *testing.T) {
	m := NewConcurrentOrderedMap[int, TestStruct]()

	m.Set(1, TestStruct{ID: 3, Name: "Charlie", Score: 85.0})
	m.Set(2, TestStruct{ID: 1, Name: "Alice", Score: 90.0})
	m.Set(3, TestStruct{ID: 2, Name: "Bob", Score: 75.0})

	// Sort by ID ascending
	result, err := m.GetOrderedByField(OrderBy{Field: "ID", Direction: ASC})
	if err != nil {
		t.Errorf("GetOrderedByField failed: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 results, got %d", len(result))
	}

	if result[0].Value.ID != 1 || result[1].Value.ID != 2 || result[2].Value.ID != 3 {
		t.Error("Incorrect ID ordering")
	}

	// Sort by Score descending
	result, err = m.GetOrderedByField(OrderBy{Field: "Score", Direction: DESC})
	if err != nil {
		t.Errorf("GetOrderedByField failed: %v", err)
	}

	if result[0].Value.Score != 90.0 || result[1].Value.Score != 85.0 || result[2].Value.Score != 75.0 {
		t.Error("Incorrect Score ordering")
	}
}

func TestOrderedByMultiField(t *testing.T) {
	m := NewConcurrentOrderedMap[int, TestStruct]()

	m.Set(1, TestStruct{ID: 1, Name: "Alice", Score: 90.0})
	m.Set(2, TestStruct{ID: 2, Name: "Bob", Score: 90.0})
	m.Set(3, TestStruct{ID: 3, Name: "Charlie", Score: 85.0})

	// Sort by Score DESC, then by Name ASC
	result, err := m.GetOrderedByMultiField([]OrderBy{
		{Field: "Score", Direction: DESC},
		{Field: "Name", Direction: ASC},
	})

	if err != nil {
		t.Errorf("GetOrderedByMultiField failed: %v", err)
	}

	// With same score (90), Alice should come before Bob
	if result[0].Value.Name != "Alice" || result[1].Value.Name != "Bob" {
		t.Error("Incorrect multi-field ordering")
	}
}

func TestFieldUpdate(t *testing.T) {
	m := NewConcurrentOrderedMap[string, TestStruct]()
	m.Set("test", TestStruct{ID: 1, Name: "Test", Score: 10.0})

	err := m.UpdateGeneric("test", FieldUpdate[TestStruct]{
		Fields: map[string]interface{}{
			"Name":  "Updated",
			"Score": 25.5,
		},
	})

	if err != nil {
		t.Errorf("FieldUpdate failed: %v", err)
	}

	val, _ := m.Get("test")
	if val.Name != "Updated" || val.Score != 25.5 {
		t.Errorf("Fields not updated correctly: %+v", val)
	}
}

// Benchmark tests
func BenchmarkSet(b *testing.B) {
	m := NewConcurrentOrderedMap[int, string]()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Set(i, fmt.Sprintf("value-%d", i))
	}
}

func BenchmarkGet(b *testing.B) {
	m := NewConcurrentOrderedMap[int, string]()
	for i := 0; i < 1000; i++ {
		m.Set(i, fmt.Sprintf("value-%d", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Get(i % 1000)
	}
}

func BenchmarkConcurrentSet(b *testing.B) {
	m := NewConcurrentOrderedMap[int, string]()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Set(i, fmt.Sprintf("value-%d", i))
			i++
		}
	})
}

func BenchmarkConcurrentGet(b *testing.B) {
	m := NewConcurrentOrderedMap[int, string]()
	for i := 0; i < 1000; i++ {
		m.Set(i, fmt.Sprintf("value-%d", i))
	}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Get(i % 1000)
			i++
		}
	})
}

func keysFromPairs[K comparable, V any](pairs []OrderedPair[K, V]) []K {
	out := make([]K, len(pairs))
	for i, p := range pairs {
		out[i] = p.Key
	}
	return out
}

// ---------- Levenshtein ----------	
func TestOrderedByLevenshteinDistance_BasicOrdering(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("a", "developer")
	m.Set("b", "deliver")
	m.Set("c", "delta")
	m.Set("d", "programmer")

	pairs, err := m.OrderedByLevenshteinDistance("developer", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysFromPairs(pairs)

	// "developer" should come first (distance 0),
	// followed by the next closest ("deliver", then "delta" typically),
	// exact trailing order beyond the first can vary based on distances.
	if len(got) == 0 || got[0] != "a" {
		t.Fatalf("expected first key to be 'a' (developer), got %v", got)
	}
}

func TestOrderedByLevenshteinDistance_CaseInsensitive(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("x", "DEVELOPER")
	m.Set("y", "developer")
	m.Set("z", "Developers") // slightly different

	pairs, err := m.OrderedByLevenshteinDistance("developer", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysFromPairs(pairs)

	// Both "DEVELOPER" and "developer" are distance 0 in case-insensitive mode.
	// Stable sort should keep insertion order among ties: x then y.
	wantPrefix := []string{"x", "y"}
	if len(got) < 2 || !reflect.DeepEqual(got[:2], wantPrefix) {
		t.Fatalf("expected first two keys %v, got %v", wantPrefix, got)
	}
}

func TestOrderedByLevenshteinDistance_TiesPreserveInsertionOrder(t *testing.T) {
	// For target "abc", "abx", "xbc", and "axc" are all distance 1 (single substitution).
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("k1", "abx")
	m.Set("k2", "xbc")
	m.Set("k3", "axc")

	pairs, err := m.OrderedByLevenshteinDistance("abc", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysFromPairs(pairs)
	want := []string{"k1", "k2", "k3"} // insertion order preserved on equal distance ties

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected order %v, got %v", want, got)
	}
}

func TestOrderedByLevenshteinDistance_NonStringValueType(t *testing.T) {
	m := NewConcurrentOrderedMap[string, int]()
	// Even empty map of non-string should still error, since V != string.
	_, err := m.OrderedByLevenshteinDistance("anything", true)
	if err == nil {
		t.Fatalf("expected error when V is not string, got nil")
	}
}

func TestOrderedByLevenshteinDistance_EmptyMap(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	pairs, err := m.OrderedByLevenshteinDistance("x", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pairs) != 0 {
		t.Fatalf("expected empty result for empty map, got %v", pairs)
	}
}

func TestLevenshtein_FilteredAndBasic(t *testing.T) {
    m := NewConcurrentOrderedMap[string, string]()
    m.Set("a", "developer")
    m.Set("b", "deliver")
    m.Set("c", "delta")
    m.Set("d", "programmer")

    // Basic: "developer" should be first
    pairs, err := m.OrderedByLevenshteinDistance("developer", true)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := keysFromPairs(pairs)
    if len(got) == 0 || got[0] != "a" {
        t.Fatalf("expected first key 'a', got %v", got)
    }

    // Filter: only values containing "dev" (case-insensitive)
    pairsF, err := m.OrderedByLevenshteinDistanceFiltered("developer", true, "dev")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // Only "developer" should pass the filter among the sample values
    gotF := keysFromPairs(pairsF)
    wantF := []string{"a"}
    if !reflect.DeepEqual(gotF, wantF) {
        t.Fatalf("expected filtered keys %v, got %v", wantF, gotF)
    }
}

func TestLevenshtein_CaseInsensitiveTiesStable(t *testing.T) {
    m := NewConcurrentOrderedMap[string, string]()
    m.Set("x", "DEVELOPER")
    m.Set("y", "developer")
    m.Set("z", "Developers")

    pairs, err := m.OrderedByLevenshteinDistance("developer", true)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := keysFromPairs(pairs)
    if len(got) < 2 || got[0] != "x" || got[1] != "y" {
        t.Fatalf("expected stable tie order ['x','y',...], got %v", got)
    }
}

func TestLevenshtein_EmptyMapAndNonString(t *testing.T) {
    m := NewConcurrentOrderedMap[string, string]()
    pairs, err := m.OrderedByLevenshteinDistance("", false)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(pairs) != 0 {
        t.Fatalf("expected empty, got %v", pairs)
    }

    // Non-string values must error
    mi := NewConcurrentOrderedMap[string, int]()
    if _, err := mi.OrderedByLevenshteinDistance("x", false); err == nil {
        t.Fatalf("expected error for non-string V, got nil")
    }
    if _, err := mi.OrderedByLevenshteinDistanceFiltered("x", false, "x"); err == nil {
        t.Fatalf("expected error for non-string V (filtered), got nil")
    }
}

// ---------- Damerau Levenshtein ----------

func TestDamerauLevenshtein_BasicAndTransposition(t *testing.T) {
    m := NewConcurrentOrderedMap[string, string]()
    m.Set("k1", "ac")       // transposition of "ca" -> distance 1 with DL (adjacent swap)
    m.Set("k2", "abc")      // likely distance 2 to "ca"
    m.Set("k3", "car")      // likely distance 1 (insertion) or 2 depending on path

    pairs, err := m.OrderedByDamerauLevenshteinDistance("ca", false)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := keysFromPairs(pairs)
    if len(got) == 0 || got[0] != "k1" {
        t.Fatalf("expected 'k1' (ac) to be closest to 'ca' via transposition; got %v", got)
    }
}

func TestDamerauLevenshtein_Filtered(t *testing.T) {
    m := NewConcurrentOrderedMap[string, string]()
    m.Set("a", "developer")
    m.Set("b", "deliver")
    m.Set("c", "delta")
    m.Set("d", "programmer")

    // Filter only items containing "del"
    pairs, err := m.OrderedByDamerauLevenshteinDistanceFiltered("deliver", true, "del")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := keysFromPairs(pairs)

    // Should include "developer", "deliver", "delta" in some distance order; all match filter "del".
    // We only assert that 'b' ("deliver") is first (exact or closest).
    if len(got) == 0 || got[0] != "b" {
        t.Fatalf("expected first key 'b' (deliver), got %v", got)
    }

    // Filter that matches nothing -> empty
    pairsNone, err := m.OrderedByDamerauLevenshteinDistanceFiltered("deliver", true, "zzz")
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(pairsNone) != 0 {
        t.Fatalf("expected empty with unmatched filter, got %v", pairsNone)
    }
}

func TestDamerauLevenshtein_NonString(t *testing.T) {
    mi := NewConcurrentOrderedMap[string, int]()
    if _, err := mi.OrderedByDamerauLevenshteinDistance("x", false); err == nil {
        t.Fatalf("expected error for non-string V, got nil")
    }
    if _, err := mi.OrderedByDamerauLevenshteinDistanceFiltered("x", false, "x"); err == nil {
        t.Fatalf("expected error for non-string V (filtered), got nil")
    }
}

// ---------- Jaro–Winkler ----------

func TestJaroWinkler_BasicAndFiltered(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("eq", "MARTHA")
	m.Set("tr", "MARHTA")   // adjacent transposition of TH
	m.Set("mi", "MARTIN")
	m.Set("ro", "ROBERT")

	pairs, err := m.OrderedByJaroWinklerSimilarity("MARTHA", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysOnly(pairs)
	if len(got) == 0 || got[0] != "eq" {
		t.Fatalf("expected exact match 'eq' first, got %v", got)
	}

	// Filter by "MAR" should remove "ROBERT"
	pairsF, err := m.OrderedByJaroWinklerSimilarityFiltered("MARTHA", true, "mar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, p := range pairsF {
		val := p.Value
		if !containsFold(val, "mar") {
			t.Fatalf("filter failed, got value %q", val)
		}
	}
}

// ---------- Cosine TF–IDF ----------

func TestCosineTFIDF_BasicAndFiltered(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("a", "software developer")
	m.Set("b", "python developer")
	m.Set("c", "golang gopher")
	m.Set("d", "senior programming")

	pairs, err := m.OrderedByCosineTFIDF("developer python", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysOnly(pairs)
	if len(got) == 0 || got[0] != "b" {
		t.Fatalf("expected 'b' (python developer) first, got %v", got)
	}

	// Filter to only those containing "dev"
	pairsF, err := m.OrderedByCosineTFIDFFiltered("developer python", true, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotF := keysOnly(pairsF)
	for _, k := range gotF {
		v, _ := m.Get(k)
		if !containsFold(v, "dev") {
			t.Fatalf("filtered set contains non-matching value: key=%s val=%q", k, v)
		}
	}
	if len(gotF) == 0 || gotF[0] != "b" {
		t.Fatalf("expected filtered top 'b', got %v", gotF)
	}
}

func TestCosineTFIDF_NonString(t *testing.T) {
	x := NewConcurrentOrderedMap[string, int]()
	if _, err := x.OrderedByCosineTFIDF("q", true); err == nil {
		t.Fatalf("expected error for non-string V")
	}
	if _, err := x.OrderedByCosineTFIDFFiltered("q", true, "q"); err == nil {
		t.Fatalf("expected error for non-string V (filtered)")
	}
}

// ---------- Soundex ----------

func TestSoundex_BasicAndFiltered(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("rob", "Robert") // R163
	m.Set("rup", "Rupert") // R163
	m.Set("rub", "Rubin")  // R150

	pairs, err := m.OrderedBySoundex("Robert", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysOnly(pairs)
	// 'rob' and 'rup' should rank before 'rub'; stable order keeps insertion
	if len(got) < 2 || got[0] != "rob" || got[1] != "rup" {
		t.Fatalf("expected ['rob','rup', ...], got %v", got)
	}

	// Filter values containing "ru" (case-insensitive)
	pairsF, err := m.OrderedBySoundexFiltered("Robert", true, "ru")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotF := keysOnly(pairsF)
	// Both 'rup' and 'rub' pass filter; 'rup' shares code with 'Robert' -> first
	if len(gotF) == 0 || gotF[0] != "rup" {
		t.Fatalf("expected 'rup' first with filter, got %v", gotF)
	}
}

func TestSoundex_NonString(t *testing.T) {
	x := NewConcurrentOrderedMap[string, int]()
	if _, err := x.OrderedBySoundex("q", false); err == nil {
		t.Fatalf("expected error for non-string V")
	}
	if _, err := x.OrderedBySoundexFiltered("q", false, "q"); err == nil {
		t.Fatalf("expected error for non-string V (filtered)")
	}
}

func keysOnly[K comparable, V any](pairs []OrderedPair[K, V]) []K {
	out := make([]K, len(pairs))
	for i, p := range pairs {
		out[i] = p.Key
	}
	return out
}

// ---------- Jaccard N-grams (trigrams) ----------

func TestJaccardNGrams_BasicAndFiltered(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("dev", "developer")
	m.Set("deve", "develope")
	m.Set("devs", "developers")
	m.Set("del", "deliver")
	m.Set("pro", "programmer")

	pairs, err := m.OrderedByJaccardNGrams("developer", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysOnly(pairs)
	if len(got) == 0 || got[0] != "dev" {
		t.Fatalf("expected exact match 'dev' first, got %v", got)
	}

	// Filter by "dev"
	pairsF, err := m.OrderedByJaccardNGramsFiltered("developer", true, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	gotF := keysOnly(pairsF)
	for _, k := range gotF {
		v, _ := m.Get(k)
		if !containsFold(v, "dev") {
			t.Fatalf("filtered set contains non-matching value: key=%s val=%q", k, v)
		}
	}
	if len(gotF) == 0 || gotF[0] != "dev" {
		t.Fatalf("expected exact match 'dev' first after filter, got %v", gotF)
	}
}

func TestJaccardNGrams_NonString(t *testing.T) {
	x := NewConcurrentOrderedMap[string, int]()
	if _, err := x.OrderedByJaccardNGrams("q", false); err == nil {
		t.Fatalf("expected error for non-string V")
	}
	if _, err := x.OrderedByJaccardNGramsFiltered("q", false, "q"); err == nil {
		t.Fatalf("expected error for non-string V (filtered)")
	}
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

func keysFromPairsCombined[K comparable, V any](pairs []OrderedPair[K, V]) []K {
	out := make([]K, len(pairs))
	for i, p := range pairs {
		out[i] = p.Key
	}
	return out
}

func TestCombinedSimilarity_BasicOrdering(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("dev", "developer")
	m.Set("del", "deliver")
	m.Set("devs", "developers")
	m.Set("prog", "programmer")
	m.Set("dlp", "develope")

	pairs, err := m.OrderedByCombinedSimilarity("developer", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysFromPairsCombined(pairs)
	if len(got) == 0 || got[0] != "dev" {
		t.Fatalf("expected exact match 'dev' first, got %v", got)
	}
}

func TestCombinedSimilarity_Filtered(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("a", "senior programming")
	m.Set("b", "python developer")
	m.Set("c", "golang gopher")
	m.Set("d", "software developer")

	// Filter to only those containing "dev"
	pairs, err := m.OrderedByCombinedSimilarityFiltered("developer python", true, "dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := keysFromPairsCombined(pairs)
	for _, k := range got {
		v, _ := m.Get(k)
		if !strings.Contains(strings.ToLower(v), "dev") {
			t.Fatalf("filtered set contains non-matching value: key=%s val=%q", k, v)
		}
	}
	// Likely "python developer" is top for this target
	if len(got) == 0 || got[0] != "b" {
		t.Fatalf("expected 'b' first, got %v", got)
	}
}

func TestCombinedSimilarity_NonString(t *testing.T) {
	mi := NewConcurrentOrderedMap[string, int]()
	if _, err := mi.OrderedByCombinedSimilarity("x", false); err == nil {
		t.Fatalf("expected error for non-string V, got nil")
	}
	if _, err := mi.OrderedByCombinedSimilarityFiltered("x", false, "x"); err == nil {
		t.Fatalf("expected error for non-string V (filtered), got nil")
	}
}

// -------- Benchmarks --------

type benchSet[K comparable] struct {
	m       *ConcurrentOrderedMap[K, string]
	target  string
	filter  string
}

func makeBenchDataset[K comparable](size int, includeFilterRatio float64) benchSet[K] {
	rnd := rand.New(rand.NewSource(42))
	m := NewConcurrentOrderedMap[K, string]()

	// half from a base vocabulary to induce partial similarity, half random
	base := []string{
		"developer", "deliver", "development", "deployment", "device", "devolve",
		"programmer", "programming", "golang gopher", "python developer",
		"data science", "machine learning", "deep learning", "vector database",
		"sound example", "jaccard grams", "cosine tf idf", "jaro winkler",
	}

	filter := "dev"
	for i := 0; i < size; i++ {
		var val string
		if rnd.Float64() < 0.5 {
			// mutate a base term
			b := base[rnd.Intn(len(base))]
			val = mutateString(b, rnd)
		} else {
			val = randASCII(rnd.Intn(8)+8, rnd)
		}
		// inject filter substring in some items
		if rnd.Float64() < includeFilterRatio && !strings.Contains(strings.ToLower(val), filter) {
			// insert "dev" at random position
			pos := rnd.Intn(len(val)+1)
			val = val[:pos] + "dev" + val[pos:]
		}
		// cast i to K safely by using string keys; benchmarks use K=string below.
		k := any("k" + strconv.Itoa(i)).(K)
		m.Set(k, val)
	}

	return benchSet[K]{m: m, target: "developer python", filter: filter}
}

func mutateString(s string, rnd *rand.Rand) string {
	b := []rune(s)
	switch rnd.Intn(4) {
	case 0: // substitution
		if len(b) > 0 {
			i := rnd.Intn(len(b))
			b[i] = rune('a' + rnd.Intn(26))
		}
	case 1: // insertion
		i := rnd.Intn(len(b) + 1)
		r := rune('a' + rnd.Intn(26))
		b = append(b[:i], append([]rune{r}, b[i:]...)...)
	case 2: // deletion
		if len(b) > 0 {
			i := rnd.Intn(len(b))
			b = append(b[:i], b[i+1:]...)
		}
	case 3: // transpose adjacent
		if len(b) > 1 {
			i := rnd.Intn(len(b)-1)
			b[i], b[i+1] = b[i+1], b[i]
		}
	}
	return string(b)
}

func randASCII(n int, rnd *rand.Rand) string {
	const letters = "abcdefghijklmnopqrstuvwxyz "
	var sb strings.Builder
	sb.Grow(n)
	for i := 0; i < n; i++ {
		sb.WriteByte(letters[rnd.Intn(len(letters))])
	}
	return sb.String()
}

func benchSize() int {
	if s := os.Getenv("SIM_BENCH_ITEMS"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 10000 // default
}

func Benchmark_Filtered_Levenshtein(b *testing.B) {
	size := benchSize()
	ds := makeBenchDataset[string](size, 0.20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ds.m.OrderedByLevenshteinDistanceFiltered(ds.target, true, ds.filter)
	}
}

func Benchmark_Filtered_DamerauLevenshtein(b *testing.B) {
	size := benchSize()
	ds := makeBenchDataset[string](size, 0.20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ds.m.OrderedByDamerauLevenshteinDistanceFiltered(ds.target, true, ds.filter)
	}
}

func Benchmark_Filtered_JaroWinkler(b *testing.B) {
	size := benchSize()
	ds := makeBenchDataset[string](size, 0.20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ds.m.OrderedByJaroWinklerSimilarityFiltered(ds.target, true, ds.filter)
	}
}

func Benchmark_Filtered_CosineTFIDF(b *testing.B) {
	size := benchSize()
	ds := makeBenchDataset[string](size, 0.20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ds.m.OrderedByCosineTFIDFFiltered(ds.target, true, ds.filter)
	}
}

func Benchmark_Filtered_Soundex(b *testing.B) {
	size := benchSize()
	ds := makeBenchDataset[string](size, 0.20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ds.m.OrderedBySoundexFiltered(ds.target, true, ds.filter)
	}
}

func Benchmark_Filtered_JaccardNGrams(b *testing.B) {
	size := benchSize()
	ds := makeBenchDataset[string](size, 0.20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ds.m.OrderedByJaccardNGramsFiltered(ds.target, true, ds.filter)
	}
}

func Benchmark_Filtered_Combined(b *testing.B) {
	size := benchSize()
	ds := makeBenchDataset[string](size, 0.20)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ds.m.OrderedByCombinedSimilarityFiltered(ds.target, true, ds.filter)
	}
}

func TestCombinedStabilityOnTies(t *testing.T) {
	m := NewConcurrentOrderedMap[string, string]()
	m.Set("k1", "abc")
	m.Set("k2", "abc") // identical -> tie; should keep insertion order
	pairs, err := m.OrderedByCombinedSimilarity("abc", false)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	got := keysFromPairsCombined(pairs)
	want := []string{"k1", "k2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func mkUnit(vec []float64) []float64 {
    var s float64
    for _, v := range vec { s += v*v }
    n := math.Sqrt(s)
    if n == 0 { return vec }
    out := make([]float64, len(vec))
    for i := range vec { out[i] = vec[i] / n }
    return out
}

func near(v []float64, eps float64, rnd *rand.Rand) []float64 {
    out := make([]float64, len(v))
    copy(out, v)
    for i := range out {
        out[i] += rnd.NormFloat64() * eps
    }
    return out
}

func randUnit(d int, rnd *rand.Rand) []float64 {
    x := make([]float64, d)
    for i := range x { x[i] = rnd.NormFloat64() }
    return mkUnit(x)
}

func TestVectorCombined_BasicOrdering(t *testing.T) {
    rnd := rand.New(rand.NewSource(7))
    dims := 1024
    q := randUnit(dims, rnd)

    m := NewConcurrentOrderedMap[float64, map[string]interface{}]()
    // Exact match vector + both tokens in text
    m.Set(1.0, map[string]interface{}{"text": "python developer", "vector": q})
    // Near vector (make it clearly less similar) + fewer tokens in text
    m.Set(2.0, map[string]interface{}{"text": "developer", "vector": mkUnit(near(q, 0.05, rnd))})
    // Far vector + unrelated text
    m.Set(3.0, map[string]interface{}{"text": "unrelated text", "vector": randUnit(dims, rnd)})

    // Heavier semantic weights to reflect the intent of this test:
    w := DefaultVectorBlendWeights()
    w.WVec = 4.0   // vectors dominate
    w.WCos = 2.0   // multi-token coverage matters
    w.WJW  = 0.5
    w.WLev = 0.5
    w.WDL  = 0.5
    w.WJac = 0.5
    w.WSdx = 0.25

    pairs, err := m.OrderedByVectorCombinedSimilarity(
        "developer python", true, q, dims, MapExtractor, &w,
    )
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if len(pairs) < 1 || pairs[0].Key != 1.0 {
        t.Fatalf("expected key 1.0 first (exact vec & both tokens), got %+v", pairs)
    }
}

func TestVectorCombined_FilterAndDimCheck(t *testing.T) {
    rnd := rand.New(rand.NewSource(42))
    dims := 1024
    q := randUnit(dims, rnd)

    m := NewConcurrentOrderedMap[int, map[string]interface{}]()
    m.Set(1, map[string]interface{}{"text": "python developer", "vector": q})
    m.Set(2, map[string]interface{}{"text": "golang gopher", "vector": randUnit(dims, rnd)})

    w := DefaultVectorBlendWeights()
    pairs, err := m.OrderedByVectorCombinedSimilarityFiltered(
        "developer", true, "dev", q, dims, MapExtractor, &w,
    )
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // Only key 1 should pass the "dev" filter
    if len(pairs) != 1 || pairs[0].Key != 1 {
        t.Fatalf("expected only key 1 after filter, got %+v", pairs)
    }

    // Dimension mismatch should error
    badQ := q[:dims-1]
    if _, err := m.OrderedByVectorCombinedSimilarity("x", true, badQ, dims, MapExtractor, &w); err == nil {
        t.Fatalf("expected dimension mismatch error")
    }
}

func TestVectorOnly_Filtered(t *testing.T) {
    rnd := rand.New(rand.NewSource(99))
    dims := 1024
    q := randUnit(dims, rnd)

    m := NewConcurrentOrderedMap[string, map[string]interface{}]()
    m.Set("A", map[string]interface{}{"text": "alpha dev", "vector": mkUnit(near(q, 0.005, rnd))})
    m.Set("B", map[string]interface{}{"text": "beta",      "vector": randUnit(dims, rnd)})
    m.Set("C", map[string]interface{}{"text": "gamma dev", "vector": mkUnit(near(q, 0.02, rnd))})

    pairs, err := m.OrderedByVectorOnlyFiltered(true, "dev", q, dims, MapExtractor)
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    got := []string{pairs[0].Key, pairs[1].Key}
    if !reflect.DeepEqual(got, []string{"A", "C"}) {
        t.Fatalf("expected [A C] by vector cosine over dev-filtered, got %v", got)
    }
}

// Sanity: substring filter respects case-insensitive matching inside vector combined search.
func TestVectorCombined_FilterCaseInsensitive(t *testing.T) {
    rnd := rand.New(rand.NewSource(5))
    dims := 1024
    q := randUnit(dims, rnd)

    m := NewConcurrentOrderedMap[int, map[string]interface{}]()
    m.Set(1, map[string]interface{}{"text": "Golang DEV Tools", "vector": randUnit(dims, rnd)})
    m.Set(2, map[string]interface{}{"text": "python developer", "vector": q})

    w := DefaultVectorBlendWeights()
    pairs, err := m.OrderedByVectorCombinedSimilarityFiltered(
        "developer", true, "DEV", q, dims, MapExtractor, &w,
    )
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    // Both 1 and 2 contain "dev" case-insensitively; vector+text should rank 2 higher.
    if len(pairs) < 2 || pairs[0].Key != 2 {
        var vals []string
        for _, p := range pairs { vals = append(vals, strings.ToLower(p.Value["text"].(string))) }
        t.Fatalf("expected key 2 first, got order texts=%v", vals)
    }
}

// Extracts "text" (string) and "vector" ([]float64) from a map[string]interface{}.
func MapExtractor(v map[string]interface{}) (string, []float64, bool) {
    rawT, okT := v["text"]
    rawV, okV := v["vector"]
    if !okT || !okV {
        return "", nil, false
    }
    txt, ok := rawT.(string)
    if !ok { return "", nil, false }
    // Support []float64 directly, or []any with float64s.
    switch vv := rawV.(type) {
    case []float64:
        return txt, vv, true
    case []any:
        out := make([]float64, len(vv))
        for i, a := range vv {
            f, ok := a.(float64)
            if !ok { return "", nil, false }
            out[i] = f
        }
        return txt, out, true
    default:
        return "", nil, false
    }
}


func benchVecSize() int {
    if s := os.Getenv("SIM_BENCH_VEC_ITEMS"); s != "" {
        if n, err := strconv.Atoi(s); err == nil && n > 0 {
            return n
        }
    }
    return 10000
}

func makeVectorDataset[K comparable](size, dims int, includeFilterRatio float64) (*ConcurrentOrderedMap[K, map[string]interface{}], []float64, string) {
    rnd := rand.New(rand.NewSource(123))
    m := NewConcurrentOrderedMap[K, map[string]interface{}]()
    q := randUnit(dims, rnd)

    baseTexts := []string{
        "python developer", "software developer", "golang gopher",
        "data engineering", "machine learning", "vector database",
        "jaro winkler example", "cosine tf idf practice",
        "soundex phonetic", "jaccard ngrams",
    }
    filter := "dev"

    for i := 0; i < size; i++ {
        txt := baseTexts[rnd.Intn(len(baseTexts))]
        // inject filter substring sometimes
        if rnd.Float64() < includeFilterRatio && !strings.Contains(strings.ToLower(txt), filter) {
            txt += " dev"
        }
        // make some vectors close to q
        var v []float64
        switch {
        case i%50 == 0:
            v = q // exact
        case i%10 == 0:
            v = mkUnit(near(q, 0.01, rnd))
        default:
            v = randUnit(dims, rnd)
        }
        key := any(float64(i)).(K)
        m.Set(key, map[string]interface{}{"text": txt, "vector": v})
    }
    return m, q, filter
}

func Benchmark_VectorOnly_Filtered(b *testing.B) {
    dims := 1024
    size := benchVecSize()
    m, q, filter := makeVectorDataset[float64](size, dims, 0.25)
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = m.OrderedByVectorOnlyFiltered(true, filter, q, dims, MapExtractor)
    }
}

func Benchmark_VectorCombined_Filtered(b *testing.B) {
    dims := 1024
    size := benchVecSize()
    m, q, filter := makeVectorDataset[float64](size, dims, 0.25)
    w := DefaultVectorBlendWeights()
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = m.OrderedByVectorCombinedSimilarityFiltered("developer python", true, filter, q, dims, MapExtractor, &w)
    }
}

// Tiny numeric check to ensure cosineVec is doing non-trivial work.
func Test_cosineVec_basic(t *testing.T) {
    a := []float64{1, 0}
    b := []float64{1, 0}
    c := []float64{0, 1}
    if got := cosineVec(a, l2norm(a), b); math.Abs(got-1) > 1e-9 {
        t.Fatalf("cosine(a,b) expected 1, got %v", got)
    }
    if got := cosineVec(a, l2norm(a), c); math.Abs(got-0) > 1e-9 {
        t.Fatalf("cosine(a,c) expected 0, got %v", got)
    }
}
