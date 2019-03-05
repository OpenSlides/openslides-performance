package main

import (
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
	var clients []oswstest.Client

	// Create admin clients
	for i := 0; i < AdminClients; i++ {
		client := oswstest.NewAdminClient(fmt.Sprintf("admin%d", i))
		clients = append(clients, client)
	}

	// Create user clients
	for i := 0; i < NormalClients; i++ {
		client := oswstest.NewUserClient(fmt.Sprintf("user%d", i))
		clients = append(clients, client)
	}

	fmt.Printf("Use %d clients\n", len(clients))

	// Login all clients
	oswstest.LoginClients(clients)
	log.Println("All Clients have logged in.")

	// Run all tests and print the results
	for _, result := range oswstest.RunTests(clients, Tests) {
		fmt.Println(result.String())
	}
}
