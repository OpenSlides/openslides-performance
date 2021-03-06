package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/openslides/openslides-performance/pkg/client"
	"github.com/openslides/openslides-performance/pkg/logger"
	"github.com/openslides/openslides-performance/pkg/tester"
)

// Aborts the program when strg+c is hit. Hart closes it at a second strg+c
func listenAbort() <-chan struct{} {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	cancel := make(chan struct{})
	go func() {
		// strg+c is send once
		<-c
		log.Printf("Abort")
		close(cancel)
		// strg+c is send a second time
		<-c
		os.Exit(1)
	}()
	return cancel
}

// selectTests returns a slice on Tests that should be run.
// Returns the default tests (all except keep open test) when
// non test is selected. The second return value is true, then
// `ConnectTest` is used.
func selectTests(flags []bool, showAllErrors bool, parallelConnections int, parallelSends int) (tests []tester.Tester, useConnectTest bool) {
	allTests := []tester.Tester{
		&tester.ConnectTest{ShowAllErrors: showAllErrors, ParallelConnections: parallelConnections},
		&tester.OneWriteTest{ShowAllErrors: showAllErrors},
		&tester.ManyWriteTest{ShowAllErrors: showAllErrors, ParallelSends: parallelSends},
		&tester.KeepOpenTest{},
	}
	tests = make([]tester.Tester, 0, 4)

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
	anonymousClients := flag.Int("anonymous", 0, "Number of anonymous clients to use")
	userClients := flag.Int("users", 10, "Number of non-admin clients to use")
	userName := flag.String("username", "user%d", "Username of normal users. `%d` is a placeholder for an incrementing number")
	userPassword := flag.String("userpassword", "password", "Login password for user clients")
	adminClients := flag.Int("admins", 10, "Number of admin clients to use")
	adminName := flag.String("adminname", "admin", "Username for the admin users")
	adminPassword := flag.String("adminpassword", "admin", "Login password for the admin clients")
	serverDomain := flag.String("server", "localhost:8000", "Domain of the OpenSlides server")
	useSSL := flag.Bool("ssl", false, "Use ssl for http and websocket requests")
	connectTest := flag.Bool("connect-test", false, "Use connect test. If all tests are false, this is used.")
	oneWriteTest := flag.Bool("one-write-test", false, "Use one write test. If all tests are false, this is used.")
	manyWriteTest := flag.Bool("many-write-test", false, "Use many write test. If all tests are false, this is used.")
	keepOpenTest := flag.Bool("keep-open-test", false, "Use keep open test.")
	showAllErrors := flag.Bool("all-errors", false, "Show all errors when represent the test results. In other case, only show the first error.")
	logStatus := flag.Bool("log-status", false, "Show connected clients, received packages and errors each second.")
	parallel := flag.Int("parallel", 8, "The default value for all parallel actions. Zero means, that all happens in parallel")
	parallelLogins := flag.Int("parallel-logins", -1, "Number at login requests at the same time. When it is -1, the value from parallel is used.")
	parallelConnections := flag.Int("parallel-connections", -1, "Number at websocket connections at the same time. When it is -1, the value from parallel is used.")
	parallelSends := flag.Int("parallel-sends", -1, "Number at send requests at the same time. When it is -1, the value from parallel is used.")

	flag.Parse()

	if *parallelLogins == -1 {
		*parallelLogins = *parallel
	}

	if *parallelConnections == -1 {
		*parallelConnections = *parallel
	}

	if *parallelSends == -1 {
		*parallelSends = *parallel
	}

	tests, useConnectTest := selectTests([]bool{*connectTest, *oneWriteTest, *manyWriteTest, *keepOpenTest}, *showAllErrors, *parallelSends, *parallelSends)

	clients := make([]*client.Client, 0, *anonymousClients+*userClients+*adminClients)
	fmt.Printf("Use %d clients\n", cap(clients))

	loginer := make([]tester.Loginer, 0)

	if *adminClients > 0 {
		session, err := client.NewSession(*serverDomain, *useSSL, *adminName, *adminPassword, true)
		if err != nil {
			log.Fatalf("Can not create admin session: %v", err)
		}

		loginer = append(loginer, session)
		for i := 0; i < *adminClients; i++ {
			clients = append(clients, client.NewClient(client.WithSession(session)))
		}
	}

	// Create user clients
	if *userClients > 0 {
		if strings.Contains(*userName, "%d") {
			// Login different users
			for i := 0; i < *userClients; i++ {
				session, err := client.NewSession(*serverDomain, *useSSL, fmt.Sprintf(*userName, i), *userPassword, false)
				if err != nil {
					log.Fatalf("Can not create user session: %v", err)
				}
				clients = append(clients, client.NewClient(client.WithSession(session)))
				loginer = append(loginer, session)
			}
		} else {
			// Login the same user
			session, err := client.NewSession(*serverDomain, *useSSL, *userName, *userPassword, false)
			if err != nil {
				log.Fatalf("Can not create user session: %v", err)
			}

			loginer = append(loginer, session)
			for i := 0; i < *userClients; i++ {
				clients = append(clients, client.NewClient(client.WithSession(session)))
			}
		}
	}

	for i := 0; i < *anonymousClients; i++ {
		clients = append(clients, client.NewClient(client.WithServer(*serverDomain, *useSSL)))
	}

	if *logStatus {
		defer logger.StartLogger(clients)()
	}

	start := time.Now()
	tester.LoginClients(loginer, *parallelLogins, nil, nil)
	log.Printf("Login %d sessions in %dms", len(loginer), time.Since(start)/time.Millisecond)

	// Connect clients if connect test is not used.
	if !useConnectTest {
		connecter := make([]tester.Connecter, 0, len(clients))
		for _, client := range clients {
			connecter = append(connecter, client)
		}
		start = time.Now()
		tester.ConnectClients(connecter, *parallelConnections, nil, nil)
		log.Printf("All clients have been connected in %dms", time.Since(start)/time.Millisecond)
	}

	// Run all tests and print the results
	start = time.Now()
	result := tester.RunTests(clients, tests, listenAbort())
	log.Printf("All tests took %dms", time.Since(start)/time.Millisecond)
	fmt.Println()
	fmt.Println(result)
}
