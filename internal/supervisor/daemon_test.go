package supervisor

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestWritePIDFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kahi.pid")

	if err := WritePIDFile(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("pid file content %q is not a number", pidStr)
	}
	if pid != os.Getpid() {
		t.Fatalf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestWritePIDFileEmpty(t *testing.T) {
	// Empty path should be a no-op.
	if err := WritePIDFile(""); err != nil {
		t.Fatal(err)
	}
}

func TestRemovePIDFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kahi.pid")

	_ = WritePIDFile(path)
	RemovePIDFile(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("pid file should be removed")
	}
}

func TestRemovePIDFileEmpty(t *testing.T) {
	// Should not panic.
	RemovePIDFile("")
}

func TestValidateSocketPermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "kahi.sock")

	err := ValidateSocketPermissions(path)
	if err != nil {
		t.Fatal(err)
	}
}

func TestValidateSocketPermissionsNonexistentDir(t *testing.T) {
	err := ValidateSocketPermissions("/nonexistent/dir/kahi.sock")
	if err == nil {
		t.Fatal("expected error for nonexistent directory")
	}
}

func TestValidateUnprivileged(t *testing.T) {
	// Just verify it doesn't error for non-root.
	original := getuid
	getuid = func() int { return 1000 }
	defer func() { getuid = original }()

	if err := ValidateUnprivileged(testLogger()); err != nil {
		t.Fatal(err)
	}
}

func TestResolveUser(t *testing.T) {
	uid, gid, err := resolveUser("1000:1000")
	if err != nil {
		t.Fatal(err)
	}
	if uid != 1000 || gid != 1000 {
		t.Fatalf("uid=%d, gid=%d, want 1000:1000", uid, gid)
	}
}

func TestResolveUserNoGroup(t *testing.T) {
	uid, gid, err := resolveUser("1000")
	if err != nil {
		t.Fatal(err)
	}
	if uid != 1000 || gid != 1000 {
		t.Fatalf("uid=%d, gid=%d, want 1000:1000", uid, gid)
	}
}

func TestResolveUserInvalid(t *testing.T) {
	_, _, err := resolveUser("notanumber")
	if err == nil {
		t.Fatal("expected error for invalid uid")
	}
}
