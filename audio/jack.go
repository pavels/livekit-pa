//go:build !nojack
// +build !nojack

package audio

import (
	"fmt"

	"github.com/xthexder/go-jack"
)

type JackClient struct {
	client      *jack.Client
	portIn      *jack.Port
	portOut     *jack.Port
	inputBuffer *CircularBuffer
	outputMixer *Mixer
}

func NewJackClient(name string, inputBuffer *CircularBuffer, outputMixer *Mixer) (*JackClient, error) {
	client, status := jack.ClientOpen(name, jack.NoStartServer)
	if status != 0 {
		return nil, fmt.Errorf("jack error: %s", jack.StrError(status))
	}

	if sr := client.GetSampleRate(); sr != 48000 {
		client.Close()
		return nil, fmt.Errorf("jack sample rate must be 48000Hz, got %d", sr)
	}

	jc := &JackClient{
		client:      client,
		portIn:      client.PortRegister("in", jack.DEFAULT_AUDIO_TYPE, jack.PortIsInput, 0),
		portOut:     client.PortRegister("out", jack.DEFAULT_AUDIO_TYPE, jack.PortIsOutput, 0),
		inputBuffer: inputBuffer,
		outputMixer: outputMixer,
	}

	client.OnShutdown(func() {
		fmt.Println("JACK shutdown")
	})

	client.SetProcessCallback(func(nframes uint32) int {
		in := jc.portIn.GetBuffer(nframes)
		out := jc.portOut.GetBuffer(nframes)

		inSamples := make([]int16, nframes)
		for i := range in {
			inSamples[i] = audioSampleToInt16(in[i])
		}
		jc.inputBuffer.Write(inSamples)

		outSamples := jc.outputMixer.Read(int(nframes))
		for i := range outSamples {
			out[i] = int16ToAudioSample(outSamples[i])
		}
		return 0
	})

	return jc, nil
}

func (jc *JackClient) Start() error {
	ret := jc.client.Activate()
	if ret != 0 {
		return fmt.Errorf("Jack Error: %d", ret)
	}

	return nil
}

func (jc *JackClient) Close() {
	jc.client.Close()
}

func audioSampleToInt16(f jack.AudioSample) int16 {
	v := float32(f) * 32767.0
	if v > 32767 {
		v = 32767
	} else if v < -32768 {
		v = -32768
	}
	return int16(v)
}

func int16ToAudioSample(i int16) jack.AudioSample {
	return jack.AudioSample(float32(i) / 32767.0)
}
