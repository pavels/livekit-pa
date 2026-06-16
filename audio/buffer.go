package audio

import (
	"fmt"
	"sync/atomic"
)

// CircularBuffer is a lockless single-producer single-consumer ring buffer.
// Write must be called from exactly one goroutine; ReadInto from exactly one
// (different) goroutine.
type CircularBuffer struct {
	buf          []int16
	size         uint64
	write        atomic.Uint64
	read         atomic.Uint64
	flushPending atomic.Bool
}

func NewCircularBuffer(size uint64) *CircularBuffer {
	fmt.Printf("Alloc buffer %d\n", size)
	return &CircularBuffer{
		buf:  make([]int16, size),
		size: size,
	}
}

// Write appends samples from the producer. On overflow, signals the consumer
// to flush stale data and drops the incoming batch.
func (cb *CircularBuffer) Write(samples []int16) {
	w := cb.write.Load()
	r := cb.read.Load()
	if uint64(len(samples)) > cb.size-(w-r) {
		fmt.Printf("Buffer overflow\n")
		cb.flushPending.Store(true)
		return
	}
	for _, s := range samples {
		cb.buf[w%cb.size] = s
		w++
	}
	cb.write.Store(w)
}

// ReadInto fills out from the consumer without allocating. Writes silence if
// fewer than 2*len(out) samples are buffered. Flushes stale data first if the
// producer signalled an overflow.
func (cb *CircularBuffer) ReadInto(out []int16) {
	if cb.flushPending.Load() {
		cb.read.Store(cb.write.Load())
		cb.flushPending.Store(false)
	}
	w := cb.write.Load()
	r := cb.read.Load()
	n := uint64(len(out))
	if w-r < n*2 {
		for i := range out {
			out[i] = 0
		}
		return
	}
	for i := range out {
		out[i] = cb.buf[r%cb.size]
		r++
	}
	cb.read.Store(r)
}

func (cb *CircularBuffer) Count() uint64 {
	return cb.write.Load() - cb.read.Load()
}

func (cb *CircularBuffer) Size() uint64 {
	return cb.size
}

// Flush discards all buffered samples. Safe to call when the consumer is idle.
func (cb *CircularBuffer) Flush() {
	cb.read.Store(cb.write.Load())
}
