package voteclient

// Options is the meta information for the cli.
type Options struct {
	PollID  int    `arg:"" help:"ID of the poll."`
	MainKey string `help:"Public main key from vote decrypt as base64." short:"k"`
}

// Help returns the help message
func (o Options) Help() string {
	return `Example client to vote in OpenSlides.`
}
