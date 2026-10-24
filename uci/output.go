package uci

import (
	"bufio"
	"fmt"
	"io"
	"sync"
)

// lineWriter serializes and flushes every protocol line. Search info and
// readyok can be produced by different goroutines, so stdout writes must be
// atomic and line-buffered.
type lineWriter struct {
	mu  sync.Mutex
	buf *bufio.Writer
	err error
}

func newLineWriter(w io.Writer) *lineWriter {
	return &lineWriter{buf: bufio.NewWriter(w)}
}

// line writes one formatted line and flushes it. Write errors are remembered
// and returned on subsequent calls; callers normally ignore them.
func (w *lineWriter) line(format string, args ...any) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.err != nil {
		return w.err
	}
	_, err := fmt.Fprintf(w.buf, format+"\n", args...)
	if err == nil {
		err = w.buf.Flush()
	}
	if err != nil {
		w.err = err
	}
	return err
}

// err returns the first write error, if any.
func (w *lineWriter) errValue() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}
