package client

import (
	"net/http"
	"sync"
)

// Task contains a long running response.
//
// It handles backend action workers.
type Task struct {
	mu sync.Mutex

	resp *http.Response
	done chan struct{}
	err  error
}

func (t *Task) Error() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.err
}

// Response returns the response or nil, if t.Error() is not nil or t.Done() is
// not closed.
func (t *Task) Response() *http.Response {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.resp
}

// Done returns a channel that is closed when the task is done.
func (t *Task) Done() <-chan struct{} {
	return t.done
}
