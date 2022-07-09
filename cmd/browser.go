package cmd

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/OpenSlides/openslides-performance/client"
	"github.com/spf13/cobra"
)

const browserHelp = `Simulates a browser

The browser command consts of two sub commands. 'record' and 'replay'.

'record' opens a local proxy. All requests to openslides are printed to 
stdout. To do so, a self singed certificate is created.

'replay' creates connections to OpenSlides by reading them from stdin.
Each connection is created many times either with the same user or with different users.
The user defined with -u and -p is used, even if there are login requests.

Both commands can be used together. In this case a click in the (real) browser
is sent to OpenSlides many times:

openslides-performance browser record | openslides-performance replay
`

func cmdBrowser(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:   "browser",
		Short: "Simulates a browser",
		Long:  browserHelp,
	}

	cmd.AddCommand(
		cmdBrowserRecord(cfg),
		cmdBrowserReplay(cfg),
	)

	return &cmd
}

const browserRecordHelp = `Creates a local proxy and prints all commands to stdout.

See openslides-performance browser --help for more information.
`

func cmdBrowserRecord(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:   "record",
		Short: "Creates a proxy and pastes all requests",
		Long:  browserRecordHelp,
	}

	port := cmd.Flags().Int("port", 8080, "Port to use for the proxy")

	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		ctx, cancel := interruptContext()
		defer cancel()

		return startProxy(ctx, cfg.addr(), *port)
	}

	return &cmd
}

const browserReplayHelp = `Replays connections controlled by stdin.

See openslides-performance browser --help for more information.
`

func cmdBrowserReplay(cfg *config) *cobra.Command {
	cmd := cobra.Command{
		Use:   "replay",
		Short: "Replays connections",
		Long:  browserReplayHelp,
	}

	cmd.RunE = func(cmd *cobra.Command, arg []string) error {
		ctx, cancel := interruptContext()
		defer cancel()

		cli, err := client.New(cfg.addr(), false)
		if err != nil {
			return fmt.Errorf("creating client: %w", err)
		}

		if err := cli.Login(ctx, cfg.username, cfg.password); err != nil {
			return fmt.Errorf("login client: %w", err)
		}

		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, prefix) {
				continue
			}

			line = strings.TrimPrefix(line, prefix)
			line = strings.TrimSpace(line)

			parts := strings.Split(line, " ")
			method := parts[0]
			url := parts[1]
			var body io.Reader
			if len(parts) > 2 {
				body = strings.NewReader(parts[2])
			}

			req, err := http.NewRequestWithContext(ctx, method, url, body)
			if err != nil {
				return fmt.Errorf("creating request: %w", err)
			}

			resp, err := cli.Do(req)
			if err != nil {
				return fmt.Errorf("sending request: %w", err)
			}

			// TODO: keep connections open
			//io.ReadAll(resp.Body)
			resp.Body.Close()
		}
		if err := scanner.Err(); err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
		return nil
	}

	return &cmd
}

const prefix = "request:"

func startProxy(ctx context.Context, remoteAddr string, localPort int) error {
	target, err := url.Parse(remoteAddr)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	proxy.FlushInterval = -1
	origDirector := proxy.Director
	proxy.Director = func(r *http.Request) {
		origDirector(r)
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(r.Body)
			r.Body = ioutil.NopCloser(bytes.NewReader(body))
			dst := new(bytes.Buffer)
			if err := json.Compact(dst, body); err == nil {
				body = dst.Bytes()
			}
		}
		if bytes.Contains(body, []byte("\n")) {
			fmt.Printf("request with newline: %s", r.URL.RequestURI())
			return
		}
		fmt.Printf("%s %s %s %s\n", prefix, r.Method, r.URL.RequestURI(), body)
	}

	cert, err := selfSingedCert()
	if err != nil {
		return fmt.Errorf("getting cert: %w", err)
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", localPort),
		Handler: proxy,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
		},
	}

	wait := make(chan error)
	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			wait <- fmt.Errorf("HTTP Proxy shutdown: %w", err)
			return
		}
		wait <- nil
	}()

	fmt.Printf("Listen on: '%s'\n", srv.Addr)
	if err := srv.ListenAndServeTLS("", ""); err != http.ErrServerClosed {
		return fmt.Errorf("HTTP Proxy failed: %v", err)
	}

	return nil
}

func selfSingedCert() (tls.Certificate, error) {
	certPem := []byte(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`)
	keyPem := []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----`)

	return tls.X509KeyPair(certPem, keyPem)
}
