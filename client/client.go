package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/quic-go/quic-go/http3"
	"nhooyr.io/websocket"
)

// Client holds the connection to the OpenSlides server.
type Client struct {
	cfg        Config
	httpClient *http.Client

	authCookie *http.Cookie
	authToken  string
	userID     int
}

// New initializes a new client.
func New(cfg Config) (*Client, error) {
	var dialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	if cfg.IPv4 {
		var zeroDialer net.Dialer
		dialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return zeroDialer.DialContext(ctx, "tcp4", addr)
		}
	}

	var transport http.RoundTripper = &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DialContext: dialContext,
	}

	if cfg.HTTP3 {
		transport = &http3.RoundTripper{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			// TODO: respect the IPv4 config
		}
	}

	c := Client{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: transport,
		},
	}

	return &c, nil
}

// Do is like http.Do but uses the credentials.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	task, err := c.DoTask(req)
	if err != nil {
		return nil, err
	}

	select {
	case <-task.Done():
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}

	return task.Result()
}

// DoRaw is like Do but without backand worker.
func (c *Client) DoRaw(req *http.Request) (*http.Response, error) {
	req.Header.Set("authentication", c.authToken)
	req.Header.Add("cookie", c.authCookie.String())

	if req.Header.Get("content-type") == "" {
		req.Header.Set("content-type", "application/json")
	}

	if req.URL.Host == "" {
		base, err := url.Parse(c.cfg.Addr())
		if err != nil {
			return nil, fmt.Errorf("parsing base url: %w", err)
		}

		req.URL = base.ResolveReference(req.URL)
	}

	return checkStatus(c.httpClient.Do(req))
}

// DoTask is like Do, but returns a Task.
func (c *Client) DoTask(req *http.Request) (*Task, error) {
	resp, err := c.DoRaw(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	if resp.StatusCode == 202 {
		return c.backendWorker(req.Context(), resp)
	}

	closedCh := make(chan struct{})
	close(closedCh)
	return &Task{
		resp: resp,
		done: closedCh,
	}, nil
}

// Dial makes a websocket connection.
func (c *Client) Dial(ctx context.Context, rawURL string) (*websocket.Conn, *http.Response, error) {
	url, err := url.Parse(rawURL)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing rawURL: %w", err)
	}

	if url.Host == "" {
		base, err := url.Parse(c.cfg.Addr())
		if err != nil {
			return nil, nil, fmt.Errorf("parsing base url: %w", err)
		}

		url = base.ResolveReference(url)
	}

	header := http.Header{}
	header.Set("authentication", c.authToken)
	header.Add("cookie", c.authCookie.String())

	return websocket.Dial(ctx, url.String(), &websocket.DialOptions{
		HTTPClient: c.httpClient,
		HTTPHeader: header,
	})
}

func (c *Client) backendWorker(ctx context.Context, resp *http.Response) (*Task, error) {
	awID, err := actionWorkerID(resp)
	if err != nil {
		return nil, fmt.Errorf("unpacking action worker id: %w", err)
	}

	autoUpdateResp, err := c.sendAutoupdate(ctx, awID)
	if err != nil {
		return nil, fmt.Errorf("requesting action worker from autoupdate: %w", err)
	}

	task := Task{
		done: make(chan struct{}),
	}

	go func() {
		task.setDone(parseAutoupdate(awID, autoUpdateResp))
		autoUpdateResp.Body.Close()
	}()

	return &task, nil
}

// actionWorkerID gets the id of the action_worker from a backend response.
//
// The backend packs this in a very strange way.
func actionWorkerID(resp *http.Response) (int, error) {
	var body struct {
		Results [][]struct {
			FQID string `json:"fqid"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("decode backend response: %w", err)
	}

	outer := body.Results
	if len(outer) == 0 {
		return 0, fmt.Errorf("invalid response, no outer list")
	}

	inner := outer[0]
	if len(inner) == 0 {
		return 0, fmt.Errorf("invalid response, no inner list")
	}

	collection, idStr, found := strings.Cut(inner[0].FQID, "/")
	if !found || collection != "action_worker" {
		return 0, fmt.Errorf("invalid response, wront fqid %s", inner[0].FQID)
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, fmt.Errorf("invalid response, wront id %s", idStr)
	}

	return id, nil
}

// sendAutoupdate requests an action worker from the autoupdate service.
func (c *Client) sendAutoupdate(ctx context.Context, awID int) (*http.Response, error) {
	autoUpdateReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"/system/autoupdate",
		strings.NewReader(fmt.Sprintf(
			`[{"collection":"action_worker","ids":[%d],"fields":{"state":null,"result":null}}]`,
			awID,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	autoUpdateResp, err := checkStatus(c.Do(autoUpdateReq))
	if err != nil {
		return nil, fmt.Errorf("request action worker from autoupdate: %w", err)
	}

	return autoUpdateResp, nil
}

// parseAutoupdate parses the response from the autoupdate.
//
// blocks until the autoupdate either sends the status 'end' or 'aborted'.
func parseAutoupdate(actionWorkerID int, autoupdateResp *http.Response) (*http.Response, error) {
	stateKey := fmt.Sprintf("action_worker/%d/state", actionWorkerID)
	resultKey := fmt.Sprintf("action_worker/%d/result", actionWorkerID)

	var worker map[string]json.RawMessage

	scanner := bufio.NewScanner(autoupdateResp.Body)
	for scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &worker); err != nil {
			return nil, fmt.Errorf("decoding autoupdate response: %w", err)
		}

		switch string(worker[stateKey]) {
		case `"end"`:
			fakeResp := http.Response{
				StatusCode: 200,
				Status:     http.StatusText(200),
				Body:       io.NopCloser(bytes.NewReader(worker[resultKey])),
			}
			return &fakeResp, nil

		case `"aborted"`:
			return nil, ErrAbborted
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner failed: %w", err)
	}

	return nil, fmt.Errorf("autoupdate connection was broken")
}

// Login uses the username and password to login the client. Sets the returned
// cookie for later requests.
func (c *Client) Login(ctx context.Context) error {
	return c.LoginWithCredentials(ctx, c.cfg.Username, c.cfg.Password)
}

// LoginWithCredentials is like Login but uses the given credentials instead of
// config.
func (c *Client) LoginWithCredentials(ctx context.Context, username, password string) error {
	if c.cfg.FakeAuth {
		c.userID = 1
		return nil
	}

	url := c.cfg.Addr() + "/system/auth/login"
	payload := fmt.Sprintf(`{"username": "%s", "password": "%s"}`, username, password)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	for retry := 0; retry < 100; retry++ {
		resp, err = checkStatus(c.httpClient.Do(req))
		// TODO: Show some sort of process in case of timeout

		var errStatus HTTPStatusError
		if errors.As(err, &errStatus) && errStatus.StatusCode == 403 || err == nil {
			break
		}

		select {
		case <-c.cfg.RetryEvent():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err != nil {
		return fmt.Errorf("sending login request: %w", err)
	}
	defer resp.Body.Close()
	io.ReadAll(resp.Body)

	c.authToken = resp.Header.Get("authentication")
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "refreshId" {
			c.authCookie = cookie
			break
		}
	}

	id, err := decodeUserID(c.authToken)
	if err != nil {
		return fmt.Errorf("decoding user id from auth token: %w", err)
	}

	c.userID = id
	return nil
}

// decodeUserID returns the user id from a jwt token.
//
// It does not validate the token.
func decodeUserID(token string) (int, error) {
	parts := strings.Split(token, ".")
	encoded, err := base64.RawStdEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, fmt.Errorf("decoding jtw token %q: %w", parts[1], err)
	}

	var data struct {
		UserID int `json:"userId"`
	}
	if err := json.Unmarshal(encoded, &data); err != nil {
		return 0, fmt.Errorf("decoding user_id: %w", err)
	}

	return data.UserID, nil
}

// UserID returns the userID of the client.
func (c *Client) UserID() int {
	return c.userID
}

// checkStatus is a helper that can be used around http.Do().
//
// It checks, that the returned status code in the 200er range.
func checkStatus(resp *http.Response, err error) (*http.Response, error) {
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			body = []byte("[can not read body]")
		}
		resp.Body.Close()
		return nil, HTTPStatusError{StatusCode: resp.StatusCode, Body: body}
	}
	return resp, nil
}

// HTTPStatusError is returned, when the http status of a client request is
// something else then in the 200er.
type HTTPStatusError struct {
	StatusCode int
	Body       []byte
}

func (err HTTPStatusError) Error() string {
	return fmt.Sprintf("got status %d %s: %s", err.StatusCode, http.StatusText(err.StatusCode), err.Body)
}
