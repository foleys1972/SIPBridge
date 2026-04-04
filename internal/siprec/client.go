package siprec

import (
	"context"
	"errors"
)

type Client interface {
	StartRecording(ctx context.Context, callID string, metadataXML string) error
	StopRecording(ctx context.Context, callID string) error
}

// NoopClient is a stub for tests. Production SIPREC INVITE/ACK/BYE is implemented in package sip (siprec_ctrl.go).
type NoopClient struct{}

func (n NoopClient) StartRecording(ctx context.Context, callID string, metadataXML string) error {
	return nil
}

func (n NoopClient) StopRecording(ctx context.Context, callID string) error {
	return nil
}

var ErrNotImplemented = errors.New("not implemented")
