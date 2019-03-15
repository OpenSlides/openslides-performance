package oswstest

import (
	"sync"
	"time"
)

// parallelWorker runs a function on a slice of work in `parallel` workers.
// Sends the time how long on each work tock on the `duration` channel. If
// an error happens, sends it on the `errC` channel.
// `f()`` is a function that have to accept one work.
// if `duration` or `errC` are nil, the values are ignored but the work is still done.
func parallelWorker(tasks []interface{}, duration chan<- time.Duration, errC chan<- error, parallel int, f func(interface{}) error) {
	// Block the function until all tasks is done
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	defer wg.Wait()

	// Start workers. The toWorker channel is used to send the tasks to the workers
	toWorker := make(chan interface{})
	defer close(toWorker)
	for i := 0; i < parallel; i++ {
		go func() {
			for task := range toWorker {
				start := time.Now()
				if err := f(task); err != nil {
					if errC != nil {
						errC <- err
					}
					wg.Done()
					continue
				}
				if duration != nil {
					duration <- time.Since(start)
				}
				wg.Done()
			}
		}()
	}

	// Send tasks to workers
	for _, task := range tasks {
		toWorker <- task
	}
}

// LoginClients logs in a slice of clients. Uses `ParallelLogins` nworkers
// to login clients in parallel.
// Returns the time how long each login took on the duration channel and any
// error on the errC channel.
func LoginClients(clients []Loginer, duration chan<- time.Duration, errC chan<- error) {
	tasks := make([]interface{}, 0, len(clients))
	for _, task := range clients {
		tasks = append(tasks, task)
	}

	parallelWorker(tasks, duration, errC, ParallelLogins, func(task interface{}) error {
		return task.(Loginer).Login()
	})
}

// ConnectClients connects a slice of clients via websocket to the server. Uses
// `ParallelConnections` workers to connect clients in parallel.
// Returns the time how long each login took on the duration channel and any
// error on the errC channel.
func ConnectClients(clients []Connecter, duration chan<- time.Duration, errC chan<- error) {
	tasks := make([]interface{}, 0, len(clients))
	for _, task := range clients {
		tasks = append(tasks, task)
	}

	parallelWorker(tasks, duration, errC, ParallelLogins, func(task interface{}) error {
		return task.(Connecter).Connect()
	})
}

// SendClients sends the write request for a slice of clients. Sends
// `ParallelSends` requests in parallel. `errChan` sends an error for each
// client, when the send request failed. `sended` sends the time it took to send
// the request. Sends a signal on the `done` channel when all clients send the
// request.
func SendClients(clients []Sender, duration chan<- time.Duration, errC chan<- error) {
	tasks := make([]interface{}, 0, len(clients))
	for _, task := range clients {
		tasks = append(tasks, task)
	}

	parallelWorker(tasks, duration, errC, ParallelSends, func(task interface{}) error {
		return task.(Sender).Send()
	})
}

// ListenToClients listens to a slice of clients. Sends the results
// via the given channels. One for the data (duration since connected) and one for errors.
// Ends the process, when each client got `count` messages or one errors.
func ListenToClients(clients []Listener, duration chan<- time.Duration, errC chan<- error, count int, sinceStart bool) {
	// Block until all clients are done
	var wg sync.WaitGroup
	wg.Add(len(clients))
	defer wg.Wait()

	for _, client := range clients {
		go func(client Listener) {
			defer wg.Done()
			start := time.Now()
			if err := client.ExpectData(count, sinceStart); err != nil {
				errC <- err
				return
			}
			if start.Before(client.Connected()) {
				start = client.Connected()
			}
			duration <- time.Since(start)
		}(client)
	}
}
