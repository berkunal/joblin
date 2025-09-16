# Tasks: Joblin CLI Tool for Kubernetes Python Job Deployment

**Input**: Design documents from `/specs/001-build-a-cli/`
**Prerequisites**: plan.md (required), research.md, data-model.md, contracts/

## Execution Flow (main)
```
1. Load plan.md from feature directory
   → Extract: Go 1.21+, Cobra CLI, kubernetes/client-go, BBolt storage
   → Architecture: Library-first (joblib, k8slib, notifylib) with CLI wrapper
2. Load optional design documents:
   → data-model.md: Job, JobStatus, ResourceSpec, JobLog, Notification entities
   → contracts/: CLI interface, Kubernetes API, Teams webhook contracts
   → quickstart.md: User scenarios and integration test cases
3. Generate tasks by category:
   → Setup: Go project, dependencies, linting, project structure
   → Tests: Contract tests for CLI/K8s/Teams, integration tests from quickstart
   → Core: Data models, service libraries, CLI commands
   → Integration: BBolt storage, K8s client, Teams webhook delivery
   → Polish: Unit tests, performance validation, documentation
4. Apply TDD ordering: Tests before implementation
5. Mark [P] for parallel execution (different files, no dependencies)
6. SUCCESS: 38 tasks ready for execution
```

## Format: `[ID] [P?] Description`
- **[P]**: Can run in parallel (different files, no dependencies)
- All file paths are absolute from repository root

## Path Conventions
Single project structure (per plan.md):
- **Source**: `src/models/`, `src/services/`, `src/cli/`, `cmd/joblin/`
- **Tests**: `tests/contract/`, `tests/integration/`, `tests/unit/`

## Phase 3.1: Setup
- [ ] T001 Create Go project structure with src/models/, src/services/, src/cli/, cmd/joblin/, tests/ directories
- [ ] T002 Initialize Go module with dependencies: cobra, viper, client-go, bbolt, logrus, testify
- [ ] T003 [P] Configure linting with golangci-lint and formatting with gofmt in .golangci.yml
- [ ] T004 [P] Setup GitHub Actions workflow for CI/CD with GoReleaser in .github/workflows/ci.yml

## Phase 3.2: Tests First (TDD) ⚠️ MUST COMPLETE BEFORE 3.3
**CRITICAL: These tests MUST be written and MUST FAIL before ANY implementation**

### Contract Tests [P]
- [ ] T005 [P] CLI interface contract test for 'joblin deploy' command in tests/contract/test_cli_deploy.go
- [ ] T006 [P] CLI interface contract test for 'joblin status/logs/terminate' commands in tests/contract/test_cli_operations.go
- [ ] T007 [P] CLI interface contract test for 'joblin list/cleanup/config' commands in tests/contract/test_cli_management.go
- [ ] T008 [P] Kubernetes API contract test for Job creation and ConfigMap in tests/contract/test_k8s_job_creation.go
- [ ] T009 [P] Kubernetes API contract test for Job status monitoring and log retrieval in tests/contract/test_k8s_job_operations.go
- [ ] T010 [P] Teams webhook contract test for success/failure notifications in tests/contract/test_teams_webhook.go

### Integration Tests from Quickstart [P]
- [ ] T011 [P] First-time user setup scenario test in tests/integration/test_first_user_setup.go
- [ ] T012 [P] Developer workflow scenario test (script with dependencies) in tests/integration/test_developer_workflow.go
- [ ] T013 [P] Error handling scenario test (syntax errors, invalid resources) in tests/integration/test_error_handling.go
- [ ] T014 [P] Multi-job management scenario test in tests/integration/test_multi_job_management.go
- [ ] T015 [P] Long-running job with monitoring test in tests/integration/test_long_running_job.go

## Phase 3.3: Core Implementation (ONLY after tests are failing)

### Data Models [P]
- [ ] T016 [P] Job entity struct with validation in src/models/job.go
- [ ] T017 [P] JobStatus enumeration in src/models/job_status.go
- [ ] T018 [P] ResourceSpec struct with Kubernetes quantity validation in src/models/resource_spec.go
- [ ] T019 [P] JobLog entity struct in src/models/job_log.go
- [ ] T020 [P] Notification entity struct in src/models/notification.go
- [ ] T021 [P] CLIConfig entity struct in src/models/config.go

### Service Libraries [P] (Core Logic)
- [ ] T022 [P] joblib service: Job lifecycle management (create, track, terminate) in src/services/joblib/job_service.go
- [ ] T023 [P] k8slib service: Kubernetes Job operations and client management in src/services/k8slib/k8s_service.go
- [ ] T024 [P] notifylib service: Teams webhook notification delivery in src/services/notifylib/notification_service.go
- [ ] T025 [P] Storage service: BBolt database operations for job persistence in src/services/storage/storage_service.go

### CLI Commands (Sequential - shared cobra root)
- [ ] T026 Root command with global flags (--config, --context, --namespace, --json) in src/cli/root.go
- [ ] T027 Deploy command implementation using joblib and k8slib in src/cli/deploy.go
- [ ] T028 Status command implementation with watch mode in src/cli/status.go
- [ ] T029 Logs command implementation with follow and filtering in src/cli/logs.go
- [ ] T030 Terminate command implementation in src/cli/terminate.go
- [ ] T031 List command implementation with filtering options in src/cli/list.go
- [ ] T032 Cleanup command implementation with TTL-based removal in src/cli/cleanup.go
- [ ] T033 Config command implementation for settings management in src/cli/config.go

## Phase 3.4: Integration
- [ ] T034 Main CLI entry point integrating all commands in cmd/joblin/main.go
- [ ] T035 Configuration loading and kubeconfig integration across services
- [ ] T036 Error handling and structured logging with context throughout application
- [ ] T037 Shell completion generation for bash/zsh/fish/powershell

## Phase 3.5: Polish
- [ ] T038 [P] Unit tests for Job entity validation logic in tests/unit/test_job_validation.go
- [ ] T039 [P] Unit tests for Kubernetes quantity parsing in tests/unit/test_resource_validation.go
- [ ] T040 Performance tests: CLI startup time (<2s) and concurrent job handling in tests/performance/test_performance.go
- [ ] T041 [P] Update README.md with installation and usage instructions
- [ ] T042 Manual testing execution following quickstart.md scenarios

## Dependencies
**Critical TDD Dependencies:**
- Tests T005-T015 MUST complete and FAIL before any T016+ implementation
- T016-T021 (models) before T022-T025 (services)
- T022-T025 (services) before T026-T033 (CLI commands)
- T026 (root) before T027-T033 (individual commands)
- T034-T037 (integration) before T038-T042 (polish)

**Parallel Blocks:**
- T005-T010: Contract tests (independent files)
- T011-T015: Integration tests (independent scenarios)
- T016-T021: Data models (independent structs)
- T022-T025: Services (independent packages)
- T038-T041: Polish tasks (independent files)

## Parallel Execution Examples

### Launch Contract Tests (T005-T010):
```bash
# All contract tests can run simultaneously
Task: "CLI interface contract test for 'joblin deploy' command in tests/contract/test_cli_deploy.go"
Task: "CLI interface contract test for 'joblin status/logs/terminate' commands in tests/contract/test_cli_operations.go"
Task: "CLI interface contract test for 'joblin list/cleanup/config' commands in tests/contract/test_cli_management.go"
Task: "Kubernetes API contract test for Job creation and ConfigMap in tests/contract/test_k8s_job_creation.go"
Task: "Kubernetes API contract test for Job status monitoring and log retrieval in tests/contract/test_k8s_job_operations.go"
Task: "Teams webhook contract test for success/failure notifications in tests/contract/test_teams_webhook.go"
```

### Launch Integration Tests (T011-T015):
```bash
# All integration scenarios can run simultaneously
Task: "First-time user setup scenario test in tests/integration/test_first_user_setup.go"
Task: "Developer workflow scenario test in tests/integration/test_developer_workflow.go"
Task: "Error handling scenario test in tests/integration/test_error_handling.go"
Task: "Multi-job management scenario test in tests/integration/test_multi_job_management.go"
Task: "Long-running job with monitoring test in tests/integration/test_long_running_job.go"
```

### Launch Data Models (T016-T021):
```bash
# All model structs can be created simultaneously
Task: "Job entity struct with validation in src/models/job.go"
Task: "JobStatus enumeration in src/models/job_status.go"
Task: "ResourceSpec struct with Kubernetes quantity validation in src/models/resource_spec.go"
Task: "JobLog entity struct in src/models/job_log.go"
Task: "Notification entity struct in src/models/notification.go"
Task: "CLIConfig entity struct in src/models/config.go"
```

### Launch Service Libraries (T022-T025):
```bash
# All service packages can be developed simultaneously
Task: "joblib service: Job lifecycle management in src/services/joblib/job_service.go"
Task: "k8slib service: Kubernetes Job operations in src/services/k8slib/k8s_service.go"
Task: "notifylib service: Teams webhook notification delivery in src/services/notifylib/notification_service.go"
Task: "Storage service: BBolt database operations in src/services/storage/storage_service.go"
```

## Notes
- **TDD Enforcement**: All tests T005-T015 must be written first and must fail
- **Constitution Compliance**: Library-first architecture, structured logging, version 0.1.0
- **File Independence**: [P] tasks modify different files with no shared dependencies
- **Go Conventions**: Follow standard Go project layout and idioms
- **Error Handling**: Comprehensive error messages and logging throughout
- **Cross-Platform**: Support Linux, macOS, Windows via GoReleaser

## Validation Checklist
**GATE: Verified before task execution begins**

- [x] All CLI interface contracts have corresponding tests (T005-T007)
- [x] All Kubernetes API contracts have corresponding tests (T008-T009)
- [x] All Teams webhook contracts have corresponding tests (T010)
- [x] All data model entities have model tasks (T016-T021)
- [x] All quickstart scenarios have integration tests (T011-T015)
- [x] All tests come before implementation (T005-T015 before T016+)
- [x] Parallel tasks are truly independent (different files/packages)
- [x] Each task specifies exact absolute file path
- [x] No [P] task modifies same file as another [P] task
- [x] TDD order enforced: Tests → Models → Services → CLI → Integration → Polish