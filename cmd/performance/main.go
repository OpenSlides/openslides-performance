package main

import (
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
	oswstest.RunDefaultTests()
}
