package client

// Config for all commands.
type Config struct {
	Domain   string `help:"Domain of the OpenSlides server to probe." short:"d" default:"localhost:8000"`
	Username string `help:"Username for logged-in requests." short:"u" default:"superadmin"`
	Password string `help:"Password to use for logged-in requests." short:"p" default:"superadmin"`
	HTTP     bool   `help:"Use http instead of https. Default is https."`
	IPv4     bool   `help:"Force IPv4 for requests." short:"4"`
	FakeAuth bool   `help:"Do not login but expect user id 1."`
}

// Addr returns the domain with the http or https prefix.
func (c *Config) Addr() string {
	proto := "https"
	if c.HTTP {
		proto = "http"
	}
	return proto + "://" + c.Domain
}
