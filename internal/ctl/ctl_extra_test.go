package ctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNewLineScannerBasic(t *testing.T) {
	r := strings.NewReader("line1\nline2\nline3\n")
	scanner := newLineScanner(r)

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line1" {
		t.Fatalf("first line = %q, want line1", lines[0])
	}
}

func TestIsTerminalBuffer(t *testing.T) {
	var buf bytes.Buffer
	if isTerminal(&buf) {
		t.Fatal("bytes.Buffer should not be detected as terminal")
	}
}

func TestAttachProcessNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/processes/{name}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	addr := strings.TrimPrefix(ts.URL, "http://")
	c := NewTCPClient(addr, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var buf bytes.Buffer
	err := c.Attach(ctx, "nonexistent", strings.NewReader(""), &buf)
	if err == nil {
		t.Fatal("expected error for nonexistent process")
	}
	if !strings.Contains(err.Error(), "no such process") {
		t.Fatalf("error = %q, want 'no such process'", err)
	}
}

func TestAttachConnectionError(t *testing.T) {
	c := NewTCPClient("127.0.0.1:1", "", "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var buf bytes.Buffer
	err := c.Attach(ctx, "web", strings.NewReader(""), &buf)
	if err == nil {
		t.Fatal("expected connection error")
	}
	if !strings.Contains(err.Error(), "connection failed") {
		t.Fatalf("error = %q, want 'connection failed'", err)
	}
}

func TestAttachStreamsOutput(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/processes/{name}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "web", "state": "RUNNING"})
	})
	mux.HandleFunc("GET /api/v1/processes/{name}/log/{stream}/stream", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "data: output line\n\n")
		flusher.Flush()
	})
	mux.HandleFunc("POST /api/v1/processes/{name}/stdin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	addr := strings.TrimPrefix(ts.URL, "http://")
	c := NewTCPClient(addr, "", "")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var stdout bytes.Buffer
	_ = c.Attach(ctx, "web", strings.NewReader("hello\n"), &stdout)

	if !strings.Contains(stdout.String(), "output line") {
		t.Fatalf("stdout = %q, want 'output line'", stdout.String())
	}
}

func TestNewUnixClientTransport(t *testing.T) {
	c := NewUnixClient("/tmp/test-kahi.sock")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.baseURL != "http://unix" {
		t.Fatalf("baseURL = %q, want http://unix", c.baseURL)
	}
	if c.httpClient.Transport == nil {
		t.Fatal("expected non-nil transport")
	}
}

func TestPIDConnectionError(t *testing.T) {
	c := NewTCPClient("127.0.0.1:1", "", "")
	_, err := c.PID("web")
	if err == nil {
		t.Fatal("expected connection error")
	}
}

func TestHealthConnectionError(t *testing.T) {
	c := NewTCPClient("127.0.0.1:1", "", "")
	_, err := c.Health()
	if err == nil {
		t.Fatal("expected connection error")
	}
}
