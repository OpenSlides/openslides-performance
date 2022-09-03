package backendaction

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
	"github.com/google/uuid"
)

// Run runs the command.
func (o Options) Run(ctx context.Context, cfg config.Config) error {
	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	if err := c.Login(ctx); err != nil {
		return fmt.Errorf("login client: %w", err)
	}

	if o.Content == "-" {
		stdinContent, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading from stdin: %w", err)
		}
		o.Content = string(stdinContent)
	}

	actions := make([]string, o.Amount)
	for i := 0; i < len(actions); i++ {
		c := o.Content
		c = strings.ReplaceAll(c, `\i`, strconv.Itoa(i+1))
		c = strings.ReplaceAll(c, `\u`, uuid.New().String())

		actions[i] = c
	}

	body := fmt.Sprintf(
		`[{
			"action": "%s",
			"data": [%s]
		}]`,
		o.Action,
		strings.Join(actions, ","),
	)

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"/system/action/handle_request",
		strings.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if _, err := c.Do(req); err != nil {
		return fmt.Errorf("sending request: %w", err)
	}

	return nil
}
