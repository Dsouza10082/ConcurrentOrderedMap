// Example: Basic Persistent Map
//
// Shows the core persistence primitives: Set, Get, VectorSearch, Snapshot.
// Run twice to observe data recovery from disk.
package main

import (
	"fmt"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"github.com/Dsouza10082/ConcurrentOrderedMap/persistence"
)

// Product is the value type stored in the persistent map.
type Product struct {
	Name        string
	Description string
	Price       float64
	Category    string
}

// entry pairs a string key with a Product for the seed list.
type entry struct {
	Key   string
	Value Product
}

func embed(text string) []float64 {
	const dims = 64
	vec := make([]float64, dims)
	r := rand.New(rand.NewSource(hashStr(text)))
	for i := range vec {
		vec[i] = r.NormFloat64()
	}
	var n float64
	for _, v := range vec {
		n += v * v
	}
	n = math.Sqrt(n)
	for i := range vec {
		vec[i] /= n
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

func main() {
	dir := filepath.Join(os.TempDir(), "persistent-map-demo")
	fmt.Printf("Data dir: %s\n\n", dir)

	opts := persistence.DefaultOptions()
	opts.VectorDims = 64

	store, err := persistence.New[string, Product](dir, opts)
	if err != nil {
		panic(err)
	}
	defer store.Close()

	isNew := store.Len() == 0

	if isNew {
		fmt.Println("── First run: populating store ──")
		products := []entry{
			{"p1", Product{"Laptop Pro 15", "High-performance laptop for developers", 1299.99, "electronics"}},
			{"p2", Product{"Mechanical Keyboard", "Tactile switches, programmable RGB", 149.99, "electronics"}},
			{"p3", Product{"Standing Desk", "Height-adjustable ergonomic desk", 699.99, "furniture"}},
			{"p4", Product{"Noise-Canceling Headphones", "Over-ear, 30h battery", 349.99, "electronics"}},
			{"p5", Product{"Coffee Grinder", "Burr grinder, 15 grind settings", 89.99, "kitchen"}},
		}

		for _, item := range products {
			vecText := item.Value.Name + " " + item.Value.Description
			if err := store.SetWithVector(item.Key, item.Value, embed(vecText)); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			fmt.Printf("  Stored: %s → %s\n", item.Key, item.Value.Name)
		}
	} else {
		fmt.Printf("── Second run: loaded %d items from disk ──\n", store.Len())
		pairs := store.Inner().GetOrderedV2()
		for _, p := range pairs {
			fmt.Printf("  Loaded: %s → %s (EUR %.2f)\n", p.Key, p.Value.Name, p.Value.Price)
		}
	}

	// Vector search
	fmt.Println("\n── Semantic Search: 'audio equipment for work' ──")
	results, err := store.VectorSearch(embed("audio equipment for work"), 3, nil)
	if err != nil {
		fmt.Printf("Search error: %v\n", err)
	} else {
		for i, r := range results {
			p, _ := store.Get(r.Key)
			fmt.Printf("  %d. [%.4f] %s - EUR %.2f\n", i+1, r.Score, p.Name, p.Price)
		}
	}

	// Snapshot
	fmt.Println("\n── Manual Snapshot ──")
	if err := store.Snapshot(); err != nil {
		fmt.Printf("Snapshot error: %v\n", err)
	} else {
		fmt.Println("  Snapshot written.")
	}

	fmt.Printf("\nTotal entries: %d\n", store.Len())
}