package slow

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/OpenSlides/openslides-performance/client"
	"golang.org/x/sync/errgroup"
)

// Run sends the request.
func (o Options) Run(ctx context.Context, cfg client.Config) error {
	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	if err := c.Login(ctx); err != nil {
		return fmt.Errorf("login client: %w", err)
	}

	eg, ctx := errgroup.WithContext(ctx)

	for i := 0; i < o.Amount; i++ {
		eg.Go(func() error {
			for ctx.Err() == nil {
				req, err := http.NewRequestWithContext(ctx, "POST", o.URL.String(), slowRandromReader{})
				if err != nil {
					return fmt.Errorf("creating request: %w", err)
				}

				resp, err := c.Do(req)
				if err != nil {
					return fmt.Errorf("sending request: %w", err)
				}
				defer resp.Body.Close()

				if _, err := io.ReadAll(resp.Body); err != nil {
					return fmt.Errorf("draining body: %w", err)
				}
			}

			return nil
		})
	}

	return eg.Wait()
}

// slowRandomReader is a reader that reads pseudo random data very slow
type slowRandromReader struct{}

func (s slowRandromReader) Read(p []byte) (int, error) {
	if len(p) > 100 {
		p = p[:100]
	}
	time.Sleep(time.Second)
	return rand.Read(p)
}
