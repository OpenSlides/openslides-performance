package oswstest

const (
	// BaseURL is the URL to the server. It is used for websocket and http. The
	// Placeholders are filled in by the code.
	BaseURL = "%s://localhost:8000/%s"

	// SSL is a flag for the BaseURL if http or https should be used.
	SSL = false

	// LoginURLPath is the path to build the url for login. It has no leading slash.
	LoginURLPath = "apps/users/login/"

	// WSURLPath is the path to build the websocket url. It has no leading slash.
	WSURLPath = "ws/?change_id=0&autoupdate=on"

	// LoginPassword is the password to login the normal clients and also the admin clients.
	LoginPassword = "password"

	// MaxLoginAttemts is the number of tries for each client to login. If one
	// client fails more then this number, then the program is quit with a fatal
	// error.
	MaxLoginAttemts = 5

	// MaxConnectionAttemts is th enumber of tries for each client, to connect via
	// websocket. If a client fails, is program is not quit, but the error is shoun
	// in the end.
	MaxConnectionAttemts = 3

	// CSRFCookieName is the name of the CSRF cookie of OpenSlides. Make sure, that
	// this is the same as in the OpenSlides config.
	CSRFCookieName = "OpenSlidesCsrfToken"

	// ParallelConnections defines the number of connections, that are done in
	// parallel. The number should be similar as the number of openslides workers.
	ParallelConnections = 2

	// Same for logins
	ParallelLogins = 10

	// Same for sends in the ManySendTest
	ParallelSends = 10
)

const (
	// If ShowAllErros is true, then all errors that happen are shoun after a result
	// Else, only the first error is shown.
	ShowAllErros = true

	// If LogStatus is true, then the program shows some output while the tests are
	// running
	LogStatus = false
)
