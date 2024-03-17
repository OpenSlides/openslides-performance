package brokenproxy

// Options is the meta information for the cli.
type Options struct {
	Port int `arg:"" help:"Port to use for the proxy. Default is 8080." default:"8080"`
}

// Help returns the help message
func (o Options) Help() string {
	return `Opens a proxy on the given port that is broken by buffering the response.
`
}
