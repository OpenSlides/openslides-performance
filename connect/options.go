package connect

import "os"

// Options is the meta information for the cli.
type Options struct {
	Amount    int      `help:"Amount of connections to use." short:"n" default:"10"`
	Body      string   `help:"Request Body." short:"b"`
	BodyFile  *os.File `help:"Request Body from a file. Use - for stdin"`
	SkipFirst bool     `help:"Use skip first flag to save traffic."`
}

// Help returns the help message
func (o Options) Help() string {
	return `Every connection is open and is waiting for messages. For each change
you see a progress bar that shows how many connections got an answer for
this change.

Example:

openslides-performance connect -b '[{"collection":"organization","ids":[1],"fields":{"committee_ids":{"type":"relation-list","collection":"committee","fields":{"name":null}}}}]'`
}
