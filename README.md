# livekit-pa

`livekit-pa` is a small Go application that bridges [LiveKit](https://livekit.io) audio calls with your local Linux audio system using either miniaudio or JACK.

## Features

- Captures microphone audio and transmits it to a LiveKit room.
- Receives audio from all participants, mixes it, and plays it locally.
- RMS-based software mixer to maintain consistent output volume.
- JACK and miniaudio support.
- Robust reconnection logic.

## Installation

### 1. Clone the repository

```bash
git clone https://github.com/pavels/livekit-pa.git
cd livekit-pa
```

### 2a. Build

```bash
go build -o livekit-pa
```

### 2b. Build excluding JACK support

```bash
go build -tags nojack -o livekit-pa
```

## Usage

```bash
./livekit-pa \
  --driver=miniaudio \
  --host=wss://livekit.example.com \
  --api-key=your-api-key \
  --api-secret=your-api-secret \
  --room=main \
  --identity=pa-amp
```

> JACK requires a running JACK server with 48kHz sampling rate.

### Flags

| Flag           | Description                          |
|----------------|--------------------------------------|
| `--driver`     | Audio backend: `miniaudio` or `jack` |
| `--host`       | LiveKit server URL                   |
| `--api-key`    | LiveKit API key                      |
| `--api-secret` | LiveKit API secret                   |
| `--room`       | Room to join                         |
| `--identity`   | Unique participant identity          |

## Project Structure

- `main.go` – CLI and startup
- `audio/` – miniaudio and JACK audio clients
- `livekit/` – LiveKit connection
- `util/` – Retry and shutdown helpers
