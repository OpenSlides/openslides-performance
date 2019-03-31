package client

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"strings"
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
	username string
	password string
	isAdmin  bool

	wsConnect wsConnecter

	connectionAttemts int
	loginAttemts      int

	messageCount int   // Counts how many websocket messages the client received
	wsError      error // Saves a websocket error if it happens

	cookies *cookiejar.Jar

	connected      time.Time
	connectedMu    sync.RWMutex
	waitForError   chan struct{} // Will be closed on error
	waitForConnect chan struct{} // will be closed when the client open connects

	serverDomain string
	useSSL       bool

	// When a websocket package is received, the done channel of all structs are closed
	// when it is the `count` message
	expectData   []condition
	exceptDataMu sync.RWMutex // Protects expectData and messageCount
}

// NewClient creates a new client. Use `Option` to add login credentials etc.
func NewClient(serverDomain string, opts ...Option) *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("Can not create cookie jar, %s\n", err)
	}
	c := &Client{
		waitForConnect:    make(chan struct{}),
		waitForError:      make(chan struct{}),
		cookies:           jar,
		serverDomain:      serverDomain,
		wsConnect:         wsConnect{},
		loginAttemts:      3,
		connectionAttemts: 5,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(c)
		}
	}

	return c
}

// Clone returns `count` copies of the client.
// All cloned clients share the same cookie and therefor the same session.
// The cloned clients are not connected to the server.
func (c *Client) Clone(count int) []*Client {
	out := make([]*Client, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, &Client{
			waitForConnect:    make(chan struct{}),
			waitForError:      make(chan struct{}),
			cookies:           c.cookies,
			serverDomain:      c.serverDomain,
			useSSL:            c.useSSL,
			wsConnect:         wsConnect{},
			loginAttemts:      c.loginAttemts,
			connectionAttemts: c.connectionAttemts,

			username: c.username,
			password: c.password,
			isAdmin:  c.isAdmin,
		})
	}
	return out
}

func (c *Client) getLoginData() string {
	return fmt.Sprintf("{\"username\": \"%s\", \"password\": \"%s\"}", c.username, c.password)
}

// Login logsin a client. This sends a login request to the server and saves
// the session cookie for later use.
func (c *Client) Login() error {
	if c.username == "" {
		return fmt.Errorf("can not login client without an username")
	}

	httpClient := &http.Client{
		Jar: c.cookies,
	}
	var resp *http.Response
	var err error

	for i := 0; i < c.loginAttemts; i++ {
		resp, err = httpClient.Post(
			getLoginURL(c.serverDomain, c.useSSL),
			"application/json",
			strings.NewReader(c.getLoginData()),
		)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Statu Code not between 500 and 600
		if !(500 <= resp.StatusCode && resp.StatusCode < 600) {
			break
		}
		// If the error is on the server side, then retry after some time
		time.Sleep(100 * time.Millisecond)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("login for client %s failed: StatusCode: %d", c, resp.StatusCode)
	}
	return nil
}

// condition is an information to a client, that the `done` channel should be closed, then
// the client received `count` messages.
type condition struct {
	count int
	done  chan<- struct{}
}

// Connect creates a websocket connection.
func (c *Client) Connect() (err error) {
	var wsConnection ReaderCloser
	success := false
	for i := 0; i < c.connectionAttemts; i++ {
		wsConnection, err = c.wsConnect.Connect(getWebsocketURL(c.serverDomain, c.useSSL), c.cookies)
		if err == nil {
			// if no error happened, then we can break the loop
			success = true
			break
		}
	}
	if !success {
		log.Printf("Could not connect client %s, %v\n", c, err)
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

// Send sends a pre defined put request to the server. Only a admin client should
// use this method.
func (c *Client) Send() (err error) {
	httpClient := &http.Client{
		Jar: c.cookies,
	}
	req := getSendRequest(c.serverDomain, c.useSSL)

	// Write csrf token from cookie into the http header
	var CSRFToken string
	for _, cookie := range c.cookies.Cookies(req.URL) {
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

// IsAdmin returns True, if the client is an admin client.
func (c *Client) IsAdmin() bool {
	return c.isAdmin
}

// Connected returns the time since the client is connected. Returns 0 if it is not connected.
func (c *Client) Connected() time.Time {
	c.connectedMu.RLock()
	defer c.connectedMu.RUnlock()
	return c.connected
}

// String returns the username of the client. `anonymous` if it is an anonymous client.
func (c *Client) String() string {
	if c.username == "" {
		return "anonymous"
	}
	return c.username
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
