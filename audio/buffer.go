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

func (cb *CircularBuffer) Read(n uint64) []int16 {
	cb.sync.Lock()
	defer cb.sync.Unlock()

	res := make([]int16, n)

	if cb.count < n*2 {
		return res
	}

	for i := uint64(0); i < n; i++ {
		res[i] = cb.buf[cb.readAt]
		cb.readAt = (cb.readAt + 1) % cb.size
		cb.count--
	}

	return res
}

func (cb *CircularBuffer) Count() uint64 {
	return cb.count
}

func (cb *CircularBuffer) Flush() {
	cb.readAt = 0
	cb.writeAt = 0
	cb.count = 0
}
