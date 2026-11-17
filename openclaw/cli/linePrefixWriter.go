package cli

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

type linePrefixWriter struct {
	prefix  string
	writer  io.Writer
	mu      sync.Mutex
	pending []byte
}

func newLinePrefixWriter(prefix string, writer io.Writer) *linePrefixWriter {
	return &linePrefixWriter{
		prefix: prefix,
		writer: writer,
	}
}

func (w *linePrefixWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	n := len(p)
	w.pending = append(w.pending, p...)

	for {
		index := bytes.IndexByte(w.pending, '\n')
		if index < 0 {
			break
		}

		line := w.pending[:index+1]
		if _, err := fmt.Fprintf(w.writer, "[%s] %s", w.prefix, line); err != nil {
			return 0, err
		}

		w.pending = w.pending[index+1:]
	}

	return n, nil
}

func (w *linePrefixWriter) Flush() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.pending) == 0 {
		return nil
	}

	_, err := fmt.Fprintf(w.writer, "[%s] %s\n", w.prefix, w.pending)
	w.pending = nil
	return err
}
