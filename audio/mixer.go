package audio

import (
	"math"
	"sync"
)

type Mixer struct {
	sync.Mutex
	buffers    map[string]*CircularBuffer
	bufferSize uint64
}

func NewMixer(bufferSize uint64) *Mixer {
	return &Mixer{
		buffers:    make(map[string]*CircularBuffer),
		bufferSize: bufferSize,
	}
}

func (m *Mixer) AddBuffer(id string) *CircularBuffer {
	m.Lock()
	defer m.Unlock()
	m.buffers[id] = NewCircularBuffer(m.bufferSize)
	return m.buffers[id]
}

func (m *Mixer) RemoveBuffer(id string) {
	m.Lock()
	defer m.Unlock()
	delete(m.buffers, id)
}

func (m *Mixer) Read(n uint64) []int16 {
	m.Lock()
	defer m.Unlock()
	out := make([]int16, n)
	energies := make(map[string]float64)
	sources := make(map[string][]int16)

	for id, buf := range m.buffers {
		samples := buf.Read(n)
		if len(samples) == 0 {
			continue
		}
		sources[id] = samples

		var sumSq float64
		for _, s := range samples {
			f := float64(s)
			sumSq += f * f
		}
		energies[id] = math.Sqrt(sumSq / float64(len(samples)))
	}

	totalEnergy := 0.0
	for _, e := range energies {
		totalEnergy += e
	}
	if totalEnergy == 0 {
		return out // all silent
	}

	for id, samples := range sources {
		weight := energies[id] / totalEnergy
		for i := uint64(0); i < n; i++ {
			out[i] += int16(float64(samples[i]) * weight)
		}
	}

	return out
}
