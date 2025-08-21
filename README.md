
<img width="1536" height="1024" alt="gopher_doing_multiple_things_strong" src="https://github.com/user-attachments/assets/9cc7a8a4-9932-4bc6-8caf-fbf4e30b5866" />

# ConcurrentOrderedMap
ConcurrentOrderedMap provides a thread-safe map implementation that preserves insertion order. It combines the safety of synchronized access with predictable iteration order, making it suitable for concurrent environments where key ordering matters

# Install
- go get github.com/Dsouza10082/ConcurrentOrderedMap@v1.0.3

# Use cases:
- Concurrent access patterns with deterministic iteration requirements
- Scenarios requiring both thread-safety and insertion order guarantees
- Replacing sync.Map when ordered traversal is needed
- Singleton for database connections
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
