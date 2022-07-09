package cmd

import (
	"context"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

const rootHelp = `Performance is an helper tool to test the limits of OpenSlies.

Each task is implemented as a subcommand.`

type config struct {
	domain    string
	username  string
	password  string
	http      bool
	forceIPv4 bool
}

func (c *config) addr() string {
	proto := "https"
	if c.http {
		proto = "http"
	}
	return proto + "://" + c.domain
}

func cmdRoot(cfg *config) *cobra.Command {
	cmd := &cobra.Command{
		Use:          "performance",
		Short:        "performance is a tool that brings OpenSlides to its limit.",
		Long:         rootHelp,
		SilenceUsage: true,
	}

	cmd.PersistentFlags().StringVarP(&cfg.domain, "domain", "d", "localhost:8000", "Domain where to send the requests")
	cmd.PersistentFlags().StringVarP(&cfg.username, "username", "u", "admin", "Username that can create the users.")
	cmd.PersistentFlags().StringVarP(&cfg.password, "password", "p", "admin", "Password to use.")
	cmd.PersistentFlags().BoolVar(&cfg.http, "http", false, "Use http instead of https. Default is https.")
	cmd.PersistentFlags().BoolVar(&cfg.forceIPv4, "4", false, "Force IPv4")

	return cmd
}

// Execute starts the root command.
func Execute() error {
	cfg := new(config)
	cmd := cmdRoot(cfg)
	cmd.AddCommand(
		cmdCreateUsers(cfg),
		cmdConnect(cfg),
		cmdVotes(cfg),
		cmdRequest(cfg),
		cmdBackendAction(cfg),
		cmdWork(cfg),
		cmdBrowser(cfg),
	)

	return cmd.Execute()
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
