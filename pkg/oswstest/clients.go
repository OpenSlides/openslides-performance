package oswstest

import (
	"fmt"
	"sync"
	"time"
)

func clientsAction(clients []Client, duration chan<- time.Duration, errC chan<- error, parallel int, f func(Client) error) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		// Close done when all clients are done
		defer close(done)

		// Block the function until all clients are done
		var wg sync.WaitGroup
		wg.Add(len(clients))
		defer wg.Wait()

		// Start workers. The toWorker channel is used to send the clients to the workers
		toWorker := make(chan Client)
		defer close(toWorker)
		for i := 0; i < parallel; i++ {
			go func() {
				for client := range toWorker {
					start := time.Now()
					if err := f(client); err != nil {
						if errC != nil {
							errC <- err
						}
						wg.Done()
						return
					}
					if duration != nil {
						duration <- time.Since(start)
					}
					wg.Done()
				}
			}()
		}

		// Send clients to workers
		for _, client := range clients {
			toWorker <- client
		}
	}()
	return done
}

// LoginClients logs in  a slice of clients. Uses `ParallelLogins` connectWorker
// to work `ParallelLogins` clients in parallel. Blocks until all clients are logged in.
func LoginClients(clients []Client, duration chan<- time.Duration, errC chan<- error) <-chan struct{} {
	return clientsAction(clients, duration, errC, ParallelLogins, func(client Client) error {
		return client.(AuthClient).Login()
	})
}

// ConnectClients connects a slice of clients via websocket to the server. Uses
// `ParallelConnections` connectWorker to work `ParallelConnections` clients in
// parallel. `errChan` sends a error, it it happens on a client. `connected`
// sends the time how long a connection took. Sends a signal on the done
// channel, when all clients are done.
func ConnectClients(clients []Client, duration chan<- time.Duration, errC chan<- error) <-chan struct{} {
	return clientsAction(clients, duration, errC, ParallelConnections, func(client Client) error {
		return client.Connect()
	})
}

// SendClients sends the write request for a slice of AdminClients. Sends
// `ParallelSends` requests in parallel. `errChan` sends an error for each
// client, when the send request failed. `sended` sends the time it took to send
// the request. Sends a signal on the `done` channel when all clients send the
// request.
func SendClients(clients []AdminClient, errChan chan<- error, sended chan<- time.Duration, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	// Block until all clients send the request
	var wg sync.WaitGroup
	wg.Add(ParallelSends)
	defer wg.Wait()

	// Start workers. The `toWorker` channel is used to send the clients to the worker.
	toWorker := make(chan AdminClient)
	defer close(toWorker)
	for i := 0; i < ParallelSends; i++ {
		go func() {
			for client := range toWorker {
				start := time.Now()
				if err := client.Send(); err != nil {
					errChan <- fmt.Errorf("client %s can not send the request: %s", client, err)
				} else {
					sended <- time.Since(start)
				}
			}
			wg.Done()
		}()
	}

	// Send clients to workers
	for _, client := range clients {
		toWorker <- client
	}
}

// ListenToClients listens to a slice of clients. Sends the results
// via the given channels. One for the data (duration since connected) and one for errors.
// Ends the process, when each client got `count` messages or one errors. When this happens,
// send a signal on the done channel.
func ListenToClients(clients []Client, data chan<- time.Duration, errC chan<- error, count int, sinceStart bool, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	// Block until all clients are done
	var wg sync.WaitGroup
	wg.Add(len(clients))
	defer wg.Wait()

	for _, client := range clients {
		go func(client Client) {
			defer wg.Done()
			start := time.Now()
			if err := client.ExpectData(count, sinceStart); err != nil {
				errC <- err
				return
			}
			if start.Before(client.Connected()) {
				start = client.Connected()
			}
			data <- time.Since(start)
		}(client)
	}
}
