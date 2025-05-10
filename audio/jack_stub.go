//go:build nojack
// +build nojack

package audio

import (
	"fmt"
)

type JackClient struct {
}

func NewJackClient(name string, inputBuffer *CircularBuffer, outputMixer *Mixer) (*JackClient, error) {
	return nil, fmt.Errorf("JACK Support is not compiled.")
}

func (jc *JackClient) Start() int {
	return 0
}

func (jc *JackClient) Close() {
}
