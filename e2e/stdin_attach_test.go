//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStdin_Write(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "cat.sh", "while read line; do echo got:$line; done")

	client, _ := startDaemon(t, `
[programs.cat]
command = "`+script+`"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "cat", "RUNNING", 5*time.Second)
	time.Sleep(500 * time.Millisecond)

	if err := client.WriteStdin("cat", "hello"); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("cat", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	if !strings.Contains(buf.String(), "got:hello") {
		t.Fatalf("output = %q, want 'got:hello'", buf.String())
	}
}

func TestStdin_WriteStopped(t *testing.T) {
	client, _ := startDaemon(t, `
[programs.stopped]
command = "/bin/cat"
autostart = false
`)
	err := client.WriteStdin("stopped", "data")
	if err == nil {
		t.Fatal("expected error writing stdin to stopped process")
	}
}

func TestStdin_Attach(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "interactive.sh", "while read line; do echo reply:$line; done")

	client, _ := startDaemon(t, `
[programs.interactive]
command = "`+script+`"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "interactive", "RUNNING", 5*time.Second)
	time.Sleep(500 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var stdout struct {
		mu  sync.Mutex
		buf bytes.Buffer
	}
	stdin := strings.NewReader("test-input\n")

	go func() {
		_ = client.Attach(ctx, "interactive", stdin, &writerFunc{func(p []byte) (int, error) {
			stdout.mu.Lock()
			defer stdout.mu.Unlock()
			return stdout.buf.Write(p)
		}})
	}()

	deadline := time.After(5 * time.Second)
	for {
		stdout.mu.Lock()
		got := stdout.buf.String()
		stdout.mu.Unlock()
		if strings.Contains(got, "reply:test-input") {
			return
		}
		select {
		case <-deadline:
			stdout.mu.Lock()
			t.Fatalf("no reply received, stdout = %q", stdout.buf.String())
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// writerFunc adapts a function to io.Writer.
type writerFunc struct {
	fn func([]byte) (int, error)
}

func (w *writerFunc) Write(p []byte) (int, error) {
	return w.fn(p)
}
