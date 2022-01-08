package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"
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
	createUserAmount := cmd.Flags().IntP("amount", "n", 10, "Amount of users to create.")
	meetingID := cmd.Flags().IntP("meeting", "m", 0, "If set, put the user in the delegated group of this meeting.")
	batch := cmd.Flags().IntP("batch", "b", 0, "Number of users to create with one request. Default is all at once.")

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

		namePrefix := ""
		extraFields := ""
		if *meetingID != 0 {
			groupID, err := delegateGroup(ctx, c, *meetingID)
			if err != nil {
				return fmt.Errorf("fetching delegated group: %w", err)
			}

			namePrefix = fmt.Sprintf("m%d", *meetingID)
			extraFields = fmt.Sprintf(`
				"is_present_in_meeting_ids": [%d],
				"group_$_ids": {"%d":[%d]},
				`,
				*meetingID,
				*meetingID,
				groupID,
			)
		}

		if *batch == 0 {
			*batch = *createUserAmount
		}

		batchCount := *createUserAmount / *batch

		progress := mpb.New()
		userBar := progress.AddBar(int64(*createUserAmount))

		for b := 0; b < batchCount; b++ {
			// TODO: Fix case that createUserAmout is not a multiple of batch
			var users []string
			for i := 0; i < *batch; i++ {
				userID := b*(*batch) + i + 1
				users = append(users, fmt.Sprintf(
					`{
					"username": "%sdummy%d",
					"default_password": "pass",
					%s
					"is_active":true
				}`,
					namePrefix,
					userID,
					extraFields,
				))
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
			userBar.IncrBy(*batch)
		}

		return nil
	}

	return &cmd
}

func delegateGroup(ctx context.Context, c *client.Client, meetingID int) (int, error) {
	url := "/system/autoupdate?single=1"
	body := fmt.Sprintf(`[{
			"collection": "meeting",
			"ids": [%d],
			"fields":{
				"group_ids": {
					"type": "relation-list",
					"collection": "group",
					"fields": {
						"name": null
					}
				}
			}
		}]`,
		meetingID,
	)
	req, err := http.NewRequestWithContext(ctx, "GET", url, strings.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("building request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sending get group request: %w", err)
	}
	defer resp.Body.Close()

	var keys map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&keys); err != nil {
		return 0, fmt.Errorf("parsing response body: %w", err)
	}

	for k, v := range keys {
		if string(v) != `"Delegates"` {
			continue
		}
		parts := strings.Split(k, "/")
		id, err := strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("decoding group id: %w", err)
		}
		return id, nil
	}
	return 0, fmt.Errorf("can not find group Delegates in meeting %d", meetingID)
}
