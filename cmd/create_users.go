package cmd

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/OpenSlides/openslides-performance/client"
	"github.com/spf13/cobra"
)

const createUsersHelp = `Creates many users

This command does not run any test. It is a helper for other tests that
require, that there a many users created at the server.

Do not run this command against a productive instance. It will change
the database.

Each user is called dummy1, dummy2 etc and has the password "pass".`

func cmdCreateUsers(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:   "create_users",
		Short: "Create a lot of users.",
		Long:  createUsersHelp,
	}
	createUserAmount := cmd.Flags().IntP("amount", "a", 10, "Amount of users to create.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		c, err := client.New(cfg.addr())
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}

		if err := c.Login(ctx, cfg.username, cfg.password); err != nil {
			return fmt.Errorf("login client: %w", err)
		}

		var users []string
		for i := 0; i < *createUserAmount; i++ {
			users = append(users, fmt.Sprintf(`{"username":"dummy%d","default_password":"pass","is_active":true}`, i+1))
		}

		createBody := fmt.Sprintf(
			`[{
				"action": "user.create",
				"data": [%s]
			}]`,
			strings.Join(users, ","),
		)

		req, err := http.NewRequestWithContext(
			ctx,
			"POST",
			cfg.addr()+"/system/action/handle_request",
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
