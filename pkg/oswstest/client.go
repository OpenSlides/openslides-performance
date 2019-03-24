package oswstest

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

// Loginer is a type that can be logged in
type Loginer interface {
	Login() error
}

// Connecter is a type that can connect.
type Connecter interface {
	Connect() error
}

// Sender is a type that can Send something.
type Sender interface {
	Send() error
}

// Listener is a type that you get except to receive data.
type Listener interface {
	ExpectData(count int, sinceConnect bool) error
	Connected() time.Time
}

func getLoginURL(serverDomain string, useSSL bool) string {
	protocol := "http"
	if useSSL {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s/%s", protocol, serverDomain, loginURLPath)
}

func getWebsocketURL(serverDomain string, useSSL bool) string {
	protocol := "ws"
	if useSSL {
		protocol = "wss"
	}
	return fmt.Sprintf("%s://%s/%s", protocol, serverDomain, wsURLPath)
}

// getSendRequest returns the request that is send by the admin clients
func getSendRequest(serverDomain string, useSSL bool) (r *http.Request) {
	protocol := "http"
	if useSSL {
		protocol = "https"
	}

	r, err := http.NewRequest(
		"PUT",
		fmt.Sprintf("%s://%s/%s", protocol, serverDomain, "rest/agenda/item/1/"),
		strings.NewReader(`
			{"id":1,"item_number":"","title":"foo1","list_view_title":"foo1",
			"comment":"test","closed":false,"type":1,"is_hidden":false,"duration":null,
			"speaker_list_closed":false,"content_object":{"collection":"topics/topic",
			"id":1},"weight":10000,"parent_id":null,"parentCount":0,"hover":true}`),
	)
	if err != nil {
		log.Fatalf("Coud not build the request, %s", err)
	}
	r.Close = true
	return r
}

// Client represents one of many openslides users
type Client struct {
	username string
	password string
	isAuth   bool
	isAdmin  bool

	WSConnect wsConnecter

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

// NewAnonymousClient creates an anonymous client.
func NewAnonymousClient(serverDomain string, useSSL bool) *Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("Can not create cookie jar, %s\n", err)
	}
	return &Client{
		waitForConnect: make(chan struct{}),
		waitForError:   make(chan struct{}),
		cookies:        jar,
		serverDomain:   serverDomain,
		useSSL:         useSSL,
		WSConnect:      wsConnect{},
	}
}

// NewUserClient creates an user client.
func NewUserClient(serverDomain string, useSSL bool, username string, password string) *Client {
	c := NewAnonymousClient(serverDomain, useSSL)
	c.username = username
	c.password = password
	c.isAuth = true
	return c
}

// NewAdminClient creates an admin client.
func NewAdminClient(serverDomain string, useSSL bool, username string, password string) *Client {
	c := NewUserClient(serverDomain, useSSL, username, password)
	c.isAdmin = true
	return c
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
	if !c.isAuth {
		return "anonymous"
	}
	return c.username
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
	for i := 0; i < MaxConnectionAttemts; i++ {
		wsConnection, err = c.WSConnect.Connect(getWebsocketURL(c.serverDomain, c.useSSL), c.cookies)
		if err == nil {
			// if no error happened, then we can break the loop
			break
		}
	}
	if err != nil {
		log.Printf("Could not connect client %s, %s\n", c, err)
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

// MessageCount returns the number of received messages.
func (c *Client) MessageCount() int {
	c.exceptDataMu.RLock()
	defer c.exceptDataMu.RUnlock()
	return c.messageCount
}

func (c *Client) getLoginData() string {
	return fmt.Sprintf("{\"username\": \"%s\", \"password\": \"%s\"}", c.username, c.password)
}

// Login logsin a client. This sends a login request to the server and saves
// the session cookie for later use.
func (c *Client) Login() (err error) {
	httpClient := &http.Client{
		Jar: c.cookies,
	}
	var resp *http.Response

	for i := 0; i < MaxLoginAttemts; i++ {
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
		if cookie.Name == CSRFCookieName {
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
