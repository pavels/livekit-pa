package livekit

import (
	"context"
	"livekit-pa/audio"
	"log"
	"time"

	"gopkg.in/hraban/opus.v2"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4/pkg/media"
)

type BufferedSampleProvider struct {
	buffer  *audio.CircularBuffer
	encoder *opus.Encoder
}

func NewBufferedSampleProvider(buffer *audio.CircularBuffer) *BufferedSampleProvider {
	encoder, err := opus.NewEncoder(48000, 1, opus.AppVoIP)
	if err != nil {
		log.Fatal(err)
	}

	return &BufferedSampleProvider{
		buffer:  buffer,
		encoder: encoder,
	}
}

func (b *BufferedSampleProvider) Close() error {
	return nil
}

func (b *BufferedSampleProvider) NextSample(_ context.Context) (media.Sample, error) {
	const frameSize = 960
	pcm := b.buffer.Read(frameSize)
	encoded := make([]byte, 4000)
	n, err := b.encoder.Encode(pcm, encoded)
	if err != nil {
		return media.Sample{}, err
	}
	return media.Sample{Data: encoded[:n], Duration: 20 * time.Millisecond}, nil
}

func (b *BufferedSampleProvider) OnBind() error {
	b.buffer.Flush()
	return nil
}

func (b *BufferedSampleProvider) OnUnbind() error {
	return nil
}

var _ lksdk.SampleProvider = (*BufferedSampleProvider)(nil)
