# Kahi -- Process Supervisor for POSIX Systems

> The canonical, actively developed codebase lives at:
> https://github.com/kahiteam/kahi

Kahi is a modern, lightweight process supervisor for Linux and macOS. It manages long-running processes with automatic restart, health monitoring, structured logging, and a REST/JSON API. Single static binary, zero runtime dependencies.

Kahi is modelled after Python's [supervisord](http://supervisord.org/) and includes a migration tool that converts supervisord.conf files to Kahi's TOML format.

---

## My Role

I started Kahi as a personal project and continue to drive it under the kahiteam organization.

**Architecture and design** -- System design for the core supervisor, process state machine, and API layer. Decisions on modularity, signal handling, and deployment patterns.

**Core implementation** -- Process lifecycle management (7-state machine, autorestart, exponential backoff), TOML configuration with variable expansion and hot reload, REST/JSON API with SSE streaming, event system with pub/sub and webhooks, structured logging with rotation and syslog forwarding.

**Developer experience and CI/CD** -- Spec-driven development workflow, Taskfile-based builds, GoReleaser cross-platform releases, GitHub Actions CI with gotestsum reporting and JUnit XML checks. 579 unit tests, 68 E2E tests across 9 domains.

For commit history and pull requests: https://github.com/kahiteam/kahi

---

## Where to Find the Code

- **Source, issues, PRs:** https://github.com/kahiteam/kahi
- **Releases:** https://github.com/kahiteam/kahi/releases
- **Go module:** github.com/kahiteam/kahi

All development, bug tracking, and releases happen under the kahiteam organization.

---

## Contact

- GitHub: https://github.com/schwichtgit
- Discussions: https://github.com/kahiteam/kahi
