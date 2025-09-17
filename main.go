package concurrentmap

// ConcurrentOrderedMap provides a thread-safe map implementation that preserves
// insertion order. It combines the safety of synchronized access with predictable
// iteration order, making it suitable for concurrent environments where key
// ordering matters.
//
// Use cases:
//   - Concurrent access patterns with deterministic iteration requirements
//   - Scenarios requiring both thread-safety and insertion order guarantees
//   - Replacing sync.Map when ordered traversal is needed
//   - Singleton for database connections
//   - Objects Pool
//
// Thread-safety:
// All operations are safe for concurrent use by multiple goroutines.
//
// Author: Dsouza
// License: MIT

import (
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
	"sync"
	"unicode"
)

type ConcurrentOrderedMap[K comparable, V any] struct {
	sync.RWMutex
	data  map[K]V
	keys  []K
	index map[K]int
}

type OrderDirection int

const (
	ASC OrderDirection = iota
	DESC
)

type OrderBy struct {
	Field     string
	Direction OrderDirection
}

type OrderedPair[K comparable, V any] struct {
	Key   K
	Value V
}

type UpdateOperation[V any] interface {
	Apply(current V) (V, error)
}

type DirectUpdate[V any] struct {
	NewValue V
}

func (u DirectUpdate[V]) Apply(current V) (V, error) {
	return u.NewValue, nil
}

// GetOrderedV2 is a method that returns a slice of OrderedPair[K, V]
// It locks the map for reading and returns a copy of the map's data
// in the order of insertion.
//
// Returns:
//   - A slice of OrderedPair[K, V] containing the key-value pairs in insertion order
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	pairs := map.GetOrderedV2()
//	fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
//
// Note:
//   - The returned slice is a copy of the map's data and is safe for concurrent use
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The order of the pairs is guaranteed to be the same as the order of insertion
func (m *ConcurrentOrderedMap[K, V]) GetOrderedV2() []OrderedPair[K, V] {
	m.RLock()
	defer m.RUnlock()

	result := make([]OrderedPair[K, V], len(m.keys))
	for i, key := range m.keys {
		result[i] = OrderedPair[K, V]{
			Key:   key,
			Value: m.data[key],
		}
	}
	return result
}

// NewConcurrentOrderedMap creates a new ConcurrentOrderedMap with the given key and value types
// It initializes the map with empty data and ensures thread-safety for concurrent operations
//
// Returns:
//   - A pointer to a new ConcurrentOrderedMap[K, V] instance
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	pairs := map.GetOrderedV2()
//	fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
func NewConcurrentOrderedMap[K comparable, V any]() *ConcurrentOrderedMap[K, V] {
	return &ConcurrentOrderedMap[K, V]{
		data:  make(map[K]V),
		keys:  make([]K, 0),
		index: make(map[K]int),
	}
}

// NewConcurrentOrderedMapWithCapacity creates a new ConcurrentOrderedMap with the given key and value types
// It initializes the map with empty data and ensures thread-safety for concurrent operations
//
// Returns:
//   - A pointer to a new ConcurrentOrderedMap[K, V] instance
//
// Example:
//
//	map := NewConcurrentOrderedMapWithCapacity[string, int](10)
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	pairs := map.GetOrderedV2()
//	fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
func NewConcurrentOrderedMapWithCapacity[K comparable, V any](initialCapacity ...int) *ConcurrentOrderedMap[K, V] {
	capacity := 0
	if len(initialCapacity) > 0 {
		capacity = initialCapacity[0]
	}
	return &ConcurrentOrderedMap[K, V]{
		data:  make(map[K]V, capacity),
		keys:  make([]K, 0, capacity),
		index: make(map[K]int, capacity),
	}
}

// Exists checks if a key exists in the map
//
// Returns:
//   - true if the key exists, false otherwise
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	exists := map.Exists("a")
//	fmt.Println(exists) // Output: true
//
// Note:
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Exists(key K) bool {
	m.RLock()
	defer m.RUnlock()
	_, exists := m.data[key]
	return exists
}

// Set adds a new key-value pair to the map
//
// Returns:
//   - The map itself
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	fmt.Println(map) // Output: {a: 1, b: 2, c: 3}
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Set(key K, value V) {
	m.Lock()
	defer m.Unlock()
	if _, exists := m.data[key]; !exists {
		m.keys = append(m.keys, key)
		m.index[key] = len(m.keys) - 1
	}
	m.data[key] = value
}

// Get retrieves a value from the map by key
//
// Returns:
//   - The value associated with the key, and a boolean indicating if the key exists
//   - The boolean is true if the key exists, false otherwise
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	value, exists := map.Get("a")
//	fmt.Println(value, exists) // Output: 1 true
//
// Note:
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Get(key K) (V, bool) {
	m.RLock()
	defer m.RUnlock()
	value, exists := m.data[key]
	return value, exists
}

// UpdateWithPointer updates a value in the map by key using a pointer function
//
// Returns:
//   - An error if the update fails
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	err := map.UpdateWithPointer("a", func(v *int) error {
//	  *v = 2
//	  return nil
//	})
//	fmt.Println(err) // Output: nil
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) UpdateWithPointer(key K, updatePointerFunc func(*V) error) error {
	m.Lock()
	defer m.Unlock()

	value, exists := m.data[key]
	if !exists {
		var zero V
		m.keys = append(m.keys, key)
		m.index[key] = len(m.keys) - 1
		value = zero
	}

	if err := updatePointerFunc(&value); err != nil {
		return err
	}

	m.data[key] = value
	return nil
}

// Update updates a value in the map by key using a function
//
// Returns:
//   - An error if the update fails
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	err := map.Update("a", func(v *int) error {
//	  *v = 2
//	  return nil
//	})
//	fmt.Println(err) // Output: nil
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Update(key K, updateFunc func(*V) error) error {
	m.Lock()
	defer m.Unlock()

	value, exists := m.data[key]
	if !exists {
		var zero V
		m.keys = append(m.keys, key)
		m.index[key] = len(m.keys) - 1
		m.data[key] = zero
		value = zero
	}

	if err := updateFunc(&value); err != nil {
		return err
	}

	m.data[key] = value
	return nil
}

// Delete removes a key-value pair from the map
//
// Returns:
//   - The map itself
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	map.Delete("b")
//	fmt.Println(map) // Output: {a: 1, c: 3}
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Delete(key K) {
	m.Lock()
	defer m.Unlock()
	if idx, exists := m.index[key]; exists {
		m.keys = append(m.keys[:idx], m.keys[idx+1:]...)
		for i := idx; i < len(m.keys); i++ {
			m.index[m.keys[i]] = i
		}
		delete(m.data, key)
		delete(m.index, key)
	}
}

// Len returns the number of key-value pairs in the map
//
// Returns:
//   - The number of key-value pairs in the map
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	fmt.Println(map.Len()) // Output: 3
//
// Note:
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Len() int {
	m.RLock()
	defer m.RUnlock()
	return len(m.data)
}

// GetOrdered returns a slice of key-value pairs in the order of insertion
//
// Returns:
//   - A slice of key-value pairs in the order of insertion
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	pairs := map.GetOrdered()
//	fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
//
// Note:
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) GetOrdered() []struct {
	Key   K
	Value V
} {
	m.RLock()
	defer m.RUnlock()
	result := make([]struct {
		Key   K
		Value V
	}, len(m.keys))
	for i, key := range m.keys {
		result[i] = struct {
			Key   K
			Value V
		}{
			Key:   key,
			Value: m.data[key],
		}
	}
	return result
}

// Keys returns a copy of the keys in the map
//
// Returns:
//   - A slice of keys in the map
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	keys := map.Keys()
//	fmt.Println(keys) // Output: [a b c]
//
// Note:
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Keys() []K {
	m.RLock()
	defer m.RUnlock()

	keys := make([]K, len(m.keys))
	copy(keys, m.keys)
	return keys
}

// Values returns a copy of the values in the map
//
// Returns:
//   - A slice of values in the map
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	values := map.Values()
//	fmt.Println(values) // Output: [1 2 3]
//
// Note:
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Values() []V {
	m.RLock()
	defer m.RUnlock()

	values := make([]V, len(m.keys))
	for i, key := range m.keys {
		values[i] = m.data[key]
	}
	return values
}

// Clear removes all key-value pairs from the map
//
// Returns:
//   - The map itself
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	map.Clear()
//	fmt.Println(map) // Output: {}
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) Clear() {
	m.Lock()
	defer m.Unlock()

	m.data = make(map[K]V)
	m.keys = make([]K, 0)
	m.index = make(map[K]int)
}

// GetOrderedByField returns a slice of key-value pairs in the order of insertion
//
// Returns:
//   - A slice of key-value pairs in the order of insertion
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	pairs := map.GetOrderedByField(OrderBy{Field: "a", Direction: ASC})
//	fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
//
// Note:
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) GetOrderedByField(orderBy OrderBy) ([]OrderedPair[K, V], error) {
	m.RLock()
	defer m.RUnlock()
	result := make([]OrderedPair[K, V], len(m.keys))
	for i, key := range m.keys {
		result[i] = OrderedPair[K, V]{
			Key:   key,
			Value: m.data[key],
		}
	}
	if len(result) == 0 {
		return result, nil
	}
	valueType := reflect.TypeOf(result[0].Value)
	if valueType.Kind() != reflect.Struct {
		return nil, errors.New("value type must be a struct")
	}
	_, found := valueType.FieldByName(orderBy.Field)
	if !found {
		return nil, errors.New("field not found in struct")
	}
	sort.Slice(result, func(i, j int) bool {
		vi := reflect.ValueOf(result[i].Value).FieldByName(orderBy.Field)
		vj := reflect.ValueOf(result[j].Value).FieldByName(orderBy.Field)
		less := false
		switch vi.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			less = vi.Int() < vj.Int()
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			less = vi.Uint() < vj.Uint()
		case reflect.Float32, reflect.Float64:
			less = vi.Float() < vj.Float()
		case reflect.String:
			less = vi.String() < vj.String()
		default:
			less = vi.String() < vj.String()
		}

		if orderBy.Direction == DESC {
			return !less
		}
		return less
	})
	return result, nil
}

// GetOrderedByMultiField returns a slice of key-value pairs in the order of insertion
//
// Returns:
//   - A slice of key-value pairs in the order of insertion
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	map.Set("b", 2)
//	map.Set("c", 3)
//	pairs := map.GetOrderedByMultiField([]OrderBy{OrderBy{Field: "a", Direction: ASC}, OrderBy{Field: "b", Direction: DESC}})
//	fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
//
// Note:
//   - The map is locked for reading during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) GetOrderedByMultiField(orderBy []OrderBy) ([]OrderedPair[K, V], error) {
	m.RLock()
	defer m.RUnlock()

	result := make([]OrderedPair[K, V], len(m.keys))
	for i, key := range m.keys {
		result[i] = OrderedPair[K, V]{
			Key:   key,
			Value: m.data[key],
		}
	}

	if len(result) == 0 {
		return result, nil
	}

	valueType := reflect.TypeOf(result[0].Value)
	if valueType.Kind() != reflect.Struct {
		return nil, errors.New("value type must be a struct")
	}

	for _, ob := range orderBy {
		_, found := valueType.FieldByName(ob.Field)
		if !found {
			return nil, errors.New("field " + ob.Field + " not found in struct")
		}
	}

	sort.Slice(result, func(i, j int) bool {
		for _, ob := range orderBy {
			vi := reflect.ValueOf(result[i].Value).FieldByName(ob.Field)
			vj := reflect.ValueOf(result[j].Value).FieldByName(ob.Field)
			var less bool
			switch vi.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				if vi.Int() != vj.Int() {
					less = vi.Int() < vj.Int()
					if ob.Direction == DESC {
						less = !less
					}
					return less
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
				if vi.Uint() != vj.Uint() {
					less = vi.Uint() < vj.Uint()
					if ob.Direction == DESC {
						less = !less
					}
					return less
				}
			case reflect.Float32, reflect.Float64:
				if vi.Float() != vj.Float() {
					less = vi.Float() < vj.Float()
					if ob.Direction == DESC {
						less = !less
					}
					return less
				}
			case reflect.String:
				if vi.String() != vj.String() {
					less = vi.String() < vj.String()
					if ob.Direction == DESC {
						less = !less
					}
					return less
				}
			}
		}
		return false
	})
	return result, nil
}

// UpdateGeneric updates a value in the map by key using a generic operation
//
// Returns:
//   - An error if the update fails
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	err := map.UpdateGeneric("a", DirectUpdate{NewValue: 2})
//	fmt.Println(err) // Output: nil
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) UpdateGeneric(key K, operation UpdateOperation[V]) error {
	m.Lock()
	defer m.Unlock()
	current, exists := m.data[key]
	if !exists {
		return errors.New("key not found")
	}
	newValue, err := operation.Apply(current)
	if err != nil {
		return err
	}
	m.data[key] = newValue
	return nil
}

// FunctionUpdate is a struct that contains a function to update a value in the map
//
// Returns:
//   - An error if the update fails
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	err := map.UpdateGeneric("a", FunctionUpdate{UpdateFunc: func(current *int) error {
//	  *current = 2
//	  return nil
//	}})
//	fmt.Println(err) // Output: nil
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
type FunctionUpdate[V any] struct {
	UpdateFunc func(current *V) error
}

// Apply is a method that applies the update function to the current value
//
// Returns:
//   - The updated value and an error if the update fails
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	updated, err := map.UpdateGeneric("a", FunctionUpdate{UpdateFunc: func(current *int) error {
//	  *current = 2
//	  return nil
//	}})
//	fmt.Println(updated, err) // Output: 2 nil
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (u FunctionUpdate[V]) Apply(current V) (V, error) {
	err := u.UpdateFunc(&current)
	return current, err
}

// FieldUpdate is a struct that contains a map of fields to update in the map
//
// Returns:
//   - An error if the update fails
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	err := map.UpdateGeneric("a", FieldUpdate{Fields: map[string]interface{}{"a": 2}})
//	fmt.Println(err) // Output: nil
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
type FieldUpdate[V any] struct {
	Fields map[string]interface{}
}

// Apply is a method that applies the update function to the current value
//
// Returns:
//   - The updated value and an error if the update fails
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.Set("a", 1)
//	updated, err := map.UpdateGeneric("a", FieldUpdate{Fields: map[string]interface{}{"a": 2}})
//	fmt.Println(updated, err) // Output: 2 nil
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
func (u FieldUpdate[V]) Apply(current V) (V, error) {
	v := reflect.ValueOf(&current).Elem()

	for fieldName, newValue := range u.Fields {
		field := v.FieldByName(fieldName)
		if !field.IsValid() {
			return current, errors.New("field not found: " + fieldName)
		}
		if !field.CanSet() {
			return current, errors.New("field cannot be set: " + fieldName)
		}

		newVal := reflect.ValueOf(newValue)
		if !newVal.Type().AssignableTo(field.Type()) {
			return current, errors.New("invalid type for field: " + fieldName)
		}

		field.Set(newVal)
	}

	return current, nil
}

// SetOrUpdate sets a value in the map if the key does not exist, otherwise it updates the value
//
// Returns:
//   - The map itself
//
// Example:
//
//	map := NewConcurrentOrderedMap[string, int]()
//	map.SetOrUpdate("a", 1, func(current *int) *int {
//	  *current = 2
//	  return current
//	})
//	fmt.Println(map) // Output: {a: 2}
//
// Note:
//   - The map is locked for writing during the operation, ensuring thread-safety
//   - The operation is thread-safe and can be used concurrently by multiple goroutines
func (m *ConcurrentOrderedMap[K, V]) SetOrUpdate(key K, value V, updateIfExists func(*V) V) {
	m.Lock()
	defer m.Unlock()

	if existing, exists := m.data[key]; exists {
		m.data[key] = updateIfExists(&existing)
	} else {
		m.keys = append(m.keys, key)
		m.index[key] = len(m.keys) - 1
		m.data[key] = value
	}
}

// internal distance function type
type distFunc func(a, b string) int

// orderedByDistanceWithFilter is a shared implementation behind both Levenshtein and
// Damerau–Levenshtein functions. It snapshots pairs, applies an optional substring filter,
// computes distances, and returns pairs ordered by ascending distance (stable on ties).
func (m *ConcurrentOrderedMap[K, V]) orderedByDistanceWithFilter(
	target string,
	caseInsensitive bool,
	filter string,
	df distFunc,
	algoName string,
) ([]OrderedPair[K, V], error) {

	m.RLock()
	defer m.RUnlock()

	// Ensure V is string
	var zeroV V
	if reflect.TypeOf(zeroV).Kind() != reflect.String {
		return nil, errors.New(algoName + ": value type V must be string")
	}

	// Normalize target/filter if needed
	t := target
	f := filter
	if caseInsensitive {
		t = strings.ToLower(t)
		f = strings.ToLower(f)
	}

	type withDist struct {
		pair OrderedPair[K, V]
		dist int
	}
	items := make([]withDist, 0, len(m.keys))

	for _, key := range m.keys {
		val := m.data[key]
		s := reflect.ValueOf(val).String()
		cmp := s
		if caseInsensitive {
			cmp = strings.ToLower(cmp)
		}

		// Apply filter (if provided). Keep only values that contain the filter substring.
		if f != "" && !strings.Contains(cmp, f) {
			continue
		}

		d := df(cmp, t)
		items = append(items, withDist{
			pair: OrderedPair[K, V]{Key: key, Value: val},
			dist: d,
		})
	}

	// Stable sort to preserve insertion order on ties
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].dist < items[j].dist
	})

	out := make([]OrderedPair[K, V], len(items))
	for i := range items {
		out[i] = items[i].pair
	}
	return out, nil
}

// Public: Levenshtein (no filter)
func (m *ConcurrentOrderedMap[K, V]) OrderedByLevenshteinDistance(
	target string,
	caseInsensitive bool,
) ([]OrderedPair[K, V], error) {
	return m.orderedByDistanceWithFilter(target, caseInsensitive, "", levenshteinDistance, "OrderedByLevenshteinDistance")
}

// Public: Levenshtein with substring filter
func (m *ConcurrentOrderedMap[K, V]) OrderedByLevenshteinDistanceFiltered(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {
	return m.orderedByDistanceWithFilter(target, caseInsensitive, filter, levenshteinDistance, "OrderedByLevenshteinDistanceFiltered")
}

// Public: Damerau–Levenshtein (OSA; adjacent transpositions)
func (m *ConcurrentOrderedMap[K, V]) OrderedByDamerauLevenshteinDistance(
	target string,
	caseInsensitive bool,
) ([]OrderedPair[K, V], error) {
	return m.orderedByDistanceWithFilter(target, caseInsensitive, "", damerauLevenshteinOSADistance, "OrderedByDamerauLevenshteinDistance")
}

// Public: Damerau–Levenshtein with substring filter
func (m *ConcurrentOrderedMap[K, V]) OrderedByDamerauLevenshteinDistanceFiltered(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {
	return m.orderedByDistanceWithFilter(target, caseInsensitive, filter, damerauLevenshteinOSADistance, "OrderedByDamerauLevenshteinDistanceFiltered")
}

// Classic Levenshtein distance with two-row DP (space O(min(m,n)))
func levenshteinDistance(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	if la > lb {
		a, b = b, a
		la, lb = lb, la
	}

	prev := make([]int, la+1)
	curr := make([]int, la+1)
	for i := 0; i <= la; i++ {
		prev[i] = i
	}

	for j := 1; j <= lb; j++ {
		curr[0] = j
		bj := b[j-1]
		for i := 1; i <= la; i++ {
			cost := 0
			if a[i-1] != bj {
				cost = 1
			}
			del := prev[i] + 1
			ins := curr[i-1] + 1
			sub := prev[i-1] + cost
			// min of three
			if ins < del {
				del = ins
			}
			if sub < del {
				del = sub
			}
			curr[i] = del
		}
		prev, curr = curr, prev
	}
	return prev[la]
}

// Damerau–Levenshtein (Optimal String Alignment variant).
// Allows insertion, deletion, substitution, and a single transposition of adjacent characters.
// Time: O(m*n). Space: O(min(m,n)) rows * 3 (prevprev, prev, curr).
func damerauLevenshteinOSADistance(a, b string) int {
	if a == b {
		return 0
	}
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	swapped := false
	if la > lb {
		a, b = b, a
		la, lb = lb, la
		swapped = true
		_ = swapped
	}

	prevprev := make([]int, la+1)
	prev := make([]int, la+1)
	curr := make([]int, la+1)

	for i := 0; i <= la; i++ {
		prev[i] = i
	}

	for j := 1; j <= lb; j++ {
		curr[0] = j
		bj := b[j-1]
		for i := 1; i <= la; i++ {
			ai := a[i-1]
			cost := 0
			if ai != bj {
				cost = 1
			}
			// deletion, insertion, substitution
			del := prev[i] + 1
			ins := curr[i-1] + 1
			sub := prev[i-1] + cost
			d := del
			if ins < d {
				d = ins
			}
			if sub < d {
				d = sub
			}

			// transposition (adjacent swap)
			if i > 1 && j > 1 && ai == b[j-2] && a[i-2] == bj {
				trans := prevprev[i-2] + 1
				if trans < d {
					d = trans
				}
			}
			curr[i] = d
		}
		prevprev, prev, curr = prev, curr, prevprev
	}
	return prev[la]
}

// scoreFunc computes a similarity in [0,1], higher is better.
type scoreFunc func(a, b string) float64

// orderedByScoreWithFilter snapshots values, applies optional substring filter,
// computes a similarity score, then sorts DESC by score. Stable on ties.
func (m *ConcurrentOrderedMap[K, V]) orderedByScoreWithFilter(
	target string,
	caseInsensitive bool,
	filter string,
	sf scoreFunc,
	algoName string,
) ([]OrderedPair[K, V], error) {

	m.RLock()
	defer m.RUnlock()

	var zeroV V
	if reflect.TypeOf(zeroV).Kind() != reflect.String {
		return nil, errors.New(algoName + ": value type V must be string")
	}

	t := target
	f := filter
	if caseInsensitive {
		t = strings.ToLower(t)
		f = strings.ToLower(f)
	}

	type withScore struct {
		pair  OrderedPair[K, V]
		score float64
	}

	items := make([]withScore, 0, len(m.keys))
	for _, key := range m.keys {
		val := m.data[key]
		s := reflect.ValueOf(val).String()
		cmp := s
		if caseInsensitive {
			cmp = strings.ToLower(cmp)
		}
		if f != "" && !strings.Contains(cmp, f) {
			continue
		}
		items = append(items, withScore{
			pair:  OrderedPair[K, V]{Key: key, Value: val},
			score: sf(cmp, t),
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].score > items[j].score // DESC
	})

	out := make([]OrderedPair[K, V], len(items))
	for i := range items {
		out[i] = items[i].pair
	}
	return out, nil
}

// Public: Jaro–Winkler (higher is better)
func (m *ConcurrentOrderedMap[K, V]) OrderedByJaroWinklerSimilarity(
	target string,
	caseInsensitive bool,
) ([]OrderedPair[K, V], error) {
	return m.orderedByScoreWithFilter(target, caseInsensitive, "", jaroWinkler, "OrderedByJaroWinklerSimilarity")
}

func (m *ConcurrentOrderedMap[K, V]) OrderedByJaroWinklerSimilarityFiltered(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {
	return m.orderedByScoreWithFilter(target, caseInsensitive, filter, jaroWinkler, "OrderedByJaroWinklerSimilarityFiltered")
}

// --- Jaro–Winkler implementation ---
func jaroWinkler(a, b string) float64 {
	if a == b {
		return 1
	}
	ja := jaro(a, b)
	// Winkler adjustment
	l := commonPrefixLen(a, b, 4)
	p := 0.1
	return ja + float64(l)*p*(1.0-ja)
}

func jaro(s1, s2 string) float64 {
	l1, l2 := len(s1), len(s2)
	if l1 == 0 && l2 == 0 {
		return 1
	}
	matchDist := int(math.Floor(math.Max(float64(l1), float64(l2))/2.0)) - 1
	if matchDist < 0 {
		matchDist = 0
	}

	s1Matches := make([]bool, l1)
	s2Matches := make([]bool, l2)
	var matches int

	// Count matches
	for i := 0; i < l1; i++ {
		start := max(0, i-matchDist)
		end := min(i+matchDist+1, l2)
		for k := start; k < end; k++ {
			if s2Matches[k] || s1[i] != s2[k] {
				continue
			}
			s1Matches[i] = true
			s2Matches[k] = true
			matches++
			break
		}
	}
	if matches == 0 {
		return 0
	}

	// Count transpositions
	var t float64
	k := 0
	for i := 0; i < l1; i++ {
		if !s1Matches[i] {
			continue
		}
		for !s2Matches[k] {
			k++
		}
		if s1[i] != s2[k] {
			t++
		}
		k++
	}
	t /= 2.0

	return (float64(matches)/float64(l1) +
		float64(matches)/float64(l2) +
		(float64(matches)-t)/float64(matches)) / 3.0
}

func commonPrefixLen(a, b string, maxL int) int {
	n := min(min(len(a), len(b)), maxL)
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return i
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Public: Cosine TF-IDF
func (m *ConcurrentOrderedMap[K, V]) OrderedByCosineTFIDF(
	target string,
	caseInsensitive bool,
) ([]OrderedPair[K, V], error) {
	return m.orderedByCosineWithFilter(target, caseInsensitive, "")
}

func (m *ConcurrentOrderedMap[K, V]) OrderedByCosineTFIDFFiltered(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {
	return m.orderedByCosineWithFilter(target, caseInsensitive, filter)
}

func (m *ConcurrentOrderedMap[K, V]) orderedByCosineWithFilter(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {

	m.RLock()
	defer m.RUnlock()

	var zeroV V
	if reflect.TypeOf(zeroV).Kind() != reflect.String {
		return nil, errors.New("OrderedByCosineTFIDF: value type V must be string")
	}

	t := target
	f := filter
	if caseInsensitive {
		t = strings.ToLower(t)
		f = strings.ToLower(f)
	}

	type doc struct {
		key   K
		text  string
		toks  []string
		tf    map[string]float64
		norm  float64
		score float64
	}
	docs := make([]doc, 0, len(m.keys))

	// collect candidate docs (respect filter)
	for _, key := range m.keys {
		val := m.data[key]
		s := reflect.ValueOf(val).String()
		cmp := s
		if caseInsensitive {
			cmp = strings.ToLower(cmp)
		}
		if f != "" && !strings.Contains(cmp, f) {
			continue
		}
		docs = append(docs, doc{key: key, text: cmp})
	}
	// Tokenize target and docs
	targetToks := tokenize(t)
	for i := range docs {
		docs[i].toks = tokenize(docs[i].text)
	}

	// Build DF over target + docs
	df := map[string]int{}
	seen := map[string]struct{}{}
	for _, tok := range unique(targetToks) {
		df[tok]++
	}
	for i := range docs {
		seen = map[string]struct{}{}
		for _, tok := range docs[i].toks {
			if _, ok := seen[tok]; !ok {
				seen[tok] = struct{}{}
				df[tok]++
			}
		}
	}
	N := float64(len(docs) + 1) // +target

	// IDF (smooth)
	idf := map[string]float64{}
	for term, dfi := range df {
		idf[term] = math.Log(N/float64(1+dfi)) + 1.0
	}

	// TF vectors
	targetTF := termFreq(targetToks)
	targetVec := weightAndNorm(targetTF, idf)
	for i := range docs {
		docs[i].tf = termFreq(docs[i].toks)
		docs[i].norm = vecNorm(weightAndNorm(docs[i].tf, idf))
	}

	// precompute target norm
	targetNorm := vecNorm(targetVec)

	// score = cosine
	for i := range docs {
		docs[i].score = cosineWeighted(targetTF, docs[i].tf, idf, targetNorm, docs[i].norm)
	}

	// stable sort desc by score
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].score > docs[j].score })

	out := make([]OrderedPair[K, V], len(docs))
	for i := range docs {
		out[i] = OrderedPair[K, V]{Key: docs[i].key, Value: m.data[docs[i].key]}
	}
	return out, nil
}

func tokenize(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
}
func unique(xs []string) []string {
	set := map[string]struct{}{}
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if _, ok := set[x]; !ok {
			set[x] = struct{}{}
			out = append(out, x)
		}
	}
	return out
}
func termFreq(toks []string) map[string]float64 {
	tf := map[string]float64{}
	for _, t := range toks {
		tf[t]++
	}
	return tf
}
func weightAndNorm(tf map[string]float64, idf map[string]float64) map[string]float64 {
	w := make(map[string]float64, len(tf))
	for t, f := range tf {
		w[t] = f * idf[t]
	}
	return w
}
func vecNorm(w map[string]float64) float64 {
	var sum float64
	for _, v := range w {
		sum += v * v
	}
	return math.Sqrt(sum)
}
func cosineWeighted(tfA, tfB map[string]float64, idf map[string]float64, normA, normB float64) float64 {
	if normA == 0 || normB == 0 {
		return 0
	}
	// iterate smaller
	if len(tfA) > len(tfB) {
		tfA, tfB = tfB, tfA
		normA, normB = normB, normA
	}
	var dot float64
	for t, fa := range tfA {
		if fb, ok := tfB[t]; ok {
			w := idf[t]
			dot += (fa * w) * (fb * w)
		}
	}
	return dot / (normA * normB)
}
// Public: Soundex-based ordering (higher is better).
// Score = 1.0 if Soundex codes equal, else common-prefix(codeA, codeB)/4.0.
func (m *ConcurrentOrderedMap[K, V]) OrderedBySoundex(
	target string,
	caseInsensitive bool,
) ([]OrderedPair[K, V], error) {
	return m.orderedByScoreWithFilter(target, caseInsensitive, "", soundexScore, "OrderedBySoundex")
}

func (m *ConcurrentOrderedMap[K, V]) OrderedBySoundexFiltered(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {
	return m.orderedByScoreWithFilter(target, caseInsensitive, filter, soundexScore, "OrderedBySoundexFiltered")
}

func soundexScore(a, b string) float64 {
	ca := soundex(a)
	cb := soundex(b)
	if ca == cb {
		return 1.0
	}
	n := 0
	for n < 4 && ca[n] == cb[n] {
		n++
	}
	return float64(n) / 4.0
}

// Standard Soundex: first letter + three digits.
func soundex(s string) string {
	if s == "" {
		return "0000"
	}
	s = strings.ToUpper(s)
	first := s[0]
	var b strings.Builder
	b.Grow(4)
	b.WriteByte(first)

	lastCode := mapSoundexRune(rune(first))
	count := 1

	for _, r := range s[1:] {
		code := mapSoundexRune(r)
		if code == '0' {
			lastCode = code
			continue
		}
		if code != lastCode {
			b.WriteByte(byte(code))
			count++
			if count == 4 {
				break
			}
		}
		lastCode = code
	}
	for count < 4 {
		b.WriteByte('0')
		count++
	}
	return b.String()
}

func mapSoundexRune(r rune) rune {
	switch r {
	case 'B', 'F', 'P', 'V':
		return '1'
	case 'C', 'G', 'J', 'K', 'Q', 'S', 'X', 'Z':
		return '2'
	case 'D', 'T':
		return '3'
	case 'L':
		return '4'
	case 'M', 'N':
		return '5'
	case 'R':
		return '6'
	default:
		return '0'
	}
}
// Public: Jaccard on character trigrams (n=3). Higher is better in [0,1].
func (m *ConcurrentOrderedMap[K, V]) OrderedByJaccardNGrams(
	target string,
	caseInsensitive bool,
) ([]OrderedPair[K, V], error) {
	return m.orderedByScoreWithFilter(target, caseInsensitive, "", func(a, b string) float64 {
		return jaccardNgrams(a, b, 3)
	}, "OrderedByJaccardNGrams")
}

func (m *ConcurrentOrderedMap[K, V]) OrderedByJaccardNGramsFiltered(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {
	return m.orderedByScoreWithFilter(target, caseInsensitive, filter, func(a, b string) float64 {
		return jaccardNgrams(a, b, 3)
	}, "OrderedByJaccardNGramsFiltered")
}

func jaccardNgrams(a, b string, n int) float64 {
	if n <= 0 {
		return 0
	}
	setA := make(map[string]struct{})
	setB := make(map[string]struct{})
	for i := 0; i+n <= len(a); i++ {
		setA[a[i:i+n]] = struct{}{}
	}
	for i := 0; i+n <= len(b); i++ {
		setB[b[i:i+n]] = struct{}{}
	}
	if len(setA) == 0 && len(setB) == 0 {
		return 1 // both empty -> identical
	}
	inter := 0
	for g := range setA {
		if _, ok := setB[g]; ok {
			inter++
		}
	}
	union := len(setA) + len(setB) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// OrderedByCombinedSimilarity orders by a weighted blend of multiple similarity metrics:
//   - Levenshtein (normalized as similarity)
//   - Damerau–Levenshtein OSA (normalized as similarity)
//   - Jaro–Winkler
//   - Cosine TF–IDF (computed over target + filtered documents)
//   - Soundex (phonetic)
//   - Jaccard N-grams (trigrams)
//
// Higher combined score is better. Stable on ties.
// Requires V == string.
func (m *ConcurrentOrderedMap[K, V]) OrderedByCombinedSimilarity(
	target string,
	caseInsensitive bool,
) ([]OrderedPair[K, V], error) {
	return m.orderedByCombinedWithFilter(target, caseInsensitive, "")
}

// OrderedByCombinedSimilarityFiltered: same as above but keeps only values that contain `filter`
// (respects caseInsensitive) before scoring.
func (m *ConcurrentOrderedMap[K, V]) OrderedByCombinedSimilarityFiltered(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {
	return m.orderedByCombinedWithFilter(target, caseInsensitive, filter)
}

// orderedByCombinedWithFilter snapshots docs, builds Cosine TF–IDF IDF across (target+docs),
// computes all metrics per doc, blends them with weights, and sorts DESC by the blended score.
func (m *ConcurrentOrderedMap[K, V]) orderedByCombinedWithFilter(
	target string,
	caseInsensitive bool,
	filter string,
) ([]OrderedPair[K, V], error) {

	m.RLock()
	defer m.RUnlock()

	var zeroV V
	if reflect.TypeOf(zeroV).Kind() != reflect.String {
		return nil, errors.New("OrderedByCombinedSimilarity: value type V must be string")
	}

	t := target
	f := filter
	if caseInsensitive {
		t = strings.ToLower(t)
		f = strings.ToLower(f)
	}

	type doc struct {
		key   K
		orig  V
		text  string
		toks  []string
		tf    map[string]float64
		score float64
	}
	docs := make([]doc, 0, len(m.keys))

	// Collect candidates (respect filter)
	for _, key := range m.keys {
		val := m.data[key]
		s := reflect.ValueOf(val).String()
		cmp := s
		if caseInsensitive {
			cmp = strings.ToLower(cmp)
		}
		if f != "" && !strings.Contains(cmp, f) {
			continue
		}
		docs = append(docs, doc{key: key, orig: val, text: cmp})
	}

	// Nothing to score
	if len(docs) == 0 {
		return []OrderedPair[K, V]{}, nil
	}

	// Cosine TF–IDF corpus is (target + docs that passed the filter).
	targetToks := tokenize(t)
	for i := range docs {
		docs[i].toks = tokenize(docs[i].text)
	}
	// Build DF across target + docs
	df := map[string]int{}
	for _, tok := range unique(targetToks) {
		df[tok]++
	}
	for i := range docs {
		seen := map[string]struct{}{}
		for _, tok := range docs[i].toks {
			if _, ok := seen[tok]; !ok {
				seen[tok] = struct{}{}
				df[tok]++
			}
		}
	}
	N := float64(len(docs) + 1) // + target
	idf := map[string]float64{}
	for term, dfi := range df {
		idf[term] = math.Log(N/float64(1+dfi)) + 1.0
	}
	targetTF := termFreq(targetToks)
	targetNorm := vecNorm(weightAndNorm(targetTF, idf))

	wLev := 1.0  // normalized edit similarity
	wDL  := 1.0  // normalized DL similarity
	wJW  := 1.0  // Jaro–Winkler
	wCos := 1.0  // Cosine TF–IDF
	wJac := 0.75 // Jaccard trigrams
	wSdx := 0.50 // Soundex
	wSum := wLev + wDL + wJW + wCos + wJac + wSdx

	// Score each doc
	for i := range docs {
		s := docs[i].text

		// Edit distances normalized to [0,1]
		maxLen := max(len(s), len(t))
		var levSim, dlSim float64
		if maxLen == 0 {
			levSim, dlSim = 1, 1
		} else {
			levSim = 1.0 - float64(levenshteinDistance(s, t))/float64(maxLen)
			dlSim = 1.0 - float64(damerauLevenshteinOSADistance(s, t))/float64(maxLen)
			if levSim < 0 {
				levSim = 0
			}
			if dlSim < 0 {
				dlSim = 0
			}
		}

		jw := jaroWinkler(s, t)
		jac := jaccardNgrams(s, t, 3)
		sdx := soundexScore(s, t)

		// Cosine (target already prepped)
		docs[i].tf = termFreq(docs[i].toks)
		docNorm := vecNorm(weightAndNorm(docs[i].tf, idf))
		cos := cosineWeighted(targetTF, docs[i].tf, idf, targetNorm, docNorm)

		score := (wLev*levSim + wDL*dlSim + wJW*jw + wCos*cos + wJac*jac + wSdx*sdx) / wSum
		docs[i].score = score
	}

	// Stable sort by score desc (best first)
	sort.SliceStable(docs, func(i, j int) bool { return docs[i].score > docs[j].score })

	out := make([]OrderedPair[K, V], len(docs))
	for i := range docs {
		out[i] = OrderedPair[K, V]{Key: docs[i].key, Value: docs[i].orig}
	}
	return out, nil
}

// -------- Vector + Text Combined Ordering (with optional filter) --------

type VectorBlendWeights struct {
    WVec float64 // vector cosine similarity
    WLev float64 // Levenshtein (normalized)
    WDL  float64 // Damerau–Levenshtein (OSA; normalized)
    WJW  float64 // Jaro–Winkler
    WCos float64 // Cosine TF-IDF (over text)
    WJac float64 // Jaccard n-grams (char)
    WSdx float64 // Soundex score
    NGramN int   // default 3
}

// Defaults tuned to give vectors strong influence while keeping textual checks meaningful.
func DefaultVectorBlendWeights() VectorBlendWeights {
    return VectorBlendWeights{
        WVec: 2.0, WLev: 1.0, WDL: 1.0, WJW: 1.0, WCos: 1.0, WJac: 0.75, WSdx: 0.50,
        NGramN: 3,
    }
}

// Extractor lets you describe where text and vector live inside your value.
type VectorTextExtractor[V any] func(v V) (text string, vec []float64, ok bool)

// OrderedByVectorCombinedSimilarity ranks by a weighted blend of:
//   - cosine(queryVec, itemVec)
//   - Levenshtein (norm), Damerau–Levenshtein (norm), Jaro–Winkler,
//   - Cosine TF–IDF (over text), Jaccard n-grams, Soundex.
// No substring filter.
func (m *ConcurrentOrderedMap[K, V]) OrderedByVectorCombinedSimilarity(
    target string,
    caseInsensitive bool,
    queryVec []float64,
    dims int, // pass 1024 for BGE-M3/OpenAI(1024); if 0, inferred from queryVec
    extract VectorTextExtractor[V],
    weights *VectorBlendWeights,
) ([]OrderedPair[K, V], error) {
    return m.orderedByVectorCombinedWithFilter(target, caseInsensitive, "", queryVec, dims, extract, weights)
}

// OrderedByVectorCombinedSimilarityFiltered same as above but keeps only items whose TEXT
// contains 'filter' (respects caseInsensitive) before scoring.
func (m *ConcurrentOrderedMap[K, V]) OrderedByVectorCombinedSimilarityFiltered(
    target string,
    caseInsensitive bool,
    filter string,
    queryVec []float64,
    dims int,
    extract VectorTextExtractor[V],
    weights *VectorBlendWeights,
) ([]OrderedPair[K, V], error) {
    return m.orderedByVectorCombinedWithFilter(target, caseInsensitive, filter, queryVec, dims, extract, weights)
}

func (m *ConcurrentOrderedMap[K, V]) orderedByVectorCombinedWithFilter(
    target string,
    caseInsensitive bool,
    filter string,
    queryVec []float64,
    dims int,
    extract VectorTextExtractor[V],
    weights *VectorBlendWeights,
) ([]OrderedPair[K, V], error) {

    if extract == nil {
        return nil, errors.New("OrderedByVectorCombinedSimilarity: extractor is nil")
    }
    if dims == 0 {
        dims = len(queryVec)
    }
    if len(queryVec) != dims {
        return nil, errors.New("query vector dimension mismatch")
    }
    if weights == nil {
        w := DefaultVectorBlendWeights()
        weights = &w
    }

    m.RLock()
    defer m.RUnlock()

    // Snapshot candidates (apply substring filter over TEXT if provided)
    type doc[K comparable, V any] struct {
        key   K
        orig  V
        text  string
        vec   []float64
        toks  []string
        tf    map[string]float64
        score float64
    }
    t := target
    f := filter
    if caseInsensitive {
        t = strings.ToLower(t)
        f = strings.ToLower(f)
    }

    docs := make([]doc[K, V], 0, len(m.keys))
    for _, key := range m.keys {
        val := m.data[key]
        txt, vec, ok := extract(val)
        if !ok || len(vec) != dims {
            continue // skip invalid items
        }
        cmp := txt
        if caseInsensitive {
            cmp = strings.ToLower(cmp)
        }
        if f != "" && !strings.Contains(cmp, f) {
            continue
        }
        docs = append(docs, doc[K, V]{key: key, orig: val, text: cmp, vec: vec})
    }
    if len(docs) == 0 {
        return []OrderedPair[K, V]{}, nil
    }

    // Prepare TF–IDF across (target + docs)
    targetToks := tokenize(t)
    for i := range docs {
        docs[i].toks = tokenize(docs[i].text)
    }
    df := map[string]int{}
    for _, tok := range unique(targetToks) { df[tok]++ }
    for i := range docs {
        seen := map[string]struct{}{}
        for _, tok := range docs[i].toks {
            if _, ok := seen[tok]; !ok {
                seen[tok] = struct{}{}
                df[tok]++
            }
        }
    }
    N := float64(len(docs) + 1)
    idf := map[string]float64{}
    for term, dfi := range df {
        idf[term] = math.Log(N/float64(1+dfi)) + 1.0
    }
    targetTF := termFreq(targetToks)
    targetNorm := vecNorm(weightAndNorm(targetTF, idf))

    // Precompute query vector norm
    qn := l2norm(queryVec)

    w := *weights
    wsum := w.WVec + w.WLev + w.WDL + w.WJW + w.WCos + w.WJac + w.WSdx
    if wsum == 0 {
        wsum = 1
    }

    for i := range docs {
        s := docs[i].text

        // Vector cosine
        cosv := cosineVec(queryVec, qn, docs[i].vec)

        // Text metrics
        maxLen := max(len(s), len(t))
        var levSim, dlSim float64
        if maxLen == 0 {
            levSim, dlSim = 1, 1
        } else {
            levSim = 1.0 - float64(levenshteinDistance(s, t))/float64(maxLen)
            dlSim  = 1.0 - float64(damerauLevenshteinOSADistance(s, t))/float64(maxLen)
            if levSim < 0 { levSim = 0 }
            if dlSim  < 0 { dlSim  = 0 }
        }
        jw  := jaroWinkler(s, t)
        jac := jaccardNgrams(s, t, max(1, w.NGramN))
        sdx := soundexScore(s, t)

        // TF–IDF cosine
        docs[i].tf = termFreq(docs[i].toks)
        dn := vecNorm(weightAndNorm(docs[i].tf, idf))
        cost := cosineWeighted(targetTF, docs[i].tf, idf, targetNorm, dn)

        score := (w.WVec*cosv + w.WLev*levSim + w.WDL*dlSim + w.WJW*jw + w.WCos*cost + w.WJac*jac + w.WSdx*sdx) / wsum
        docs[i].score = score
    }

    sort.SliceStable(docs, func(i, j int) bool { return docs[i].score > docs[j].score })

    out := make([]OrderedPair[K, V], len(docs))
    for i := range docs {
        out[i] = OrderedPair[K, V]{Key: docs[i].key, Value: docs[i].orig}
    }
    return out, nil
}

// Pure vector-only ordering (handy for benchmarks/comparison). Filtered.
func (m *ConcurrentOrderedMap[K, V]) OrderedByVectorOnlyFiltered(
    caseInsensitive bool, // unused but kept for signature symmetry
    filter string,        // substring over TEXT (via extractor)
    queryVec []float64,
    dims int,
    extract VectorTextExtractor[V],
) ([]OrderedPair[K, V], error) {

    if extract == nil {
        return nil, errors.New("OrderedByVectorOnlyFiltered: extractor is nil")
    }
    if dims == 0 {
        dims = len(queryVec)
    }
    if len(queryVec) != dims {
        return nil, errors.New("query vector dimension mismatch")
    }
    m.RLock()
    defer m.RUnlock()

    qn := l2norm(queryVec)
    type pairScore struct {
        pair  OrderedPair[K, V]
        score float64
    }
    f := filter
    if caseInsensitive {
        f = strings.ToLower(f)
    }

    items := make([]pairScore, 0, len(m.keys))
    for _, key := range m.keys {
        val := m.data[key]
        txt, vec, ok := extract(val)
        if !ok || len(vec) != dims {
            continue
        }
        cmp := txt
        if caseInsensitive {
            cmp = strings.ToLower(cmp)
        }
        if f != "" && !strings.Contains(cmp, f) {
            continue
        }
        items = append(items, pairScore{
            pair:  OrderedPair[K, V]{Key: key, Value: val},
            score: cosineVec(queryVec, qn, vec),
        })
    }
    sort.SliceStable(items, func(i, j int) bool { return items[i].score > items[j].score })
    out := make([]OrderedPair[K, V], len(items))
    for i := range items { out[i] = items[i].pair }
    return out, nil
}

// -------- Helpers for vectors --------

func cosineVec(q []float64, qn float64, v []float64) float64 {
    if qn == 0 {
        return 0
    }
    var dot, vn2 float64
    for i := 0; i < len(q); i++ {
        dot += q[i] * v[i]
        vn2 += v[i] * v[i]
    }
    if vn2 == 0 {
        return 0
    }
    return dot / (qn * math.Sqrt(vn2))
}

func l2norm(x []float64) float64 {
    var s float64
    for _, v := range x { s += v * v }
    return math.Sqrt(s)
}

