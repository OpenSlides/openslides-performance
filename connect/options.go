package connect

import "os"

// Options is the meta information for the cli.
type Options struct {
	Amount        int      `help:"Amount of connections to use." short:"n" default:"10"`
	Body          string   `help:"Request Body." short:"b"`
	BodyFile      *os.File `help:"Request Body from a file. Use - for stdin"`
	Action        *os.File `help:"Request Body to use as an action. If set, press enter to sent the action"`
	MultiUser     bool     `help:"Use dummy user accounts" short:"m" default:"false"`
	BaseName      string   `help:"The base name of the users" default:"dummy"`
	UsersPassword string   `help:"The password used for all users" default:"pass"`
}

// Help returns the help message
func (o Options) Help() string {
	return `Every connection is open and is waiting for messages. For each change
you see a progress bar that shows how many connections got an answer for
this change.

Example:

openslides-performance connect -b '[{"collection":"organization","ids":[1],"fields":{"committee_ids":{"type":"relation-list","collection":"committee","fields":{"name":null}}}}]'

With --muli-user each connection uses a different user account.
The accounts can be created with the "create-users" command. The attribute
needs the meeting id that was used to create the users. Use "0" when the
users where created without a meeting.
`
}
