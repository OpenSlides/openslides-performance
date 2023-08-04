package connect

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/OpenSlides/openslides-performance/client"
	"github.com/OpenSlides/openslides-performance/vote"
	"github.com/eiannone/keyboard"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

// Run runs the command.
func (o Options) Run(ctx context.Context, cfg client.Config) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if o.Body != "" && o.BodyFile != nil {
		return fmt.Errorf("--body and --body-file are set at the same time. Only one is allowed")
	}

	body := `[{"collection":"organization","ids":[1],"fields":{"committee_ids":{"type":"relation-list","collection":"committee","fields":{"name":null}}}}]`
	if o.BodyFile != nil {
		bodyFileContent, err := io.ReadAll(o.BodyFile)
		if err != nil {
			return fmt.Errorf("reading body file: %w", err)
		}

		body = string(bodyFileContent)
	}

	if o.Body != "" {
		body = o.Body
	}

	var clients []*client.Client

	if o.MuliUserMeeting == -1 {
		c, err := client.New(cfg)
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}

		if err := c.Login(ctx); err != nil {
			return fmt.Errorf("login client: %w", err)
		}

		clients = []*client.Client{c}
	} else {
		clients = make([]*client.Client, o.Amount)
		for i := 0; i < len(clients); i++ {
			c, err := client.New(cfg)
			if err != nil {
				return fmt.Errorf("creating client: %w", err)
			}
			clients[i] = c
		}

		fmt.Println("login clients")
		vote.MassLogin(ctx, clients, o.MuliUserMeeting)
	}

	actionCh := make(chan struct{})
	if o.Action != nil {
		content, err := io.ReadAll(o.Action)
		if err != nil {
			return fmt.Errorf("reading action file: %w", err)
		}

		c, err := client.New(cfg)
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}

		if err := c.Login(ctx); err != nil {
			return fmt.Errorf("login client: %w", err)
		}

		go func() {
			listenForAction(ctx, c, content, actionCh)
			cancel()
		}()
	}

	progress := mpb.New()
	received := make(chan int, 1)

	for i := 0; i < o.Amount; i++ {
		go func(i int) {
			client := clients[0]
			if o.MuliUserMeeting != -1 {
				client = clients[i]
			}
			var r io.ReadCloser
			for tries := 0; ; tries++ {
				if tries > 100 {
					return
				}

				skipFirstAttr := ""
				if o.SkipFirst {
					skipFirstAttr = "&skip_first=1"
				}

				var err error
				r, err = keepOpen(ctx, client, "/system/autoupdate?compress=1"+skipFirstAttr, strings.NewReader(body))
				if err != nil {
					if ctx.Err() != nil {
						return
					}

					log.Printf("Can not send request %d: %v", i, err)
					time.Sleep(time.Second)
					continue
				}
				break
			}
			defer r.Close()

			scanner := bufio.NewScanner(r)
			const MB = 1 << 20
			scanner.Buffer(make([]byte, 10), 16*MB)

			changeID := 0
			for scanner.Scan() {
				received <- changeID
				changeID++
			}

			if err := scanner.Err(); err != nil {
				if errors.Is(err, context.Canceled) {
					return
				}
				log.Printf("Can not read body: %v", err)
				return
			}
		}(i)
	}

	var bars []*mpb.Bar

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-actionCh:
			bar := progress.AddBar(
				int64(o.Amount),
				mpb.PrependDecorators(decor.Name(fmt.Sprintf("change %d", len(bars)+1))),
				mpb.AppendDecorators(decor.Elapsed(decor.ET_STYLE_GO)),
				mpb.AppendDecorators(decor.CountersNoUnit(" %d/%d")),
			)
			bars = append(bars, bar)

		case barID := <-received:
			for len(bars) < barID+1 {
				bar := progress.AddBar(
					int64(o.Amount),
					mpb.PrependDecorators(decor.Name(fmt.Sprintf("change %d", barID+1))),
					mpb.AppendDecorators(decor.Elapsed(decor.ET_STYLE_GO)),
					mpb.AppendDecorators(decor.CountersNoUnit(" %d/%d")),
				)
				bars = append(bars, bar)
			}

			bar := bars[barID]
			bar.Increment()
		}
	}
}

func keepOpen(ctx context.Context, c *client.Client, path string, body io.Reader) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", path, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request to %s: %w", path, err)
	}
	return resp.Body, nil
}

func listenForAction(ctx context.Context, cli *client.Client, body []byte, actionCh chan struct{}) {
	if err := keyboard.Open(); err != nil {
		log.Printf("open keyboard: %v", err)
		return
	}
	defer keyboard.Close()

	for {
		_, key, err := keyboard.GetKey()
		if err != nil {
			log.Printf("listening on keyboard: %v", err)
			return
		}

		if key == keyboard.KeyCtrlC {
			return
		}

		if key != keyboard.KeyEnter {
			continue
		}

		if err := sendAction(ctx, cli, body, actionCh); err != nil {
			log.Printf("sending action: %v", err)
			return
		}
	}
}

func sendAction(ctx context.Context, cli *client.Client, body []byte, actionCh chan struct{}) error {
	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"/system/action/handle_request",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if _, err := cli.Do(req); err != nil {
		return fmt.Errorf("sending request: %w", err)
	}

	actionCh <- struct{}{}
	return nil
}
