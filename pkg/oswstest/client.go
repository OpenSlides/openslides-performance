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

	"github.com/gorilla/websocket"
)

var httpProtocol, wsProtocol string

func init() {
	if SSL {
		httpProtocol = "https"
		wsProtocol = "wss"
	} else {
		httpProtocol = "http"
		wsProtocol = "ws"
	}
}

// Client represents one connection to the server
type Client interface {
	Connect() error
	String() string
	IsAdmin() bool
	IsConnected() bool
	SetChannels(read chan []byte, err chan error)
	ClearChannels()
	ExpectData(sinceTime chan<- time.Duration, err chan<- error, count int)
}

// AuthClient is a Client which can login to the server
type AuthClient interface {
	Client
	Login() error
}

// AdminClient is a AuthClient that is an admin on the server
type AdminClient interface {
	AuthClient
	Send() error
}

func getLoginURL() string {
	return fmt.Sprintf(BaseURL, httpProtocol, LoginURLPath)
}

func getWebsocketURL() string {
	return fmt.Sprintf(BaseURL, wsProtocol, WSURLPath)
}

// getSendRequest returns the request that is send by the admin clients
func getSendRequest() (r *http.Request) {
	r, err := http.NewRequest(
		"PUT",
		fmt.Sprintf(BaseURL, "http", "rest/agenda/item/1/"),
		strings.NewReader(`
			{"id":1,"item_number":"","title":"foo1","list_view_title":"foo1",
			"comment":"test","closed":false,"type":1,"is_hidden":false,"duration":null,
			"speaker_list_closed":false,"content_object":{"collection":"topics/topic",
			"id":1},"weight":10000,"parent_id":null,"parentCount":0,"hover":true}`),
	)
	if err != nil {
		log.Fatalf("Coud not build the request, %s", err)
	}
	return r
}

// Client represents one of many openslides users
type client struct {
	username string
	isAuth   bool
	isAdmin  bool

	wsRead  chan []byte
	wsError chan error
	wsMu    sync.Mutex

	wsConnection *websocket.Conn
	cookies      *cookiejar.Jar

	connected       time.Time
	connectionError chan struct{}
	waitForConnect  chan struct{}
}

// NewAnonymousClient creates an anonymous client.
func NewAnonymousClient() Client {
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("Can not create cookie jar, %s\n", err)
	}
	return &client{
		waitForConnect:  make(chan struct{}),
		connectionError: make(chan struct{}),
		cookies:         jar,
	}
}

// NewUserClient creates an user client.
func NewUserClient(username string) AuthClient {
	client := NewAnonymousClient().(*client)
	client.username = username
	client.isAuth = true
	return client
}

// NewAdminClient creates an admin client.
func NewAdminClient(username string) AdminClient {
	client := NewUserClient(username).(*client)
	client.isAdmin = true
	return client
}

// IsAdmin returns True, if the client is an admin client.
func (c *client) IsAdmin() bool {
	return c.isAdmin
}

// IsConnected returns true, if the client has an open websocket connection
func (c *client) IsConnected() bool {
	return !c.connected.IsZero()
}

// String returns the username of the client. `anonymous` if it is an anonymous client.
func (c *client) String() string {
	if !c.isAuth {
		return "anonymous"
	}
	return c.username
}

// Connect creates a websocket connection. It blocks until the connection is
// established.
func (c *client) Connect() (err error) {
	for i := 0; i < MaxConnectionAttemts; i++ {
		dialer := websocket.Dialer{
			Jar: c.cookies,
		}
		c.wsConnection, _, err = dialer.Dial(getWebsocketURL(), nil)
		if err == nil {
			// if no error happend, then we can break the loop
			break
		}
	}
	if err != nil {
		log.Printf("Could not connect client %s, %s\n", c, err)
		close(c.connectionError)
		return err
	}

	// Set the connected time to now and close the waitForConnect channel to signal
	// that the client is now connected.
	c.connected = time.Now()
	close(c.waitForConnect)

	go func() {
		// Write all incomming messages into c.wsRead.
		// Before SetChannel() is called, this channel is nil
		defer c.wsConnection.Close()
		for {
			_, m, err := c.wsConnection.ReadMessage()
			if err != nil {
				c.wsError <- err
				break
			}
			// Send the message to the channel. If no channel is set, then the message ignored
			select {
			case c.wsRead <- m:
			default:
			}
		}
	}()
	return nil
}

// Set the channels to receive data.
func (c *client) SetChannels(read chan []byte, err chan error) {
	c.wsMu.Lock()
	c.wsRead = read
	c.wsError = err
}

func (c *client) ClearChannels() {
	c.wsRead = nil
	c.wsError = nil
	c.wsMu.Unlock()
}

// ExpectData runs, until there are `count` websocket messages or one websocket error.
// It sends the time since the start of this function, but not before the websocket
// connection was established.
func (c *client) ExpectData(sinceTime chan<- time.Duration, err chan<- error, count int) {
	// Wait until the client is connected or the connection has failed
	var start time.Time
	select {
	case <-c.waitForConnect:
		start = time.Now()

	case <-c.connectionError:
		// If the connection faild, then there is nothing to do here.
		return
	}

	// Sets the channels to receive the data
	readChan := make(chan []byte)
	errChan := make(chan error)
	c.SetChannels(readChan, errChan)
	defer c.ClearChannels()

	for i := 0; i < count; i++ {
		select {
		case <-readChan:
			// The clients receives a message

		case data := <-errChan:
			err <- data
			return
		}
	}
	sinceTime <- time.Since(start)
}

func (c *client) getLoginData() string {
	return fmt.Sprintf("{\"username\": \"%s\", \"password\": \"%s\"}", c.username, LoginPassword)
}

// Login logsin a client. This sends a login request to the server and saves
// the session cookie for later use.
func (c *client) Login() (err error) {
	httpClient := &http.Client{
		Jar: c.cookies,
	}
	var resp *http.Response

	for i := 0; i < MaxLoginAttemts; i++ {
		resp, err = httpClient.Post(
			getLoginURL(),
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
func (c *client) Send() (err error) {
	httpClient := &http.Client{
		Jar: c.cookies,
	}
	req := getSendRequest()

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
