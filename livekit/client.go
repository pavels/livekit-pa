package livekit

import (
	"fmt"
	"log"

	"github.com/livekit/protocol/auth"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"

	"livekit-pa/audio"

	"gopkg.in/hraban/opus.v2"
)

type LiveKitClient struct {
	host      string
	apiKey    string
	apiSecret string
	roomName  string
	identity  string

	inputBuffer *audio.CircularBuffer
	outputMixer *audio.Mixer

	room       *lksdk.Room
	localTrack *lksdk.LocalTrack
}

func NewLiveKitClient(host, apiKey, apiSecret, roomName, identity string, buffer *audio.CircularBuffer, mixer *audio.Mixer) *LiveKitClient {
	return &LiveKitClient{
		host:        host,
		apiKey:      apiKey,
		apiSecret:   apiSecret,
		roomName:    roomName,
		identity:    identity,
		inputBuffer: buffer,
		outputMixer: mixer,
	}
}

func (lk *LiveKitClient) Connect() error {
	done := make(chan struct{})
	provider := NewBufferedSampleProvider(lk.inputBuffer)
	localTrack, err := lksdk.NewLocalSampleTrack(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus})
	if err != nil {
		return err
	}
	lk.localTrack = localTrack

	if err := localTrack.StartWrite(provider, func() {}); err != nil {
		return err
	}

	room := lksdk.NewRoom(&lksdk.RoomCallback{
		ParticipantCallback: lksdk.ParticipantCallback{
			OnTrackSubscribed: func(track *webrtc.TrackRemote, pub *lksdk.RemoteTrackPublication, rp *lksdk.RemoteParticipant) {
				if track.Codec().MimeType == webrtc.MimeTypeOpus {
					go lk.handleRemoteTrack(track, rp.Identity())
				}
			},
		},
		OnDisconnected: func() {
			fmt.Println("Disconnected from LiveKit")
			close(done)
		},
	})
	lk.room = room

	token, err := newAccessToken(lk.apiKey, lk.apiSecret, lk.roomName, lk.identity)
	if err != nil {
		return err
	}

	if err := room.JoinWithToken(lk.host, token); err != nil {
		return err
	}

	_, err = room.LocalParticipant.PublishTrack(localTrack, &lksdk.TrackPublicationOptions{Name: "mic"})
	if err != nil {
		return err
	}

	<-done
	return fmt.Errorf("LiveKit disconnected")
}

func (lk *LiveKitClient) Disconnect() {
	if lk.room != nil {
		lk.room.Disconnect()
	}
}

func (lk *LiveKitClient) handleRemoteTrack(track *webrtc.TrackRemote, participantID string) {
	buf := audio.NewCircularBuffer()
	decoder, err := opus.NewDecoder(48000, 1)
	if err != nil {
		log.Fatal(err)
	}

	lk.outputMixer.AddBuffer(participantID, buf)
	defer lk.outputMixer.RemoveBuffer(participantID)

	log.Println("Track Started -", track.ID())
	defer log.Println("Track Ended -", track.ID())

	for {
		pkt, _, err := track.ReadRTP()
		if err != nil {
			break
		}
		pcm := make([]int16, 4000)
		n, err := decoder.Decode(pkt.Payload, pcm)
		if err != nil {
			continue
		}
		buf.Write(pcm[:n])
	}
}

func newAccessToken(apiKey, apiSecret, roomName, identity string) (string, error) {
	at := auth.NewAccessToken(apiKey, apiSecret)
	grant := &auth.VideoGrant{RoomJoin: true, Room: roomName}
	at.SetVideoGrant(grant).
		SetIdentity(identity).
		SetName(identity)
	return at.ToJWT()
}
