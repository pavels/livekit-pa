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
	driver := flag.String("driver", "alsa", "audio driver: jack or alsa")
	host := flag.String("host", "", "LiveKit server URL")
	apiKey := flag.String("api-key", "", "LiveKit API key")
	apiSecret := flag.String("api-secret", "", "LiveKit API secret")
	roomName := flag.String("room", "", "Room name")
	identity := flag.String("identity", "", "Participant identity")
	flag.Parse()

	if *host == "" || *apiKey == "" || *apiSecret == "" || *roomName == "" || *identity == "" {
		log.Fatal("All LiveKit parameters are required")
	}

	inputBuffer := audio.NewCircularBuffer(960 * 50)
	outputMixer := audio.NewMixer()

	var client interface {
		Start() int
		Close()
	}

	var err error

	switch *driver {
	case "jack":
		client, err = audio.NewJackClient("livekit-pa-"+*identity, inputBuffer, outputMixer)
	case "alsa":
		client, err = audio.NewAlsaClient("default", inputBuffer, outputMixer)
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

	if code := client.Start(); code != 0 {
		log.Fatalf("Failed to activate audio client: %d", code)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	fmt.Println("Shutting down")
}
