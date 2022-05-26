package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const rworkHelp = `Generates background work on the server

It uses different strategie to create the load. The strategie can
be set via the argument --strategy which is a string. Possible strategies are:

* topic-done: sets the done status of a topic to true and false
* motion-state: sets the state of a motion to 2 and then resets it`

func cmdWork(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:   "work",
		Short: "generates background work",
		Long:  rworkHelp,
	}

	meetingID := cmd.Flags().IntP("meeting", "m", 1, "MeetingID to use")
	amount := cmd.Flags().IntP("amount", "n", 10, "Amount of workers to use.")
	strategy := cmd.Flags().StringP("strategy", "s", "topic-done", "Strategy for the background tasks")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx, cancel := interruptContext()
		defer cancel()

		eg, ctx := errgroup.WithContext(ctx)
		for i := 0; i < *amount; i++ {
			eg.Go(func() error {
				c, err := client.New(cfg.addr())
				if err != nil {
					return fmt.Errorf("creating client: %w", err)
				}

				if err := c.Login(ctx, cfg.username, cfg.password); err != nil {
					return fmt.Errorf("login client: %w", err)
				}

				f := topicDone
				switch *strategy {
				case "topic-done":
					f = topicDone
				case "motion-state":
					f = motionState
				}

				return f(ctx, c, *meetingID)
			})
		}

		return eg.Wait()
	}

	return &cmd
}

func motionState(ctx context.Context, client *client.Client, meetingID int) (err error) {
	motionID, err := createWorkerMotion(ctx, client, meetingID)
	if err != nil {
		return fmt.Errorf("creating motion: %w", err)
	}

	defer func() {
		deleteErr := deleteWorkerMotion(context.Background(), client, motionID)
		if err == nil && deleteErr != nil {
			err = fmt.Errorf("deleting motion: %w", err)
		}
	}()

	if err := toggleWorkerMotion(ctx, client, motionID); err != nil {
		return fmt.Errorf("toggle motion: %w", err)
	}

	return nil
}

func createWorkerMotion(ctx context.Context, client *client.Client, meetingID int) (int, error) {
	body := fmt.Sprintf(
		`[{"action":"motion.create","data":[{"meeting_id":%d,"title":"worker-motion","text":"<p>dummy</p>","workflow_id":1}]}]`,
		meetingID,
	)

	var respBody struct {
		Success bool `json:"success"`
		Results [][]struct {
			MotionID int `json:"id"`
		} `json:"results"`
	}

	if err := backendAction(ctx, client, body, &respBody); err != nil {
		return 0, fmt.Errorf("sending action to backend: %w", err)
	}

	if !respBody.Success {
		return 0, fmt.Errorf("backend returned no success")
	}

	return respBody.Results[0][0].MotionID, nil
}

func toggleWorkerMotion(ctx context.Context, client *client.Client, motionID int) error {
	toggleState := false
	for {
		body := fmt.Sprintf(
			`[{"action":"motion.set_state","data":[{"id":%d,"state_id":2}]}]`,
			motionID,
		)
		if toggleState {
			body = fmt.Sprintf(
				`[{"action":"motion.reset_state","data":[{"id":%d}]}]`,
				motionID,
			)
		}

		var respBody struct {
			Success bool `json:"success"`
		}

		if err := backendAction(ctx, client, body, &respBody); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("sending action to backend: %w", err)
		}

		if !respBody.Success {
			return fmt.Errorf("backend returned no success")
		}

		toggleState = !toggleState
	}
}

func deleteWorkerMotion(ctx context.Context, client *client.Client, motionID int) error {
	body := fmt.Sprintf(
		`[{"action":"motion.delete","data":[{"id":%d}]}]`,
		motionID,
	)

	var respBody struct {
		Success bool `json:"success"`
	}

	if err := backendAction(ctx, client, body, &respBody); err != nil {
		return fmt.Errorf("sending action to backend: %w", err)
	}

	if !respBody.Success {
		return fmt.Errorf("backend returned no success")
	}

	return nil
}

func topicDone(ctx context.Context, client *client.Client, meetingID int) (err error) {
	topicID, err := createWorkerTopic(ctx, client, meetingID)
	if err != nil {
		return fmt.Errorf("creating topic: %w", err)
	}

	defer func() {
		deleteErr := deleteWorkerTopic(context.Background(), client, topicID)
		if err == nil && deleteErr != nil {
			err = fmt.Errorf("deleting topic: %w", err)
		}
	}()

	if err := toggleWorkerTopic(ctx, client, topicID); err != nil {
		return fmt.Errorf("toggle topic: %w", err)
	}

	return nil
}

func createWorkerTopic(ctx context.Context, client *client.Client, meetingID int) (topicID int, err error) {
	body := fmt.Sprintf(
		`[{"action":"topic.create","data":[{"meeting_id":%d,"title":"woker-topic"}]}]`,
		meetingID,
	)

	var respBody struct {
		Success bool `json:"success"`
		Results [][]struct {
			TopicID int `json:"id"`
		} `json:"results"`
	}

	if err := backendAction(ctx, client, body, &respBody); err != nil {
		return 0, fmt.Errorf("sending action to backend: %w", err)
	}

	if !respBody.Success {
		return 0, fmt.Errorf("backend returned no success")
	}

	return respBody.Results[0][0].TopicID, nil
}

func toggleWorkerTopic(ctx context.Context, client *client.Client, topicID int) error {
	toState := "true"
	for {
		body := fmt.Sprintf(
			`[{"action":"agenda_item.update","data":[{"id":%d,"closed":%s}]}]`,
			topicID,
			toState,
		)

		var respBody struct {
			Success bool `json:"success"`
		}

		if err := backendAction(ctx, client, body, &respBody); err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			return fmt.Errorf("sending action to backend: %w", err)
		}

		if !respBody.Success {
			return fmt.Errorf("backend returned no success")
		}

		if toState == "true" {
			toState = "false"
		} else {
			toState = "true"
		}
	}
}

func deleteWorkerTopic(ctx context.Context, client *client.Client, topicID int) error {
	body := fmt.Sprintf(
		`[{"action":"topic.delete","data":[{"id":%d}]}]`,
		topicID,
	)

	var respBody struct {
		Success bool `json:"success"`
	}

	if err := backendAction(ctx, client, body, &respBody); err != nil {
		return fmt.Errorf("sending action to backend: %w", err)
	}

	if !respBody.Success {
		return fmt.Errorf("backend returned no success")
	}

	return nil
}

func backendAction(ctx context.Context, client *client.Client, reqBody string, respBody any) error {
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"/system/action/handle_request",
		strings.NewReader(reqBody),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return fmt.Errorf("decoding body: %w", err)
	}

	return nil
}
