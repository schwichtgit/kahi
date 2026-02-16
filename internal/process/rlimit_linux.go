package process

// Platform-specific RLIMIT constants for linux.
const (
	rlimitNproc = 6 // RLIMIT_NPROC
	rlimitAS    = 9 // RLIMIT_AS
	rlimitRSS   = 5 // RLIMIT_RSS
)
