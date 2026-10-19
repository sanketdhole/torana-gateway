# Contributing to Torana

First off, thank you for considering contributing to Torana! 🎉

Torana is building the zero-trust security perimeter for autonomous AI agents, MCP tool execution, and enterprise assets. We believe that securing AI systems requires an open, collaborative effort. Whether you are fixing a typo in documentation, adding support for a new AI framework, reporting a bug, or optimizing our sub-millisecond hot path, your contributions are welcome.

---

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [How Can I Contribute?](#how-can-i-contribute)
  - [Reporting Bugs](#reporting-bugs)
  - [Suggesting Features & Enhancements](#suggesting-features--enhancements)
  - [Improving Documentation](#improving-documentation)
  - [Contributing Code](#contributing-code)
- [Development Setup](#development-setup)
  - [Prerequisites](#prerequisites)
  - [Forking and Cloning](#forking-and-cloning)
  - [Building from Source](#building-from-source)
  - [Running the Development Environment](#running-the-development-environment)
- [Development & Coding Guidelines](#development--coding-guidelines)
  - [Directory Structure](#directory-structure)
  - [Go Conventions & Performance Standards](#go-conventions--performance-standards)
  - [Code Quality & Linting](#code-quality--linting)
- [Testing & Benchmarking](#testing--benchmarking)
  - [Unit & Integration Tests](#unit--integration-tests)
  - [Latency & Memory Benchmarks](#latency--memory-benchmarks)
  - [Fuzzing](#fuzzing)
- [Submitting a Pull Request](#submitting-a-pull-request)
  - [Commit Message Guidelines](#commit-message-guidelines)
  - [PR Checklist](#pr-checklist)
  - [Review Process](#review-process)
- [License & Contributor Agreement](#license--contributor-agreement)
- [Questions & Community](#questions--community)

---

## Code of Conduct

All contributors and participants in the Torana project are expected to adhere to our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it to understand our community expectations and reporting procedures.

---

## How Can I Contribute?

### Reporting Bugs

Encountered unexpected behavior or an error? Please check existing GitHub Issues before submitting a new report to avoid duplicates.

When submitting a bug report via our [Bug Report Form](https://github.com/sanketdhole/torana-enterprise/issues/new?template=bug_report.yml), include:
- A clear, descriptive title.
- Torana version / commit hash.
- Operating system, Go version, and deployment mode (Docker / binary / Kubernetes).
- Steps to reproduce the issue (including sample requests or policies if applicable).
- Expected behavior vs. actual behavior.
- Relevant logs and terminal output.

> [!NOTE]
> If you discover a security vulnerability or exploit, please **do not** open a public issue. Refer to [SECURITY.md](SECURITY.md) and report privately to [security@phaselume.com](mailto:security@phaselume.com).

### Suggesting Features & Enhancements

We are eager to support emerging agent patterns, protocols, and enterprise integrations!
- Search existing issues to ensure the proposal is not already being tracked.
- Open a discussion or submit a [Feature Request](https://github.com/sanketdhole/torana-enterprise/issues/new?template=feature_request.yml) describing the motivation, architecture impact, and proposed API / configuration changes.
- For substantial architectural changes, we recommend opening an RFC (Request for Comments) issue before implementing code.

### Improving Documentation

High-quality documentation, architecture diagrams, tutorials, and policy recipes help developers adopt Torana safely. If you notice outdated information, unclear instructions, or missing examples, please submit a PR!

### Contributing Code

Look for issues tagged:
- [`good first issue`](https://github.com/sanketdhole/torana-enterprise/labels/good%20first%20issue): Great introductory tasks for new contributors.
- [`help wanted`](https://github.com/sanketdhole/torana-enterprise/labels/help%20wanted): Well-defined features and improvements awaiting community hands.

---

## Development Setup

### Prerequisites

- **Go**: Version `1.25` or higher ([Download Go](https://go.dev/dl/))
- **Docker & Docker Compose**: For containerized development and peer mesh testing
- **Make**: For running standardized build, test, and benchmark targets
- **Node.js (v20+) & npm** *(optional)*: Only required if modifying the Control Plane UI (`controlplane/ui`)
- **Protocol Buffers Compiler (`protoc`)** *(optional)*: Only required if updating `.proto` contracts in `api/proto/`

### Forking and Cloning

1. Fork the repository on GitHub: [https://github.com/sanketdhole/torana-enterprise](https://github.com/sanketdhole/torana-enterprise)
2. Clone your fork locally:
   ```bash
   git clone https://github.com/<your-username>/torana-enterprise.git
   cd torana-enterprise
   ```
3. Add the upstream remote:
   ```bash
   git remote add upstream https://github.com/sanketdhole/torana-enterprise.git
   ```

### Building from Source

Verify your environment by building the binaries:

```bash
# Build data plane binary (bin/gateway-data)
make build

# Build control plane binary (bin/torana-controlplane)
make controlplane

# Build both along with vet & test
make all
```

### Running the Development Environment

You can run the gateway locally with automatic live-reloading via Docker Compose and [Air](https://github.com/air-verse/air):

```bash
# Start development container with hot-reloading
make docker-dev

# Stop development container
make docker-dev-down
```

---

## Development & Coding Guidelines

### Directory Structure

```text
├── api/             # Protobuf definitions and generated contracts
├── cmd/             # Main executable entrypoints (e.g., cmd/gateway-data)
├── controlplane/    # Control Plane service and Admin UI
├── internal/        # Private gateway runtime packages
│   ├── egress/      # Upstream outbound connection broker
│   ├── ingress/     # HTTP/gRPC ingress handlers
│   ├── mcp/         # Model Context Protocol handlers & proxying
│   ├── pipeline/    # Core filter pipeline (identity, policy, rate limits)
│   ├── policy/      # CEL engine and policy compilation
│   └── telemetry/   # OpenTelemetry tracing, metrics, audit logs
├── pkg/             # Public packages and reusable client libraries
├── scripts/         # Automation and deployment scripts
└── test/            # Integration, benchmark, fuzz, and chaos tests
```

### Go Conventions & Performance Standards

Torana is a zero-trust gateway deployed directly in the high-throughput request path. We hold performance to extreme standards:

1. **Zero-Allocations on the Hot Path**:
   - In `internal/pipeline` and filter chains, avoid allocating memory per request.
   - Reuse buffers with `sync.Pool` where appropriate.
   - Avoid unnecessary interface boxing, dynamic type assertions, and string conversions on hot paths.
   - Refer to [System Design (DESIGN.md)](DESIGN.md) for architectural guidelines.

2. **Idiomatic Go**:
   - Write clean, self-documenting code following standard [Effective Go](https://go.dev/doc/effective_go) patterns.
   - Return errors explicitly; do not panic unless during catastrophic startup failures.
   - Wrap errors with context (`fmt.Errorf("failed to parse token: %w", err)`).
   - Ensure exported functions, structs, and interfaces have clear Go docstrings.

3. **Concurrency Safety**:
   - Always run tests with the Go race detector (`go test -race`).
   - Prefer lock-free mechanisms (atomic values, snapshot pointers) over coarse mutexes in reader paths.

### Code Quality & Linting

Before opening a pull request, format and lint your code:

```bash
# Format code
go fmt ./...

# Run static analysis
make vet

# Run golangci-lint (if installed)
make lint
```

---

## Testing & Benchmarking

Quality and performance regressions must be caught before code lands on `main`.

### Unit & Integration Tests

All pull requests must include unit or integration tests verifying new logic and bug fixes:

```bash
# Run all unit tests with the race detector
make test

# Run control plane tests
make test-controlplane

# Run chaos / resilience tests
make chaos
```

### Latency & Memory Benchmarks

If you touch code in `cmd/`, `internal/pipeline/`, `internal/policy/`, or `internal/mcp/`, you must verify that throughput and p99 latency are not degraded:

```bash
# Run standard benchmarks
make bench

# Run extended 300s load benchmarks
make bench-load

# Generate CPU/memory profiling artifacts
make profile
```

### Fuzzing

If modifying parsers (CEL expressions, JWT tokens, MCP JSON-RPC, or router paths), run the fuzz test suite:

```bash
make fuzz
```

---

## Submitting a Pull Request

### Commit Message Guidelines

We follow the [Conventional Commits](https://www.conventionalcommits.org/) convention to keep our git history clean and understandable:

```text
<type>(<scope>): <short description>

[optional body]

[optional footer(s)]
```

Common types:
- `feat`: A new feature or capability
- `fix`: A bug fix
- `perf`: A code change that improves performance or reduces allocations
- `docs`: Documentation only changes
- `test`: Adding or correcting tests
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `chore`: Changes to build process, dependencies, or tooling

Example:
```text
feat(mcp): support dynamic tool schema validation in filter chain

Add json-schema validation filter in the hot path for MCP tool calls.
Reuses cached compiled schemas to maintain zero-alloc throughput.

Fixes #42
```

### PR Checklist

Before submitting your pull request, check the following:

- [ ] My branch is rebased on top of the latest `main` branch.
- [ ] Code follows project formatting (`go fmt ./...`).
- [ ] Static analysis passes (`make vet` and `make lint`).
- [ ] All tests pass with race detection enabled (`make test`).
- [ ] Any hot-path performance impact was verified (`make bench`).
- [ ] New features or fixes include tests covering edge cases.
- [ ] Documentation has been updated to reflect changes (if applicable).
- [ ] The PR title follows Conventional Commits format.

### Review Process

- Once submitted, maintainers and automated CI workflows will review your pull request.
- Be responsive to feedback and discussions. If changes are requested, push additional commits to your branch—GitHub will automatically update the PR.
- Once approved, a maintainer will merge your PR (typically squash-and-merge).

---

## License & Contributor Agreement

By contributing to Torana, you agree that your contributions will be licensed under the project's [Apache License 2.0 with Managed Service Clause](LICENSE). You represent that you have the right to grant this license for your contributions.

---

## Questions & Community

- **Discussions & Q&A**: [GitHub Discussions](https://github.com/sanketdhole/torana-enterprise/discussions)
- **Issue Tracker**: [GitHub Issues](https://github.com/sanketdhole/torana-enterprise/issues)
- **Security Inquiries**: [security@phaselume.com](mailto:security@phaselume.com)

Thank you for helping make autonomous AI agents safer and enterprise-ready! 🚀
