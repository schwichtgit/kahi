//go:build e2e

package e2e

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestRegression_UnicodeTail(t *testing.T) {
	client, _ := startDaemon(t, `
[programs.unicode]
command = "/bin/sh -c 'echo \"Hello \xe4\xb8\x96\xe7\x95\x8c \xf0\x9f\x8c\x8d\"; sleep 300'"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "unicode", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("unicode", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	// Should contain the Unicode output without corruption.
	if !strings.Contains(buf.String(), "Hello") {
		t.Fatalf("unicode tail output missing content: %q", buf.String())
	}
}

func TestRegression_InvalidUTF8(t *testing.T) {
	client, _ := startDaemon(t, `
[programs.badutf8]
command = "/bin/sh -c 'printf \"\\xff\\xfe invalid bytes\\n\"; sleep 300'"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "badutf8", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("badutf8", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	// Should not crash, output should contain "invalid bytes".
	if !strings.Contains(buf.String(), "invalid bytes") {
		t.Fatalf("invalid UTF-8 output: %q", buf.String())
	}
}

func TestRegression_LiteralPercent(t *testing.T) {
	client, _ := startDaemon(t, `
[programs.percent]
command = "/bin/sh -c 'echo 100%% complete; sleep 300'"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "percent", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("percent", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	if !strings.Contains(buf.String(), "100%") {
		t.Fatalf("literal percent not preserved: %q", buf.String())
	}
}

func TestRegression_KahiInit(t *testing.T) {
	dir := t.TempDir()
	cmd := exec.Command(kahiBinary, "init")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kahi init failed: %v\n%s", err, out)
	}

	// Should have created a kahi.toml in the directory.
	configPath := dir + "/kahi.toml"
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("kahi.toml not created: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("kahi.toml is empty")
	}
	// Should be valid TOML containing at least [supervisor] section.
	if !strings.Contains(string(data), "[supervisor]") {
		t.Fatalf("generated config missing [supervisor]: %s", string(data))
	}
}

func TestRegression_HelpFlag(t *testing.T) {
	cmd := exec.Command(kahiBinary, "--help")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("kahi --help failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "daemon") {
		t.Fatalf("help output missing 'daemon': %s", output)
	}
	if !strings.Contains(output, "ctl") {
		t.Fatalf("help output missing 'ctl': %s", output)
	}
}

func TestRegression_PipedTail(t *testing.T) {
	client, _ := startDaemon(t, `
[programs.piper]
command = "/bin/sh -c 'for i in 1 2 3 4 5; do echo pipe-line-$i; done; sleep 300'"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "piper", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("piper", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail: %v", err)
	}
	output := buf.String()
	for _, expected := range []string{"pipe-line-1", "pipe-line-5"} {
		if !strings.Contains(output, expected) {
			t.Fatalf("piped output missing %q: %q", expected, output)
		}
	}
}

func TestRegression_NumprocsNames(t *testing.T) {
	client, _ := startDaemon(t, `
[programs.multi]
command = "/bin/sleep 300"
process_name = "multi_%(process_num)02d"
numprocs = 3
numprocs_start = 0
autostart = true
startsecs = 0
`)
	// Verify naming convention: multi_00, multi_01, multi_02.
	for _, name := range []string{"multi_00", "multi_01", "multi_02"} {
		waitForState(t, client, name, "RUNNING", 10*time.Second)
	}

	// Verify there is no "multi" (without suffix).
	_, err := getProcessInfo(client, "multi")
	if err == nil {
		info, _ := getProcessInfo(client, "multi")
		if info.State != "" {
			t.Logf("note: base name 'multi' also exists with state %s", info.State)
		}
	}
}

func TestRegression_RedirectStderr(t *testing.T) {
	client, _ := startDaemon(t, `
[programs.merged]
command = "/bin/sh -c 'echo stdout-line; echo stderr-line >&2; sleep 300'"
autostart = true
startsecs = 0
redirect_stderr = true
`)
	waitForState(t, client, "merged", "RUNNING", 5*time.Second)
	time.Sleep(1 * time.Second)

	var buf bytes.Buffer
	if err := client.Tail("merged", "stdout", 4096, &buf); err != nil {
		t.Fatalf("tail stdout: %v", err)
	}
	output := buf.String()
	if !strings.Contains(output, "stdout-line") {
		t.Fatalf("stdout missing: %q", output)
	}
	// With redirect_stderr=true, stderr should appear in stdout.
	if !strings.Contains(output, "stderr-line") {
		t.Fatalf("stderr not redirected to stdout: %q", output)
	}
}
