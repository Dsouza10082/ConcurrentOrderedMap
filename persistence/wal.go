// Package persistence adds durable storage to ConcurrentOrderedMap.
// It uses a Write-Ahead Log (WAL) for crash-safe individual writes and
// periodic binary snapshots for fast recovery.
//
// Design goals:
//   - Zero external dependencies (stdlib only: os, encoding/gob, compress/zlib)
//   - Non-blocking reads; writers hold the lock only during encode+append
//   - Recovery is deterministic: snapshot base + WAL replay in sequence order
//   - Optional fsync-on-write for mission-critical durability
package persistence

import (
	"bufio"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
)

// OpType identifies the kind of mutation recorded in the WAL.
type OpType uint8

const (
	OpSet    OpType = 1
	OpDelete OpType = 2
	walMagic        = 0xC0A2_0A2A // "COM" marker to detect corrupt entries
)

// walEntry is the serialisable unit written to the WAL file.
type walEntry[K comparable, V any] struct {
	Magic uint32
	Seq   uint64
	Op    OpType
	Key   K
	Value V // only meaningful when Op == OpSet
}

// WALWriter appends entries to a WAL file.
// It is safe for concurrent use.
type WALWriter[K comparable, V any] struct {
	mu   sync.Mutex
	f    *os.File
	bw   *bufio.Writer
	enc  *gob.Encoder
	seq  atomic.Uint64
	sync bool // fsync on every write
}

// openWAL opens (or creates) the WAL at path.
func openWAL[K comparable, V any](path string, syncWrites bool) (*WALWriter[K, V], error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("persistence: open WAL %q: %w", path, err)
	}
	bw := bufio.NewWriterSize(f, 64*1024)
	return &WALWriter[K, V]{
		f:    f,
		bw:   bw,
		enc:  gob.NewEncoder(bw),
		sync: syncWrites,
	}, nil
}

// Append writes a single entry to the WAL.
func (w *WALWriter[K, V]) Append(op OpType, key K, value V) error {
	seq := w.seq.Add(1)
	entry := walEntry[K, V]{
		Magic: walMagic,
		Seq:   seq,
		Op:    op,
		Key:   key,
		Value: value,
	}

	// Prefix each gob record with its byte-length so we can detect truncated
	// entries during replay without buffering the whole file into memory.
	w.mu.Lock()
	defer w.mu.Unlock()

	// Two-phase write: encode into an intermediate buffer first to get the size.
	pr, pw := io.Pipe()
	encBuf := gob.NewEncoder(pw)
	errCh := make(chan error, 1)
	go func() {
		errCh <- encBuf.Encode(entry)
		pw.Close()
	}()

	raw, err := io.ReadAll(pr)
	if err != nil {
		return err
	}
	if err := <-errCh; err != nil {
		return fmt.Errorf("persistence: encode WAL entry: %w", err)
	}

	// Write 4-byte little-endian length prefix
	var lenBuf [4]byte
	binary.LittleEndian.PutUint32(lenBuf[:], uint32(len(raw)))
	if _, err := w.bw.Write(lenBuf[:]); err != nil {
		return err
	}
	if _, err := w.bw.Write(raw); err != nil {
		return err
	}
	if err := w.bw.Flush(); err != nil {
		return err
	}
	if w.sync {
		return w.f.Sync()
	}
	return nil
}

// Flush ensures the OS buffer is flushed to disk.
func (w *WALWriter[K, V]) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := w.bw.Flush(); err != nil {
		return err
	}
	return w.f.Sync()
}

// Close flushes and closes the underlying file.
func (w *WALWriter[K, V]) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.bw.Flush()
	return w.f.Close()
}

// replayWAL reads all valid entries from the WAL file at path and calls
// onEntry for each one in sequence order. Truncated / corrupt records at the
// tail are silently ignored (crash-safe).
func replayWAL[K comparable, V any](path string, onEntry func(walEntry[K, V])) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("persistence: open WAL for replay %q: %w", path, err)
	}
	defer f.Close()

	br := bufio.NewReader(f)
	for {
		var lenBuf [4]byte
		if _, err := io.ReadFull(br, lenBuf[:]); err != nil {
			break // EOF or truncation
		}
		size := binary.LittleEndian.Uint32(lenBuf[:])
		raw := make([]byte, size)
		if _, err := io.ReadFull(br, raw); err != nil {
			break
		}

		var entry walEntry[K, V]
		if err := gob.NewDecoder(
			&byteReader{b: raw},
		).Decode(&entry); err != nil {
			break // corrupt; stop replay here
		}
		if entry.Magic != walMagic {
			break
		}
		onEntry(entry)
	}
	return nil
}

// byteReader wraps a []byte as an io.Reader for gob.NewDecoder.
type byteReader struct {
	b   []byte
	pos int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.b) {
		return 0, io.EOF
	}
	n = copy(p, r.b[r.pos:])
	r.pos += n
	return n, nil
}
