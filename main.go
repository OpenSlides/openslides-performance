package main

// reading_client simulates clients, that login to openslides and after a
// successfull login send all request, that the client usual sends.
import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/OpenSlides/openslides-performance/backendaction"
	"github.com/OpenSlides/openslides-performance/browser"
	"github.com/OpenSlides/openslides-performance/client"
	"github.com/OpenSlides/openslides-performance/connect"
	"github.com/OpenSlides/openslides-performance/createusers"
	"github.com/OpenSlides/openslides-performance/request"
	"github.com/OpenSlides/openslides-performance/slow"
	"github.com/OpenSlides/openslides-performance/vote"
	"github.com/OpenSlides/openslides-performance/work"
	"github.com/alecthomas/kong"
)

func main() {
	ctx, cancel := interruptContext()
	defer cancel()

	cliCtx := kong.Parse(&cli, kong.UsageOnError(), kong.Configuration(kong.JSON, "config.json"))
	cliCtx.BindTo(ctx, (*context.Context)(nil))
	cliCtx.Bind(cli.Config)
	if err := cliCtx.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// interruptContext works like signal.NotifyContext
//
// In only listens on os.Interrupt. If the signal is received two times,
// os.Exit(1) is called.
func interruptContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt)
		<-sigint
		cancel()

		// If the signal was send for the second time, make a hard cut.
		<-sigint
		os.Exit(1)
	}()
	return ctx, cancel
}

var cli struct {
	client.Config

	BackendAction backendaction.Options `cmd:"" help:"Calls a backend action multiple times."`
	Browser       browser.Options       `cmd:"" help:"Simulates a browser."`
	Connect       connect.Options       `cmd:"" help:"Opens many connections to autoupdate and keeps them open."`
	CreateUsers   createusers.Options   `cmd:"" help:"Create many users."`
	Request       request.Options       `cmd:"" help:"Sends a logged-in request to OpenSlides."`
	Slow          slow.Options          `cmd:"" help:"Sends many slow requests."`
	Vote          vote.Options          `cmd:"" help:"Sends many votes from different users."`
	Work          work.Options          `cmd:"" help:"Generates background work."`
}
