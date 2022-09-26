package connect

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/OpenSlides/openslides-performance/internal/client"
	"github.com/OpenSlides/openslides-performance/internal/config"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

// Run runs the command.
func (o Options) Run(ctx context.Context, cfg config.Config) error {
	if o.Body == "" {
		o.Body = `[{"collection":"organization","ids":[1],"fields":{"committee_ids":{"type":"relation-list","collection":"committee","fields":{"name":null}}}}]`
	}

	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	if err := c.Login(ctx); err != nil {
		return fmt.Errorf("login client: %w", err)
	}

	progress := mpb.New()
	received := make(chan string, 1)

	for i := 0; i < o.Amount; i++ {
		go func(i int) {
			var r io.ReadCloser
			for tries := 0; ; tries++ {
				if tries > 100 {
					return
				}

				skipFirstAttr := ""
				if o.SkipFirst {
					skipFirstAttr = "&skip_first=1"
				}

				r, err = keepOpen(ctx, c, "/system/autoupdate?compress=1"+skipFirstAttr, strings.NewReader(o.Body))
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
				changeID++
				received <- fmt.Sprintf("Change %d", changeID)
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

	cidToBar := make(map[string]*mpb.Bar)

	for {
		select {
		case <-ctx.Done():
			return nil
		case msg := <-received:
			bar, ok := cidToBar[msg]
			if !ok {
				bar = progress.AddBar(
					int64(o.Amount),
					mpb.PrependDecorators(decor.Name(msg)),
					mpb.AppendDecorators(decor.Elapsed(decor.ET_STYLE_GO)),
					mpb.AppendDecorators(decor.CountersNoUnit(" %d/%d")),
				)
				cidToBar[msg] = bar
			}
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
