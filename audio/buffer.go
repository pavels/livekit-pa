package audio

type CircularBuffer struct {
	buf     []int16
	size    int
	writeAt int
	readAt  int
	count   int
}

func NewCircularBuffer(size int) *CircularBuffer {
	return &CircularBuffer{buf: make([]int16, size), size: size}
}

func (cb *CircularBuffer) Write(samples []int16) {
	for _, s := range samples {
		if cb.count < cb.size {
			cb.buf[cb.writeAt] = s
			cb.writeAt = (cb.writeAt + 1) % cb.size
			cb.count++
		} else {
			// drop sample when full
			break
		}
	}
}

func (cb *CircularBuffer) Read(n int) []int16 {
	if cb.count < cb.size/4 {
		return make([]int16, n) // return silence if insufficient data
	}
	res := make([]int16, n)
	for i := 0; i < n; i++ {
		res[i] = cb.buf[cb.readAt]
		cb.readAt = (cb.readAt + 1) % cb.size
	}
	cb.count -= n
	return res
}

func (cb *CircularBuffer) Count() int {
	return cb.count
}

func (cb *CircularBuffer) Flush() {
	cb.readAt = 0
	cb.writeAt = 0
	cb.count = 0
}
