package request

import (
	"net/url"
	"os"
)

// Options is the meta information for the cli.
type Options struct {
	URL *url.URL `arg:"" help:"URL for the request"`

	Body     string   `help:"HTTP Post body." short:"b"`
	BodyFile *os.File `help:"Request Body from a file. Use - for stdin"`
}

// Help returns the help message
func (o Options) Help() string {
	return `Before the request, a login request is send and the credentials are used for the actual request.`
}
