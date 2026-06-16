package audio

/*
#cgo LDFLAGS: -lasound
#include <alsa/asoundlib.h>
#include <stdlib.h>

int alsa_set_hw_params(snd_pcm_t *handle, snd_pcm_stream_t stream, unsigned int rate, snd_pcm_format_t format) {
	snd_pcm_hw_params_t *params;
	snd_pcm_hw_params_alloca(&params);
	int err;
	if ((err = snd_pcm_hw_params_any(handle, params)) < 0) return err;
	if ((err = snd_pcm_hw_params_set_access(handle, params, SND_PCM_ACCESS_RW_INTERLEAVED)) < 0) return err;
	if ((err = snd_pcm_hw_params_set_format(handle, params, format)) < 0) return err;
	if ((err = snd_pcm_hw_params_set_channels(handle, params, 1)) < 0) return err;
	if ((err = snd_pcm_hw_params_set_rate_near(handle, params, &rate, 0)) < 0) return err;
	if ((err = snd_pcm_hw_params(handle, params)) < 0) return err;
	return 0;
}
*/
import "C"
import (
	"fmt"
	"log"
	"time"
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
	return alsaError(C.int(C.snd_pcm_prepare(p.handle)))
}

// resumePcm recovers from ESTRPIPE (PCM suspended by power management).
// It retries snd_pcm_resume until the hardware is ready, then falls back
// to prepare if the hardware doesn't support resume.
func (p *pcm) resumePcm() error {
	for {
		ret := C.int(C.snd_pcm_resume(p.handle))
		if ret == 0 {
			return nil
		}
		if ret != -C.EAGAIN {
			return p.preparePcm()
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (p *pcm) readi(buf []int32) (bool, error) {
	ret := C.snd_pcm_readi(p.handle, unsafe.Pointer(&buf[0]), C.snd_pcm_uframes_t(len(buf)))
	if ret == -C.EPIPE {
		log.Printf("Input overrun (EPIPE), recovering\n")
		if err := p.preparePcm(); err != nil {
			return false, err
		}
		if err := alsaError(C.snd_pcm_start(p.handle)); err != nil {
			return false, err
		}
		return false, nil
	}
	if ret == -C.ESTRPIPE {
		log.Printf("Input suspended (ESTRPIPE), recovering\n")
		if err := p.resumePcm(); err != nil {
			return false, err
		}
		return false, nil
	}
	if ret < 0 {
		return false, alsaError(C.int(ret))
	}
	return true, nil
}

func (p *pcm) writei(buf []int32) (bool, error) {
	offset := int64(0)
	for offset < int64(len(buf)) {
		wbuf := buf[offset:]
		n := C.snd_pcm_writei(p.handle, unsafe.Pointer(&wbuf[0]), C.snd_pcm_uframes_t(len(wbuf)))
		if n == -C.EPIPE {
			log.Printf("Output underrun (EPIPE), recovering\n")
			if err := p.preparePcm(); err != nil {
				return false, err
			}
			continue
		}
		if n == -C.ESTRPIPE {
			log.Printf("Output suspended (ESTRPIPE), recovering\n")
			if err := p.resumePcm(); err != nil {
				return false, err
			}
			continue
		}
		if n < 0 {
			return false, alsaError(C.int(n))
		}
		offset += int64(n)
	}
	return true, nil
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

	if errCode := C.snd_pcm_link(in.handle, out.handle); errCode < 0 {
		in.close()
		out.close()
		return nil, fmt.Errorf("pcm link: %w", alsaError(errCode))
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
	inSamples := make([]int16, ac.pcmIn.period)
	outSamples := make([]int16, ac.pcmOut.period)
	outBuf := make([]int32, ac.pcmOut.period)

	for {
		select {
		case <-ac.quit:
			ac.pcmIn.close()
			return
		default:
		}

		res, err := ac.pcmIn.readi(inBuf)
		if err != nil {
			log.Fatalf("Alsa read error: %s\n", err.Error())
		}
		if !res {
			continue
		}

		ac.outputMixer.ReadInto(outSamples)
		for i := range outSamples {
			outBuf[i] = int32(outSamples[i]) << 16
		}

		_, err = ac.pcmOut.writei(outBuf)
		if err != nil {
			log.Fatalf("Alsa write error: %s\n", err.Error())
		}

		for i := 0; i < len(inBuf); i++ {
			inSamples[i] = int16(inBuf[i] >> 16)
		}
		ac.inputBuffer.Write(inSamples)
	}
}

func (ac *AlsaClient) Start() error {
	go ac.process()
	return nil
}

func (ac *AlsaClient) Close() {
	close(ac.quit)
}
