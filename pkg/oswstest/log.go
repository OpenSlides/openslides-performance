package oswstest

import (
	"log"
	"time"
)

func startLogger(clients []*Client) (cancel func()) {
	done := make(chan struct{})
	cancel = func() {
		close(done)
	}

	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
			case <-done:
				return
			}
			connected := 0
			received := 0
			errors := 0
			for _, c := range clients {
				if !c.Connected().IsZero() {
					connected++
				}
				if c.wsError != nil {
					errors++
				}
				received += c.messageCount
			}
			log.Printf("connected: %d, received: %d, errors: %d", connected, received, errors)
		}
	}()
	return
}
