package oswstest_test

import (
	"fmt"
	"net/http/cookiejar"
	"testing"

	"github.com/openslides/openslides-performance/pkg/oswstest"
)

type fakeWSConnect struct {
	connected bool
	rc        *fakeReaderCloser
}

func (ws *fakeWSConnect) Connect(uri string, cookieJar *cookiejar.Jar) (conn oswstest.ReaderCloser, err error) {
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

func TestNewAnonymousClientIsAnonymous(t *testing.T) {
	c := oswstest.NewAnonymousClient("domain", false)

	if c.IsAdmin() {
		t.Errorf("Expect an anonymous client not to be admin.")
	}
	if c.String() != "anonymous" {
		t.Errorf("Except anonymous client to be called anonymous.")
	}
}

func TestNewAnonymousClientNotConnected(t *testing.T) {
	c := oswstest.NewAnonymousClient("domain", false)

	if !c.Connected().IsZero() {
		t.Errorf("Expect a client not to be connected at startup.")
	}
}

func TestNewUserClientNotAdminWithName(t *testing.T) {
	c := oswstest.NewUserClient("domain", false, "myname", "password")

	if c.IsAdmin() {
		t.Errorf("Expect an user client not to be admin.")
	}
	if c.String() != "myname" {
		t.Errorf("Except user client to be called be its name, not %s.", c.String())
	}
}

func TestNewAdminClientIsAdmin(t *testing.T) {
	c := oswstest.NewAdminClient("domain", false, "myname", "password")
	if !c.IsAdmin() {
		t.Errorf("Exect an admin user to be admin.")
	}
}

func TestClientConnectExpectData(t *testing.T) {
	c := oswstest.NewAnonymousClient("domain", false)
	connection := &fakeWSConnect{}
	c.WSConnect = connection

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
