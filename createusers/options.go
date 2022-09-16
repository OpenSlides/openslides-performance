package createusers

// Options is the meta information for the cli.
type Options struct {
	Amount    int `help:"Amount of user to be created." short:"n" default:"10"`
	MeetingID int `help:"If set, put the user in the delegated group of this meeting." short:"m"`
}

// Help returns the help message
func (o Options) Help() string {
	return `This command does not run any test. It is a helper for other tests that
require, that there a many users created at the server.

Do not run this command against a productive instance. It will change
the database.

Each user is called dummy1, dummy2 etc and has the password "pass".`
}
