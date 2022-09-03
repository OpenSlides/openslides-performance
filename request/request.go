package request

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
)

// Run sends the request.
func (o Options) Run(ctx context.Context, cfg config.Config) error {
	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	if err := c.Login(ctx); err != nil {
		return fmt.Errorf("login client: %w", err)
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

	resp, err := c.Do(req)
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
