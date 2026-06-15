# livekit-pa

Go application that bridges a local Linux soundcard to a LiveKit room, acting as a headless audio node for a public announcement (PA) system. The PC running this connects to amplifiers/speakers; browser clients join the same LiveKit room to talk and listen.

## Architecture

Two concurrent data streams:

**Capture (mic → LiveKit)**
```
ALSA/JACK → CircularBuffer (inputBuffer) → SampleProvider → Opus encoder → LiveKit WebRTC
```

**Playback (LiveKit → speaker)**
```
LiveKit WebRTC → Opus decoder → per-participant CircularBuffer → Mixer (RMS-weighted) → ALSA/JACK
```

All audio is fixed at **48 kHz, mono**. LiveKit frames are 960 samples (20 ms).

## Key Files

| File | Role |
|------|------|
| `main.go` | CLI flags, wires components together, blocks on SIGINT/SIGTERM |
| `audio/buffer.go` | Thread-safe circular ring buffer for int16 PCM samples |
| `audio/mixer.go` | RMS-weighted multi-source mixer; one buffer per remote participant |
| `audio/alsa.go` | ALSA capture/playback via cgo (`-lasound`); handles EPIPE re-prepare |
| `audio/jack.go` | Optional JACK driver (float32 ↔ int16 conversion, real-time callback) |
| `audio/jack_stub.go` | Compile-time stub when JACK is excluded |
| `livekit/client.go` | JWT auth, publishes local track, spawns goroutine per remote track |
| `livekit/sampleprovider.go` | `NextSample()` feeds 960-sample Opus frames to the LiveKit SDK |
| `util/retry.go` | Infinite retry loop with 2-second delay on error |

## Build

```bash
# Default (ALSA + JACK support)
go build

# ALSA only (no JACK dependency)
go build -tags nojack
```

Requires `libasound2-dev` for ALSA. JACK requires the JACK2 development libraries.

## Run

```bash
./livekit-pa \
  --driver alsa \
  --host wss://livekit.example.com \
  --api-key <key> \
  --api-secret <secret> \
  --room <room-name> \
  --identity <unique-name> \
  --buffer-size 4096
```

All flags except `--driver` and `--buffer-size` are required. The `--identity` must be unique per room participant.

## Design Decisions

- **RMS-weighted mixing**: Each remote participant's audio is weighted by its RMS energy before summing. Louder sources are not clipped out; quieter sources remain audible.
- **Overflow flush**: When `CircularBuffer` overflows, the entire buffer is flushed rather than dropping individual samples. Prevents stale audio build-up at the cost of a brief silence.
- **Per-participant buffers**: `Mixer` holds a `map[string]*CircularBuffer`. Buffers are added/removed dynamically as participants join or leave.
- **JACK build tag**: `//go:build !nojack` keeps JACK optional. `jack_stub.go` provides the same interface but returns an error immediately if compiled without JACK.
- **ALSA sample width**: The ALSA driver uses int32 frames from libasound; int16 samples occupy the top 16 bits (shift left on write, extract top bits on read).
- **Reconnect**: `util/retry.go` wraps `lk.Connect()` with 2-second retry. No exponential backoff yet.

## Audio Parameters

| Parameter | Value |
|-----------|-------|
| Sample rate | 48 000 Hz |
| Channels | 1 (mono) |
| Codec | Opus (VoIP mode) |
| Frame size | 960 samples = 20 ms |
| Default buffer | 4096 samples ≈ 85 ms |

## Dependencies

| Module | Purpose |
|--------|---------|
| `github.com/livekit/server-sdk-go/v2` | LiveKit WebRTC client SDK |
| `github.com/pion/webrtc/v4` | WebRTC transport (used by LiveKit SDK) |
| `github.com/livekit/protocol` | JWT auth and LiveKit messaging types |
| `github.com/xthexder/go-jack` | JACK audio server bindings |
| `gopkg.in/hraban/opus.v2` | Opus encoder/decoder |
