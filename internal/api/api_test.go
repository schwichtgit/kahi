package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kahidev/kahi/internal/events"
)

// --- Mock implementations ---

type mockProcessManager struct {
	processes []ProcessInfo
}

func (m *mockProcessManager) List() []ProcessInfo { return m.processes }
func (m *mockProcessManager) Get(name string) (ProcessInfo, error) {
	for _, p := range m.processes {
		if p.Name == name {
			return p, nil
		}
	}
	return ProcessInfo{}, fmt.Errorf("no such process: %s", name)
}
func (m *mockProcessManager) Start(name string) error {
	for _, p := range m.processes {
		if p.Name == name {
			if p.State == "RUNNING" {
				return fmt.Errorf("process already started: %s", name)
			}
			return nil
		}
	}
	return fmt.Errorf("no such process: %s", name)
}
func (m *mockProcessManager) Stop(name string) error {
	for _, p := range m.processes {
		if p.Name == name {
			if p.State != "RUNNING" {
				return fmt.Errorf("process not running: %s", name)
			}
			return nil
		}
	}
	return fmt.Errorf("no such process: %s", name)
}
func (m *mockProcessManager) Restart(name string) error {
	_, err := m.Get(name)
	return err
}
func (m *mockProcessManager) Signal(name string, sig string) error {
	if _, err := m.Get(name); err != nil {
		return err
	}
	valid := map[string]bool{"TERM": true, "HUP": true, "INT": true, "KILL": true, "USR1": true, "USR2": true, "QUIT": true}
	if !valid[strings.ToUpper(sig)] {
		return fmt.Errorf("invalid signal: %s", sig)
	}
	return nil
}
func (m *mockProcessManager) WriteStdin(name string, data []byte) error {
	_, err := m.Get(name)
	return err
}
func (m *mockProcessManager) ReadLog(name string, stream string, offset int64, length int) ([]byte, error) {
	if _, err := m.Get(name); err != nil {
		return nil, err
	}
	return []byte("log output\n"), nil
}

type mockGroupManager struct {
	groups []string
}

func (m *mockGroupManager) ListGroups() []string { return m.groups }
func (m *mockGroupManager) StartGroup(name string) error {
	for _, g := range m.groups {
		if g == name {
			return nil
		}
	}
	return fmt.Errorf("no such group: %s", name)
}
func (m *mockGroupManager) StopGroup(name string) error    { return m.StartGroup(name) }
func (m *mockGroupManager) RestartGroup(name string) error { return m.StartGroup(name) }

type mockConfigManager struct {
	cfg any
}

func (m *mockConfigManager) GetConfig() any { return m.cfg }
func (m *mockConfigManager) Reload() ([]string, []string, []string, error) {
	return []string{"new"}, []string{"changed"}, []string{"removed"}, nil
}

type mockDaemonInfo struct {
	shuttingDown bool
	ready        bool
}

func (m *mockDaemonInfo) IsShuttingDown() bool { return m.shuttingDown }
func (m *mockDaemonInfo) IsReady() bool        { return m.ready }
func (m *mockDaemonInfo) CheckReady(processes []string) (bool, []string, error) {
	for _, p := range processes {
		if p == "unknown" {
			return false, nil, fmt.Errorf("unknown process: %s", p)
		}
	}
	if m.ready {
		return true, nil, nil
	}
	return false, processes, nil
}
func (m *mockDaemonInfo) Version() map[string]string {
	return map[string]string{"version": "dev", "commit": "abc123"}
}
func (m *mockDaemonInfo) PID() int  { return 12345 }
func (m *mockDaemonInfo) Shutdown() {}

func testServer() (*Server, *mockProcessManager, *mockDaemonInfo) {
	pm := &mockProcessManager{
		processes: []ProcessInfo{
			{Name: "web", Group: "web", State: "RUNNING", PID: 1234, Uptime: 3600},
			{Name: "worker", Group: "worker", State: "STOPPED", PID: 0},
		},
	}
	gm := &mockGroupManager{groups: []string{"web", "worker"}}
	cm := &mockConfigManager{cfg: map[string]string{"test": "config"}}
	di := &mockDaemonInfo{ready: true}
	bus := events.NewBus(slog.New(slog.NewTextHandler(io.Discard, nil)))
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	srv := NewServer(Config{}, pm, gm, cm, di, bus, logger)
	return srv, pm, di
}

// --- Health endpoint tests ---

func TestHealthzOK(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %s", body["status"])
	}
}

func TestHealthzShuttingDown(t *testing.T) {
	srv, _, di := testServer()
	di.shuttingDown = true
	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHealthzNoAuth(t *testing.T) {
	srv, _, _ := testServer()
	srv.authUser = "admin"
	srv.authPass = "secret"

	req := httptest.NewRequest("GET", "/healthz", nil)
	req.RemoteAddr = "127.0.0.1:12345" // Simulate TCP connection.
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// /healthz should work without auth.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

// --- Readiness endpoint tests ---

func TestReadyzReady(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadyzNotReady(t *testing.T) {
	srv, _, di := testServer()
	di.ready = false
	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestReadyzWithProcessFilter(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/readyz?process=web,worker", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadyzWithUnknownProcess(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/readyz?process=unknown", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Process endpoint tests ---

func TestListProcesses(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/processes", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected application/json, got %s", ct)
	}

	var procs []ProcessInfo
	if err := json.Unmarshal(w.Body.Bytes(), &procs); err != nil {
		t.Fatal(err)
	}
	if len(procs) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(procs))
	}
}

func TestGetProcess(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/processes/web", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestGetProcessNotFound(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/processes/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %s", body["code"])
	}
}

func TestStartProcess(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/processes/worker/start", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStartProcessAlreadyRunning(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/processes/web/start", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestStopProcess(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/processes/web/stop", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStopProcessNotRunning(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/processes/worker/stop", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRestartProcess(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/processes/web/restart", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSignalProcess(t *testing.T) {
	srv, _, _ := testServer()
	body := strings.NewReader(`{"signal":"HUP"}`)
	req := httptest.NewRequest("POST", "/api/v1/processes/web/signal", body)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestSignalProcessInvalid(t *testing.T) {
	srv, _, _ := testServer()
	body := strings.NewReader(`{"signal":"INVALID"}`)
	req := httptest.NewRequest("POST", "/api/v1/processes/web/signal", body)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestSignalProcessNoBody(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/processes/web/signal", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestReadLog(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/processes/web/log/stdout", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Fatalf("expected text/plain, got %s", ct)
	}
}

func TestReadLogInvalidStream(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/processes/web/log/invalid", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Group endpoint tests ---

func TestListGroups(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/groups", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStartGroup(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/groups/web/start", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStartGroupNotFound(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/groups/nonexistent/start", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

// --- Config endpoint tests ---

func TestGetConfig(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/config", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReloadConfig(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/config/reload", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["status"] != "reloaded" {
		t.Fatalf("expected reloaded, got %v", body["status"])
	}
}

// --- Version and Shutdown tests ---

func TestVersion(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/version", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["version"] != "dev" {
		t.Fatalf("expected dev, got %s", body["version"])
	}
}

func TestShutdown(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("POST", "/api/v1/shutdown", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestStartDuringShutdown(t *testing.T) {
	srv, _, di := testServer()
	di.shuttingDown = true
	req := httptest.NewRequest("POST", "/api/v1/processes/worker/start", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

// --- Auth tests ---

func TestAuthRequiredOnTCP(t *testing.T) {
	srv, _, _ := testServer()
	srv.authUser = "admin"
	srv.authPass = "secret"

	req := httptest.NewRequest("GET", "/api/v1/processes", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w.Header().Get("WWW-Authenticate") == "" {
		t.Fatal("expected WWW-Authenticate header")
	}
}

func TestAuthValidCredentials(t *testing.T) {
	srv, _, _ := testServer()
	srv.authUser = "admin"
	srv.authPass = "secret"

	req := httptest.NewRequest("GET", "/api/v1/processes", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.SetBasicAuth("admin", "secret")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestAuthInvalidCredentials(t *testing.T) {
	srv, _, _ := testServer()
	srv.authUser = "admin"
	srv.authPass = "secret"

	req := httptest.NewRequest("GET", "/api/v1/processes", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	req.SetBasicAuth("admin", "wrong")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthSkippedOnUnixSocket(t *testing.T) {
	srv, _, _ := testServer()
	srv.authUser = "admin"
	srv.authPass = "secret"

	req := httptest.NewRequest("GET", "/api/v1/processes", nil)
	req.RemoteAddr = "" // Unix socket.
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 (auth skipped on unix), got %d", w.Code)
	}
}

// --- Content-Type tests ---

func TestAllEndpointsReturnJSON(t *testing.T) {
	srv, _, _ := testServer()
	endpoints := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/v1/processes", ""},
		{"GET", "/api/v1/processes/web", ""},
		{"POST", "/api/v1/processes/web/stop", ""},
		{"POST", "/api/v1/processes/web/restart", ""},
		{"GET", "/api/v1/groups", ""},
		{"GET", "/api/v1/config", ""},
		{"GET", "/api/v1/version", ""},
		{"GET", "/healthz", ""},
		{"GET", "/readyz", ""},
	}

	for _, ep := range endpoints {
		var body io.Reader
		if ep.body != "" {
			body = strings.NewReader(ep.body)
		}
		req := httptest.NewRequest(ep.method, ep.path, body)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)

		ct := w.Header().Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("%s %s: expected application/json, got %s", ep.method, ep.path, ct)
		}
	}
}

// --- Unix socket server tests ---

func TestUnixSocketServer(t *testing.T) {
	srv, _, _ := testServer()
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	if err := srv.StartUnix(sockPath, 0770); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Stop(context.Background()) }()

	// Verify socket exists.
	info, err := os.Stat(sockPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		t.Fatal("expected socket file")
	}

	// Make a request over the socket.
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return net.Dial("unix", sockPath)
			},
		},
	}
	resp, err := client.Get("http://unix/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestUnixSocketStaleCleanup(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "stale.sock")

	// Create a stale socket.
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	ln.Close()

	srv, _, _ := testServer()
	if err := srv.StartUnix(sockPath, 0770); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = srv.Stop(context.Background()) }()

	if srv.UnixAddr() == "" {
		t.Fatal("expected non-empty unix addr")
	}
}

func TestUnixSocketCleanupOnShutdown(t *testing.T) {
	srv, _, _ := testServer()
	// Use /tmp directly to avoid long macOS temp paths exceeding Unix socket limit.
	sockPath := filepath.Join("/tmp", fmt.Sprintf("kahi-test-%d.sock", os.Getpid()))
	t.Cleanup(func() { os.Remove(sockPath) })

	if err := srv.StartUnix(sockPath, 0700); err != nil {
		t.Fatal(err)
	}
	_ = srv.Stop(context.Background())
}

// --- TCP server tests ---

func TestTCPServer(t *testing.T) {
	srv, _, _ := testServer()
	if err := srv.StartTCP("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	addr := srv.TCPAddr()
	if addr == "" {
		t.Fatal("expected non-empty tcp addr")
	}

	resp, err := http.Get("http://" + addr + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestTCPAuthRequired(t *testing.T) {
	srv, _, _ := testServer()
	srv.authUser = "admin"
	srv.authPass = "secret"

	if err := srv.StartTCP("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	addr := srv.TCPAddr()

	// Without auth.
	resp, err := http.Get("http://" + addr + "/api/v1/processes")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	// With auth.
	req, _ := http.NewRequest("GET", "http://"+addr+"/api/v1/processes", nil)
	req.SetBasicAuth("admin", "secret")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}
}

// --- SSE tests ---

func TestEventStreamSSE(t *testing.T) {
	srv, _, _ := testServer()

	// Start TCP server for real SSE test.
	if err := srv.StartTCP("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	addr := srv.TCPAddr()

	// Connect to event stream in background.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+addr+"/api/v1/events/stream", nil)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Fatalf("expected text/event-stream, got %s", ct)
	}
	if resp.Header.Get("X-Accel-Buffering") != "no" {
		t.Fatal("expected X-Accel-Buffering: no")
	}

	// Give the SSE connection time to establish.
	time.Sleep(100 * time.Millisecond)

	// Publish an event.
	srv.bus.Publish(events.Event{
		Type: events.ProcessStateRunning,
		Data: map[string]string{"name": "web"},
	})

	// Read from the stream with a timeout.
	buf := make([]byte, 1024)
	n, _ := resp.Body.Read(buf)
	data := string(buf[:n])
	if !strings.Contains(data, "PROCESS_STATE_RUNNING") {
		t.Fatalf("expected event in SSE stream, got: %s", data)
	}
}

func TestEventStreamWithTypeFilter(t *testing.T) {
	srv, _, _ := testServer()
	if err := srv.StartTCP("127.0.0.1:0"); err != nil {
		t.Fatal(err)
	}
	defer srv.Stop(context.Background())

	addr := srv.TCPAddr()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET",
		"http://"+addr+"/api/v1/events/stream?types=PROCESS_STATE_RUNNING", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("expected text/event-stream")
	}
}

func TestLogStreamSSENotFound(t *testing.T) {
	srv, _, _ := testServer()
	req := httptest.NewRequest("GET", "/api/v1/processes/nonexistent/log/stdout/stream", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
