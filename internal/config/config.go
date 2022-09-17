package config

import (
	"strings"
	"time"
)

// Config for all commands.
type Config struct {
	Domain   string `help:"Domain of the OpenSlides server to probe." short:"d" default:"localhost:8000"`
	Username string `help:"Username for logged-in requests." short:"u" default:"admin"`
	Password string `help:"Password to use for logged-in requests." short:"p" default:"admin"`
	HTTP     bool   `help:"Use http instead of https. Default is https."`
	IPv4     bool   `help:"Force IPv4 for requests." short:"4"`

	RetryEventProvider func() <-chan struct{} `kong:"-"`
}

// Addr returns the domain with the http or https prefix.
func (c *Config) Addr() string {
	if strings.HasPrefix(c.Domain, "http") {
		return c.Domain
	}

	proto := "https"
	if c.HTTP {
		proto = "http"
	}
	return proto + "://" + c.Domain
}

// RetryEvent returns a channel that is closed after some time. Defaults to a pause for one second.
func (c *Config) RetryEvent() <-chan struct{} {
	if c.RetryEventProvider != nil {
		return c.RetryEventProvider()
	}

	ch := make(chan struct{})
	go func() {
		time.Sleep(1)
		close(ch)
	}()
	return ch
}
