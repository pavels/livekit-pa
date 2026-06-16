package audio

import (
	"math"
	"sync"
)

type Mixer struct {
	sync.Mutex
	buffers    map[string]*CircularBuffer
	bufferSize uint64
	readBufs   map[string][]int16
	energies   map[string]float64
}

func NewMixer(bufferSize uint64) *Mixer {
	return &Mixer{
		buffers:    make(map[string]*CircularBuffer),
		bufferSize: bufferSize,
		readBufs:   make(map[string][]int16),
		energies:   make(map[string]float64),
	}
}

func (m *Mixer) AddBuffer(id string) *CircularBuffer {
	buf := NewCircularBuffer(m.bufferSize)
	readBuf := make([]int16, m.bufferSize)

	m.Lock()
	defer m.Unlock()
	m.buffers[id] = buf
	m.readBufs[id] = readBuf
	m.energies[id] = 0

	return buf
}

func (m *Mixer) RemoveBuffer(id string) {
	m.Lock()
	defer m.Unlock()
	delete(m.buffers, id)
	delete(m.readBufs, id)
	delete(m.energies, id)
}

// ReadInto fills out with mixed audio without allocating.
// Uses pre-allocated per-participant buffers; safe to call from an RT thread
// as long as no participant is added/removed concurrently (the mutex ensures this).
func (m *Mixer) ReadInto(out []int16) {
	m.Lock()
	defer m.Unlock()

	n := uint64(len(out))

	totalEnergy := 0.0
	for id, buf := range m.buffers {
		rb := m.readBufs[id][:n]
		buf.ReadInto(rb)
		var sumSq float64
		for _, s := range rb {
			f := float64(s)
			sumSq += f * f
		}
		m.energies[id] = math.Sqrt(sumSq / float64(n))
		totalEnergy += m.energies[id]
	}

	for i := range out {
		out[i] = 0
	}

	if totalEnergy == 0 {
		return
	}

	for id := range m.buffers {
		weight := m.energies[id] / totalEnergy
		rb := m.readBufs[id][:n]
		for i := uint64(0); i < n; i++ {
			out[i] += int16(float64(rb[i]) * weight)
		}
	}
}
