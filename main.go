package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
)

var hasAborted *bool

func init() {
	abort := false
	hasAborted = &abort
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Printf("Abort")
		abort = true
		<-c
		os.Exit(1)
	}()
}

func main() {
	var clients []Client

	// Create admin clients
	for i := 0; i < AdminClients; i++ {
		client := NewAdminClient(fmt.Sprintf("admin%d", i))
		clients = append(clients, client)
	}

	// Create user clients
	for i := 0; i < NormalClients; i++ {
		client := NewUserClient(fmt.Sprintf("user%d", i))
		clients = append(clients, client)
	}

	fmt.Printf("Use %d clients\n", len(clients))

	// Login all clients
	loginClients(clients)
	log.Println("All Clients have logged in.")

	// Run all tests and print the results
	for _, result := range RunTests(clients, Tests) {
		fmt.Println(result.String())
	}
}
