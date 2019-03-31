package tester_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/openslides/openslides-performance/pkg/tester"
)

type FakeClient struct {
	loggedin bool
	raise    error
}

func (c *FakeClient) Login() error {
	if c.raise != nil {
		return c.raise
	}
	c.loggedin = true
	return nil
}

func TestLoginClientsTestGetsLoggedInOnWorkingClients(t *testing.T) {
	clients := []tester.Loginer{&FakeClient{}, &FakeClient{}, &FakeClient{}}

	tester.LoginClients(clients, 1, nil, nil)

	for i, client := range clients {
		if !client.(*FakeClient).loggedin {
			t.Errorf("Client %d did not login", i)
		}
	}
}

func TestLoginClientsGetDurationOnWorkingClients(t *testing.T) {
	clients := []tester.Loginer{&FakeClient{}, &FakeClient{}, &FakeClient{}}
	duration := make(chan time.Duration, 3)

	tester.LoginClients(clients, 1, duration, nil)

	// Make sure there are the right amount of messages
	for i := 0; i < cap(duration); i++ {
		select {
		case <-duration:
		default:
			t.Errorf("Expect %d messages, got only %d", cap(duration), i)
			return
		}
	}
}

func TestLoginClientsGetOneErrorOnWorkingClients(t *testing.T) {
	clients := []tester.Loginer{&FakeClient{}, &FakeClient{raise: fmt.Errorf("No good reason")}, &FakeClient{}}
	duration := make(chan time.Duration, 2)
	errC := make(chan error, 1)

	tester.LoginClients(clients, 1, duration, errC)

	// Make sure there are the right amount of messages
	for i := 0; i < cap(errC); i++ {
		select {
		case <-duration:
		default:
			t.Errorf("Expect %d errors, got only %d", cap(errC), i)
			return
		}
	}
}
