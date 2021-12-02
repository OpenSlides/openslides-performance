package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
	"github.com/spf13/cobra"
)

const requestHelp = `Sends a requst to OpenSlides

The request uses the given credentials to log in first.`

func cmdRequest(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:   "request",
		Short: "Sends a logged in request to OpenSlides",
		Long:  requestHelp,
	}

	data := cmd.Flags().StringP("body", "b", "", "HTTP POST data")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx, cancel := interruptContext()
		defer cancel()

		c, err := client.New(cfg.addr())
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}

		if err := c.Login(ctx, cfg.username, cfg.password); err != nil {
			return fmt.Errorf("login client: %w", err)
		}

		method := "GET"
		var body io.Reader
		if *data != "" {
			method = "POST"
			body = strings.NewReader(*data)
		}

		if len(args) == 0 {
			return fmt.Errorf("No url given")
		}

		req, err := http.NewRequestWithContext(ctx, method, args[0], body)
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

	return &cmd
}
