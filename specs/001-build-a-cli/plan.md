# Implementation Plan: Joblin CLI Tool for Kubernetes Python Job Deployment


**Branch**: `001-build-a-cli` | **Date**: 2025-09-15 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/001-build-a-cli/spec.md`

## Execution Flow (/plan command scope)
```
1. Load feature spec from Input path
   → If not found: ERROR "No feature spec at {path}"
2. Fill Technical Context (scan for NEEDS CLARIFICATION)
   → Detect Project Type from context (web=frontend+backend, mobile=app+api)
   → Set Structure Decision based on project type
3. Evaluate Constitution Check section below
   → If violations exist: Document in Complexity Tracking
   → If no justification possible: ERROR "Simplify approach first"
   → Update Progress Tracking: Initial Constitution Check
4. Execute Phase 0 → research.md
   → If NEEDS CLARIFICATION remain: ERROR "Resolve unknowns"
5. Execute Phase 1 → contracts, data-model.md, quickstart.md, agent-specific template file (e.g., `CLAUDE.md` for Claude Code, `.github/copilot-instructions.md` for GitHub Copilot, or `GEMINI.md` for Gemini CLI).
6. Re-evaluate Constitution Check section
   → If new violations: Refactor design, return to Phase 1
   → Update Progress Tracking: Post-Design Constitution Check
7. Plan Phase 2 → Describe task generation approach (DO NOT create tasks.md)
8. STOP - Ready for /tasks command
```

**IMPORTANT**: The /plan command STOPS at step 7. Phases 2-4 are executed by other commands:
- Phase 2: /tasks command creates tasks.md
- Phase 3-4: Implementation execution (manual or via tools)

## Summary
Joblin is a CLI tool that enables developers to deploy Python scripts to Kubernetes clusters easily. The tool will be built in Golang and provide commands to create, terminate, and track Kubernetes jobs, with Teams webhook notifications for job completion status. The primary requirement is to abstract away Kubernetes complexity while providing full job lifecycle management.

## Technical Context
**Language/Version**: Go 1.21+
**Primary Dependencies**: kubernetes/client-go, cobra (CLI framework), viper (config), logrus (logging)
**Storage**: Local filesystem for job metadata and history (JSON/YAML files), Kubernetes etcd for job state
**Testing**: Go testing package, testify for assertions, kind for Kubernetes integration tests
**Target Platform**: Linux, macOS, Windows (cross-platform CLI)
**Project Type**: single (CLI application)
**Performance Goals**: <2s startup time, handle 100+ concurrent jobs
**Constraints**: <50MB memory usage, kubeconfig-based auth only, 1-week job history retention
**Scale/Scope**: Support for multiple K8s clusters, 1000+ jobs per user, Teams webhook integration

## Constitution Check
*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Simplicity**:
- Projects: 1 (cli only)
- Using framework directly? (cobra CLI framework, kubernetes client-go directly)
- Single data model? (Yes, unified Job struct for all operations)
- Avoiding patterns? (Direct file storage, no ORM, no complex abstractions)

**Architecture**:
- EVERY feature as library? (Yes: joblib for core logic, separate from CLI)
- Libraries listed: joblib (job lifecycle), k8slib (Kubernetes operations), notifylib (Teams webhooks)
- CLI per library: (joblin deploy/status/logs/terminate/list commands with --help/--version/--json)
- Library docs: llms.txt format planned? (Yes, for each library)

**Testing (NON-NEGOTIABLE)**:
- RED-GREEN-Refactor cycle enforced? (Yes, all tests fail first)
- Git commits show tests before implementation? (Yes, test commits precede implementation)
- Order: Contract→Integration→E2E→Unit strictly followed? (Yes)
- Real dependencies used? (Yes, kind clusters for K8s tests, real Teams webhooks)
- Integration tests for: joblib-k8slib interaction, webhook delivery, file persistence
- FORBIDDEN: Implementation before test, skipping RED phase (Strict enforcement)

**Observability**:
- Structured logging included? (Yes, logrus with JSON format)
- Frontend logs → backend? (N/A - CLI application)
- Error context sufficient? (Yes, detailed error messages with job IDs and timestamps)

**Versioning**:
- Version number assigned? (0.1.0 - initial release)
- BUILD increments on every change? (Yes, automated via CI)
- Breaking changes handled? (Config migration, backward compatibility tests)

## Project Structure

### Documentation (this feature)
```
specs/[###-feature]/
├── plan.md              # This file (/plan command output)
├── research.md          # Phase 0 output (/plan command)
├── data-model.md        # Phase 1 output (/plan command)
├── quickstart.md        # Phase 1 output (/plan command)
├── contracts/           # Phase 1 output (/plan command)
└── tasks.md             # Phase 2 output (/tasks command - NOT created by /plan)
```

### Source Code (repository root)
```
# Option 1: Single project (DEFAULT)
src/
├── models/
├── services/
├── cli/
└── lib/

tests/
├── contract/
├── integration/
└── unit/

# Option 2: Web application (when "frontend" + "backend" detected)
backend/
├── src/
│   ├── models/
│   ├── services/
│   └── api/
└── tests/

frontend/
├── src/
│   ├── components/
│   ├── pages/
│   └── services/
└── tests/

# Option 3: Mobile + API (when "iOS/Android" detected)
api/
└── [same as backend above]

ios/ or android/
└── [platform-specific structure]
```

**Structure Decision**: Option 1 (Single project) - CLI application with library structure

## Phase 0: Outline & Research
1. **Extract unknowns from Technical Context** above:
   - For each NEEDS CLARIFICATION → research task
   - For each dependency → best practices task
   - For each integration → patterns task

2. **Generate and dispatch research agents**:
   ```
   For each unknown in Technical Context:
     Task: "Research {unknown} for {feature context}"
   For each technology choice:
     Task: "Find best practices for {tech} in {domain}"
   ```

3. **Consolidate findings** in `research.md` using format:
   - Decision: [what was chosen]
   - Rationale: [why chosen]
   - Alternatives considered: [what else evaluated]

**Output**: research.md with all NEEDS CLARIFICATION resolved

## Phase 1: Design & Contracts
*Prerequisites: research.md complete*

1. **Extract entities from feature spec** → `data-model.md`:
   - Entity name, fields, relationships
   - Validation rules from requirements
   - State transitions if applicable

2. **Generate API contracts** from functional requirements:
   - For each user action → endpoint
   - Use standard REST/GraphQL patterns
   - Output OpenAPI/GraphQL schema to `/contracts/`

3. **Generate contract tests** from contracts:
   - One test file per endpoint
   - Assert request/response schemas
   - Tests must fail (no implementation yet)

4. **Extract test scenarios** from user stories:
   - Each story → integration test scenario
   - Quickstart test = story validation steps

5. **Update agent file incrementally** (O(1) operation):
   - Run `/scripts/bash/update-agent-context.sh claude` for your AI assistant
   - If exists: Add only NEW tech from current plan
   - Preserve manual additions between markers
   - Update recent changes (keep last 3)
   - Keep under 150 lines for token efficiency
   - Output to repository root

**Output**: data-model.md, /contracts/*, failing tests, quickstart.md, agent-specific file

## Phase 2: Task Planning Approach
*This section describes what the /tasks command will do - DO NOT execute during /plan*

**Task Generation Strategy**:
- Load `/templates/tasks-template.md` as base
- Generate tasks from Phase 1 design docs (contracts, data model, quickstart)
- CLI interface contract → CLI command test tasks [P]
- Kubernetes API contract → K8s integration test tasks [P]
- Teams webhook contract → notification test tasks [P]
- Each data model entity → model creation task [P]
- Each user scenario from quickstart → integration test task
- Implementation tasks to make tests pass (libraries before CLI)

**Ordering Strategy**:
- TDD order: Contract tests → Integration tests → Unit tests → Implementation
- Dependency order: Models → Services (joblib/k8slib/notifylib) → CLI commands
- Mark [P] for parallel execution (independent files/packages)
- Group by package for efficient development

**Specific Task Categories**:
1. **Contract Tests (5-7 tasks)**: CLI interface validation, K8s API integration, Teams webhook delivery
2. **Model Creation (4-5 tasks)**: Job, JobStatus, ResourceSpec, JobLog, Notification structs
3. **Library Implementation (8-10 tasks)**: joblib, k8slib, notifylib packages
4. **CLI Commands (7-8 tasks)**: deploy, status, logs, terminate, list, cleanup, config commands
5. **Integration Tests (6-8 tasks)**: User scenarios from quickstart.md
6. **Infrastructure (3-4 tasks)**: GoReleaser, CI/CD, documentation

**Estimated Output**: 33-42 numbered, ordered tasks in tasks.md

**IMPORTANT**: This phase is executed by the /tasks command, NOT by /plan

## Phase 3+: Future Implementation
*These phases are beyond the scope of the /plan command*

**Phase 3**: Task execution (/tasks command creates tasks.md)  
**Phase 4**: Implementation (execute tasks.md following constitutional principles)  
**Phase 5**: Validation (run tests, execute quickstart.md, performance validation)

## Complexity Tracking
*Fill ONLY if Constitution Check has violations that must be justified*

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| [e.g., 4th project] | [current need] | [why 3 projects insufficient] |
| [e.g., Repository pattern] | [specific problem] | [why direct DB access insufficient] |


## Progress Tracking
*This checklist is updated during execution flow*

**Phase Status**:
- [x] Phase 0: Research complete (/plan command)
- [x] Phase 1: Design complete (/plan command)
- [x] Phase 2: Task planning complete (/plan command - describe approach only)
- [ ] Phase 3: Tasks generated (/tasks command)
- [ ] Phase 4: Implementation complete
- [ ] Phase 5: Validation passed

**Gate Status**:
- [x] Initial Constitution Check: PASS
- [x] Post-Design Constitution Check: PASS
- [x] All NEEDS CLARIFICATION resolved
- [x] Complexity deviations documented (none required)

---
*Based on Constitution v2.1.1 - See `/memory/constitution.md`*