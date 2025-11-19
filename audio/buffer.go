package audio

type CircularBuffer struct {
	buf       []int16
	size      int
	writeAt   int
	readAt    int
	count     int
	underflow bool
	overflow  bool
}

func NewCircularBuffer() *CircularBuffer {
	size := 960 * 8
	return &CircularBuffer{buf: make([]int16, size), size: size, underflow: true, overflow: false}
}

func (cb *CircularBuffer) Write(samples []int16) {
	if cb.overflow {
		return
	}

	for _, s := range samples {
		if cb.count < cb.size {
			cb.buf[cb.writeAt] = s
			cb.writeAt = (cb.writeAt + 1) % cb.size
			cb.count++
			if cb.underflow && cb.count > cb.size/2 {
				cb.underflow = false
			}
		} else {
			cb.overflow = true
			break
		}
	}
}

func (cb *CircularBuffer) Read(n int) []int16 {
	res := make([]int16, n)

	if cb.underflow {
		return res
	}

	for i := 0; i < n; i++ {
		if cb.count > 0 {
			res[i] = cb.buf[cb.readAt]
			cb.readAt = (cb.readAt + 1) % cb.size
			cb.count--
			if cb.overflow && cb.count < cb.size/2 {
				cb.overflow = false
			}
		} else {
			cb.underflow = true
			break
		}
	}

	return res
}

func (cb *CircularBuffer) Count() int {
	return cb.count
}

func (cb *CircularBuffer) Flush() {
	cb.readAt = 0
	cb.writeAt = 0
	cb.count = 0
	cb.underflow = true
	cb.overflow = false
}
