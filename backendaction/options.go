package backendaction

// Options is the meta information for the cli.
type Options struct {
	Action  string `arg:"" help:"Name of the action."`
	Amount  int    `help:"Amount of action to be called." short:"n" default:"10"`
	Content string `arg:"" help:"content of the action."`
}

// Help returns the help message
func (o Options) Help() string {
	return `All actions are send to the backend in one request.

In the action content \i is replaced with a number between 1 and amount.

\u is replaced with a random uuid.

If content is "-", then the content is read from stdin.

Example:

openslides-performance backend-action motion.create '{"meeting_id":1,"text":"hello world","title":"motion\u"}'`
}
