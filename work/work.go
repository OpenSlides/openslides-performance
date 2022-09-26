package work

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
	"golang.org/x/sync/errgroup"
)

// Run runs the command.
func (o Options) Run(ctx context.Context, cfg config.Config) error {
	workFunc := topicDone
	switch o.Strategy {
	case "topic-done":
		workFunc = topicDone
	case "motion-state":
		workFunc = motionState
	}

	eg, ctx := errgroup.WithContext(ctx)
	for i := 0; i < o.Amount; i++ {
		eg.Go(func() error {
			cli, err := client.New(cfg)
			if err != nil {
				return fmt.Errorf("creating client: %w", err)
			}

			if err := cli.Login(ctx); err != nil {
				return fmt.Errorf("login client: %w", err)
			}

			return workFunc(ctx, cli, o.MeetingID)
		})
	}

	return eg.Wait()
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

	aid, err := agendaID(ctx, client, topicID)
	if err != nil {
		return fmt.Errorf("fetching agenda id for topic %d: %w", topicID, err)
	}

	defer func() {
		deleteErr := deleteWorkerTopic(context.Background(), client, topicID)
		if err == nil && deleteErr != nil {
			err = fmt.Errorf("deleting topic: %w", err)
		}
	}()

	if err := toggleWorkerAgendaItem(ctx, client, aid); err != nil {
		return fmt.Errorf("toggle agenda item: %w", err)
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

func agendaID(ctx context.Context, client *client.Client, topicID int) (int, error) {
	key := fmt.Sprintf("topic/%d/agenda_item_id", topicID)

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("/system/autoupdate?k=%s&single=1", key),
		nil,
	)
	if err != nil {
		return 0, fmt.Errorf("creating request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	var body map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("decoding body: %w", err)
	}

	val, ok := body[key]
	if !ok {
		return 0, fmt.Errorf("topic %d does not exist", topicID)
	}

	var agendaID int
	if err := json.Unmarshal(val, &agendaID); err != nil {
		return 0, fmt.Errorf("decoding agenda id: %w", err)
	}

	return agendaID, nil

}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func toggleWorkerAgendaItem(ctx context.Context, client *client.Client, topicID int) error {
	toState := true
	for {
		body := fmt.Sprintf(
			`[{"action":"agenda_item.update","data":[{"id":%d,"closed":%s}]}]`,
			topicID,
			boolToStr(toState),
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

		toState = !toState
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
