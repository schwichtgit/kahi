// Package fcgi provides FastCGI protocol support for the Kahi supervisor.
package fcgi

import (
	"fmt"
	"net"
	"os"
	"sync"
)

// Protocol specifies the FastCGI socket protocol.
type Protocol string

const (
	ProtocolTCP  Protocol = "tcp"
	ProtocolUnix Protocol = "unix"
)

// ProgramConfig holds FastCGI-specific configuration for a program.
type ProgramConfig struct {
	SocketPath  string      // Unix socket path or TCP address
	Protocol    Protocol    // "tcp" or "unix"
	SocketOwner string      // chown target (user:group)
	SocketMode  os.FileMode // chmod for socket
}

// Socket manages a FastCGI listener socket.
type Socket struct {
	mu       sync.Mutex
	config   ProgramConfig
	listener net.Listener
	fd       *os.File
}

// NewSocket creates a FastCGI socket from config.
func NewSocket(cfg ProgramConfig) *Socket {
	return &Socket{config: cfg}
}

// Open creates and binds the socket. The resulting file descriptor
// should be passed to the child process via ExtraFiles.
func (s *Socket) Open() (*os.File, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.listener != nil {
		return s.fd, nil
	}

	var ln net.Listener
	var err error

	switch s.config.Protocol {
	case ProtocolUnix:
		// Remove stale socket.
		os.Remove(s.config.SocketPath)
		ln, err = net.Listen("unix", s.config.SocketPath)
		if err != nil {
			return nil, fmt.Errorf("cannot create FastCGI socket: %s: %w", s.config.SocketPath, err)
		}
		if s.config.SocketMode != 0 {
			if err := os.Chmod(s.config.SocketPath, s.config.SocketMode); err != nil {
				ln.Close()
				return nil, fmt.Errorf("cannot chmod FastCGI socket: %s: %w", s.config.SocketPath, err)
			}
		}
	case ProtocolTCP:
		ln, err = net.Listen("tcp", s.config.SocketPath)
		if err != nil {
			return nil, fmt.Errorf("cannot bind FastCGI socket: %s: %w", s.config.SocketPath, err)
		}
	default:
		return nil, fmt.Errorf("unknown FastCGI protocol: %s", s.config.Protocol)
	}

	s.listener = ln

	// Get the file descriptor for the listener.
	switch l := ln.(type) {
	case *net.TCPListener:
		f, err := l.File()
		if err != nil {
			ln.Close()
			return nil, fmt.Errorf("cannot get socket fd: %w", err)
		}
		s.fd = f
	case *net.UnixListener:
		f, err := l.File()
		if err != nil {
			ln.Close()
			return nil, fmt.Errorf("cannot get socket fd: %w", err)
		}
		s.fd = f
	}

	return s.fd, nil
}

// Close cleans up the socket.
func (s *Socket) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.fd != nil {
		s.fd.Close()
		s.fd = nil
	}
	if s.listener != nil {
		err := s.listener.Close()
		s.listener = nil
		// Clean up Unix socket file.
		if s.config.Protocol == ProtocolUnix {
			os.Remove(s.config.SocketPath)
		}
		return err
	}
	return nil
}

// Addr returns the listener address, or empty if not open.
func (s *Socket) Addr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}
