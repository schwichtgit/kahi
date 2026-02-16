package process

import (
	"fmt"
	"strconv"
	"strings"
	"syscall"
)

// ParseCredential parses a "uid:gid" or "uid" string into SysProcAttr Credential.
func ParseCredential(user string) (*syscall.Credential, error) {
	if user == "" {
		return nil, nil
	}

	parts := strings.SplitN(user, ":", 2)
	uid, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid uid in user %q: %w", user, err)
	}

	gid := uid // default gid = uid
	if len(parts) > 1 {
		gid, err = strconv.ParseUint(parts[1], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid gid in user %q: %w", user, err)
		}
	}

	return &syscall.Credential{
		Uid: uint32(uid),
		Gid: uint32(gid),
	}, nil
}

// BuildSysProcAttr creates SysProcAttr with process group isolation
// and optional credential switching.
func BuildSysProcAttr(user string) (*syscall.SysProcAttr, error) {
	attr := &syscall.SysProcAttr{
		Setpgid: true,
	}

	cred, err := ParseCredential(user)
	if err != nil {
		return nil, err
	}
	if cred != nil {
		attr.Credential = cred
	}

	return attr, nil
}
