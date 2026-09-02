package vote

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/OpenSlides/openslides-performance/client"
	"github.com/vbauerster/mpb/v7"
)

// Run runs the command.
func (o Options) Run(ctx context.Context, cfg client.Config) error {
	admin, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}
	if err := admin.Login(ctx); err != nil {
		return fmt.Errorf("login admin: %w", err)
	}

	clients := make([]*client.Client, o.Amount)
	for i := range len(clients) {
		c, err := client.New(cfg)
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}
		clients[i] = c
	}

	log.Printf("Login %d clients", len(clients))
	start := time.Now()
	MassLogin(ctx, clients, o.BaseName, o.UsersPassword)
	log.Printf("All clients logged in %v", time.Now().Sub(start))

	parallel := len(clients)
	if o.Parallel != 0 {
		parallel = o.Parallel
	}

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
		url := fmt.Sprintf("/system/vote/poll/%d/vote", o.PollID)
		if err := massVotes(clients, url, parallel); err != nil {
			return fmt.Errorf("mass vote: %w", err)
		}
		log.Printf("All Clients have voted in %v", time.Now().Sub(start))
	}

	return nil
}

// MassLogin logs in a list of clients.
func MassLogin(ctx context.Context, clients []*client.Client, basename string, password string) {
	var wgLogin sync.WaitGroup
	progress := mpb.New(mpb.WithWaitGroup(&wgLogin))
	loginBar := progress.AddBar(int64(len(clients)))

	for i := range len(clients) {
		wgLogin.Add(1)
		go func(i int) {
			defer wgLogin.Done()

			client := clients[i]

			username := fmt.Sprintf("%s%d", basename, i+1)

			if err := client.LoginWithCredentials(ctx, username, password); err != nil {
				log.Printf("Login failed for user %s: %v", username, err)
				return
			}

			loginBar.Increment()
		}(i)
	}
	progress.Wait()
}

func massVotes(clients []*client.Client, url string, parallel int) error {
	payload := `{"value": "yes"}`

	var wgVote sync.WaitGroup
	progress := mpb.New(mpb.WithWaitGroup(&wgVote))
	voteBar := progress.AddBar(int64(len(clients)))
	lock := make(chan struct{}, parallel)
	for i := range len(clients) {
		lock <- struct{}{}
		wgVote.Add(1)
		go func(i int) {
			defer wgVote.Done()
			defer voteBar.Increment()
			defer func() { <-lock }()

			client := clients[i]
			req, err := http.NewRequest("POST", url, strings.NewReader(payload))
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
	return nil
}
