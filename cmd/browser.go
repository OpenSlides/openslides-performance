package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/spf13/cobra"
)

const browserHelp = `Simulates a browser

The browser command consts of two sub commands. 'record' and 'replay'.

'record' opens a local proxy. All requests to openslides are printed to 
stdout. Only login requests are ignored. To do so, a self singed certificate
is created.

'replay' creates connections to OpenSlides by reading them from stdin.
Each connection is created many times either with the same user or with different users.

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

	return &cmd
}

func startProxy(ctx context.Context, remoteAddr string, localPort int) error {
	// TODO:	self singed cert
	//			print incomming
	//			proxy but not print login
	//			Handle shutdown

	addr := fmt.Sprintf(":%d", localPort)
	target, err := url.Parse(remoteAddr)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	proxy.FlushInterval = -1

	http.ListenAndServe(addr, proxy)

	return nil
}
