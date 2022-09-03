package vote

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
	"github.com/vbauerster/mpb/v7"
)

// Run runs the command.
func (o Options) Run(ctx context.Context, cfg config.Config) error {
	admin, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}
	if err := admin.Login(ctx); err != nil {
		return fmt.Errorf("login admin: %w", err)
	}

	meetingID, optionID, err := pollData(ctx, admin, o.PollID)
	if err != nil {
		return fmt.Errorf("getting poll data: %w", err)
	}

	clients := make([]*client.Client, o.Amount)
	for i := 0; i < len(clients); i++ {
		c, err := client.New(cfg)
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}
		clients[i] = c
	}

	log.Printf("Login %d clients", len(clients))
	start := time.Now()
	massLogin(ctx, clients, meetingID)
	log.Printf("All clients logged in %v", time.Now().Sub(start))

	first := true

	for first || o.Loop {
		first = false

		if o.Interrupt || o.Loop {
			reader := bufio.NewReader(os.Stdin)
			fmt.Println("Hit enter to continue")
			reader.ReadString('\n')
			log.Println("Starting voting")
		}

		start := time.Now()
		url := "/system/vote"
		massVotes(ctx, clients, url, o.PollID, optionID)
		log.Printf("All Clients have voted in %v", time.Now().Sub(start))
	}

	return nil
}

func pollData(ctx context.Context, client *client.Client, pollID int) (meetingID, optionID int, err error) {
	url := fmt.Sprintf("/system/autoupdate?single=1&k=poll/%d/meeting_id,poll/%d/option_ids", pollID, pollID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("building request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("getting response: %w", err)
	}
	defer resp.Body.Close()

	var data map[string]json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, 0, fmt.Errorf("decoding response body: %w", err)
	}

	rawMeetingID, ok := data[fmt.Sprintf("poll/%d/meeting_id", pollID)]
	if !ok {
		return 0, 0, fmt.Errorf("meeting_id not in response, got %v", dataKeys(data))
	}

	rawOptionIDs, ok := data[fmt.Sprintf("poll/%d/option_ids", pollID)]
	if !ok {
		return 0, 0, fmt.Errorf("option_ids not in response, got %v", dataKeys(data))
	}

	if err := json.Unmarshal(rawMeetingID, &meetingID); err != nil {
		return 0, 0, fmt.Errorf("decoding meeting_id from %q: %w", rawMeetingID, err)
	}

	var optionIDs []int
	if err := json.Unmarshal(rawOptionIDs, &optionIDs); err != nil {
		return 0, 0, fmt.Errorf("decoding meeting_id from %q: %w", rawMeetingID, err)
	}

	if len(optionIDs) != 1 {
		return 0, 0, fmt.Errorf("option_ids is %q, expected one value", rawOptionIDs)
	}

	return meetingID, optionIDs[0], nil
}

func dataKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func massLogin(ctx context.Context, clients []*client.Client, meetingID int) {
	var wgLogin sync.WaitGroup
	progress := mpb.New(mpb.WithWaitGroup(&wgLogin))
	loginBar := progress.AddBar(int64(len(clients)))

	for i := 0; i < len(clients); i++ {
		wgLogin.Add(1)
		go func(i int) {
			defer wgLogin.Done()

			client := clients[i]

			username := fmt.Sprintf("m%ddummy%d", meetingID, i+1)

			if err := client.LoginWithCredentials(ctx, username, "pass"); err != nil {
				log.Printf("Login failed for user %s: %v", username, err)
				return
			}

			loginBar.Increment()
		}(i)
	}
	progress.Wait()
}

func massVotes(ctx context.Context, clients []*client.Client, url string, pollID, optionID int) {
	payload := fmt.Sprintf(`{"value": {"%d": "Y"}}`, optionID)

	var wgVote sync.WaitGroup
	progress := mpb.New(mpb.WithWaitGroup(&wgVote))
	voteBar := progress.AddBar(int64(len(clients)))
	for i := 0; i < len(clients); i++ {
		wgVote.Add(1)
		go func(i int) {
			defer wgVote.Done()
			defer voteBar.Increment()

			client := clients[i]
			req, err := http.NewRequest("POST", fmt.Sprintf("%s?id=%d", url, pollID), strings.NewReader(payload))
			if err != nil {
				log.Printf("Error creating request: %v", err)
				return
			}

			req.Header.Set("Content-Type", "application/json")

			resp, err := client.Do(req)
			if err != nil {
				log.Printf("Error sending vote request to %s for user %d: %v", url, i+1, err)
				return
			}
			defer resp.Body.Close()
			io.ReadAll(resp.Body)

			return
		}(i)
	}
	progress.Wait()
}
