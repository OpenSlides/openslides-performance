package oswstest

import (
	"fmt"
	"log"
	"time"
)

// Test is a function, that expect a slice of clients and returns a slice of
// test results.
type Test func(clients []Client) []fmt.Stringer

// testLogStatus decides, if the tests output log information. Is set in RunTests
var testLogStatus bool

var inTests bool

// RunTests runs some tests for a slice of clients. It returns the TestResults
// for each test.
func RunTests(clients []Client, tests []Test, showAllErrors bool, logStatus bool) (r []fmt.Stringer) {
	inTests = true
	defer func() { inTests = false }()

	testLogStatus = logStatus
	start := time.Now()
	defer func() { fmt.Printf("\nAll tests took %dms\n\n", time.Since(start)/time.Millisecond) }()

	for _, test := range tests {
		for _, result := range test(clients) {
			// TODO
			//result.showAllErrors = showAllErrors
			r = append(r, result)
		}

		select {
		case <-hasAborted:
			return
		default:
		}
	}
	return
}

// ConnectTest opens connections for any given client. It returns two
// TestResults. The first measures the time until the connection was open, the
// second measures the time until the first data was received. Expects, that the
// wsconnection of the clients are closed.
func ConnectTest(clients []Client) (r []fmt.Stringer) {
	log.Println("Start ConnectTest")
	startTest := time.Now()
	defer func() { log.Printf("ConnectionTest took %dms", time.Since(startTest)/time.Millisecond) }()

	// Connect all Clients
	connected := make(chan time.Duration)
	connectedError := make(chan error)
	connectionDone := make(chan struct{})
	go ConnectClients(clients, connectedError, connected, connectionDone)

	// Listen to all clients to receive the response.
	dataReceived := make(chan time.Duration)
	errorReceived := make(chan error)
	receivedDone := make(chan struct{})
	go ListenToClients(clients, dataReceived, errorReceived, 1, true, receivedDone)

	var connectFinished, receivedFinished bool
	connectedResult := testResult{description: "Time to established connection"}
	dataReceivedResult := testResult{description: "Time until data has been reveiced since the connection"}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

Loop:
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

		case <-ticker.C:
			if testLogStatus {
				log.Println(connectedResult.countBoth(), dataReceivedResult.countBoth())
			}

		case <-hasAborted:
			break Loop

		case <-connectionDone:
			connectFinished = true

		case <-receivedDone:
			receivedFinished = true
		}

		if connectFinished && receivedFinished {
			break Loop
		}
	}
	return []fmt.Stringer{&connectedResult, &dataReceivedResult}
}

// OneWriteTest tests, that all clients get a response when there is one write
// request. Expects, that the first client is a logged-in admin client and that
// all clients have open websocket connections.
func OneWriteTest(clients []Client) (r []fmt.Stringer) {
	log.Println("Start OneWriteTest")
	startTest := time.Now()
	defer func() { log.Printf("OneWriteTest took %dms\n", time.Since(startTest)/time.Millisecond) }()

	// Find the admin client.
	admin, ok := clients[0].(AdminClient)
	if !ok || !admin.IsAdmin() || (admin.Connected() == time.Time{}) {
		log.Fatalf("Fatal: Expect the first client in OneWriteTest to be a connected AdminClient")
	}

	// Listen to all clients to receive the response.
	dataReceived := make(chan time.Duration)
	errorReceived := make(chan error)
	listenToClientsDone := make(chan struct{})
	go ListenToClients(clients, dataReceived, errorReceived, 1, false, listenToClientsDone)

	// Send the request.
	if err := admin.Send(); err != nil {
		log.Fatalf("Can not send request, %s", err)
	}

	dataReceivedResult := testResult{description: "Time until data is received after one write request"}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

Loop:
	for {
		select {
		case value := <-dataReceived:
			dataReceivedResult.add(value)

		case value := <-errorReceived:
			dataReceivedResult.addError(value)

		case <-ticker.C:
			if testLogStatus {
				log.Println(dataReceivedResult.count() + dataReceivedResult.errCount())
			}

		case <-hasAborted:
			break Loop

		case <-listenToClientsDone:
			break Loop
		}
	}

	return []fmt.Stringer{&dataReceivedResult}
}

// ManyWriteTest tests behave like the OneWriteTest but send one write request
// per admin client. Expects, that at least one client is a logged-in admin
// client and that all clients have open websocket connections.
func ManyWriteTest(clients []Client) (r []fmt.Stringer) {
	log.Println("Start ManyWriteTest")
	startTest := time.Now()
	defer func() { log.Printf("ManyWriteTest took %dms\n", time.Since(startTest)/time.Millisecond) }()

	// Find all connected admins in clients
	var admins []AdminClient
	for _, client := range clients {
		admin, ok := client.(AdminClient)
		if ok && admin.IsAdmin() && admin.Connected().After(time.Time{}) {
			admins = append(admins, admin)
		}
	}
	if len(admins) == 0 {
		log.Fatalf("Fatal: Expect one client in ManyWriteTest to be a connected AdminClient")
	}

	// Send requests for all admin clients
	dataSended := make(chan time.Duration)
	errorSended := make(chan error)
	sendClientsDone := make(chan struct{})
	go SendClients(admins, errorSended, dataSended, sendClientsDone)

	// Listen for all clients to receive messages
	dataReceived := make(chan time.Duration)
	errorReceived := make(chan error)
	listenToClientsDone := make(chan struct{})
	go ListenToClients(clients, dataReceived, errorReceived, len(admins), false, listenToClientsDone)

	var sendFinished, receiveFinished bool
	sendedResult := testResult{description: "Time until all requests have been sended"}
	receivedResult := testResult{description: "Time until all responses have been received"}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

Loop:
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

		case <-ticker.C:
			if testLogStatus {
				log.Println(sendedResult.countBoth(), receivedResult.countBoth())
			}

		case <-hasAborted:
			break Loop

		case <-listenToClientsDone:
			receiveFinished = true

		case <-sendClientsDone:
			sendFinished = true
		}

		// End the test when all admins have sended there data and each client got
		// as many responces as there are admins.
		if sendFinished && receiveFinished {
			break
		}
	}

	return []fmt.Stringer{&sendedResult, &receivedResult}
}

// KeepOpenTest is not a normal test. All it does is keeps the connection
// open for all given clients forever. You have to kill the program to exit.
// Expects the clients to be connected.
func KeepOpenTest(clients []Client) (r []fmt.Stringer) {
	log.Println("Start KeepOpenTest")
	startTest := time.Now()
	defer func() { log.Printf("KeepOpenTest took %dms\n", time.Since(startTest)/time.Millisecond) }()

	readChan := make(chan []byte)
	errChan := make(chan error)

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	counter := 0
	errCounter := 0

	for {
		select {
		case <-readChan:
			counter++

		case <-errChan:
			errCounter++

		case <-ticker.C:
			if testLogStatus {
				log.Println(counter, errCounter)
			}

		case <-hasAborted:
			return []fmt.Stringer{}
		}
	}
}
