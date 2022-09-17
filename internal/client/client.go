package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/OpenSlides/openslides-performance/internal/config"
)

// Client holds the connection to the OpenSlides server.
type Client struct {
	cfg        config.Config
	httpClient *http.Client

	authCookie *http.Cookie
	authToken  string
	userID     int

	retryEvent func() <-chan struct{}
}

// New initializes a new client.
func New(cfg config.Config) (*Client, error) {
	var dialContext func(ctx context.Context, network, addr string) (net.Conn, error)
	if cfg.IPv4 {
		var zeroDialer net.Dialer
		dialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
			return zeroDialer.DialContext(ctx, "tcp4", addr)
		}
	}

	c := Client{
		cfg: cfg,
		httpClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
				DialContext:     dialContext,
			},
		},
	}

	return &c, nil
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

	return task.Response(), task.Error()
}

// DoTask is like Do, but returns a Task.
func (c *Client) DoTask(req *http.Request) (*Task, error) {
	resp, err := c.DoRaw(req)

	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}

	if resp.StatusCode != 202 {
		c := make(chan struct{})
		close(c)
		return &Task{
			resp: resp,
			done: c,
		}, nil
	}

	return c.backendWorker(req.Context(), resp)
}

func (c *Client) backendWorker(ctx context.Context, resp *http.Response) (*Task, error) {
	var body struct {
		Results [][]struct {
			FQID string `json:"fqid"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode backend response: %w", err)
	}

	outer := body.Results
	if len(outer) == 0 {
		return nil, fmt.Errorf("invalid response, no outer list")
	}

	inner := outer[0]
	if len(inner) == 0 {
		return nil, fmt.Errorf("invalid response, no inner list")
	}

	collection, idStr, found := strings.Cut(inner[0].FQID, "/")
	if !found || collection != "action_worker" {
		return nil, fmt.Errorf("invalid response, wront fqid %s", inner[0].FQID)
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid response, wront id %s", idStr)
	}

	autoUpdateReq, err := http.NewRequestWithContext(
		ctx,
		"GET",
		"/system/autoupdate",
		strings.NewReader(fmt.Sprintf(
			`[{"collection":"action_worker","ids":[%d],"fields":{"state":null,"result":null}}]`,
			id,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	autoUpdateResp, err := checkStatus(c.Do(autoUpdateReq))
	if err != nil {
		return nil, fmt.Errorf("request work from autoupdate: %w", err)
	}

	task := Task{
		done: make(chan struct{}),
	}

	go func() {
		defer autoUpdateReq.Body.Close()

		stateKey := fmt.Sprintf("action_worker/%d/state", id)
		resultKey := fmt.Sprintf("action_worker/%d/result", id)

		var worker map[string]json.RawMessage

		scanner := bufio.NewScanner(autoUpdateResp.Body)
		for scanner.Scan() {
			line := scanner.Bytes()
			task.mu.Lock()
			if err := json.Unmarshal(line, &worker); err != nil {
				task.err = fmt.Errorf("decoding autoupdate response: %w", err)
				task.mu.Unlock()
				return
			}

			switch string(worker[stateKey]) {
			case `"end"`:
				fakeResp := http.Response{
					StatusCode: 200,
					Status:     http.StatusText(200),
					Body:       io.NopCloser(bytes.NewReader(worker[resultKey])),
				}
				task.resp = &fakeResp

			case `"aborted"`:
				task.err = fmt.Errorf("task aborted")

			default:
				task.mu.Unlock()
				continue
			}
			close(task.done)
			task.mu.Unlock()
			return
		}
		if err := scanner.Err(); err != nil {
			task.mu.Lock()
			task.err = fmt.Errorf("scanner failed: %w", err)
			task.mu.Unlock()
		}
	}()

	return &task, nil
}

// Login uses the username and password to login the client. Sets the returned
// cookie for later requests.
func (c *Client) Login(ctx context.Context) error {
	return c.LoginWithCredentials(ctx, c.cfg.Username, c.cfg.Password)
}

// LoginWithCredentials is like Login but uses the given credentials instead of
// config.
func (c *Client) LoginWithCredentials(ctx context.Context, username, password string) error {
	url := c.cfg.Addr() + "/system/auth/login"
	payload := fmt.Sprintf(`{"username": "%s", "password": "%s"}`, c.cfg.Username, c.cfg.Password)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	var resp *http.Response
	for retry := 0; retry < 100; retry++ {
		resp, err = checkStatus(c.httpClient.Do(req))
		if err == nil {
			break
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.cfg.RetryEvent():
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
		return nil, fmt.Errorf("got status %s: %s", resp.Status, body)
	}
	return resp, nil
}
