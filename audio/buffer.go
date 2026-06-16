package audio

import (
	"fmt"
	"sync"
)

type CircularBuffer struct {
	buf     []int16
	size    uint64
	writeAt uint64
	readAt  uint64
	count   uint64
	sync    sync.Mutex
}

func NewCircularBuffer(size uint64) *CircularBuffer {
	fmt.Printf("Alloc buffer %d\n", size)
	return &CircularBuffer{buf: make([]int16, size), size: size}
}

func (cb *CircularBuffer) Write(samples []int16) {
	cb.sync.Lock()
	defer cb.sync.Unlock()

	for _, s := range samples {
		if cb.count < cb.size {
			cb.buf[cb.writeAt] = s
			cb.writeAt = (cb.writeAt + 1) % cb.size
			cb.count++
		} else {
			// Overflow condition - flush everything and start over
			fmt.Printf("Buffer overflow\n")
			cb.Flush()
			break
		}
	}
}

// ReadInto fills out with samples from the buffer without allocating.
// Writes silence if fewer than 2*len(out) samples are available.
func (cb *CircularBuffer) ReadInto(out []int16) {
	cb.sync.Lock()
	defer cb.sync.Unlock()

	n := uint64(len(out))
	if cb.count < n*2 {
		for i := range out {
			out[i] = 0
		}
		return
	}

	for i := uint64(0); i < n; i++ {
		out[i] = cb.buf[cb.readAt]
		cb.readAt = (cb.readAt + 1) % cb.size
		cb.count--
	}
}

func (cb *CircularBuffer) Count() uint64 {
	return cb.count
}

func (cb *CircularBuffer) Flush() {
	cb.readAt = 0
	cb.writeAt = 0
	cb.count = 0
}
