<!--
╔════════════════════════════════════════════════════════════════════════════╗
║                        CONSTITUTION SYNC REPORT                             ║
╚════════════════════════════════════════════════════════════════════════════╝

Version Change: Initial → 1.0.0
Change Type: MAJOR (initial ratification)
Date: 2025-01-21

Principles Established:
  ✓ I. Code Quality & Maintainability
  ✓ II. Testing Standards
  ✓ III. User Experience Consistency
  ✓ IV. Performance Requirements

Templates Status:
  ✅ .specify/templates/spec-template.md (validated - no updates needed)
  ✅ .specify/templates/plan-template.md (validated - constitution check placeholder ready)
  ✅ .specify/templates/tasks-template.md (validated - task categorization compatible)

Follow-up Actions: None

Notes:
  - Initial constitution for status-dashboard-v3 Go project
  - Based on user requirements + existing project patterns
  - Aligned with existing golangci-lint configuration (v2.4.0)
  - Testing patterns: unit tests (internal/) + integration tests (tests/)
-->

# Status Dashboard Constitution

## Core Principles

### I. Code Quality & Maintainability

**Every line of code MUST be clear, maintainable, and follow consistent patterns.**

- **Go Standards**: Code MUST follow standard Go idioms and conventions (Effective Go guidelines)
- **Linting**: All code MUST pass golangci-lint v2.4.0 with the project's enabled ruleset (no exceptions without documented justification)
- **Package Organization**: Code MUST be organized in logical packages under `internal/` following clear domain boundaries (api, db, conf, checker, event, rss)
- **Error Handling**: Errors MUST be properly wrapped with context using `fmt.Errorf("context: %w", err)` patterns; errors MUST NOT be silently ignored
- **Dependencies**: External dependencies MUST be justified and kept minimal; prefer standard library solutions where viable
- **Code Complexity**: Functions exceeding cyclomatic complexity thresholds flagged by `gocyclo` MUST be refactored or explicitly justified in code review
- **Documentation**: Exported functions, types, and packages MUST have godoc comments explaining their purpose, parameters, and return values

**Rationale**: Maintainable code reduces technical debt, accelerates onboarding, and enables confident refactoring. Go's simplicity and standard idioms make code predictable across the team.

### II. Testing Standards

**Every feature MUST be proven correct through comprehensive, well-organized tests.**

- **Coverage Requirements**: 
  - Unit tests: Minimum 80% coverage for business logic in `internal/` packages
  - Integration tests: All API endpoints and database interactions MUST have integration tests in `tests/`
- **Test Organization**:
  - Unit tests: Co-located with source (`*_test.go` files in same package)
  - Integration tests: Separate `tests/` directory using testcontainers for real database interactions
- **Testing Practices**:
  - Table-driven tests MUST be used for multiple test cases of the same logic
  - Test names MUST follow `TestFunctionName_Scenario_ExpectedOutcome` pattern
  - Use `testify/assert` and `testify/require` for clean assertions
  - Mock external dependencies using interfaces (sqlmock for database unit tests)
  - Integration tests MUST use testcontainers-go/modules/postgres for real PostgreSQL instances
- **Test Execution**:
  - `make test` MUST run all unit tests (fast, no external dependencies)
  - `make test-acc` MUST run integration tests (requires Docker)
  - All tests MUST pass before merge; no flaky tests tolerated
- **Test Quality**: Tests MUST be deterministic, isolated, and repeatable; use `-count 1` to disable test caching during development

**Rationale**: Comprehensive testing prevents regressions, documents behavior, and enables confident refactoring. Separation of unit/integration tests provides fast feedback loops while ensuring system-level correctness.

### III. User Experience Consistency

**Users MUST experience predictable, accessible, and responsive interfaces across all touchpoints.**

- **API Consistency**:
  - RESTful principles MUST be followed (proper HTTP methods, status codes, resource naming)
  - API responses MUST follow consistent JSON structure with predictable field naming (snake_case)
  - Error responses MUST include helpful messages and appropriate HTTP status codes
  - API versioning MUST be explicit in URL paths (`/api/v1/`, `/api/v2/`)
- **UI/UX Patterns**:
  - Status indicators MUST use consistent color semantics (green=healthy, yellow=degraded, red=down)
  - Timestamps MUST be displayed in user's timezone with clear formatting
  - Loading states MUST be indicated during async operations
  - Empty states MUST provide helpful guidance, not blank screens
- **Accessibility**:
  - Web interfaces MUST meet WCAG 2.1 Level AA standards
  - Color MUST NOT be the only means of conveying information (include text labels/icons)
  - Keyboard navigation MUST work for all interactive elements
  - Screen reader-friendly semantic HTML MUST be used
- **Responsive Design**:
  - Interfaces MUST be usable on mobile, tablet, and desktop viewports
  - Critical information MUST be accessible without horizontal scrolling
  - Touch targets MUST be minimum 44x44 pixels for mobile interfaces
- **Data Presentation**:
  - RSS feeds MUST follow standard formats (github.com/gorilla/feeds)
  - Dashboards MUST prioritize critical information (current incidents over historical data)

**Rationale**: Consistent UX reduces cognitive load, improves user productivity, and ensures accessibility for all users regardless of abilities or devices. Status dashboards are often viewed during incidents when clarity is critical.

### IV. Performance Requirements

**The system MUST deliver fast, efficient responses under expected load conditions.**

- **Response Time Targets**:
  - API endpoints MUST respond within 200ms at p95 for reads (GET requests)
  - API endpoints MUST respond within 500ms at p95 for writes (POST/PUT/DELETE)
  - Dashboard page loads MUST complete within 2 seconds at p95 (including initial data fetch)
  - RSS feed generation MUST complete within 1 second
- **Resource Usage**:
  - Memory usage MUST stay below 512MB under normal load (monitoring required)
  - Database connections MUST be pooled and reused (GORM connection pool configured)
  - Long-running queries (>100ms) MUST be identified and optimized or indexed
- **Optimization Guidelines**:
  - Database queries MUST use appropriate indexes (monitor with EXPLAIN ANALYZE)
  - N+1 query problems MUST be eliminated using proper eager loading or joins
  - API responses MUST use pagination for list endpoints (default: 50 items, max: 100)
  - Static assets MUST be cacheable with appropriate headers
  - Expensive computations MUST be cached when appropriate (with documented invalidation strategy)
- **Monitoring**:
  - Structured logging with zap MUST capture performance metrics (request duration, DB query time)
  - Critical paths MUST be instrumented for observability
  - Performance regressions in CI MUST be caught before production (benchmark tests for critical paths)

**Rationale**: Performance directly impacts user trust in the status dashboard. During outages, users need fast, reliable status information. Resource efficiency ensures cost-effective operation and scalability.

## Technical Stack Standards

This section defines mandatory technology choices and configurations for the Status Dashboard project.

- **Language**: Go 1.24.2+ (MUST use Go modules)
- **Web Framework**: Gin (github.com/gin-gonic/gin) for HTTP routing and middleware
- **Database**: PostgreSQL with GORM v1.31.0+ for ORM
- **Migrations**: golang-migrate/migrate v4.19.0+ (SQL migrations in `db/migrations/`)
- **Authentication**: OpenID Connect via coreos/go-oidc v3.16.0+ with JWT tokens (golang-jwt/jwt v5.3.0+)
- **Logging**: Structured logging with uber-go/zap v1.27.0+; integrated with GORM via zapgorm2
- **Testing**: 
  - testify/assert and testify/require for assertions
  - testcontainers-go v0.39.0+ for integration tests
  - sqlmock for database unit test mocking
- **Configuration**: Environment variables loaded via godotenv (kelseyhightower/envconfig for structured config)
- **Build Tool**: Makefile with standard targets (test, test-acc, build, lint, migrate-up, migrate-down)

**Upgrade Policy**: Dependencies SHOULD be kept reasonably current (within 6 months of latest stable). Security patches MUST be applied within 2 weeks of disclosure.

## Development Workflow

This section defines mandatory processes for code changes and quality gates.

### Code Review Requirements

- All changes MUST be submitted via Pull Request (no direct commits to main)
- PRs MUST pass all CI checks before merge:
  - `make lint` (golangci-lint v2.4.0)
  - `make test` (all unit tests)
  - `make test-acc` (all integration tests)
- PRs MUST have at least one approval from a team member
- PRs MUST reference related issues/specs where applicable

### Quality Gates

1. **Pre-commit**: Developer MUST run `make lint` and `make test` locally
2. **CI Pipeline**: Automated checks MUST pass (lint + test + test-acc)
3. **Code Review**: Reviewer MUST verify:
   - Constitution compliance (all 4 core principles)
   - Test coverage meets requirements (80%+ for new code)
   - Documentation is present and clear
   - No security vulnerabilities introduced
4. **Pre-merge**: All conversations resolved, all checks green

### Commit Conventions

- Commit messages SHOULD follow conventional commits format: `type(scope): description`
- Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`
- Examples:
  - `feat(api): add v2 incident filtering endpoint`
  - `fix(checker): resolve nil pointer on empty response`
  - `test(db): add integration tests for incident queries`

## Governance

### Amendment Process

This constitution defines the non-negotiable principles for the Status Dashboard project. All development decisions MUST align with these principles.

- **Constitution Authority**: This constitution supersedes conflicting guidance in other documents
- **Amendments**: Changes to core principles require:
  1. Written proposal with rationale
  2. Team discussion and consensus
  3. Version bump (see Versioning Policy below)
  4. Update to dependent templates (spec-template.md, plan-template.md, tasks-template.md)
- **Exceptions**: Principle exceptions require:
  1. Documented justification in PR description
  2. Explicit approval from tech lead
  3. Addition to constitution as clarification if pattern repeats

### Versioning Policy

- **MAJOR** (X.0.0): Backward-incompatible changes to principles (removal/redefinition)
- **MINOR** (x.Y.0): New principle added or materially expanded guidance
- **PATCH** (x.y.Z): Clarifications, wording improvements, non-semantic refinements

### Compliance Review

- All PRs MUST include constitution compliance check in PR template
- Monthly review: Team SHOULD review constitution relevance and propose updates if practices evolved
- Annual review: Team MUST review constitution and update version metadata

### Related Guidance

- Runtime development workflows: See `.specify/templates/commands/` for agent-specific guidance
- Feature specification process: See `.specify/templates/spec-template.md`
- Implementation planning: See `.specify/templates/plan-template.md`
- Task generation: See `.specify/templates/tasks-template.md`

**Version**: 1.0.0 | **Ratified**: 2025-01-21 | **Last Amended**: 2025-01-21
