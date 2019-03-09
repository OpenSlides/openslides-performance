package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

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

func main() {
	normalClients := flag.Int("users", 10, "Number of non-admin clients to use")
	adminClients := flag.Int("admins", 10, "Number of admin clients to use")
	password := flag.String("password", "password", "Login password for normal and admin clients")
	serverDomain := flag.String("server", "localhost:8000", "Domain of the OpenSlides server")
	useSSL := flag.Bool("ssl", false, "Use ssl for http and websocket requests")

	flag.Parse()

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
	oswstest.LoginClients(clients)
	log.Println("All Clients have logged in.")

	// Run all tests and print the results
	for _, result := range oswstest.RunTests(clients, defaultTests) {
		fmt.Println(result.String())
	}
}
