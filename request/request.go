package request

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
)

// Run sends the request.
func (o Options) Run(ctx context.Context, cfg client.Config) error {
	if o.BodyFile != nil {
		bodyFileContent, err := io.ReadAll(o.BodyFile)
		if err != nil {
			return fmt.Errorf("reading body file: %w", err)
		}

		o.Body = append(o.Body, string(bodyFileContent))
	}

	c, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("creating client: %w", err)
	}

	if err := c.Login(ctx); err != nil {
		return fmt.Errorf("login client: %w", err)
	}

	method := "GET"
	var body io.Reader

	boundary := ""
	if len(o.Body) != 0 {
		method = "POST"
		body = strings.NewReader(o.Body[0])
		if len(o.Body) > 1 {
			buf := new(bytes.Buffer)
			mp := multipart.NewWriter(buf)
			for _, body := range o.Body {
				w, err := mp.CreatePart(textproto.MIMEHeader{})
				if err != nil {
					return fmt.Errorf("create multipart part: %w", err)
				}
				if _, err := w.Write([]byte(body)); err != nil {
					return fmt.Errorf("write body part: %w", err)
				}
			}
			mp.Close()
			body = buf
			boundary = mp.Boundary()
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, o.URL.String(), body)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if boundary != "" {
		req.Header.Set("Content-Type", fmt.Sprintf("multipart/mixed; boundary=%s", boundary))
	}

	do := c.Do
	if o.NoBackendWorker {
		do = c.DoRaw
	}

	resp, err := do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(os.Stdout, resp.Body); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil
		}
		return fmt.Errorf("writing response body to stdout: %w", err)
	}
	return nil
}
