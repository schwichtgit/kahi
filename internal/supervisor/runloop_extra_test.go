package supervisor

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/kahidev/kahi/internal/config"
	"github.com/kahidev/kahi/internal/events"
	"github.com/kahidev/kahi/internal/logging"
	"github.com/kahidev/kahi/internal/process"
)

func testSupervisorWithMock(cfg *config.Config, spawner *process.MockSpawner) *Supervisor {
	logger := discardLogger()
	bus := events.NewBus(logger)
	mgr := process.NewManager(spawner, bus, logger)

	return &Supervisor{
		config:     cfg,
		configPath: "/nonexistent/kahi.toml",
		manager:    mgr,
		bus:        bus,
		logger:     logger,
		captures:   make(map[string]*logging.CaptureWriter),
		shutdownCh: make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

func TestWaitForShutdownTimeout(t *testing.T) {
	autostart := true
	cfg := &config.Config{
		Supervisor: config.SupervisorConfig{ShutdownTimeout: 1},
		Programs: map[string]config.ProgramConfig{
			"sleeper": {
				Command:      "/bin/sleep 3600",
				Numprocs:     1,
				Priority:     999,
				Autostart:    &autostart,
				Autorestart:  "false",
				Startsecs:    0,
				Startretries: 0,
				Exitcodes:    []int{0},
				Stopsignal:   "TERM",
				Stopwaitsecs: 10,
			},
		},
	}

	spawner := &process.MockSpawner{}
	s := testSupervisorWithMock(cfg, spawner)
	s.manager.LoadConfig(cfg)

	for _, p := range s.manager.Processes() {
		if err := p.Start(); err != nil {
			t.Fatalf("start failed: %v", err)
		}
	}

	start := time.Now()
	s.waitForShutdown()
	elapsed := time.Since(start)

	if elapsed < 900*time.Millisecond {
		t.Fatalf("waitForShutdown returned too early: %v", elapsed)
	}
	if elapsed > 3*time.Second {
		t.Fatalf("waitForShutdown took too long: %v", elapsed)
	}
}

func TestWaitForShutdownWithStoppedProcesses(t *testing.T) {
	autostart := false
	cfg := &config.Config{
		Supervisor: config.SupervisorConfig{ShutdownTimeout: 5},
		Programs: map[string]config.ProgramConfig{
			"worker": {
				Command:      "/bin/echo",
				Numprocs:     1,
				Priority:     999,
				Autostart:    &autostart,
				Autorestart:  "false",
				Stopsignal:   "TERM",
				Stopwaitsecs: 10,
				Exitcodes:    []int{0},
			},
		},
	}

	spawner := &process.MockSpawner{}
	s := testSupervisorWithMock(cfg, spawner)
	s.manager.LoadConfig(cfg)

	start := time.Now()
	s.waitForShutdown()
	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Fatalf("waitForShutdown with stopped processes took %v", elapsed)
	}
}

func TestHandleReloadDuringShutdown(t *testing.T) {
	s := testSupervisor()
	s.shutting = true
	s.handleReload()

	if _, ok := s.config.Programs["web"]; !ok {
		t.Fatal("config should not change during shutdown reload")
	}
}

func TestHandleReloadConfigLoadError(t *testing.T) {
	s := testSupervisor()
	s.handleReload()

	if _, ok := s.config.Programs["web"]; !ok {
		t.Fatal("config should not change after failed reload")
	}
}

func TestHandleSignalSIGHUP(t *testing.T) {
	s := testSupervisor()
	s.manager.LoadConfig(s.config)

	if s.handleSignal(syscall.SIGHUP) {
		t.Fatal("SIGHUP should not trigger shutdown")
	}
}

func TestReloadErrorPath(t *testing.T) {
	s := testSupervisor()
	added, changed, removed, err := s.Reload()
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
	if added != nil || changed != nil || removed != nil {
		t.Fatal("expected nil slices on error")
	}
}

func TestReloadSuccessPath(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "kahi.toml")

	content := `[supervisor]
shutdown_timeout = 5

[programs.web]
command = "/bin/echo hello"
numprocs = 1
priority = 999
autorestart = "false"
stopsignal = "TERM"
stopwaitsecs = 10
exitcodes = [0]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Load initial config from the same file so both sides of ConfigDiff match.
	cfg, _, err := config.LoadWithIncludes(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	s := New(SupervisorConfig{
		Config:     cfg,
		ConfigPath: cfgPath,
		Logger:     discardLogger(),
	})

	added, changed, removed, err := s.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 0 || len(changed) != 0 || len(removed) != 0 {
		t.Fatalf("expected no diff, got added=%v changed=%v removed=%v", added, changed, removed)
	}
}

func TestReapAllZombies(t *testing.T) {
	ReapAllZombies(discardLogger())
}

func TestRunPIDFileError(t *testing.T) {
	cfg := &config.Config{
		Supervisor: config.SupervisorConfig{ShutdownTimeout: 1},
		Programs:   map[string]config.ProgramConfig{},
	}

	s := New(SupervisorConfig{
		Config:  cfg,
		PIDFile: "/nonexistent/dir/kahi.pid",
		Logger:  discardLogger(),
	})

	err := s.Run()
	if err == nil {
		t.Fatal("expected error for invalid PID file path")
	}
}

func TestHandleLogReopenWithCaptures(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	cw, err := logging.NewCaptureWriter(logging.CaptureConfig{
		ProcessName: "test-proc",
		Stream:      "stdout",
		Logfile:     logPath,
		Logger:      discardLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cw.Close()

	cw.Write([]byte("before\n"))

	s := testSupervisor()
	s.captures["test-proc:stdout"] = cw

	s.handleLogReopen()

	cw.Write([]byte("after\n"))

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty log after reopen")
	}
}

func TestHandleLogReopenWithBadCapture(t *testing.T) {
	tmpDir := t.TempDir()
	subdir := filepath.Join(tmpDir, "subdir")
	os.MkdirAll(subdir, 0755)
	logPath := filepath.Join(subdir, "test.log")

	cw, err := logging.NewCaptureWriter(logging.CaptureConfig{
		ProcessName: "bad-proc",
		Stream:      "stdout",
		Logfile:     logPath,
		Logger:      discardLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	defer cw.Close()

	os.RemoveAll(subdir)

	s := testSupervisor()
	s.captures["bad-proc:stdout"] = cw
	s.handleLogReopen()
}

func TestRunClosesCaptures(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "capture.log")

	cfg := &config.Config{
		Supervisor: config.SupervisorConfig{ShutdownTimeout: 1},
		Programs:   map[string]config.ProgramConfig{},
	}

	s := New(SupervisorConfig{
		Config: cfg,
		Logger: discardLogger(),
	})

	cw, err := logging.NewCaptureWriter(logging.CaptureConfig{
		ProcessName: "cap-test",
		Stream:      "stdout",
		Logfile:     logPath,
		Logger:      discardLogger(),
	})
	if err != nil {
		t.Fatal(err)
	}
	s.captures["cap-test:stdout"] = cw

	go func() {
		time.Sleep(50 * time.Millisecond)
		s.Shutdown()
	}()

	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
}

func TestHandleReloadWithValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "kahi.toml")

	content := `[supervisor]
shutdown_timeout = 5

[programs.api]
command = "/bin/api"
numprocs = 1
priority = 100
autorestart = "false"
stopsignal = "TERM"
stopwaitsecs = 10
exitcodes = [0]
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	autostart := true
	cfg := &config.Config{
		Supervisor: config.SupervisorConfig{ShutdownTimeout: 5},
		Programs: map[string]config.ProgramConfig{
			"web": {
				Command:      "/bin/echo hello",
				Numprocs:     1,
				Priority:     999,
				Autostart:    &autostart,
				Autorestart:  "unexpected",
				Startsecs:    0,
				Startretries: 3,
				Exitcodes:    []int{0},
				Stopsignal:   "TERM",
				Stopwaitsecs: 10,
			},
		},
	}

	spawner := &process.MockSpawner{}
	logger := discardLogger()
	bus := events.NewBus(logger)
	mgr := process.NewManager(spawner, bus, logger)

	s := &Supervisor{
		config:     cfg,
		configPath: cfgPath,
		manager:    mgr,
		bus:        bus,
		logger:     logger,
		captures:   make(map[string]*logging.CaptureWriter),
		shutdownCh: make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
	s.manager.LoadConfig(cfg)

	s.handleReload()

	if _, ok := s.config.Programs["api"]; !ok {
		t.Fatal("expected 'api' program after reload")
	}
}
