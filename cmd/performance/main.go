package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/openslides/openslides-performance/pkg/oswstest"
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
func selectTests(flags []bool, showAllErrors bool, parallelConnections int, parallelSends int) (tests []oswstest.Tester, useConnectTest bool) {
	allTests := []oswstest.Tester{
		&oswstest.ConnectTest{ShowAllErrors: showAllErrors, ParallelConnections: parallelConnections},
		&oswstest.OneWriteTest{ShowAllErrors: showAllErrors},
		&oswstest.ManyWriteTest{ShowAllErrors: showAllErrors, ParallelSends: parallelSends},
		&oswstest.KeepOpenTest{},
	}
	tests = make([]oswstest.Tester, 0, 4)

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

	clients := make([]*oswstest.Client, 0, *anonymousClients+*userClients+*adminClients)

	fmt.Printf("Use %d clients\n", cap(clients))

	if *logStatus {
		defer oswstest.StartLogger(clients)()
	}

	start := time.Now()

	// Create admin clients
	if *adminClients > 0 {
		admin := oswstest.NewAdminClient(*serverDomain, *useSSL, *adminName, *adminPassword)
		clients = append(clients, admin)
		if err := admin.Login(); err != nil {
			log.Fatalf("Can not log in admin client: %v", err)
		}
		clients = append(clients, admin.Clone(*adminClients-1)...)
	}

	// Create anonymous clients
	if *anonymousClients > 0 {
		anonymous := oswstest.NewAnonymousClient(*serverDomain, *useSSL)
		clients = append(clients, anonymous)
		clients = append(clients, anonymous.Clone(*anonymousClients-1)...)
	}

	// Create user clients
	if *userClients > 0 {
		if strings.Contains(*userName, "%d") {
			// Login different users
			users := make([]*oswstest.Client, 0, *userClients)
			for i := 0; i < *userClients; i++ {
				users = append(users, oswstest.NewUserClient(*serverDomain, *useSSL, fmt.Sprintf(*userName, i), *userPassword))
			}
			clients = append(clients, users...)
			loginer := make([]oswstest.Loginer, 0, len(users))
			for _, user := range users {
				loginer = append(loginer, user)
			}
			oswstest.LoginClients(loginer, *parallelLogins, nil, nil)
		} else {
			// Login the same user
			user := oswstest.NewUserClient(*serverDomain, *useSSL, *userName, *userPassword)
			clients = append(clients, user)
			if err := user.Login(); err != nil {
				log.Fatalf("Can not log in user client: %v", err)
			}
			clients = append(clients, user.Clone(*userClients-1)...)
		}
	}

	log.Printf("All clients have been created and logged in %dms", time.Since(start)/time.Millisecond)

	// Connect clients if connect test is not used.
	if !useConnectTest {
		connecter := make([]oswstest.Connecter, 0, len(clients))
		for _, client := range clients {
			connecter = append(connecter, client)
		}
		start = time.Now()
		oswstest.ConnectClients(connecter, *parallelConnections, nil, nil)
		log.Printf("All clients have been connected in %dms", time.Since(start)/time.Millisecond)
	}

	start = time.Now()
	// Run all tests and print the results
	result := oswstest.RunTests(clients, tests, listenAbort())
	log.Printf("All tests took %dms", time.Since(start)/time.Millisecond)
	fmt.Println()
	fmt.Println(result)
}
