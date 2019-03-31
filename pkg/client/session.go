package client

import (
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"time"
)

type Session struct {
	username string
	password string
	isAdmin  bool

	loginAttemts int
	cookies      *cookiejar.Jar

	server string
	ssl    bool
}

func NewSession(server string, ssl bool, username, password string, isAdmin bool) (*Session, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("can not create cookie jar, %v", err)
	}
	if username == "" || password == "" {
		return nil, fmt.Errorf("session needs a username and password.")

	}
	return &Session{
		username:     username,
		password:     password,
		isAdmin:      isAdmin,
		server:       server,
		ssl:          ssl,
		cookies:      jar,
		loginAttemts: 5,
	}, nil
}

func (s *Session) getLoginData() string {
	return fmt.Sprintf("{\"username\": \"%s\", \"password\": \"%s\"}", s.username, s.password)
}

// Login logsin a session. This sends a login request to the server and saves
// the session-cookie.
func (s *Session) Login() error {
	httpClient := &http.Client{
		Jar: s.cookies,
	}
	var resp *http.Response
	var err error

	for i := 0; i < s.loginAttemts; i++ {
		resp, err = httpClient.Post(
			getLoginURL(s.server, s.ssl),
			"application/json",
			strings.NewReader(s.getLoginData()),
		)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		// Statu Code not between 500 and 600
		if !(500 <= resp.StatusCode && resp.StatusCode < 600) {
			break
		}
		// If the error is on the server side, then retry after some time
		time.Sleep(100 * time.Millisecond)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("login failed with status code: %d", resp.StatusCode)
	}
	return nil
}
