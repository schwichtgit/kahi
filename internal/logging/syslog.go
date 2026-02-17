package logging

import (
	"fmt"
	"log/syslog"
)

// SyslogForwarder sends process output to syslog.
type SyslogForwarder struct {
	writer *syslog.Writer
	tag    string
}

// NewSyslogForwarder creates a syslog forwarder for a process.
func NewSyslogForwarder(tag string) (*SyslogForwarder, error) {
	w, err := syslog.New(syslog.LOG_INFO|syslog.LOG_DAEMON, tag)
	if err != nil {
		return nil, fmt.Errorf("cannot connect to syslog: %w", err)
	}
	return &SyslogForwarder{writer: w, tag: tag}, nil
}

// Write sends data to syslog.
func (sf *SyslogForwarder) Write(p []byte) (int, error) {
	if err := sf.writer.Info(string(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close closes the syslog connection.
func (sf *SyslogForwarder) Close() error {
	return sf.writer.Close()
}
