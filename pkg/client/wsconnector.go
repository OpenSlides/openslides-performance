package client

import (
	"net/http/cookiejar"

	"github.com/gorilla/websocket"
)

type wsConnecter interface {
	Connect(uri string, cookieJar *cookiejar.Jar) (conn ReaderCloser, err error)
}

// ReaderCloser can be Closed and read messages from. Is used as
// an abstraction of websocket.Conn
type ReaderCloser interface {
	Close() error
	ReadMessage() (int, []byte, error)
}

type wsConnect struct{}

func (ws wsConnect) Connect(uri string, cookieJar *cookiejar.Jar) (conn ReaderCloser, err error) {
	dialer := websocket.Dialer{}
	if cookieJar != nil {
		dialer.Jar = cookieJar
	}
	conn, _, err = dialer.Dial(uri, nil)
	return
}
