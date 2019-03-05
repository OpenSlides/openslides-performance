package main

import "github.com/openslides/openslides-performance/pkg/oswstest"

// NormalClients and AdminClients are all clients, that are logged in. For the
// ConnectionTest there is no difference between the to clients. The AdminClient
// is needed to write data.
const (
	NormalClients = 10
	AdminClients  = 10
)

// Tests is a list of all tests to performe
var Tests = []oswstest.Test{
	// ConnectTest connects all clients. Measures the time until all clients are
	// connected and until they all got there first data.
	oswstest.ConnectTest,

	// OneWriteTest expects the first client to be an admin client and all clients
	// to be connected. Therefore the test requires, that the ConnectTest is run
	// before. This test sends one write request with the first client and measures
	// the time until all clients get the changed data.
	oswstest.OneWriteTest,

	// ManyWriteTests expects at least one client to be an admin client and all clients
	// to be connected. Therefore the test requires, that the ConnectTest is run
	// before. This test sends one write request for each admin client and measures
	// the time until all write requests are send and until all data is received.
	oswstest.ManyWriteTest,

	// Keeps the connections open. This is not usual for a testrun of this program
	// but can help to open a lot of connections with this tool to test manuely
	// how OpenSlides reacts with a lot of open connections.
	//oswstest.KeepOpenTest,
}
