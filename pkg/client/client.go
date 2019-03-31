package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"sync"
	"time"
)

const (
	// loginURLPath is the path to build the url for login. It has no leading slash.
	loginURLPath = "apps/users/login/"

	// wsURLPath is the path to build the websocket url. It has no leading slash.
	wsURLPath = "ws/?change_id=0&autoupdate=on"

	// CSRFCookieName is the name of the CSRF cookie of OpenSlides. Make sure, that
	// this is the same as in the OpenSlides config.
	csrfCookieName = "OpenSlidesCsrfToken"
)

// Client represents one connection to the OpenSlides server.
type Client struct {
	wsConnect         wsConnecter
	connectionAttemts int
	connected         time.Time
	connectedMu       sync.RWMutex
	waitForError      chan struct{} // Will be closed on error
	waitForConnect    chan struct{} // will be closed when the client open connects

	messageCount int   // Counts how many websocket messages the client received
	wsError      error // Saves a websocket error if it happens

	server  string
	ssl     bool
	session *Session

	// When a websocket package is received, the done channel of all structs are closed
	// when it is the `count` message
	expectData   []condition
	exceptDataMu sync.RWMutex // Protects expectData and messageCount
}

// NewClient creates a new client. Use `Option` to add login credentials etc.
func NewClient(opts ...Option) *Client {
	c := &Client{
		waitForConnect:    make(chan struct{}),
		waitForError:      make(chan struct{}),
		wsConnect:         wsConnect{},
		connectionAttemts: 5,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	return c
}

// condition is an information to a client, that the `done` channel should be closed, then
// the client received `count` messages.
type condition struct {
	count int
	done  chan<- struct{}
}

// Connect creates a websocket connection.
func (c *Client) Connect() (err error) {
	if c.session == nil && c.server == "" {
		return fmt.Errorf("Client has not server domain. You either have to create it WithSession or WithServer")
	}
	var wsConnection ReaderCloser
	var cookies *cookiejar.Jar
	if c.session != nil {
		cookies = c.session.cookies
	}
	success := false
	for i := 0; i < c.connectionAttemts; i++ {
		wsConnection, err = c.wsConnect.Connect(getWebsocketURL(c.serverSSL()), cookies)
		if err == nil {
			// if no error happened, then we can break the loop
			success = true
			break
		}
	}
	if !success {
		close(c.waitForError)
		c.wsError = err
		return err
	}

	// Set the connected time to now and close the waitForConnect channel to signal
	// that the client is now connected.
	c.connectedMu.Lock()
	c.connected = time.Now()
	c.connectedMu.Unlock()
	close(c.waitForConnect)

	go func() {
		// Write all incomming messages into c.wsRead.
		// Before SetChannel() is called, this channel is nil
		defer wsConnection.Close()
		defer func() { c.connected = time.Time{} }()

		for {
			_, _, err := wsConnection.ReadMessage()
			if err != nil {
				c.wsError = err
				close(c.waitForError)
				return
			}
			c.exceptDataMu.Lock()
			c.messageCount++
			for i, condition := range c.expectData {
				if c.messageCount >= condition.count {
					close(condition.done)
					c.expectData[i] = c.expectData[len(c.expectData)-1]
					c.expectData = c.expectData[:len(c.expectData)-1]
				}
			}
			c.exceptDataMu.Unlock()
		}
	}()
	return nil
}

// ExpectData runs, until there are `count` websocket messages or one websocket error.
// If `sinceConnect`, if starts counting at the beginning of the connection, even
// when this function is called later
func (c *Client) ExpectData(count int, sinceConnect bool) error {
	if !sinceConnect {
		count += c.MessageCount()
	}

	done := make(chan struct{})
	c.exceptDataMu.Lock()
	c.expectData = append(c.expectData, condition{count, done})
	c.exceptDataMu.Unlock()

	// Wait until condition happens or there is an error
	select {
	case <-done:
	case <-c.waitForError:
		return c.wsError
	}
	return nil
}

// Send sends a pre defined put request to the server. Only a admin client can
// use this method.
func (c *Client) Send() error {
	if !c.IsAdmin() {
		return fmt.Errorf("only an admin client can use Send().")
	}

	httpClient := &http.Client{
		Jar: c.session.cookies,
	}
	req := getSendRequest(c.serverSSL())

	// Write csrf token from cookie into the http header
	var CSRFToken string
	for _, cookie := range c.session.cookies.Cookies(req.URL) {
		if cookie.Name == csrfCookieName {
			CSRFToken = cookie.Value
			break
		}
	}
	if CSRFToken == "" {
		log.Fatalln("No CSRF-Token in cookies")
	}

	req.Header.Set("X-CSRFToken", CSRFToken)
	req.Header.Set("Content-Type", "application/json;charset=UTF-8")
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}

	// StatusCode not between 200 and 300
	if !(200 <= resp.StatusCode && resp.StatusCode < 300) {
		bodyBuffer, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("%s\n", bodyBuffer)
		return fmt.Errorf("got an error by sending the request, status: %s", resp.Status)
	}
	return nil
}

func (c *Client) serverSSL() (string, bool) {
	if c.session == nil {
		return c.server, c.ssl
	}
	return c.session.server, c.session.ssl
}

// IsAdmin returns True, if the client is an admin client.
func (c *Client) IsAdmin() bool {
	if c.session == nil {
		return false
	}
	return c.session.isAdmin
}

// Connected returns the time since the client is connected. Returns 0 if it is not connected.
func (c *Client) Connected() time.Time {
	c.connectedMu.RLock()
	defer c.connectedMu.RUnlock()
	return c.connected
}

// MessageCount returns the number of received messages.
func (c *Client) MessageCount() int {
	c.exceptDataMu.RLock()
	defer c.exceptDataMu.RUnlock()
	return c.messageCount
}

// WaitForError blogs until an websocket error happens at the client or the
// cancel channel is closed.
// Returns the websocket error or nil
func (c *Client) WaitForError(cancel <-chan struct{}) error {
	select {
	case <-c.waitForError:
		return c.wsError
	case <-cancel:
	}
	return nil
}
