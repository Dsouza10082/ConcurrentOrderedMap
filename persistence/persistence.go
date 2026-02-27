package persistence

import (
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	com "github.com/Dsouza10082/ConcurrentOrderedMap/v2"
)

// ─── Options ──────────────────────────────────────────────────────────────────

// Options configures the persistent map.
type Options struct {
	// SnapshotInterval is how often a compaction snapshot is written.
	// Set to 0 to disable automatic snapshots (you can call Snapshot() manually).
	SnapshotInterval time.Duration

	// MaxWALEntries triggers a snapshot + WAL truncation once the WAL grows
	// beyond this many entries. 0 = disabled.
	MaxWALEntries int

	// SyncOnWrite calls fdatasync after every WAL append.
	// Safer but slower. Recommended for critical data.
	SyncOnWrite bool

	// VectorDims is the embedding dimensionality. Set >0 to enable the vector
	// index. Vectors are stored alongside values using SetWithVector.
	VectorDims int
}

// DefaultOptions returns sensible defaults.
func DefaultOptions() Options {
	return Options{
		SnapshotInterval: 5 * time.Minute,
		MaxWALEntries:    100_000,
		SyncOnWrite:      false,
		VectorDims:       0,
	}
}

// ─── Snapshot format ─────────────────────────────────────────────────────────

// snapshot is the gob-serialised form of the whole map at a point in time.
type snapshot[K comparable, V any] struct {
	Entries []snapshotEntry[K, V]
	Vectors []snapshotVector[K] // only populated when VectorDims > 0
	Seq     uint64              // last WAL sequence included in this snapshot
}

type snapshotEntry[K comparable, V any] struct {
	Key   K
	Value V
}

type snapshotVector[K comparable] struct {
	Key    K
	Vector []float64
}

// ─── PersistentConcurrentOrderedMap ─────────────────────────────────────────

// PersistentConcurrentOrderedMap wraps ConcurrentOrderedMap with WAL-based
// durability and an optional in-memory vector index for semantic search.
//
// Files created under `dir`:
//   - data.wal   — append-only write-ahead log
//   - data.snap  — latest binary snapshot
//   - data.snap.tmp — transient during snapshot write (atomic rename)
type PersistentConcurrentOrderedMap[K comparable, V any] struct {
	inner *com.ConcurrentOrderedMap[K, V]
	wal   *WALWriter[K, V]
	vec   *FlatVectorIndex[K] // nil when VectorDims == 0

	dir    string
	opts   Options
	mu     sync.Mutex // guards snapshot operations
	closed bool

	walEntryCount int64 // approximate; used for MaxWALEntries check
	stopCh        chan struct{}
	doneCh        chan struct{}

	// vectors is a parallel key→vector map (not persisted in the primary map).
	vectors map[K][]float64
}

// New opens or creates a PersistentConcurrentOrderedMap in `dir`.
// If existing data is found it is loaded before the function returns.
func New[K comparable, V any](dir string, opts Options) (*PersistentConcurrentOrderedMap[K, V], error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("persistence: mkdir %q: %w", dir, err)
	}

	p := &PersistentConcurrentOrderedMap[K, V]{
		inner:   com.NewConcurrentOrderedMap[K, V](),
		dir:     dir,
		opts:    opts,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		vectors: make(map[K][]float64),
	}

	if opts.VectorDims > 0 {
		p.vec = NewFlatVectorIndex[K](opts.VectorDims)
	}

	if err := p.load(); err != nil {
		return nil, err
	}

	if opts.SnapshotInterval > 0 {
		go p.snapshotLoop()
	}

	return p, nil
}

// ─── Public API (wraps ConcurrentOrderedMap) ─────────────────────────────────

// Set stores key/value and appends a WAL entry.
func (p *PersistentConcurrentOrderedMap[K, V]) Set(key K, value V) error {
	p.inner.Set(key, value)
	if err := p.wal.Append(OpSet, key, value); err != nil {
		return err
	}
	return p.maybeSnapshot()
}

// SetWithVector stores key/value and its embedding vector.
// The vector is indexed for semantic search (requires VectorDims > 0).
func (p *PersistentConcurrentOrderedMap[K, V]) SetWithVector(key K, value V, vec []float64) error {
	if err := p.Set(key, value); err != nil {
		return err
	}
	if p.vec != nil {
		if err := p.vec.Upsert(key, vec); err != nil {
			return fmt.Errorf("persistence: vector upsert: %w", err)
		}
		p.mu.Lock()
		vCopy := make([]float64, len(vec))
		copy(vCopy, vec)
		p.vectors[key] = vCopy
		p.mu.Unlock()
	}
	return nil
}

// Get retrieves a value by key.
func (p *PersistentConcurrentOrderedMap[K, V]) Get(key K) (V, bool) {
	return p.inner.Get(key)
}

// Delete removes key/value and appends a WAL delete entry.
func (p *PersistentConcurrentOrderedMap[K, V]) Delete(key K) error {
	p.inner.Delete(key)
	var zero V
	if err := p.wal.Append(OpDelete, key, zero); err != nil {
		return err
	}
	if p.vec != nil {
		p.vec.Delete(key)
		p.mu.Lock()
		delete(p.vectors, key)
		p.mu.Unlock()
	}
	return p.maybeSnapshot()
}

// Exists reports whether key is present.
func (p *PersistentConcurrentOrderedMap[K, V]) Exists(key K) bool {
	return p.inner.Exists(key)
}

// Len returns the number of stored entries.
func (p *PersistentConcurrentOrderedMap[K, V]) Len() int {
	return p.inner.Len()
}

// Inner exposes the underlying ConcurrentOrderedMap for read operations
// and similarity searches (Levenshtein, vector, etc.).
func (p *PersistentConcurrentOrderedMap[K, V]) Inner() *com.ConcurrentOrderedMap[K, V] {
	return p.inner
}

// VectorSearch performs a cosine-similarity search over all stored vectors.
// Requires VectorDims > 0 in Options.
func (p *PersistentConcurrentOrderedMap[K, V]) VectorSearch(
	queryVec []float64,
	topK int,
	filter func(K) bool,
) ([]VectorResult[K], error) {
	if p.vec == nil {
		return nil, fmt.Errorf("persistence: VectorDims not configured")
	}
	return p.vec.Search(queryVec, topK, filter)
}

// ─── Persistence internals ───────────────────────────────────────────────────

func (p *PersistentConcurrentOrderedMap[K, V]) walPath() string {
	return filepath.Join(p.dir, "data.wal")
}
func (p *PersistentConcurrentOrderedMap[K, V]) snapPath() string {
	return filepath.Join(p.dir, "data.snap")
}
func (p *PersistentConcurrentOrderedMap[K, V]) snapTmpPath() string {
	return filepath.Join(p.dir, "data.snap.tmp")
}

// load restores state from the latest snapshot then replays the WAL.
func (p *PersistentConcurrentOrderedMap[K, V]) load() error {
	// 1. Load snapshot (if any)
	var baseSeq uint64
	if snapData, err := p.readSnapshot(); err == nil {
		for _, e := range snapData.Entries {
			p.inner.Set(e.Key, e.Value)
		}
		for _, sv := range snapData.Vectors {
			if p.vec != nil {
				_ = p.vec.Upsert(sv.Key, sv.Vector)
			}
			p.vectors[sv.Key] = sv.Vector
		}
		baseSeq = snapData.Seq
	}

	// 2. Replay WAL (entries with Seq > baseSeq)
	err := replayWAL[K, V](p.walPath(), func(entry walEntry[K, V]) {
		if entry.Seq <= baseSeq {
			return
		}
		switch entry.Op {
		case OpSet:
			p.inner.Set(entry.Key, entry.Value)
		case OpDelete:
			p.inner.Delete(entry.Key)
			if p.vec != nil {
				p.vec.Delete(entry.Key)
			}
			delete(p.vectors, entry.Key)
		}
	})
	if err != nil {
		return err
	}

	// 3. Open WAL for appending
	wal, err := openWAL[K, V](p.walPath(), p.opts.SyncOnWrite)
	if err != nil {
		return err
	}
	p.wal = wal
	return nil
}

// Snapshot writes a compacted snapshot and truncates the WAL.
func (p *PersistentConcurrentOrderedMap[K, V]) Snapshot() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	pairs := p.inner.GetOrderedV2()
	snap := snapshot[K, V]{
		Entries: make([]snapshotEntry[K, V], len(pairs)),
		Seq:     p.wal.seq.Load(),
	}
	for i, pair := range pairs {
		snap.Entries[i] = snapshotEntry[K, V]{Key: pair.Key, Value: pair.Value}
	}
	for k, v := range p.vectors {
		snap.Vectors = append(snap.Vectors, snapshotVector[K]{Key: k, Vector: v})
	}

	// Atomic write: tmp → rename
	tmp := p.snapTmpPath()
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("persistence: create snap tmp: %w", err)
	}
	if err := gob.NewEncoder(f).Encode(snap); err != nil {
		f.Close()
		return fmt.Errorf("persistence: encode snapshot: %w", err)
	}
	f.Sync()
	f.Close()
	if err := os.Rename(tmp, p.snapPath()); err != nil {
		return fmt.Errorf("persistence: rename snapshot: %w", err)
	}

	// Truncate WAL (reopen for append from empty)
	_ = p.wal.Close()
	if err := os.Truncate(p.walPath(), 0); err != nil {
		return fmt.Errorf("persistence: truncate WAL: %w", err)
	}
	p.wal, err = openWAL[K, V](p.walPath(), p.opts.SyncOnWrite)
	return err
}

func (p *PersistentConcurrentOrderedMap[K, V]) readSnapshot() (snapshot[K, V], error) {
	var snap snapshot[K, V]
	f, err := os.Open(p.snapPath())
	if err != nil {
		return snap, err
	}
	defer f.Close()
	err = gob.NewDecoder(f).Decode(&snap)
	return snap, err
}

func (p *PersistentConcurrentOrderedMap[K, V]) maybeSnapshot() error {
	if p.opts.MaxWALEntries <= 0 {
		return nil
	}
	p.walEntryCount++
	if int(p.walEntryCount)%p.opts.MaxWALEntries == 0 {
		return p.Snapshot()
	}
	return nil
}

// snapshotLoop runs the background snapshot goroutine.
func (p *PersistentConcurrentOrderedMap[K, V]) snapshotLoop() {
	defer close(p.doneCh)
	ticker := time.NewTicker(p.opts.SnapshotInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			_ = p.Snapshot()
		case <-p.stopCh:
			return
		}
	}
}

// Sync flushes WAL buffers to OS.
func (p *PersistentConcurrentOrderedMap[K, V]) Sync() error {
	return p.wal.Flush()
}

// Close flushes, takes a final snapshot, and releases all resources.
func (p *PersistentConcurrentOrderedMap[K, V]) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	if p.opts.SnapshotInterval > 0 {
		close(p.stopCh)
		<-p.doneCh
	}
	_ = p.Snapshot()
	return p.wal.Close()
}
