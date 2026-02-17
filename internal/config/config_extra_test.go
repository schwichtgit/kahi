package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandStringTemplateVars(t *testing.T) {
	ctx := ExpandContext{
		Here:        "/etc/kahi",
		ProgramName: "worker",
		ProcessNum:  3,
		GroupName:   "workers",
		NumProcs:    5,
	}

	result, err := ExpandString("%(here)s/logs/%(program_name)s-%(process_num)d.log", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "/etc/kahi/logs/worker-3.log" {
		t.Fatalf("result = %q, want /etc/kahi/logs/worker-3.log", result)
	}
}

func TestExpandStringEnvVars(t *testing.T) {
	t.Setenv("KAHI_EXTRA_TEST_VAR", "myvalue")

	ctx := ExpandContext{Here: "/etc"}
	result, err := ExpandString("prefix-${KAHI_EXTRA_TEST_VAR}-suffix", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "prefix-myvalue-suffix" {
		t.Fatalf("result = %q, want prefix-myvalue-suffix", result)
	}
}

func TestExpandStringEmpty(t *testing.T) {
	ctx := ExpandContext{}
	result, err := ExpandString("", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("result = %q, want empty", result)
	}
}

func TestExpandStringNumprocs(t *testing.T) {
	ctx := ExpandContext{NumProcs: 8}
	result, err := ExpandString("%(numprocs)d workers", ctx)
	if err != nil {
		t.Fatal(err)
	}
	if result != "8 workers" {
		t.Fatalf("result = %q, want '8 workers'", result)
	}
}

func TestExpandStringUnclosedTemplate(t *testing.T) {
	ctx := ExpandContext{}
	_, err := ExpandString("%(unclosed", ctx)
	if err == nil {
		t.Fatal("expected error for unclosed template")
	}
}

func TestExpandStringUnclosedEnvVar(t *testing.T) {
	ctx := ExpandContext{}
	_, err := ExpandString("${UNCLOSED", ctx)
	if err == nil {
		t.Fatal("expected error for unclosed env var")
	}
}

func TestLoadWithIncludesHappyPath(t *testing.T) {
	dir := t.TempDir()

	mainCfg := `
include = ["conf.d/*.toml"]

[supervisor]
log_level = "info"
`
	confDir := filepath.Join(dir, "conf.d")
	if err := os.MkdirAll(confDir, 0755); err != nil {
		t.Fatal(err)
	}

	webCfg := `[programs.web]
command = "/usr/bin/web"
autostart = true
autorestart = "unexpected"
stopsignal = "TERM"
stopwaitsecs = 10
exitcodes = [0]
`
	if err := os.WriteFile(filepath.Join(confDir, "web.toml"), []byte(webCfg), 0644); err != nil {
		t.Fatal(err)
	}

	mainPath := filepath.Join(dir, "kahi.toml")
	if err := os.WriteFile(mainPath, []byte(mainCfg), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, _, err := LoadWithIncludes(mainPath)
	if err != nil {
		t.Fatal(err)
	}

	if _, ok := cfg.Programs["web"]; !ok {
		t.Fatal("expected program 'web' after include")
	}
}

func TestLoadWithIncludesNonexistentFile(t *testing.T) {
	_, _, err := LoadWithIncludes("/nonexistent/kahi.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent config")
	}
}

func TestLoadWithIncludesInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(path, []byte("[[invalid"), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := LoadWithIncludes(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
}

func TestMergeGroupsNilInit(t *testing.T) {
	dst := &Config{
		Programs: map[string]ProgramConfig{},
	}
	src := &Config{
		Groups: map[string]GroupConfig{
			"web": {Programs: []string{"web1"}},
		},
	}

	mergeGroups(dst, src)

	if dst.Groups == nil {
		t.Fatal("expected groups map to be initialized")
	}
	if _, ok := dst.Groups["web"]; !ok {
		t.Fatal("expected group 'web'")
	}
}

func TestMergeWebhooksNilInit(t *testing.T) {
	dst := &Config{}
	src := &Config{
		Webhooks: map[string]WebhookConfig{
			"notify": {URL: "https://example.com/hook"},
		},
	}

	mergeWebhooks(dst, src)

	if dst.Webhooks == nil {
		t.Fatal("expected webhooks map to be initialized")
	}
	if _, ok := dst.Webhooks["notify"]; !ok {
		t.Fatal("expected webhook 'notify'")
	}
}

func TestLoadInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "invalid.toml")
	if err := os.WriteFile(path, []byte("not valid toml [[["), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid TOML")
	}
	if !strings.Contains(err.Error(), "parse error") {
		t.Fatalf("error = %q, want parse error", err)
	}
}

func TestLoadNonexistentFile(t *testing.T) {
	_, _, err := Load("/nonexistent/file.toml")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestExpandVariablesServerField(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Unix: UnixServerConfig{
				File: "%(here)s/kahi.sock",
			},
		},
		Programs: make(map[string]ProgramConfig),
	}

	err := ExpandVariables(cfg, "/etc/kahi/kahi.toml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Unix.File != "/etc/kahi/kahi.sock" {
		t.Fatalf("server.unix.file = %q, want /etc/kahi/kahi.sock", cfg.Server.Unix.File)
	}
}

func TestExpandVariablesUserField(t *testing.T) {
	t.Setenv("KAHI_USER_TEST", "appuser")

	cfg := &Config{
		Programs: map[string]ProgramConfig{
			"web": {User: "${KAHI_USER_TEST}"},
		},
	}

	err := ExpandVariables(cfg, "/etc/kahi.toml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Programs["web"].User != "appuser" {
		t.Fatalf("user = %q, want appuser", cfg.Programs["web"].User)
	}
}
