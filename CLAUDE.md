# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**Description:** [PROJECT_DESCRIPTION]
**Tech Stack:** [PRIMARY_LANGUAGE], [FRAMEWORK], [DATABASE]
**Repository:** [REPO_URL]

## Development Workflow

### Subagent Policy

The main conversation orchestrates, summarizes, and interacts with the user. All heavy lifting MUST be delegated to subagents via the Task tool. Never perform these tasks inline:

**Mandatory delegation:**

- Schema validation, JSON validation, data integrity checks
- Dependency graph analysis (cycle detection, depth, width)
- Multi-file exploration and codebase search (3+ queries)
- Spec scoring, quality analysis, coverage analysis
- Running test suites and collecting results
- Complex multi-file changes
- Any operation that reads more than 3 files to produce a result

**Parallelization:**

- When multiple independent analyses are needed, launch them as parallel subagents
- Skill sub-commands that involve scoring dimensions should split each dimension into its own subagent where practical

**Main conversation responsibilities:**

- Orchestrate subagent launches
- Aggregate and format subagent results for the user
- Ask clarifying questions
- Make single-file edits based on clear user instructions
- Present summaries and scorecards

### Commands

```bash
# Start development
./init.sh

# Run tests
[TEST_COMMAND]

# Run single test
[SINGLE_TEST_COMMAND]

# Lint
[LINT_COMMAND]

# Type check
[TYPECHECK_COMMAND]

# Build
[BUILD_COMMAND]
```

## Architecture

[Describe the project architecture: main modules, data flow, key abstractions.]

## Quality Standards

Quality is enforced by the Claude Project Foundation hooks:

- **Pre-edit:** Blocks modification of sensitive files (.env, keys, credentials, lock files)
- **Pre-bash:** Blocks destructive commands (rm -rf /, force push, hard reset)
- **Post-edit:** Auto-formats files by language (Prettier, Ruff, rustfmt, etc.)
- **On stop:** Runs lint, type check, and test suite before allowing session end
- **Pre-commit:** Checks for secrets, forbidden files, runs linters
- **Commit-msg:** Validates conventional commits, blocks AI-isms and emoji

Coverage threshold: [COVERAGE_THRESHOLD]% (configured in constitution)

## Git Commit Guidelines

- Format: Conventional Commits (`feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert`)
- Subject line: <= 72 characters
- No emoji in commit messages or PR titles
- No AI-isms or self-referential language
- No Co-Authored-By trailers

## Communication Style

- Technical and direct
- No emoji
- No AI-isms ("I have", "Certainly", "seamless", "robust", "elegant")
- No marketing adjectives
- Terse: prefer short, factual statements
