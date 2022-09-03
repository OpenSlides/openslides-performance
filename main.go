package main

// reading_client simulates clients, that login to openslides and after a
// successfull login send all request, that the client usual sends.
import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/OpenSlides/openslides-performance/internal/config"
	"github.com/OpenSlides/openslides-performance/request"
	"github.com/alecthomas/kong"
)

// func main() {
// 	if err := cmd.Execute(); err != nil {
// 		os.Exit(1)
// 	}
// }

func main() {
	ctx, cancel := interruptContext()
	defer cancel()

	cliCtx := kong.Parse(&cli, kong.UsageOnError())
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
	Request request.Options `cmd:"" help:"Sends a logged-in request to OpenSlides."`
	config.Config
}
