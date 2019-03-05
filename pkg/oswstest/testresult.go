package oswstest

import (
	"fmt"
	"time"
)

// TestResult holds the result of one test.
type TestResult struct {
	values      []time.Duration
	errors      []error
	description string
}

// Add adds one value to the testresult.
func (t *TestResult) Add(value time.Duration) {
	t.values = append(t.values, value)
}

// AddError adds an error to the test result
func (t *TestResult) AddError(err error) {
	t.errors = append(t.errors, err)
}

// String shows the test result
func (t *TestResult) String() string {
	s := fmt.Sprintf(
		"%s\ncount: %d\nmin: %dms\nmax: %dms\nave: %dms\n",
		t.description,
		t.Count(),
		t.min()/time.Millisecond,
		t.max()/time.Millisecond,
		t.ave()/time.Millisecond,
	)
	if t.ErrCount() > 0 {
		s += fmt.Sprintf("error count: %d\n", len(t.errors))
		if ShowAllErros {
			for i, err := range t.errors {
				s += fmt.Sprintf("%3d error: %s\n", i+1, err)
			}
		} else {
			s += fmt.Sprintf("first error: %s\n", t.errors[0])
		}
	}
	return s
}

// Count returns the number of values in the testresult.
func (t *TestResult) Count() int {
	return len(t.values)
}

// ErrCount returns the number of error values in the testresult.
func (t *TestResult) ErrCount() int {
	return len(t.errors)
}

// CountBoth returns the number of values and errors i nthe testresult.
func (t *TestResult) CountBoth() int {
	return t.Count() + t.ErrCount()
}

func (t *TestResult) min() (m time.Duration) {
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

func (t *TestResult) max() (m time.Duration) {
	for _, v := range t.values {
		if v > m {
			m = v
		}
	}
	return
}

func (t *TestResult) ave() (m time.Duration) {
	var a time.Duration
	if len(t.values) == 0 {
		return 0
	}

	for _, v := range t.values {
		a += v
	}
	return time.Duration(a.Nanoseconds() / int64(len(t.values)))
}
