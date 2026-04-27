// Package logger initialises the process-wide log output.
// All existing log.Printf calls automatically write to both stdout and the
// rotating daily log file once Init is called.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// rotatingFile writes to a date-stamped log file and reopens it each day.
type rotatingFile struct {
	mu      sync.Mutex
	dir     string
	prefix  string
	current *os.File
	day     string // YYYY-MM-DD of the current file
}

func newRotatingFile(dir, prefix string) (*rotatingFile, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("log dir: %w", err)
	}
	r := &rotatingFile{dir: dir, prefix: prefix}
	if err := r.rotate(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *rotatingFile) rotate() error {
	today := time.Now().Format("2006-01-02")
	if r.current != nil && r.day == today {
		return nil
	}
	if r.current != nil {
		_ = r.current.Close()
	}
	name := filepath.Join(r.dir, fmt.Sprintf("%s-%s.log", r.prefix, today))
	f, err := os.OpenFile(name, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %s: %w", name, err)
	}
	r.current = f
	r.day = today
	return nil
}

func (r *rotatingFile) Write(p []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = r.rotate() // no-op if already today's file; silently ignore rotate errors
	if r.current == nil {
		return len(p), nil
	}
	return r.current.Write(p)
}

func (r *rotatingFile) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current != nil {
		err := r.current.Close()
		r.current = nil
		return err
	}
	return nil
}

// Init redirects the standard logger to write to both stdout and a rotating
// daily log file under dir.  The returned io.Closer should be deferred at
// process exit to flush and close the file.
//
// Log file naming: <dir>/sipbridge-YYYY-MM-DD.log
//
// If dir is empty, only stdout is used (no-op for the file side).
func Init(dir string) (io.Closer, error) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	if dir == "" {
		return io.NopCloser(nil), nil
	}

	rf, err := newRotatingFile(dir, "sipbridge")
	if err != nil {
		return nil, err
	}

	multi := io.MultiWriter(os.Stdout, rf)
	log.SetOutput(multi)

	log.Printf("logger: writing to %s/sipbridge-YYYY-MM-DD.log", dir)
	return rf, nil
}
