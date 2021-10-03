package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Client holds the connection to the OpenSlides server.
type Client struct {
	addr string
	hc   *http.Client

	authCookie *http.Cookie
	authToken  string
}

// New initializes a new client.
func New(addr string) (*Client, error) {
	c := Client{
		addr: addr,
		hc: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}

	return &c, nil
}

// Do is like http.Do but uses the credentials.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("authentication", c.authToken)
	req.Header.Add("cookie", c.authCookie.String())

	if req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", "application/json")
	}

	return checkStatus(c.hc.Do(req))
}

// Addr returns the basis addr the client was initializes with.
func (c *Client) Addr() string {
	return c.addr
}

// Login uses the username and password to login the client. Sets the returned
// cookie for later requests.
func (c *Client) Login(ctx context.Context, username, password string) error {
	url := c.addr + "/system/auth/login"
	payload := fmt.Sprintf(`{"username": "%s", "password": "%s"}`, username, password)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := checkStatus(c.hc.Do(req))
	if err != nil {
		return fmt.Errorf("sending login request: %w", err)
	}
	defer resp.Body.Close()

	c.authToken = resp.Header.Get("authentication")
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "refreshId" {
			c.authCookie = cookie
			break
		}
	}
	return nil
}

// checkStatus is a helper that can be used around http.Do().
//
// It checks, that the returned status code in the 200er range.
func checkStatus(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, fmt.Errorf("sending login request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			body = []byte("[can not read body]")
		}
		resp.Body.Close()
		return nil, fmt.Errorf("got status %s: %s", resp.Status, body)
	}
	return resp, nil
}
