package browser

import "os"

// Options is the meta information for the cli.
type Options struct {
	Record record `cmd:"" help:"Creates a proxy that records all requests."`
	Replay replay `cmd:"" help:"Replays connections recorded with record."`
}

// Help returns the help message
func (o Options) Help() string {
	return `The browser command contains of two sub commands. 'record' and 'replay'.

'record' opens a local proxy. All requests to openslides are printed to
stdout. To do so, a self singed certificate is created.

'replay' creates connections to OpenSlides by reading them from stdin.
Each connection is created many times either with the same user or with different users.
The user defined with -u and -p is used, even if there are login requests.

Both commands can be used together. In this case a click in the (real) browser
is sent to OpenSlides many times:

openslides-performance browser record | openslides-performance replay`
}

type record struct {
	Port   int    `arg:"" help:"Port to use for the proxy. Default is 8080." default:"8080"`
	Filter string `help:"Filter the URL path" short:"f" default:""`
	Files  string `help:"File prefix to write request bodies to separate files." short:"o" default:""`

	count int
}

type replay struct {
	Amount   int      `help:"Amount browsers to simulare." short:"n" default:"10"`
	Commands *os.File `arg:"" help:"File with the replay commands. Use - for stdin. stdin is the default." default:"-"`
	Close    bool     `help:"Exit when all connections are closed"`
}
