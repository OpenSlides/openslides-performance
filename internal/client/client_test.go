package client_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
)

func TestLogin(t *testing.T) {
	ctx := context.Background()
	fakeServer := newServerSub()
	ts := httptest.NewServer(fakeServer)
	event := make(chan struct{})
	close(event)

	c, err := client.New(config.Config{
		Domain:             ts.URL,
		RetryEventProvider: func() <-chan struct{} { return event },
	})
	if err != nil {
		t.Fatalf("client.New(): %v", err)
	}

	if err := c.Login(ctx); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if c.UserID() != 42 {
		t.Errorf("Got userid %d, expected 42", c.UserID())
	}
}

func TestBackendActionWorker(t *testing.T) {
	ctx := context.Background()
	fakeServer := newServerSub()
	ts := httptest.NewServer(fakeServer)
	c, err := client.New(config.Config{
		Domain: ts.URL,
	})
	if err != nil {
		t.Fatalf("client.New(): %v", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"/system/action/handle_request",
		strings.NewReader("fake-body"),
	)
	if err != nil {
		t.Fatalf("creating request: %v", err)
	}

	fakeServer.backendReturnStatus = 202
	fakeServer.backendReturnBody = `{"results":[[{"fqid":"action_worker/1"}]]}`
	messages := make(chan string)
	go func() {
		messages <- `{"action_worker/1/state":"running"}`
		messages <- `{"action_worker/1/state":"end","action_worker/1/result":"autoupdate success"}`
		close(messages)
	}()
	fakeServer.autoupdateMessages = messages

	res, err := c.Do(req)
	if err != nil {
		t.Fatalf("sending request: %v", err)
	}

	body, _ := io.ReadAll(res.Body)

	if string(body) != `"autoupdate success"` {
		t.Errorf("got body `%s`, expected `autoupdate success`", body)
	}
}
