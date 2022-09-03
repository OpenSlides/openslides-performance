package work

// Options is the meta information for the cli.
type Options struct {
	Amount int `help:"Amount of action to be called." short:"n" default:"10"`

	MeetingID int    `help:"Meeting id to use." short:"m" default:"1"`
	Strategy  string `help:"Strategy for the background tasks." short:"s" default:"topic-done" enum:"topic-done,motion-state"`
}

// Help returns the help message
func (o Options) Help() string {
	return `It uses different strategie to create the load. The strategie can
be set via the argument --strategy which is a string. Possible strategies are:

* topic-done: sets the done status of a topic to true and false

* motion-state: sets the state of a motion to 2 and then resets it.`
}
