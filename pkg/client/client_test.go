package client_test

import (
	"fmt"
	"net/http/cookiejar"
	"testing"

	"github.com/openslides/openslides-performance/pkg/client"
)

type fakeWSConnect struct {
	connected bool
	rc        *fakeReaderCloser
}

func (ws *fakeWSConnect) Connect(uri string, cookieJar *cookiejar.Jar) (conn client.ReaderCloser, err error) {
	ws.connected = true
	rc := fakeReaderCloser{
		connection:   ws,
		nextMessageC: make(chan message, 10),
		closed:       make(chan struct{}),
	}
	ws.rc = &rc
	return &rc, nil
}

type message struct {
	m   []byte
	err error
}

type fakeReaderCloser struct {
	connection   *fakeWSConnect
	closed       chan struct{}
	nextMessageC chan message
}

func (rc *fakeReaderCloser) Close() error {
	rc.connection.connected = false
	return nil
}

func (rc *fakeReaderCloser) nextMessage(m []byte, err error) {
	rc.nextMessageC <- message{
		m,
		err,
	}

}

func (rc *fakeReaderCloser) ReadMessage() (int, []byte, error) {
	select {
	case nm := <-rc.nextMessageC:
		if nm.err != nil {
			return 0, nil, nm.err
		}
		return len(nm.m), nm.m, nil
	case <-rc.closed:
		return 0, nil, fmt.Errorf("connection closed")
	}

}

func TestNewClientIsAnonymous(t *testing.T) {
	c := client.NewClient()

	if c.IsAdmin() {
		t.Errorf("Expect an anonymous client not to be admin.")
	}
}

func TestNewClientNotConnected(t *testing.T) {
	c := client.NewClient()

	if !c.Connected().IsZero() {
		t.Errorf("Expect a client not to be connected at startup.")
	}
}

func TestNewClientWithAdminSessionIsAdmin(t *testing.T) {
	session, err := client.NewSession("mydomain", false, "myname", "password", true)
	if err != nil {
		t.Fatalf("Can not create session: %v", err)
	}
	c := client.NewClient(client.WithSession(session))
	if !c.IsAdmin() {
		t.Errorf("Exect an admin user to be admin.")
	}
}

func TestClientConnectExpectData(t *testing.T) {
	connection := &fakeWSConnect{}
	c := client.NewClient(client.WithServer("domain", false), client.WithConnecter(connection))

	if err := c.Connect(); err != nil {
		t.Errorf("Connect failed: %v", err)
	}
	defer connection.rc.nextMessage(nil, fmt.Errorf("close"))

	connection.rc.nextMessage([]byte("some message"), nil)
	// wait until there is at least one message
	if err := c.ExpectData(1, false); err != nil {
		t.Errorf("ExpectData failed: %v", err)
	}
}

// func TestClientCloneUserClient(t *testing.T) {
// 	c := client.NewClient("domain", client.WithCredentials("myname", "password"))

// 	clone := c.Clone(1)

// 	if clone[0].String() != "myname" {
// 		t.Errorf("Expect the clone to have the same name es the original client, got: %s", clone[0].String())
// 	}
// }
