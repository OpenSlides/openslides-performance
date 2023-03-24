package slow

import (
	"net/url"
)

// Options is the meta information for the cli.
type Options struct {
	URL    *url.URL `arg:"" help:"URL for the request"`
	Amount int      `help:"Amount of user to be created." short:"n" default:"10"`
}

// Help returns the help message
func (o Options) Help() string {
	return `Sends many slow request with an endless body.`
}
