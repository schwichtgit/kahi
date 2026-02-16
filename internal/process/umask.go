package process

import (
	"fmt"
	"strconv"
	"syscall"
)

// ParseUmask parses a umask string (octal) into an integer.
// Returns -1 if the string is empty (meaning inherit parent umask).
func ParseUmask(s string) (int, error) {
	if s == "" {
		return -1, nil
	}
	val, err := strconv.ParseInt(s, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid umask %q: %w", s, err)
	}
	if val < 0 || val > 0777 {
		return 0, fmt.Errorf("umask %q out of range (must be 0-0777)", s)
	}
	return int(val), nil
}

// ApplyUmask sets the process umask. Returns the previous umask.
func ApplyUmask(mask int) int {
	if mask < 0 {
		return 0 // no-op, inherit
	}
	return syscall.Umask(mask)
}
