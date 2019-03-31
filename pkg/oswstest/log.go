package oswstest

import (
	"log"
	"time"
)

// StartLogger starts a logger, that prints some progress informations each seconds.
// Returns a cancel function to stop the logging.
func StartLogger(clients []*Client) func() {
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
			var errors int
			for _, c := range clients {
				if !c.Connected().IsZero() {
					connected++
				}
				if c.wsError != nil {
					errors++
				}
				received += c.MessageCount()
			}
			log.Printf("connected: %d, received: %d, errors: %d", connected, received, errors)
		}
	}()
	return func() {
		close(done)
	}
}
