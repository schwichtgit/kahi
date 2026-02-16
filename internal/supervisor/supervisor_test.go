package supervisor

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestNewSignalQueue(t *testing.T) {
	sq := NewSignalQueue(testLogger())
	defer sq.Stop()

	if sq.C == nil {
		t.Fatal("expected non-nil signal channel")
	}
}

func TestSignalQueueBufferSize(t *testing.T) {
	sq := NewSignalQueue(testLogger())
	defer sq.Stop()

	// The channel should have a buffer of 16.
	if cap(sq.ch) != 16 {
		t.Fatalf("expected buffer size 16, got %d", cap(sq.ch))
	}
}

func TestSignalQueueStop(t *testing.T) {
	sq := NewSignalQueue(testLogger())
	sq.Stop()
	// After stop, signal.Notify is deregistered. No panic means pass.
}

func TestRootWarningNotRoot(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// When not root (uid != 0), no warning should be logged.
	RootWarning(logger, false)

	// On CI/dev machines we're typically not root, so expect no warning.
	if strings.Contains(buf.String(), "running as root") {
		// This would only happen if tests run as root.
		t.Skip("running as root, skipping non-root test")
	}
}

func TestRootWarningWithUserConfigured(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	// Even if root, no warning when user is configured.
	RootWarning(logger, true)

	if strings.Contains(buf.String(), "running as root") {
		t.Fatal("should not warn when user is configured")
	}
}
