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

Creates a lot of topics in one meeting and toggels the done status forever.`

func cmdWork(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:   "work",
		Short: "generates background work",
		Long:  rworkHelp,
	}

	meetingID := cmd.Flags().IntP("meeting", "m", 1, "MeetingID to use")
	amount := cmd.Flags().IntP("amount", "n", 10, "Amount of workers to use.")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx, cancel := interruptContext()
		defer cancel()

		eg, ctx := errgroup.WithContext(ctx)
		for i := 0; i < *amount; i++ {
			eg.Go(func() error {
				return work(ctx, cfg, *meetingID)
			})
		}

		return eg.Wait()
	}

	return &cmd
}

func work(ctx context.Context, cfg *config, meetingID int) (err error) {
	c, err := client.New(cfg.addr())
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	if err := c.Login(ctx, cfg.username, cfg.password); err != nil {
		return fmt.Errorf("login client: %w", err)
	}

	topicID, err := createWorkerTopic(ctx, c, meetingID)
	if err != nil {
		return fmt.Errorf("creating topic: %w", err)
	}
	defer func() {
		deleteErr := deleteWorkerTopic(context.Background(), c, topicID)
		if err == nil && deleteErr != nil {
			err = fmt.Errorf("deleting topic: %w", err)
		}
	}()

	if err := toggleWorkerTopic(ctx, c, topicID); err != nil {
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
