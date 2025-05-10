package audio

/*
#cgo LDFLAGS: -lasound
#include <alsa/asoundlib.h>
#include <stdlib.h>

int alsa_set_hw_params(snd_pcm_t *handle, snd_pcm_stream_t stream, unsigned int rate, snd_pcm_format_t format, snd_pcm_uframes_t frames) {
	snd_pcm_hw_params_t *params;
	snd_pcm_hw_params_alloca(&params);
	if (snd_pcm_hw_params_any(handle, params) < 0) return -1;
	if (snd_pcm_hw_params_set_access(handle, params, SND_PCM_ACCESS_RW_INTERLEAVED) < 0) return -2;
	if (snd_pcm_hw_params_set_format(handle, params, format) < 0) return -3;
	if (snd_pcm_hw_params_set_channels(handle, params, 1) < 0) return -4;
	if (snd_pcm_hw_params_set_rate_near(handle, params, &rate, 0) < 0) return -5;
	if (snd_pcm_hw_params_set_period_size_near(handle, params, &frames, 0) < 0) return -6;
	if (snd_pcm_hw_params(handle, params) < 0) return -7;
	return 0;
}
*/
import "C"
import (
	"fmt"
	"time"
	"unsafe"
)

type pcm struct {
	handle *C.snd_pcm_t
}

func openPCM(device string, stream C.snd_pcm_stream_t) (*pcm, error) {
	cdev := C.CString(device)
	defer C.free(unsafe.Pointer(cdev))
	var handle *C.snd_pcm_t
	if err := C.snd_pcm_open(&handle, cdev, stream, 0); err < 0 {
		return nil, fmt.Errorf("open failed: %d", err)
	}
	if err := C.alsa_set_hw_params(handle, stream, 48000, C.SND_PCM_FORMAT_S32_LE, 960); err < 0 {
		C.snd_pcm_close(handle)
		return nil, fmt.Errorf("hw params failed: %d", err)
	}
	return &pcm{handle: handle}, nil
}

func (p *pcm) readi(buf []int32) int {
	n := C.snd_pcm_readi(p.handle, unsafe.Pointer(&buf[0]), C.snd_pcm_uframes_t(len(buf)))

	return int(n)
}

func (p *pcm) writei(buf []int32) int {
	n := C.snd_pcm_writei(p.handle, unsafe.Pointer(&buf[0]), C.snd_pcm_uframes_t(len(buf)))

	return int(n)
}

func (p *pcm) close() {
	C.snd_pcm_drain(p.handle)
	C.snd_pcm_close(p.handle)
}

type AlsaClient struct {
	pcmIn       *pcm
	pcmOut      *pcm
	quit        chan struct{}
	inputBuffer *CircularBuffer
	outputMixer *Mixer
}

func NewAlsaClient(device string, inputBuffer *CircularBuffer, outputMixer *Mixer) (*AlsaClient, error) {
	in, err := openPCM(device, C.SND_PCM_STREAM_CAPTURE)
	if err != nil {
		return nil, fmt.Errorf("capture open: %w", err)
	}
	out, err := openPCM(device, C.SND_PCM_STREAM_PLAYBACK)
	if err != nil {
		in.close()
		return nil, fmt.Errorf("playback open: %w", err)
	}
	return &AlsaClient{
		pcmIn: in, pcmOut: out, quit: make(chan struct{}), inputBuffer: inputBuffer, outputMixer: outputMixer,
	}, nil
}

func (ac *AlsaClient) run() {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	inBuf := make([]int32, 960)
	outBuf := make([]int32, 960)
	for {
		select {
		case <-ac.quit:
			ac.pcmIn.close()
			ac.pcmOut.close()
			return
		case <-ticker.C:
			if n := ac.pcmIn.readi(inBuf); n > 0 {
				samples := make([]int16, n)
				for i := 0; i < n; i++ {
					samples[i] = int16(inBuf[i] >> 16)
				}
				ac.inputBuffer.Write(samples)
			}
			samples := ac.outputMixer.Read(960)
			for i := range samples {
				outBuf[i] = int32(samples[i]) << 16
			}
			ac.pcmOut.writei(outBuf)
		}
	}
}

func (ac *AlsaClient) Start() int {
	go ac.run()
	return 0
}

func (ac *AlsaClient) Close() {
	close(ac.quit)
}
