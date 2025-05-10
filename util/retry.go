package util

import (
	"fmt"
	"time"
)

func WithReconnect(fn func() error) {
	for {
		err := fn()
		if err != nil {
			fmt.Println("Reconnect error:", err)
			time.Sleep(2 * time.Second)
		} else {
			fmt.Println("Exited reconnect loop unexpectedly")
			return
		}
	}
}
