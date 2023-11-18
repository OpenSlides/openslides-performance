package request

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
	"nhooyr.io/websocket"
)

// Run sends the request.
func (o Options) Run(ctx context.Context, cfg client.Config) error {
	if o.BodyFile != nil {
		bodyFileContent, err := io.ReadAll(o.BodyFile)
		if err != nil {
			return fmt.Errorf("reading body file: %w", err)
		}

		o.Body = string(bodyFileContent)
	}

	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	if err := c.Login(ctx); err != nil {
		return fmt.Errorf("login client: %w", err)
	}

	if o.Websocket {
		return o.doWebsocketStuff(ctx, c)
	}

	method := "GET"
	var body io.Reader
	if o.Body != "" {
		method = "POST"
		body = strings.NewReader(o.Body)
	}

	req, err := http.NewRequestWithContext(ctx, method, o.URL.String(), body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	do := c.Do
	if o.NoBackendWorker {
		do = c.DoRaw
	}

	resp, err := do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		return fmt.Errorf("writing response body to stdout: %w", err)
	}
	return nil
}

func (o Options) doWebsocketStuff(ctx context.Context, c *client.Client) error {
	conn, _, err := c.Dial(ctx, o.URL.String())
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer conn.CloseNow()

	conn.SetReadLimit(-1)

	if err := conn.Write(ctx, websocket.MessageText, []byte(o.Body)); err != nil {
		return fmt.Errorf("write body: %w", err)
	}

	for {
		_, reader, err := conn.Reader(ctx)
		if err != nil {
			return fmt.Errorf("start reading: %w", err)
		}

		if _, err := io.Copy(os.Stdout, reader); err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil
			}
			return fmt.Errorf("writing response body to stdout: %w", err)
		}
	}
}
