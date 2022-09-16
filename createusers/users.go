package createusers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
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

	namePrefix := ""
	extraFields := ""
	if o.MeetingID != 0 {
		groupID, err := delegateGroup(ctx, c, o.MeetingID)
		if err != nil {
			return fmt.Errorf("fetching delegated group: %w", err)
		}

		namePrefix = fmt.Sprintf("m%d", o.MeetingID)
		extraFields = fmt.Sprintf(`
				"is_present_in_meeting_ids": [%d],
				"group_$_ids": {"%d":[%d]},
				`,
			o.MeetingID,
			o.MeetingID,
			groupID,
		)
	}

	var users []string
	for i := 0; i < o.Amount; i++ {
		userID := i + 1
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

	return nil
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
