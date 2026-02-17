//go:build e2e

package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReload_AddProgram(t *testing.T) {
	client, socketPath := startDaemon(t, `
[programs.existing]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "existing", "RUNNING", 5*time.Second)

	// Determine config path from socket directory.
	dir := filepath.Dir(socketPath)
	configPath := filepath.Join(dir, "kahi.toml")

	// Add a new program to the config.
	newConfig := fmt.Sprintf(`[supervisor]
log_level = "debug"
shutdown_timeout = 10

[server.unix]
file = %q

[programs.existing]
command = "/bin/sleep 300"
autostart = true
startsecs = 0

[programs.newproc]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`, socketPath)
	if err := os.WriteFile(configPath, []byte(newConfig), 0644); err != nil {
		t.Fatalf("write updated config: %v", err)
	}

	// Reload.
	diff, err := client.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	t.Logf("reload diff: %v", diff)

	// New process should appear and start.
	waitForState(t, client, "newproc", "RUNNING", 10*time.Second)
}

func TestReload_RemoveProgram(t *testing.T) {
	client, socketPath := startDaemon(t, `
[programs.keeper]
command = "/bin/sleep 300"
autostart = true
startsecs = 0

[programs.goner]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "keeper", "RUNNING", 5*time.Second)
	waitForState(t, client, "goner", "RUNNING", 5*time.Second)

	// Remove goner from config.
	dir := filepath.Dir(socketPath)
	configPath := filepath.Join(dir, "kahi.toml")
	newConfig := fmt.Sprintf(`[supervisor]
log_level = "debug"
shutdown_timeout = 10

[server.unix]
file = %q

[programs.keeper]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`, socketPath)
	if err := os.WriteFile(configPath, []byte(newConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	diff, err := client.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	t.Logf("reload diff: %v", diff)

	// goner should no longer be in the process list.
	time.Sleep(2 * time.Second)
	_, err = getProcessInfo(client, "goner")
	if err == nil {
		t.Fatal("goner should have been removed after reload")
	}

	// keeper should still be running.
	info, _ := getProcessInfo(client, "keeper")
	if info.State != "RUNNING" {
		t.Fatalf("keeper state = %s, want RUNNING", info.State)
	}
}

func TestReload_ChangeProgram(t *testing.T) {
	client, socketPath := startDaemon(t, `
[programs.mutable]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "mutable", "RUNNING", 5*time.Second)
	info1, _ := getProcessInfo(client, "mutable")

	// Change the command.
	dir := filepath.Dir(socketPath)
	configPath := filepath.Join(dir, "kahi.toml")
	newConfig := fmt.Sprintf(`[supervisor]
log_level = "debug"
shutdown_timeout = 10

[server.unix]
file = %q

[programs.mutable]
command = "/bin/sleep 600"
autostart = true
startsecs = 0
`, socketPath)
	if err := os.WriteFile(configPath, []byte(newConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	diff, err := client.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	t.Logf("reload diff: %v", diff)

	// Process should be restarted with new command.
	waitForState(t, client, "mutable", "RUNNING", 10*time.Second)
	info2, _ := getProcessInfo(client, "mutable")
	if info2.PID == info1.PID {
		t.Fatal("PID did not change after config change and reload")
	}
}

func TestReload_Reread(t *testing.T) {
	client, socketPath := startDaemon(t, `
[programs.stable]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "stable", "RUNNING", 5*time.Second)

	// Add a new program to config without reloading.
	dir := filepath.Dir(socketPath)
	configPath := filepath.Join(dir, "kahi.toml")
	newConfig := fmt.Sprintf(`[supervisor]
log_level = "debug"
shutdown_timeout = 10

[server.unix]
file = %q

[programs.stable]
command = "/bin/sleep 300"
autostart = true
startsecs = 0

[programs.preview]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`, socketPath)
	if err := os.WriteFile(configPath, []byte(newConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	// Reread should show diff without applying.
	diff, err := client.Reread()
	if err != nil {
		t.Fatalf("reread: %v", err)
	}
	t.Logf("reread diff: %v", diff)

	// preview should NOT be running yet (reread only previews).
	_, err = getProcessInfo(client, "preview")
	if err == nil {
		info, _ := getProcessInfo(client, "preview")
		if info.State == "RUNNING" {
			t.Fatal("preview should not be running after reread (only preview, not apply)")
		}
	}
}

func TestReload_NoChange(t *testing.T) {
	client, _ := startDaemon(t, `
[programs.static]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "static", "RUNNING", 5*time.Second)

	info1, _ := getProcessInfo(client, "static")

	// Reload without changing anything.
	diff, err := client.Reload()
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	t.Logf("no-change reload diff: %v", diff)

	// Process should still be running with same PID.
	info2, _ := getProcessInfo(client, "static")
	if info2.PID != info1.PID {
		t.Fatal("PID changed after no-change reload")
	}
}

func TestReload_InvalidConfig(t *testing.T) {
	client, socketPath := startDaemon(t, `
[programs.safe]
command = "/bin/sleep 300"
autostart = true
startsecs = 0
`)
	waitForState(t, client, "safe", "RUNNING", 5*time.Second)

	info1, _ := getProcessInfo(client, "safe")

	// Write invalid TOML.
	dir := filepath.Dir(socketPath)
	configPath := filepath.Join(dir, "kahi.toml")
	if err := os.WriteFile(configPath, []byte("{{invalid toml"), 0644); err != nil {
		t.Fatalf("write bad config: %v", err)
	}

	// Reload should fail.
	_, err := client.Reload()
	if err == nil {
		t.Fatal("expected error reloading invalid config")
	}
	t.Logf("reload error (expected): %v", err)

	// Original process should still be running.
	info2, _ := getProcessInfo(client, "safe")
	if info2.State != "RUNNING" {
		t.Fatalf("state = %s, want RUNNING after failed reload", info2.State)
	}
	if info2.PID != info1.PID {
		t.Fatal("PID changed after failed reload")
	}
}
