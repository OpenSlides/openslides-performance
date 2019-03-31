package client

// Option is an option for the NewClient() function.
type Option func(*Client)

// WithSSL tels a client to use an ssl connection.
func WithSSL() Option {
	return func(c *Client) { c.useSSL = true }
}

// WithCredentials adds username and passwort to an client.
func WithCredentials(username, password string) Option {
	return func(c *Client) { c.username = username; c.password = password }
}

// WithIsAdmin tells an client, that it is an admin.
func WithIsAdmin() Option {
	return func(c *Client) { c.isAdmin = true }
}

// WithConnecter sets the connection interface of the client that manages the websocket connection.
func WithConnecter(connect wsConnecter) Option {
	return func(c *Client) { c.wsConnect = connect }
}

// WithConnectionAttemts sets number of connection attems before an error.
func WithConnectionAttemts(attemts int) Option {
	return func(c *Client) { c.connectionAttemts = attemts }
}

// WithLoginAttemts sets number of login attems before an fatal error.
func WithLoginAttemts(attemts int) Option {
	return func(c *Client) { c.loginAttemts = attemts }
}
