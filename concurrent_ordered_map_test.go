package concurrentmap

import (
	"fmt"
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

func TestUpdateOperations(t *testing.T) {
	m := NewConcurrentOrderedMap[string, TestStruct]()

	// Set initial value
	m.Set("test", TestStruct{ID: 1, Name: "Test", Score: 10.0})

	// Test Update
	err := m.Update("test", func(v *TestStruct) error {
		v.Score = 20.0
		return nil
	})

	if err != nil {
		t.Errorf("Update failed: %v", err)
	}

	val, _ := m.Get("test")
	if val.Score != 20.0 {
		t.Errorf("Expected score 20.0, got %f", val.Score)
	}

	// Test Update on non-existent key
	err = m.Update("nonexistent", func(v *TestStruct) error {
		v.ID = 100
		return nil
	})

	if err == nil {
		t.Error("Expected error for non-existent key")
	}

	if err != nil {
		t.Errorf("SetOrUpdate failed: %v", err)
	}

	val, exists := m.Get("new")
	if !exists || val.ID != 2 {
		t.Error("New value not set correctly")
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
