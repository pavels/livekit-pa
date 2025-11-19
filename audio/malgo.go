package audio

import (
	"encoding/binary"
	"fmt"

	"github.com/gen2brain/malgo"

	"C"
)
import "bytes"

type MalgoClient struct {
	context     *malgo.AllocatedContext
	device      *malgo.Device
	inputBuffer *CircularBuffer
	outputMixer *Mixer
}

func NewMalgoClient(inputBuffer *CircularBuffer, outputMixer *Mixer) (*MalgoClient, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(message string) {
		fmt.Printf(message)
	})

	if err != nil {
		return nil, err
	}

	deviceConfig := malgo.DefaultDeviceConfig(malgo.Duplex)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.Playback.Format = malgo.FormatS16
	deviceConfig.Playback.Channels = 1
	deviceConfig.SampleRate = 48000
	deviceConfig.Alsa.NoMMap = 1

	onData := func(pSampleOutput, pSampleCapture []byte, framecount uint32) {
		samplesReceived, err := bytesToInt16(pSampleCapture)
		if err != nil {
			fmt.Printf(err.Error())
		} else {
			inputBuffer.Write(samplesReceived)
		}

		samplesToSend, err := int16ToBytes(outputMixer.Read(int(framecount)))
		if err != nil {
			fmt.Printf(err.Error())
		} else {
			copy(pSampleOutput, samplesToSend)
		}
	}

	captureCallbacks := malgo.DeviceCallbacks{
		Data: onData,
	}

	device, err := malgo.InitDevice(ctx.Context, deviceConfig, captureCallbacks)
	if err != nil {
		return nil, err
	}

	return &MalgoClient{
		context: ctx, device: device, inputBuffer: inputBuffer, outputMixer: outputMixer,
	}, nil
}

func (ac *MalgoClient) Start() error {
	err := ac.device.Start()
	if err != nil {
		return err
	}

	return nil
}

func (ac *MalgoClient) Close() {
	if ac.device != nil {
		ac.device.Stop()
	}
}

func bytesToInt16(data []byte) ([]int16, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("byte slice length must be even")
	}

	out := make([]int16, len(data)/2)
	buf := bytes.NewReader(data)

	err := binary.Read(buf, binary.LittleEndian, &out)
	if err != nil {
		return nil, err
	}

	return out, nil
}
func int16ToBytes(data []int16) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, data)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
