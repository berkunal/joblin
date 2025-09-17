# Joblin

<div align="center">

![Joblin Logo](./joblin.png)

**Deploy Python scripts to Kubernetes clusters with ease**

[![CI](https://github.com/berkunal/joblin/workflows/CI/badge.svg)](https://github.com/berkunal/joblin/actions)
[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://golang.org)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Release](https://img.shields.io/github/release/berkunal/joblin.svg)](https://github.com/berkunal/joblin/releases)

</div>

## Overview

Joblin is a command-line interface (CLI) tool that enables developers to deploy Python scripts to Kubernetes clusters effortlessly. It provides comprehensive job lifecycle management including creation, monitoring, termination, and cleanup, with integrated Microsoft Teams webhook notifications.

### Key Features

- 🚀 **Simple Deployment**: Deploy Python scripts with a single command
- 📦 **Dependency Management**: Automatic handling of Python requirements
- 🔍 **Job Monitoring**: Real-time status tracking and log streaming
- ⚡ **Resource Control**: Configurable CPU, memory, and storage limits
- 🧹 **Automatic Cleanup**: TTL-based job cleanup and resource management
- 📢 **Teams Integration**: Webhook notifications for job completion
- 🌐 **Multi-cluster Support**: Work with different Kubernetes contexts
- 🖥️ **Shell Completion**: Auto-completion for bash, zsh, fish, and PowerShell
- 📊 **Rich Output**: JSON and human-readable output formats

## Installation

### Option 1: Download Binary (Recommended)

Download the latest release for your platform:

```bash
# Linux (x86_64)
curl -LO https://github.com/berkunal/joblin/releases/latest/download/joblin-linux-amd64
chmod +x joblin-linux-amd64
sudo mv joblin-linux-amd64 /usr/local/bin/joblin

# macOS (x86_64)
curl -LO https://github.com/berkunal/joblin/releases/latest/download/joblin-darwin-amd64
chmod +x joblin-darwin-amd64
sudo mv joblin-darwin-amd64 /usr/local/bin/joblin

# macOS (ARM64)
curl -LO https://github.com/berkunal/joblin/releases/latest/download/joblin-darwin-arm64
chmod +x joblin-darwin-arm64
sudo mv joblin-darwin-arm64 /usr/local/bin/joblin

# Windows (x86_64)
# Download joblin-windows-amd64.exe from releases page
```

### Option 2: Build from Source

```bash
git clone https://github.com/berkunal/joblin.git
cd joblin
go build -o joblin ./cmd/joblin
sudo mv joblin /usr/local/bin/joblin
```

### Option 3: Go Install

```bash
go install github.com/berkunal/joblin/cmd/joblin@latest
```

### Verify Installation

```bash
joblin --version
```

## Prerequisites

- **Kubernetes cluster** with kubectl access configured
- **Go 1.24+** (if building from source)
- **Python 3.11+** runtime in your Kubernetes cluster (automatically handled)

## Quick Start

### 1. Initialize Configuration

```bash
# Create default configuration
joblin config init

# Set Teams webhook URL (optional)
joblin config set webhook-url "https://your-teams-webhook-url"

# Verify configuration
joblin config show
```

### 2. Deploy Your First Script

Create a simple Python script:

```bash
cat > hello.py << 'EOF'
#!/usr/bin/env python3
print("Hello from Kubernetes!")
print("This job is running successfully.")
EOF
```

Deploy it:

```bash
joblin deploy hello.py --name hello-world
```

### 3. Monitor the Job

```bash
# Check job status
joblin status <job-id>

# Follow logs in real-time
joblin logs <job-id> --follow

# List all jobs
joblin list
```

## Usage Examples

### Basic Deployment

```bash
# Deploy a simple script
joblin deploy script.py --name my-job

# Deploy with custom resources
joblin deploy script.py \
  --name data-processing \
  --cpu "500m" \
  --memory "1Gi" \
  --storage "2Gi"
```

### Python Dependencies

```bash
# With requirements file
joblin deploy script.py --requirements requirements.txt

# With inline dependencies
joblin deploy script.py --requirements "pandas,numpy,requests"
```

### Advanced Options

```bash
# Deploy with labels and TTL
joblin deploy script.py \
  --name batch-job \
  --label "env=prod" \
  --label "team=data" \
  --ttl "2h" \
  --webhook-url "https://your-webhook"

# Deploy to specific namespace/context
joblin deploy script.py \
  --namespace my-team \
  --context production-cluster
```

### Job Management

```bash
# List jobs with filtering
joblin list --status Running
joblin list --label "team=data"

# Terminate a running job
joblin terminate <job-id>

# Clean up old jobs
joblin cleanup --dry-run  # Preview what will be cleaned
joblin cleanup            # Actually clean up
```

### Log Management

```bash
# View logs with different sources
joblin logs <job-id>                    # Standard output
joblin logs <job-id> --source stderr    # Standard error
joblin logs <job-id> --source system    # Kubernetes system logs
joblin logs <job-id> --source all       # All sources

# Advanced log options
joblin logs <job-id> --follow --tail 50 --timestamp
```

## Commands Reference

### Global Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--config` | | Config file path | `$HOME/.joblin/config.yaml` |
| `--context` | | Kubernetes context | From kubeconfig |
| `--namespace` | `-n` | Kubernetes namespace | `default` |
| `--json` | | JSON output format | `false` |
| `--verbose` | `-v` | Verbose output | `false` |
| `--log-level` | | Log level (debug, info, warn, error) | `info` |

### Commands

| Command | Description |
|---------|-------------|
| [`deploy`](#deploy) | Deploy a Python script to Kubernetes |
| [`status`](#status) | Get the status of a job |
| [`logs`](#logs) | View logs from a job |
| [`terminate`](#terminate) | Terminate a running job |
| [`list`](#list) | List jobs with filtering |
| [`cleanup`](#cleanup) | Clean up expired jobs and old data |
| [`config`](#config) | Manage configuration settings |
| [`completion`](#completion) | Generate shell completion scripts |

#### Deploy

Deploy a Python script to Kubernetes as a Job.

```bash
joblin deploy <script.py> [flags]
```

**Flags:**
- `--name string`: Job name (default: derived from script filename)
- `--cpu string`: CPU limit (e.g., 100m, 1, 2)
- `--memory string`: Memory limit (e.g., 128Mi, 1Gi)
- `--storage string`: Ephemeral storage limit (e.g., 1Gi, 10Gi)
- `--requirements string`: Python dependencies (file path or comma-separated list)
- `--ttl string`: Time-to-live for job cleanup (e.g., 1h, 24h, 7d)
- `--webhook-url string`: Teams webhook URL for notifications
- `--label strings`: Labels to apply (key=value format)
- `--dry-run`: Validate job without creating it

#### Status

Get detailed status information for a job.

```bash
joblin status <job-id> [flags]
```

**Flags:**
- `--watch`, `-w`: Watch job status until completion
- `--interval string`: Polling interval for watch mode (default: 5s)

#### Logs

View logs from a deployed job.

```bash
joblin logs <job-id> [flags]
```

**Flags:**
- `--follow`, `-f`: Follow log output (for running jobs)
- `--tail int`: Number of lines to show from the end (default: 100)
- `--since string`: Show logs since timestamp (e.g., 5m, 1h, 2006-01-02T15:04:05Z)
- `--source string`: Log source: stdout, stderr, system, all (default: stdout)
- `--no-color`: Disable colored output
- `--timestamp`: Show timestamps

#### List

List jobs with optional filtering and sorting.

```bash
joblin list [flags]
```

**Flags:**
- `--status string`: Filter by job status (Pending, Running, Succeeded, Failed, Terminated)
- `--label strings`: Filter by labels (key=value format)
- `--limit int`: Limit number of results (0 = no limit)
- `--sort-by string`: Sort by field (name, status, created, started, duration)
- `--reverse`: Reverse sort order
- `--wide`, `-w`: Show additional columns

#### Cleanup

Clean up expired jobs and old data based on TTL settings.

```bash
joblin cleanup [flags]
```

**Flags:**
- `--dry-run`: Show what would be cleaned without actually doing it
- `--force`: Skip confirmation prompts
- `--age string`: Clean jobs older than specified age
- `--include-running`: Include running jobs in cleanup
- `--include-notifications`: Clean old notification records

## Shell Completion

Joblin supports shell completion for enhanced productivity:

### Installation

```bash
# Bash (Linux)
joblin completion bash > /etc/bash_completion.d/joblin

# Bash (macOS)
joblin completion bash > $(brew --prefix)/etc/bash_completion.d/joblin

# Zsh
joblin completion zsh > "${fpath[1]}/_joblin"

# Fish
joblin completion fish > ~/.config/fish/completions/joblin.fish

# PowerShell
joblin completion powershell > joblin.ps1
```

### Features

- **Dynamic job ID completion** with status information
- **Namespace completion** for common Kubernetes namespaces
- **Flag value completion** for log levels, job statuses, and sort options
- **File path completion** for Python scripts and requirements files

## Configuration

Joblin uses a YAML configuration file located at `$HOME/.joblin/config.yaml`.

### Default Configuration

```yaml
default_cluster: ""
default_namespace: "default"
default_resources:
  cpu: "100m"
  memory: "128Mi"
  ephemeral_storage: "1Gi"
default_ttl: "24h"
teams_webhook_url: ""
log_level: "info"
data_dir: "$HOME/.joblin"
config_version: "1.0"
```

### Configuration Commands

```bash
# Initialize default configuration
joblin config init

# Show current configuration
joblin config show

# Set configuration values
joblin config set webhook-url "https://your-webhook-url"
joblin config set default-namespace "my-team"
joblin config set default-cpu "200m"
joblin config set default-memory "256Mi"

# Get configuration values
joblin config get webhook-url
joblin config get default-namespace
```

## Teams Integration

Configure Microsoft Teams webhook notifications for job completion:

### Setup

1. Create an incoming webhook in your Teams channel
2. Configure the webhook URL:

```bash
joblin config set webhook-url "https://your-teams-webhook-url"
```

3. Deploy jobs with notifications:

```bash
joblin deploy script.py --webhook-url "https://specific-webhook"
```

### Notification Format

Joblin sends rich notifications including:
- Job name and status
- Execution duration
- Exit code (for completed jobs)
- Error messages (for failed jobs)
- Links to job details

## Architecture

Joblin is built with a library-first architecture:

- **CLI Layer** (`src/cli/`): Command-line interface built with Cobra
- **Service Layer** (`src/services/`):
  - `joblib`: Core job lifecycle management
  - `k8slib`: Kubernetes API operations
  - `notifylib`: Teams webhook notifications
  - `storage`: Local job metadata storage (BBolt)
- **Models** (`src/models/`): Data structures and validation
- **Libraries** (`src/lib/`): Shared utilities and configuration

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/berkunal/joblin.git
cd joblin

# Install dependencies
go mod download

# Build the binary
go build -o joblin ./cmd/joblin

# Run tests
make test

# Run linting
make lint

# Build for all platforms
make build-all
```

### Project Structure

```
├── cmd/joblin/          # Main CLI entry point
├── src/
│   ├── cli/             # CLI command implementations
│   ├── services/        # Business logic services
│   ├── models/          # Data models and validation
│   └── lib/             # Shared utilities
├── tests/               # Test suites
│   └── unit/            # Unit tests
├── specs/               # Feature specifications
└── docs/                # Documentation
```

### Testing

```bash
# Run all tests
make test

# Run unit tests
make test-unit

# Run tests with coverage
make coverage
```

## Troubleshooting

### Common Issues

#### Job stays in Pending state
- **Cause**: Insufficient cluster resources
- **Solution**: Check resource quotas and node capacity
```bash
kubectl describe resourcequota
kubectl describe nodes
```

#### Authentication errors
- **Cause**: Invalid kubeconfig or permissions
- **Solution**: Verify cluster access and RBAC permissions
```bash
kubectl cluster-info
kubectl auth can-i create jobs
```

#### Teams notifications not working
- **Cause**: Invalid or expired webhook URL
- **Solution**: Test webhook URL manually and check expiration

#### Script dependencies fail to install
- **Cause**: Network policies or invalid requirements
- **Solution**: Verify requirements.txt format and network access

### Getting Help

```bash
# Show help for any command
joblin [command] --help

# Enable verbose logging
joblin --verbose [command]

# Check configuration
joblin config show

# Validate job without deploying
joblin deploy script.py --dry-run
```

### Support

- 📖 [Documentation](https://github.com/berkunal/joblin/docs)
- 🐛 [Bug Reports](https://github.com/berkunal/joblin/issues)
- 💬 [Discussions](https://github.com/berkunal/joblin/discussions)

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Changelog

See [CHANGELOG.md](CHANGELOG.md) for a detailed list of changes and releases.

---

<div align="center">

**Built with ❤️ for the Kubernetes community**

[⬆ Back to top](#joblin)

</div>