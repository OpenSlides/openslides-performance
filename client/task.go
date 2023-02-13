package client

import (
	"errors"
	"net/http"
	"sync"
)

// ErrAbborted is returned from a task, when the task was abborted.
var ErrAbborted = errors.New("backend action abborted")

// Task contains a long running response.
//
// It handles backend action workers.
type Task struct {
	mu sync.Mutex

	resp *http.Response
	done chan struct{}
	err  error
}

// Result returns the response and error. Both will be nil until task is done.
func (t *Task) Result() (*http.Response, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.resp, t.err
}

// Done returns a channel that is closed when the task is done.
func (t *Task) Done() <-chan struct{} {
	return t.done
}

func (t *Task) setDone(resp *http.Response, err error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.resp = resp
	t.err = err
	close(t.done)
}
