package process

// Platform-specific RLIMIT constants for darwin.
const (
	rlimitNproc = 7 // RLIMIT_NPROC on macOS
	rlimitAS    = 5 // RLIMIT_AS on macOS
	rlimitRSS   = 5 // RLIMIT_RSS not in darwin syscall, use RLIMIT_AS
)
