package client

// Option is an option for the NewClient() function.
type Option func(*Client)

// WithServer defines the server domain and if to use ssl
func WithServer(server string, ssl bool) Option {
	return func(c *Client) { c.server = server; c.ssl = ssl }
}

// WithConnecter sets the connection interface of the client that manages the websocket connection.
func WithConnecter(connect wsConnecter) Option {
	return func(c *Client) { c.wsConnect = connect }
}

// WithConnectionAttemts sets number of connection attems before an error.
func WithConnectionAttemts(attemts int) Option {
	return func(c *Client) { c.connectionAttemts = attemts }
}

func WithSession(s *Session) Option {
	return func(c *Client) { c.session = s }
}
