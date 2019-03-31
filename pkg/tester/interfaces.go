package tester

import "time"

// Loginer is a type that can be logged in
type Loginer interface {
	Login() error
}

// Connecter is a type that can connect.
type Connecter interface {
	Connect() error
}

// Sender is a type that can Send something.
type Sender interface {
	Send() error
}

// Listener is a type that you get except to receive data.
type Listener interface {
	ExpectData(count int, sinceConnect bool) error
	Connected() time.Time
}
