package logger

import (
	"log"
	"time"

	"github.com/openslides/openslides-performance/pkg/client"
)

// StartLogger starts a logger, that prints some progress informations each seconds.
// Returns a cancel function to stop the logging.
func StartLogger(clients []*client.Client) func() {
	done := make(chan struct{})

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
			case <-done:
				return
			}
			var connected int
			var received int
			for _, c := range clients {
				if !c.Connected().IsZero() {
					connected++
				}
				received += c.MessageCount()
			}
			log.Printf("connected: %d, received: %d", connected, received)
		}
	}()
	return func() {
		close(done)
	}
}
