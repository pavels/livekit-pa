package audio

/*
#cgo LDFLAGS: -lasound
#include <alsa/asoundlib.h>
#include <stdlib.h>

int alsa_set_hw_params(snd_pcm_t *handle, snd_pcm_stream_t stream, unsigned int rate, snd_pcm_format_t format) {
	snd_pcm_hw_params_t *params;
	snd_pcm_hw_params_alloca(&params);
	if (snd_pcm_hw_params_any(handle, params) < 0) return -1;
	if (snd_pcm_hw_params_set_access(handle, params, SND_PCM_ACCESS_RW_INTERLEAVED) < 0) return -2;
	if (snd_pcm_hw_params_set_format(handle, params, format) < 0) return -3;
	if (snd_pcm_hw_params_set_channels(handle, params, 1) < 0) return -4;
	if (snd_pcm_hw_params_set_rate_near(handle, params, &rate, 0) < 0) return -5;
	if (snd_pcm_hw_params(handle, params) < 0) return -7;
	return 0;
}
*/
import "C"
import (
	"fmt"
	"log"
	"unsafe"
)

type pcm struct {
	handle *C.snd_pcm_t
	period uint64
}

func alsaError(code C.int) error {
	if code >= 0 {
		return nil
	}
	return fmt.Errorf("alsa error: %s", C.GoString(C.snd_strerror(code)))
}

func getCurrentPeriodSize(pcm *C.snd_pcm_t) (uint64, error) {
	var params *C.snd_pcm_hw_params_t

	if errCode := C.snd_pcm_hw_params_malloc(&params); errCode < 0 {
		return 0, alsaError(errCode)
	}
	defer C.snd_pcm_hw_params_free(params)

	if errCode := C.snd_pcm_hw_params_current(pcm, params); errCode < 0 {
		return 0, alsaError(errCode)
	}

	var period C.snd_pcm_uframes_t

	if errCode := C.snd_pcm_hw_params_get_period_size(params, &period, nil); errCode < 0 {
		return 0, alsaError(errCode)
	}

	return uint64(period), nil
}

func openPCM(device string, stream C.snd_pcm_stream_t) (*pcm, error) {
	cdev := C.CString(device)
	defer C.free(unsafe.Pointer(cdev))
	var handle *C.snd_pcm_t
	if errCode := C.snd_pcm_open(&handle, cdev, stream, 0); errCode < 0 {
		return nil, alsaError(errCode)
	}
	if errCode := C.alsa_set_hw_params(handle, stream, 48000, C.SND_PCM_FORMAT_S32_LE); errCode < 0 {
		C.snd_pcm_close(handle)
		return nil, alsaError(errCode)
	}

	return &pcm{handle: handle}, nil
}

func (p *pcm) preparePcm() error {
	errCode := C.snd_pcm_prepare(p.handle)

	return alsaError(C.int(errCode))
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
		return nil, fmt.Errorf("capture open: %w\n", err)
	}
	out, err := openPCM(device, C.SND_PCM_STREAM_PLAYBACK)
	if err != nil {
		in.close()
		return nil, fmt.Errorf("playback open: %w\n", err)
	}

	capturePeriod, err := getCurrentPeriodSize(in.handle)
	if err != nil {
		in.close()
		out.close()
		return nil, fmt.Errorf("capture period: %w\n", err)
	}
	in.period = capturePeriod

	playbackPeriod, err := getCurrentPeriodSize(out.handle)
	if err != nil {
		in.close()
		out.close()
		return nil, fmt.Errorf("playback period: %w\n", err)
	}
	out.period = playbackPeriod

	fmt.Printf("Capture device period: %d\n", in.period)
	fmt.Printf("Playback device period: %d\n", out.period)

	if in.period > inputBuffer.size/4 || out.period > inputBuffer.size/4 {
		return nil, fmt.Errorf("Internal buffer too small\n")
	}

	err = in.preparePcm()
	if err != nil {
		in.close()
		out.close()
		return nil, fmt.Errorf("capture prepare: %w\n", err)
	}

	err = out.preparePcm()
	if err != nil {
		in.close()
		out.close()
		return nil, fmt.Errorf("playback prepare: %w\n", err)
	}

	return &AlsaClient{
		pcmIn: in, pcmOut: out, quit: make(chan struct{}), inputBuffer: inputBuffer, outputMixer: outputMixer,
	}, nil
}

func (ac *AlsaClient) process() {
	inBuf := make([]int32, ac.pcmIn.period)
	outBuf := make([]int32, ac.pcmOut.period)
	for {
		select {
		case <-ac.quit:
			ac.pcmIn.close()
			return
		default:
		}

		ret := ac.pcmIn.readi(inBuf)
		if ret == -C.EPIPE {
			fmt.Printf("Input EPIPE.\n")
			err := ac.pcmIn.preparePcm()
			if err != nil {
				log.Fatalf(err.Error())
			}

			continue // Skip the rest
		} else if ret < 0 {
			log.Fatalf("Alsa read error: %s\n", alsaError(C.int(ret)).Error())
		} else {
			samples := make([]int16, ret)
			for i := 0; i < ret; i++ {
				samples[i] = int16(inBuf[i] >> 16)
			}
			ac.inputBuffer.Write(samples)
		}

		samples := ac.outputMixer.Read(ac.pcmOut.period)
		for i := range samples {
			outBuf[i] = int32(samples[i]) << 16
		}
		ret = ac.pcmOut.writei(outBuf)
		if ret == -C.EPIPE {
			fmt.Printf("Output EPIPE.\n")
			err := ac.pcmOut.preparePcm()
			if err != nil {
				log.Fatalf(err.Error())
			}
			ac.pcmOut.writei(outBuf)
		} else if ret < 0 {
			log.Fatalf("Alsa write error: %s\n", alsaError(C.int(ret)).Error())
		}
	}
}

func (ac *AlsaClient) Start() error {
	go ac.process()
	return nil
}

func (ac *AlsaClient) Close() {
	close(ac.quit)
}
