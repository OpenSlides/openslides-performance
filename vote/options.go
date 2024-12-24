package vote

// Options is the meta information for the cli.
type Options struct {
	Amount        int    `help:"Amount users to use." short:"n" default:"10"`
	PollID        int    `help:"ID of the poll to use." short:"i" default:"1"`
	Interrupt     bool   `help:"Wait for a user input after login."`
	Loop          bool   `help:"After the test, start it again with the logged in users."`
	BaseName      string `help:"The name string that is concatenated with meeting id and user id, e.g. m1dummy1." default:"dummy"`
	UsersPassword string `help:"The password used for all users" default:"pass"`
}

// Help returns the help message
func (o Options) Help() string {
	return `This command requires, that there are many user created at the
backend. You can use the command "create_users" for this job.

Example:

openslides-performance vote --amount 100 --poll_id 42`
}
