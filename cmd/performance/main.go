package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/openslides/openslides-performance/pkg/oswstest"
)

// Aborts the program when strg+c is hit. Hart closes it at a second strg+c
func init() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		// strg+c is send once
		<-c
		log.Printf("Abort")
		oswstest.Abort()
		// strg+c is send a second time
		<-c
		os.Exit(1)
	}()
}

// selectTests returns a slice on Tests that should be run.
// Returns the default tests (all except keep open test) when
// non test is selected. The second return value is true, then
// `ConnectTest` is used.
func selectTests(flags []bool) (tests []oswstest.Test, useConnectTest bool) {
	allTests := []oswstest.Test{oswstest.ConnectTest, oswstest.OneWriteTest, oswstest.ManyWriteTest, oswstest.KeepOpenTest}
	tests = make([]oswstest.Test, 0, 4)

	for i, flag := range flags {
		if flag {
			tests = append(tests, allTests[i])
		}
	}

	if len(tests) == 0 {
		// If non is selected, use all except the keep open test.
		return allTests[:3], true
	}
	return tests, flags[0]
}

func main() {
	normalClients := flag.Int("users", 10, "Number of non-admin clients to use")
	adminClients := flag.Int("admins", 10, "Number of admin clients to use")
	password := flag.String("password", "password", "Login password for normal and admin clients")
	serverDomain := flag.String("server", "localhost:8000", "Domain of the OpenSlides server")
	useSSL := flag.Bool("ssl", false, "Use ssl for http and websocket requests")
	connectTest := flag.Bool("connect-test", false, "Use connect test. If all tests are false, this is used.")
	oneWriteTest := flag.Bool("one-write-test", false, "Use one write test. If all tests are false, this is used.")
	manyWriteTest := flag.Bool("many-write-test", false, "Use many write test. If all tests are false, this is used.")
	keepOpenTest := flag.Bool("keep-open-test", false, "Use keep open test.")
	showAllErrors := flag.Bool("all-errors", false, "Show all errors when represent the test results. In other case, only show the first error.")
	logStatus := flag.Bool("log-status", false, "Show some output each second as long as a test runs. The output depends on the test.")

	flag.Parse()

	tests, useConnectTest := selectTests([]bool{*connectTest, *oneWriteTest, *manyWriteTest, *keepOpenTest})

	clients := make([]oswstest.Client, 0, *normalClients+*adminClients)

	// Create admin clients
	for i := 0; i < *adminClients; i++ {
		clients = append(clients, oswstest.NewAdminClient(*serverDomain, *useSSL, fmt.Sprintf("admin%d", i), *password))
	}

	// Create user clients
	for i := 0; i < *normalClients; i++ {
		clients = append(clients, oswstest.NewUserClient(*serverDomain, *useSSL, fmt.Sprintf("user%d", i), *password))
	}

	fmt.Printf("Use %d clients\n", len(clients))

	// Login all clients
	start := time.Now()
	<-oswstest.LoginClients(clients, nil, nil)
	log.Printf("All clients have logged in %dms", time.Since(start)/time.Millisecond)

	// Connect clients if connect test is not used.
	if !useConnectTest {
		start = time.Now()
		<-oswstest.ConnectClients(clients, nil, nil)
		log.Printf("All clients have been connected in %dms", time.Since(start)/time.Millisecond)
	}

	// Run all tests and print the results
	for _, result := range oswstest.RunTests(clients, tests, *showAllErrors, *logStatus) {
		fmt.Println(result.String())
	}
}
