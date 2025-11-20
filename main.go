package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"livekit-pa/audio"
	"livekit-pa/livekit"
	"livekit-pa/util"
)

func main() {
	driver := flag.String("driver", "miniaudio", "audio driver: jack or miniaudio")
	host := flag.String("host", "", "LiveKit server URL")
	apiKey := flag.String("api-key", "", "LiveKit API key")
	apiSecret := flag.String("api-secret", "", "LiveKit API secret")
	roomName := flag.String("room", "", "Room name")
	identity := flag.String("identity", "", "Participant identity")
	bufferSize := flag.Int("buffer-size", 960*8, "Buffer size")
	flag.Parse()

	if *host == "" || *apiKey == "" || *apiSecret == "" || *roomName == "" || *identity == "" {
		log.Fatal("All LiveKit parameters are required")
	}

	inputBuffer := audio.NewCircularBuffer(*bufferSize)
	outputMixer := audio.NewMixer(*bufferSize)

	var err error
	var client interface {
		Start() error
		Close()
	}

	switch *driver {
	case "jack":
		client, err = audio.NewJackClient("livekit-pa-"+*identity, inputBuffer, outputMixer)
	case "miniaudio":
		client, err = audio.NewMalgoClient(inputBuffer, outputMixer)
	default:
		log.Fatalf("Unknown driver: %s", *driver)
	}

	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	lk := livekit.NewLiveKitClient(*host, *apiKey, *apiSecret, *roomName, *identity, inputBuffer, outputMixer)
	go util.WithReconnect(func() error {
		return lk.Connect()
	})

	if err := client.Start(); err != nil {
		log.Fatal(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	fmt.Println("Shutting down")
}
