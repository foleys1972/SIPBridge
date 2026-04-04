package recording

import (
	"context"
	"errors"
)

type Recorder interface {
	Start(ctx context.Context, sessionID string) error
	Stop(ctx context.Context, sessionID string) error
}

type NoopRecorder struct{}

func (n NoopRecorder) Start(ctx context.Context, sessionID string) error { return nil }
func (n NoopRecorder) Stop(ctx context.Context, sessionID string) error  { return nil }

var ErrNotImplemented = errors.New("not implemented")
