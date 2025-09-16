# Claude Code Context: Joblin CLI

## Project Overview
Joblin is a CLI tool built in Go that enables developers to deploy Python scripts to Kubernetes clusters easily. The tool provides job lifecycle management (create, monitor, terminate) with Teams webhook notifications.

## Technology Stack
- **Language**: Go 1.21+
- **CLI Framework**: Cobra with Viper configuration
- **Kubernetes Client**: kubernetes/client-go
- **Local Storage**: BBolt embedded database
- **Teams Integration**: github.com/atc0005/go-teams-notify
- **Testing**: Go testing package with testify assertions
- **Distribution**: GoReleaser with GitHub Actions

## Architecture
- **Libraries**: joblib (core logic), k8slib (Kubernetes operations), notifylib (Teams webhooks)
- **CLI Structure**: Nested commands (deploy/status/logs/terminate/list/cleanup/config)
- **Data Model**: Job, JobStatus, ResourceSpec, JobLog, Notification entities
- **Storage**: BBolt for job metadata, Kubernetes etcd for job state

## Key Features
1. **Job Deployment**: Deploy Python scripts with dependencies to Kubernetes
2. **Resource Management**: Configurable CPU/memory limits with defaults
3. **Job Monitoring**: Real-time status tracking and log streaming
4. **Lifecycle Control**: Terminate jobs, automatic cleanup with TTL
5. **Notifications**: Teams webhook integration for job completion events
6. **Multi-cluster Support**: Work with different Kubernetes contexts/namespaces

## Current Development Phase
- **Phase**: Implementation Planning (001-build-a-cli)
- **Status**: Design complete, ready for task generation
- **Next**: Use `/tasks` command to generate implementation tasks

## Key Files
- `/specs/001-build-a-cli/spec.md`: Feature specification
- `/specs/001-build-a-cli/plan.md`: Implementation plan
- `/specs/001-build-a-cli/data-model.md`: Data structures
- `/specs/001-build-a-cli/contracts/`: API contracts
- `/specs/001-build-a-cli/quickstart.md`: User scenarios

## Constitutional Requirements
- **Testing**: TDD mandatory, RED-GREEN-Refactor cycle
- **Libraries**: Every feature as library with CLI wrapper
- **Simplicity**: Single project, direct framework usage, no unnecessary patterns
- **Observability**: Structured logging with logrus
- **Versioning**: 0.1.0 initial release, BUILD increments

## Dependencies
```go
// Core dependencies
github.com/spf13/cobra
github.com/spf13/viper
k8s.io/client-go
k8s.io/api
k8s.io/apimachinery
github.com/atc0005/go-teams-notify
go.etcd.io/bbolt
github.com/sirupsen/logrus
github.com/stretchr/testify

// Development
sigs.k8s.io/kind // For integration tests
```

## Project Structure
```
src/
├── models/         # Data structures (Job, JobStatus, etc.)
├── services/       # Business logic libraries
│   ├── joblib/     # Job lifecycle management
│   ├── k8slib/     # Kubernetes operations
│   └── notifylib/  # Teams webhook notifications
├── cli/            # Cobra command implementations
└── lib/            # Shared utilities

tests/
├── contract/       # API contract tests
├── integration/    # Full workflow tests
└── unit/           # Component tests

cmd/joblin/         # Main CLI entry point
```

## Recent Context
- Completed feature specification with 13 functional requirements
- Researched Go/Kubernetes best practices and selected tech stack
- Designed data model with Job, Notification, and Log entities
- Created CLI interface, Kubernetes API, and Teams webhook contracts
- Defined user scenarios and quickstart guide for testing

Ready for implementation task generation using the `/tasks` command.