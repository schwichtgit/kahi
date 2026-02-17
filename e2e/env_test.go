//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestEnv_Passthrough(t *testing.T) {
	os.Setenv("KAHI_TEST_VAR", "passthrough-value")
	defer os.Unsetenv("KAHI_TEST_VAR")

	dir := t.TempDir()
	script := writeScript(t, dir, "envcheck.sh", "echo KAHI_TEST_VAR=$KAHI_TEST_VAR\nexec sleep 300")

	client, _ := startDaemon(t, `
[programs.envcheck]
command = "`+script+`"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "envcheck", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("envcheck", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	if !strings.Contains(buf.String(), "KAHI_TEST_VAR=passthrough-value") {
		t.Fatalf("env not passed through: %q", buf.String())
	}
}

func TestEnv_CleanEnvironment(t *testing.T) {
	os.Setenv("KAHI_CLEAN_TEST", "should-not-appear")
	defer os.Unsetenv("KAHI_CLEAN_TEST")

	dir := t.TempDir()
	script := writeScript(t, dir, "clean.sh", "echo CLEAN=$KAHI_CLEAN_TEST\necho CONFIGURED=$MY_VAR\nexec sleep 300")

	client, _ := startDaemon(t, `
[programs.clean]
command = "`+script+`"
autostart = true
startsecs = 0
clean_environment = true

[programs.clean.environment]
MY_VAR = "configured-value"
`)
	waitForState(t, client, "clean", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("clean", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	output := buf.String()
	if strings.Contains(output, "CLEAN=should-not-appear") {
		t.Fatalf("parent env leaked through clean_environment: %q", output)
	}
	if !strings.Contains(output, "CONFIGURED=configured-value") {
		t.Fatalf("configured env missing: %q", output)
	}
}

func TestEnv_ProgramOverridesGlobal(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "override.sh", "echo SHARED=$SHARED_VAR\nexec sleep 300")

	client, _ := startDaemon(t, `
[supervisor.environment]
SHARED_VAR = "global-value"

[programs.override]
command = "`+script+`"
autostart = true
startsecs = 0

[programs.override.environment]
SHARED_VAR = "program-value"
`)
	waitForState(t, client, "override", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("override", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	if !strings.Contains(buf.String(), "SHARED=program-value") {
		t.Fatalf("program env did not override global: %q", buf.String())
	}
}

func TestEnv_ProgramNameExpansion(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "expander.sh", "echo NAME=$PROC_NAME NUM=$PROC_NUM\nexec sleep 300")

	client, _ := startDaemon(t, `
[programs.expander]
command = "`+script+`"
process_name = "expander_%(process_num)d"
numprocs = 1
autostart = true
startsecs = 0

[programs.expander.environment]
PROC_NAME = "%(program_name)s"
PROC_NUM = "%(process_num)d"
`)
	waitForState(t, client, "expander_0", "RUNNING", 10*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("expander_0", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "NAME=expander") {
		t.Fatalf("%%(program_name)s not expanded: %q", output)
	}
	if !strings.Contains(output, "NUM=0") {
		t.Fatalf("%%(process_num)d not expanded: %q", output)
	}
}

func TestEnv_HereExpansion(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "here.sh", "echo HERE=$HERE_DIR\nexec sleep 300")

	client, socketPath := startDaemon(t, `
[programs.here]
command = "`+script+`"
autostart = true
startsecs = 0

[programs.here.environment]
HERE_DIR = "%(here)s"
`)
	waitForState(t, client, "here", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("here", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}

	configDir := filepath.Dir(socketPath)
	if !strings.Contains(buf.String(), "HERE="+configDir) {
		t.Fatalf("%%(here)s not expanded correctly: %q, want dir=%s", buf.String(), configDir)
	}
}
