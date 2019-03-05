package oswstest

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// LoginClients logs in  a slice of clients. Uses `ParallelLogins` connectWorker
// to work `ParallelLogins` clients in parallel. Expects the clients to be
// AuthClients. Blocks until all clients are logged in.
func LoginClients(clients []Client) {
	// Block the function until all clients are logged in
	var wg sync.WaitGroup
	wg.Add(len(clients))
	defer wg.Wait()

	// Start workers. The toWorker channel is used to send the clients to the workers
	toWorker := make(chan Client)
	defer close(toWorker)
	for i := 0; i < ParallelLogins; i++ {
		go func() {
			for client := range toWorker {
				if err := client.(AuthClient).Login(); err != nil {
					log.Fatalf("Can not login client %s: %s", client, err)
				}
				wg.Done()
			}
		}()
	}

	// Send clients to workers
	for _, client := range clients {
		toWorker <- client
	}
}

// ConnectClients connects a slice of clients via websocket to the server. Uses
// `ParallelConnections` connectWorker to work `ParallelConnections` clients in
// parallel. `errChan` sends a error, it it happens on a client. `connected`
// sends the time how long a connection took. Sends a signal on the done
// channel, when all clients are done.
func ConnectClients(clients []Client, errChan chan<- error, connected chan<- time.Duration, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	// Block the function until all clients are connected.
	var wg sync.WaitGroup
	wg.Add(len(clients))
	defer wg.Wait()

	// Start workers. The `toWorker` channel is used to send the clients to the workers.
	toWorker := make(chan Client)
	defer close(toWorker)
	for i := 0; i < ParallelConnections; i++ {
		go func() {
			for client := range toWorker {
				start := time.Now()
				if err := client.Connect(); err != nil {
					errChan <- fmt.Errorf("can not connect client %s: %s", client, err)
				} else {
					connected <- time.Since(start)
				}
				wg.Done()
			}
		}()
	}

	// Send clients to workers
	for _, client := range clients {
		toWorker <- client
	}
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
func ListenToClients(clients []Client, data chan<- time.Duration, err chan<- error, count int, done chan<- struct{}) {
	defer func() { done <- struct{}{} }()

	// Block until all clients are done
	var wg sync.WaitGroup
	wg.Add(len(clients))
	defer wg.Wait()

	for _, client := range clients {
		go func(client Client) {
			client.ExpectData(data, err, count)
			wg.Done()
		}(client)
	}
}
