package tester

import (
	"fmt"
	"time"
)

// testResult holds the result of one test.
type testResult struct {
	values        []time.Duration
	errors        []error
	description   string
	showAllErrors bool
}

// add adds one value to the testresult.
func (t *testResult) add(value time.Duration) {
	t.values = append(t.values, value)
}

// addError adds an error to the test result
func (t *testResult) addError(err error) {
	t.errors = append(t.errors, err)
}

// String shows the test result
func (t *testResult) String() string {
	s := fmt.Sprintf(
		"%s\ncount: %d\nmin: %dms\nmax: %dms\nave: %dms\n",
		t.description,
		t.count(),
		t.min()/time.Millisecond,
		t.max()/time.Millisecond,
		t.ave()/time.Millisecond,
	)
	if t.errCount() > 0 {
		s += fmt.Sprintf("error count: %d\n", len(t.errors))
		if t.showAllErrors {
			for i, err := range t.errors {
				s += fmt.Sprintf("%3d error: %s\n", i+1, err)
			}
		} else {
			s += fmt.Sprintf("first error: %s\n", t.errors[0])
		}
	}
	return s
}

// count returns the number of values in the testresult.
func (t *testResult) count() int {
	return len(t.values)
}

// errCount returns the number of error values in the testresult.
func (t *testResult) errCount() int {
	return len(t.errors)
}

func (t *testResult) min() (m time.Duration) {
	for i, v := range t.values {
		if i == 0 {
			m = v
			continue
		}

		if v < m {
			m = v
		}
	}
	return
}

func (t *testResult) max() (m time.Duration) {
	for _, v := range t.values {
		if v > m {
			m = v
		}
	}
	return
}

func (t *testResult) ave() (m time.Duration) {
	var a time.Duration
	if len(t.values) == 0 {
		return 0
	}

	for _, v := range t.values {
		a += v
	}
	return time.Duration(a.Nanoseconds() / int64(len(t.values)))
}
