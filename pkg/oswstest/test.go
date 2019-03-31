package oswstest

import (
	"log"
	"strings"
	"time"
)

// RunTests runs some tests for a slice of clients. It returns the result of the all
// called test as string message.
// The tests are canceld, when the cancel channel is closed.
func RunTests(clients []*Client, tests []Tester, cancel <-chan struct{}) (r string) {
	rs := make([]string, 0)
	defer func() { r = strings.Join(rs, "\n") }()

	for _, test := range tests {
		rs = append(rs, test.Test(clients, cancel))

		select {
		case <-cancel:
			return
		default:
		}
	}
	return
}

// Tester can be tested with a slice of clients.
type Tester interface {
	Test([]*Client, <-chan struct{}) string
}

// ConnectTest opens connections for any given client. It measures the time
// until the connection was open andthe time until the first data was received.
// Expects, that the wsconnection of the clients are closed.
type ConnectTest struct {
	ShowAllErrors       bool
	ParallelConnections int
}

// Test runs the test for the ConnectTest
func (t *ConnectTest) Test(clients []*Client, cancel <-chan struct{}) (r string) {
	log.Println("Start ConnectTest")
	startTest := time.Now()
	defer func() { log.Printf("ConnectionTest took %dms", time.Since(startTest)/time.Millisecond) }()

	// Connect all Clients
	connected := make(chan time.Duration, 10)
	connectedError := make(chan error, 10)
	connectionDone := make(chan struct{})
	go func() {
		defer close(connectionDone)

		// Convert slice of clients to slice of Connectors
		connecters := make([]Connecter, 0, len(clients))
		for _, client := range clients {
			connecters = append(connecters, client)
		}
		ConnectClients(connecters, t.ParallelConnections, connected, connectedError)
	}()

	// Listen to all clients to receive the response.
	dataReceived := make(chan time.Duration)
	errorReceived := make(chan error)
	receivedDone := make(chan struct{})
	go func() {
		defer close(receivedDone)

		// Convert slice of clients to slice of Listeners
		listeners := make([]Listener, 0, len(clients))
		for _, client := range clients {
			listeners = append(listeners, client)
		}
		ListenToClients(listeners, dataReceived, errorReceived, 1, true)
	}()

	connectedResult := testResult{description: "Time to established connection", showAllErrors: t.ShowAllErrors}
	dataReceivedResult := testResult{description: "Time until data has been reveiced since the connection", showAllErrors: t.ShowAllErrors}
	defer func() { r = connectedResult.String() + "\n" + dataReceivedResult.String() }()

	for {
		select {
		case value := <-connected:
			connectedResult.add(value)

		case value := <-connectedError:
			connectedResult.addError(value)

		case value := <-dataReceived:
			dataReceivedResult.add(value)

		case value := <-errorReceived:
			dataReceivedResult.addError(value)

		case <-cancel:
			return

		case <-connectionDone:
			connectionDone = nil

		case <-receivedDone:
			receivedDone = nil
		}

		if connectionDone == nil && receivedDone == nil {
			return
		}
	}
}

// OneWriteTest tests, that all clients get a response when there is one write
// request. Expects, that the first client is a logged-in admin client and that
// all clients have open websocket connections.
type OneWriteTest struct {
	ShowAllErrors bool
}

// Test runs the OneWriteTest
func (t *OneWriteTest) Test(clients []*Client, cancel <-chan struct{}) (r string) {
	log.Println("Start OneWriteTest")
	startTest := time.Now()
	defer func() { log.Printf("OneWriteTest took %dms\n", time.Since(startTest)/time.Millisecond) }()

	// Find the admin client.
	admin := clients[0]
	if !admin.IsAdmin() || (admin.Connected() == time.Time{}) {
		log.Fatalf("Fatal: Expect the first client in OneWriteTest to be a connected AdminClient")
	}

	// Listen to all clients to receive the response.
	dataReceived := make(chan time.Duration)
	errorReceived := make(chan error)
	listenToClientsDone := make(chan struct{})
	go func() {
		defer close(listenToClientsDone)

		// Convert slice of clients to slice of Listeners
		listeners := make([]Listener, 0, len(clients))
		for _, client := range clients {
			listeners = append(listeners, client)
		}
		ListenToClients(listeners, dataReceived, errorReceived, 1, false)
	}()

	// Send the request.
	if err := admin.Send(); err != nil {
		log.Fatalf("Can not send request, %s", err)
	}

	dataReceivedResult := testResult{description: "Time until data is received after one write request", showAllErrors: t.ShowAllErrors}
	defer func() { r = dataReceivedResult.String() }()

	for {
		select {
		case value := <-dataReceived:
			dataReceivedResult.add(value)

		case value := <-errorReceived:
			dataReceivedResult.addError(value)

		case <-cancel:
			return

		case <-listenToClientsDone:
			return
		}
	}
}

// ManyWriteTest tests behave like the OneWriteTest but send one write request
// per admin client. Expects, that at least one client is a logged-in admin
// client and that all clients have open websocket connections.
type ManyWriteTest struct {
	ShowAllErrors bool
	ParallelSends int
}

// Test runs the ManyWriteTest
func (t *ManyWriteTest) Test(clients []*Client, cancel <-chan struct{}) (r string) {
	log.Println("Start ManyWriteTest")
	startTest := time.Now()
	defer func() { log.Printf("ManyWriteTest took %dms\n", time.Since(startTest)/time.Millisecond) }()

	// Find all connected admins in clients
	var admins []Sender
	for _, client := range clients {
		if client.IsAdmin() && !client.Connected().IsZero() {
			admins = append(admins, client)
		}
	}
	if len(admins) == 0 {
		log.Fatalf("Fatal: Expect one client in ManyWriteTest to be a connected AdminClient")
	}

	// Send requests for all admin clients
	dataSended := make(chan time.Duration)
	errorSended := make(chan error)
	sendClientsDone := make(chan struct{})
	go func() {
		defer close(sendClientsDone)
		SendClients(admins, t.ParallelSends, dataSended, errorSended)
	}()

	// Listen for all clients to receive messages
	dataReceived := make(chan time.Duration)
	errorReceived := make(chan error)
	listenToClientsDone := make(chan struct{})
	go func() {
		defer close(listenToClientsDone)

		// Convert slice of clients to slice of Listeners
		listeners := make([]Listener, 0, len(clients))
		for _, client := range clients {
			listeners = append(listeners, client)
		}
		ListenToClients(listeners, dataReceived, errorReceived, len(admins), false)
	}()

	sendedResult := testResult{description: "Time until all requests have been sended", showAllErrors: t.ShowAllErrors}
	receivedResult := testResult{description: "Time until all responses have been received", showAllErrors: t.ShowAllErrors}
	defer func() { r = sendedResult.String() + "\n" + receivedResult.String() }()

	for {
		select {
		case value := <-dataSended:
			sendedResult.add(value)

		case value := <-errorSended:
			sendedResult.addError(value)

		case value := <-dataReceived:
			receivedResult.add(value)

		case value := <-errorReceived:
			receivedResult.addError(value)

		case <-cancel:
			return

		case <-listenToClientsDone:
			listenToClientsDone = nil

		case <-sendClientsDone:
			sendClientsDone = nil
		}

		// End the test when all admins have sended there data and each client got
		// as many response as there are admins.
		if listenToClientsDone == nil && sendClientsDone == nil {
			return
		}
	}
}

// KeepOpenTest is not a normal test. All it does is keeps the connection
// open for all given clients forever. You have to kill the program to exit.
// Expects the clients to be connected.
type KeepOpenTest struct{}

// Test runs the KeepOpenTest
func (t *KeepOpenTest) Test(clients []*Client, cancel <-chan struct{}) (r string) {
	log.Println("Start KeepOpenTest")
	startTest := time.Now()
	defer func() { log.Printf("KeepOpenTest took %dms\n", time.Since(startTest)/time.Millisecond) }()

	errChan := make(chan error)
	done := make(chan struct{})
	defer close(done)

	// Listen for all clients on the error chan to see when it closes the connection.
	for _, client := range clients {
		go func(c *Client, done <-chan struct{}) {
			select {
			case <-c.waitForError:
				errChan <- c.wsError
			case <-done:
			}
		}(client, done)
	}

	errCounter := 0

	for {
		select {
		case <-errChan:
			errCounter++
			if errCounter >= len(clients) {
				// All clients have failed
				return
			}

		case <-cancel:
			return
		}
	}
}
