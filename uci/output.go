package uci

import (
	"bufio"
	"fmt"
	"io"
	"sync"
)

// lineWriter serializes output from the command loop and search goroutine.
type lineWriter struct {
	mu  sync.Mutex
	buf *bufio.Writer
	err error
}

func newLineWriter(w io.Writer) *lineWriter {
	return &lineWriter{buf: bufio.NewWriter(w)}
}

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

func (w *lineWriter) errValue() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.err
}
