package main


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
	"reflect"
	"sort"
	"sync"
)

type ConcurrentOrderedMap[K comparable, V any] struct {
	sync.RWMutex
	data  map[K]V
	keys  []K
	index map[K]int
}

type OrderDirection int

const (
    ASC  OrderDirection = iota
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   pairs := map.GetOrderedV2()
//   fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   pairs := map.GetOrderedV2()
//   fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
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
//   map := NewConcurrentOrderedMapWithCapacity[string, int](10)
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   pairs := map.GetOrderedV2()
//   fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   exists := map.Exists("a")
//   fmt.Println(exists) // Output: true
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   fmt.Println(map) // Output: {a: 1, b: 2, c: 3}
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   value, exists := map.Get("a")
//   fmt.Println(value, exists) // Output: 1 true
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   err := map.UpdateWithPointer("a", func(v *int) error {
//     *v = 2
//     return nil
//   })
//   fmt.Println(err) // Output: nil
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   err := map.Update("a", func(v *int) error {
//     *v = 2
//     return nil
//   })
//   fmt.Println(err) // Output: nil
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   map.Delete("b")
//   fmt.Println(map) // Output: {a: 1, c: 3}
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   fmt.Println(map.Len()) // Output: 3
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   pairs := map.GetOrdered()
//   fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   keys := map.Keys()
//   fmt.Println(keys) // Output: [a b c]
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   values := map.Values()
//   fmt.Println(values) // Output: [1 2 3]
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   map.Clear()
//   fmt.Println(map) // Output: {}
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   pairs := map.GetOrderedByField(OrderBy{Field: "a", Direction: ASC})
//   fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   map.Set("b", 2)
//   map.Set("c", 3)
//   pairs := map.GetOrderedByMultiField([]OrderBy{OrderBy{Field: "a", Direction: ASC}, OrderBy{Field: "b", Direction: DESC}})
//   fmt.Println(pairs) // Output: [{a 1} {b 2} {c 3}]
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   err := map.UpdateGeneric("a", DirectUpdate{NewValue: 2})
//   fmt.Println(err) // Output: nil
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   err := map.UpdateGeneric("a", FunctionUpdate{UpdateFunc: func(current *int) error {
//     *current = 2
//     return nil
//   }})
//   fmt.Println(err) // Output: nil
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
//   map := NewConcurrentOrderedMap[string, int]()	
//   map.Set("a", 1)
//   updated, err := map.UpdateGeneric("a", FunctionUpdate{UpdateFunc: func(current *int) error {
//     *current = 2
//     return nil
//   }})
//   fmt.Println(updated, err) // Output: 2 nil
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   err := map.UpdateGeneric("a", FieldUpdate{Fields: map[string]interface{}{"a": 2}})
//   fmt.Println(err) // Output: nil
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.Set("a", 1)
//   updated, err := map.UpdateGeneric("a", FieldUpdate{Fields: map[string]interface{}{"a": 2}})
//   fmt.Println(updated, err) // Output: 2 nil
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
//   map := NewConcurrentOrderedMap[string, int]()
//   map.SetOrUpdate("a", 1, func(current *int) *int {
//     *current = 2
//     return current
//   })
//   fmt.Println(map) // Output: {a: 2}
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
