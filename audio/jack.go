//go:build !nojack
// +build !nojack

package audio

/*
#cgo LDFLAGS: -ljack
#include <jack/jack.h>
#include <stdlib.h>

extern int goJackProcess(jack_nframes_t nframes, void *arg);
extern void goJackShutdown(void *arg);

static jack_client_t* jackClientOpen(const char *name, jack_status_t *status) {
	return jack_client_open(name, JackNoStartServer, status);
}

static int jackSetCallbacks(jack_client_t *client, void *arg) {
	jack_on_shutdown(client, goJackShutdown, arg);
	return jack_set_process_callback(client, goJackProcess, arg);
}
*/
import "C"
import (
	"fmt"
	"log"
	"runtime/cgo"
	"unsafe"
)

type JackClient struct {
	client       *C.jack_client_t
	portIn       *C.jack_port_t
	portOut      *C.jack_port_t
	inputBuffer  *CircularBuffer
	outputMixer  *Mixer
	handle       cgo.Handle
	inputConvBuf []int16
	outputBuf    []int16
}

//export goJackProcess
func goJackProcess(nframes C.jack_nframes_t, arg unsafe.Pointer) C.int {
	jc := cgo.Handle(uintptr(arg)).Value().(*JackClient)

	n := uint32(nframes)
	inPtr := C.jack_port_get_buffer(jc.portIn, nframes)
	outPtr := C.jack_port_get_buffer(jc.portOut, nframes)

	inBuf := (*[1 << 20]C.jack_default_audio_sample_t)(inPtr)[:n:n]
	outBuf := (*[1 << 20]C.jack_default_audio_sample_t)(outPtr)[:n:n]

	inSamples := jc.inputConvBuf[:n]
	for i, s := range inBuf {
		inSamples[i] = audioSampleToInt16(float32(s))
	}
	jc.inputBuffer.Write(inSamples)

	outSamples := jc.outputBuf[:n]
	jc.outputMixer.ReadInto(outSamples)
	for i, s := range outSamples {
		outBuf[i] = C.jack_default_audio_sample_t(int16ToAudioSample(s))
	}

	return 0
}

//export goJackShutdown
func goJackShutdown(arg unsafe.Pointer) {
	log.Fatal("JACK server shut down")
}

func NewJackClient(name string, inputBuffer *CircularBuffer, outputMixer *Mixer) (*JackClient, error) {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))

	var status C.jack_status_t
	client := C.jackClientOpen(cname, &status)
	if client == nil {
		return nil, fmt.Errorf("jack_client_open failed: status=0x%x", uint(status))
	}

	if sr := C.jack_get_sample_rate(client); sr != 48000 {
		C.jack_client_close(client)
		return nil, fmt.Errorf("jack sample rate must be 48000Hz, got %d", uint(sr))
	}

	cIn := C.CString("in")
	defer C.free(unsafe.Pointer(cIn))
	cOut := C.CString("out")
	defer C.free(unsafe.Pointer(cOut))
	cType := C.CString("32 bit float mono audio")
	defer C.free(unsafe.Pointer(cType))

	portIn := C.jack_port_register(client, cIn, cType, C.JackPortIsInput, 0)
	portOut := C.jack_port_register(client, cOut, cType, C.JackPortIsOutput, 0)
	if portIn == nil || portOut == nil {
		C.jack_client_close(client)
		return nil, fmt.Errorf("jack_port_register failed")
	}

	bufSize := uint32(C.jack_get_buffer_size(client))
	jc := &JackClient{
		client:       client,
		portIn:       portIn,
		portOut:      portOut,
		inputBuffer:  inputBuffer,
		outputMixer:  outputMixer,
		inputConvBuf: make([]int16, bufSize),
		outputBuf:    make([]int16, bufSize),
	}
	jc.handle = cgo.NewHandle(jc)

	if ret := C.jackSetCallbacks(client, unsafe.Pointer(uintptr(jc.handle))); ret != 0 {
		jc.handle.Delete()
		C.jack_client_close(client)
		return nil, fmt.Errorf("jack_set_process_callback failed: %d", int(ret))
	}

	return jc, nil
}

func (jc *JackClient) Start() error {
	if ret := C.jack_activate(jc.client); ret != 0 {
		return fmt.Errorf("jack_activate failed: %d", int(ret))
	}
	return nil
}

func (jc *JackClient) Close() {
	C.jack_deactivate(jc.client)
	C.jack_client_close(jc.client)
	jc.handle.Delete()
}

func audioSampleToInt16(f float32) int16 {
	v := f * 32767.0
	if v > 32767 {
		v = 32767
	} else if v < -32768 {
		v = -32768
	}
	return int16(v)
}

func int16ToAudioSample(i int16) float32 {
	return float32(i) / 32767.0
}
