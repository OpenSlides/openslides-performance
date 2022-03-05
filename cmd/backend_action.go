package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

const backendActionHelp = `Call an action in the backend multiple times.

In the action content \i is replaced with a number between 1 and amount.

\u is replaced with a random uuid.

If content is "-", then the content is read from stdin.`

func cmdBackendAction(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:   "backend_action",
		Short: "Calls an backend action multiple times.",
		Long:  backendActionHelp,
	}
	amount := cmd.Flags().IntP("amount", "n", 10, "Amount of action to be called.")
	name := cmd.Flags().StringP("name", "a", "motion.create", "Name of the action")
	content := cmd.Flags().StringP("content", "c", `{"meeting_id":2,"text":"hello world","title":"motion\u"}`, "content of the action")

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

		if *content == "-" {
			stdinContent, err := io.ReadAll(os.Stdin)
			if err != nil {
				return fmt.Errorf("reading from stdin: %w", err)
			}
			*content = string(stdinContent)
		}

		actions := make([]string, *amount)
		for i := 0; i < *amount; i++ {
			c := *content
			c = strings.ReplaceAll(c, `\i`, strconv.Itoa(i+1))
			c = strings.ReplaceAll(c, `\u`, uuid.New().String())

			actions[i] = c
		}

		createBody := fmt.Sprintf(
			`[{
				"action": "%s",
				"data": [%s]
			}]`,
			*name,
			strings.Join(actions, ","),
		)

		req, err := http.NewRequestWithContext(
			ctx,
			"POST",
			"/system/action/handle_request",
			strings.NewReader(createBody),
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

	return &cmd
}
