package process

import (
	"testing"
)

func TestParseCredentialEmpty(t *testing.T) {
	cred, err := ParseCredential("")
	if err != nil {
		t.Fatal(err)
	}
	if cred != nil {
		t.Fatal("expected nil credential for empty string")
	}
}

func TestParseCredentialUIDOnly(t *testing.T) {
	cred, err := ParseCredential("1000")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Uid != 1000 {
		t.Fatalf("uid = %d, want 1000", cred.Uid)
	}
	if cred.Gid != 1000 {
		t.Fatalf("gid = %d, want 1000 (default to uid)", cred.Gid)
	}
}

func TestParseCredentialUIDGID(t *testing.T) {
	cred, err := ParseCredential("1000:2000")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Uid != 1000 {
		t.Fatalf("uid = %d, want 1000", cred.Uid)
	}
	if cred.Gid != 2000 {
		t.Fatalf("gid = %d, want 2000", cred.Gid)
	}
}

func TestParseCredentialRoot(t *testing.T) {
	cred, err := ParseCredential("0:0")
	if err != nil {
		t.Fatal(err)
	}
	if cred.Uid != 0 || cred.Gid != 0 {
		t.Fatalf("uid=%d gid=%d, want 0:0", cred.Uid, cred.Gid)
	}
}

func TestParseCredentialInvalidUID(t *testing.T) {
	_, err := ParseCredential("notanumber")
	if err == nil {
		t.Fatal("expected error for invalid uid")
	}
}

func TestParseCredentialInvalidGID(t *testing.T) {
	_, err := ParseCredential("1000:abc")
	if err == nil {
		t.Fatal("expected error for invalid gid")
	}
}

func TestBuildSysProcAttrEmpty(t *testing.T) {
	attr, err := BuildSysProcAttr("")
	if err != nil {
		t.Fatal(err)
	}
	if !attr.Setpgid {
		t.Fatal("expected Setpgid=true")
	}
	if attr.Credential != nil {
		t.Fatal("expected nil credential for empty user")
	}
}

func TestBuildSysProcAttrWithUser(t *testing.T) {
	attr, err := BuildSysProcAttr("1000:1000")
	if err != nil {
		t.Fatal(err)
	}
	if !attr.Setpgid {
		t.Fatal("expected Setpgid=true")
	}
	if attr.Credential == nil {
		t.Fatal("expected non-nil credential")
	}
	if attr.Credential.Uid != 1000 {
		t.Fatalf("uid = %d, want 1000", attr.Credential.Uid)
	}
}

func TestBuildSysProcAttrInvalid(t *testing.T) {
	_, err := BuildSysProcAttr("invalid")
	if err == nil {
		t.Fatal("expected error for invalid user")
	}
}
