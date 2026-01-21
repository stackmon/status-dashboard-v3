---
description: "Backend task list for Maintenance Management RBAC feature implementation"
specId: "001-maintenance-rbac"
title: "Maintenance Management RBAC - Backend Implementation"
---

# Backend Tasks: Maintenance Management RBAC

**Specification**: `/specs/001-maintenance-rbac/spec.md`  
**Feature Branch**: `001-maintenance-rbac`  
**Scope**: Backend API, middleware, authorization layer, and database schemas only  
**Status**: Ready for implementation

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- **Max Duration**: 4 working hours per task
- Include exact file paths in descriptions
- **Definition of Done**: Unit tests for each role (sd_admins, sd_operators, sd_creators)

## Path Conventions

- Backend API handlers: `internal/api/v2/` (existing versioned API structure)
- Middleware: `internal/api/middleware.go`
- Database models: `internal/db/models.go`
- Authentication/Authorization: `internal/api/auth/`
- Database migrations: `db/migrations/`
- Tests: `tests/` directory

---

## Phase 1: Setup - RBAC Infrastructure & Role Validation

**Purpose**: Initialize role management system, environment configuration, and role validation schemes

**Duration**: 8 hours (4 tasks × 2 hours each)

- [ ] T001 Create RBAC role definitions and constants in internal/conf/rbac.go
  - Define role hierarchy: sd_admins > sd_operators > sd_creators
  - Define role permission matrices for each role
  - Define permission enums (CREATE_MAINTENANCE, MODIFY_MAINTENANCE, DELETE_MAINTENANCE, APPROVE_MAINTENANCE, VIEW_MAINTENANCE)
  - **Definition of Done**: Constants defined, no implementation needed yet; document role precedence logic

- [ ] T002 [P] Load and validate RBAC environment variables in internal/conf/conf.go
  - Load SD_ADMINS_GROUP, SD_OPERATORS_GROUP, SD_CREATORS_GROUP from environment
  - Load ALLOWED_EMAIL_DOMAINS (comma-separated) from environment
  - Validate environment variable format and provide defaults or error handling
  - **Definition of Done**: Unit tests verifying correct loading of env vars; tests for missing/invalid values

- [ ] T003 [P] Create role validation scheme in internal/api/auth/rbac.go
  - Implement GetUserRoles(jwtToken) → []string function extracting 'groups' claim from JWT
  - Implement GetHighestPrivilegeRole(roles []string, config) → Role function
  - Implement role comparison logic: sd_admins > sd_operators > sd_creators
  - Handle backward compatibility: if 'admin-group' in groups, map to sd_admins
  - **Definition of Done**: Unit tests for role extraction, precedence logic for each role combination (admin+creator, operator+creator, etc.); edge cases for missing roles

- [ ] T004 [P] Create permission checking utility functions in internal/api/auth/rbac.go
  - Implement HasPermission(role, permission) → bool
  - Implement HasRole(role, requiredRole) → bool
  - Implement CanPerformAction(user, action, resource) → bool
  - **Definition of Done**: Unit tests for each role checking each permission type; tests for role precedence affecting permissions

---

## Phase 2: Foundational - Authorization Middleware & User Context

**Purpose**: Core authorization infrastructure that ALL user stories depend on

**Duration**: 12 hours (3 tasks × 4 hours each)

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Enhance JWT middleware to extract and validate user identity in internal/api/middleware.go
  - Parse JWT token 'groups' claim (array of role names)
  - Extract user_id from JWT token 'sub' or 'user_id' claim
  - Parse JWT token 'email' claim if available
  - Validate token signature and expiration
  - Return 401 Unauthorized for invalid/missing tokens
  - Attach user context to request (user_id, roles, email)
  - **Definition of Done**: Unit tests for token validation, role parsing, user_id extraction; tests for malformed tokens, missing claims, invalid signatures; tests for each role being correctly identified

- [ ] T006 Create RBAC middleware for authorization checks in internal/api/middleware.go
  - Create RBACAuthMiddleware(requiredRole Role) middleware factory
  - Check if user has required role or higher privilege
  - Return 403 Forbidden for insufficient permissions
  - Log authorization attempts and failures
  - Handle multiple roles with precedence logic
  - **Definition of Done**: Unit tests for each role checking required role gates; tests for role precedence (admin accessing creator-only endpoints); tests for 403 responses; unit tests for each role combination

- [ ] T007 Create context utilities for user information access in internal/api/middleware.go
  - Implement GetUserFromContext(ctx) → User function
  - Implement GetUserRolesFromContext(ctx) → []string function
  - Implement GetUserIDFromContext(ctx) → string function
  - Implement GetHighestRoleFromContext(ctx) → Role function
  - Make user context accessible throughout request lifecycle
  - **Definition of Done**: Unit tests extracting each field from context; tests for missing context; integration with T005 JWT middleware

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Creator Initiates Maintenance Event (Priority: P1) 🎯 MVP

**Goal**: Allow sd_creators users to create maintenance events that enter "pending review" status. Enable modification and deletion of pending events by their creators. Implement email validation (RFC 5322 format + domain whitelist). Store creator user_id with each event.

**Independent Test**: Can be fully tested by authenticating as sd_creators user, submitting valid maintenance creation request, and verifying event is stored with "pending review" status and creator's user_id captured. Verify unauthorized users cannot create events.

**Duration**: 20 hours

### Data Model for User Story 1

- [ ] T008 [P] Extend maintenance event database schema in db/migrations/000006_add_rbac_fields.up.sql
  - Add creator (user_id TEXT NOT NULL) column to maintenance table
  - Add contact_email (TEXT NOT NULL) column to maintenance table
  - Create index on creator user_id for queries
  - Create index on status for pending review filtering
  - **Definition of Done**: Migration can rollback cleanly; schema validates NOT NULL constraints

- [ ] T009 [P] Extend Maintenance model in internal/db/models.go
  - Add Creator string field to Maintenance struct (user_id)
  - Add ContactEmail string field to Maintenance struct
  - Add JSON tags for API serialization
  - **Definition of Done**: Model compiles; serialization to JSON verified

- [ ] T010 [P] Create database query functions for status filtering in internal/db/db.go
  - Implement GetMaintenanceByStatus(ctx, status) → []Maintenance query function
  - Implement GetMaintenanceByCreator(ctx, userID) → []Maintenance query function
  - Implement CountByStatus(ctx, status) → int query function (for badge count)
  - **Definition of Done**: Unit tests for each query; tests with empty results; tests filtering by correct status

### Authorization & Validation for User Story 1

- [ ] T011 Create email validation utility in internal/api/v2/validation.go
  - Implement ValidateEmailFormat(email) → error function (RFC 5322 compliant)
  - Implement ValidateEmailDomain(email, allowedDomains []string) → error function
  - Load ALLOWED_EMAIL_DOMAINS from config via internal/conf/conf.go
  - Return 400 Bad Request with clear error message for invalid emails
  - **Definition of Done**: Unit tests for valid/invalid email formats; tests for domain whitelist checking; unit tests for each role attempting invalid domains

- [ ] T012 Create request model and validation for maintenance creation in internal/api/v2/models.go
  - Create CreateMaintenanceRequest struct with all required fields
  - Implement validation for request fields (service, time window, description, contact email)
  - Ensure creator field is NOT accepted from request (set from JWT)
  - **Definition of Done**: Unit tests validating each field; tests for missing required fields; tests rejecting creator field from request

### API Endpoints for User Story 1

- [ ] T013 Implement POST /api/v2/maintenance endpoint in internal/api/v2/v2.go
  - Extract user_id and roles from JWT context (uses T005, T006, T007)
  - Check HasRole(userRole, sd_creators or higher) permission (uses T004)
  - Validate request body using CreateMaintenanceRequest (uses T012)
  - Validate contact email format and domain (uses T011)
  - Set status to "pending review" for sd_creators, "planned" for sd_operators/sd_admins (FR-005, FR-005a, FR-005b)
  - Store creator user_id from JWT token
  - Store contact email from request
  - Return 201 Created with created event details
  - Return 400 Bad Request for validation errors
  - Return 403 Forbidden for insufficient permissions
  - Log creation attempt with user_id and result
  - **Definition of Done**: Unit tests for sd_creators creating event (status=pending review); tests for sd_operators creating event (status=planned); tests for sd_admins creating event (status=planned); tests for unauthorized users (401); tests for invalid email; tests for email domain validation; tests response structure and status codes

- [ ] T014 Implement GET /api/v2/maintenance/{id} endpoint enhancement in internal/api/v2/v2.go
  - Add CreatorUserID and ContactEmail fields to response
  - Return creator as "creator" field in JSON (FR-017)
  - Return contact_email as "contact_email" field in JSON (FR-020)
  - Ensure both logged-in and appropriate roles can view these fields
  - **Definition of Done**: Unit tests verifying creator field in response; tests verifying contact_email in response; tests for different roles viewing the fields

- [ ] T015 [P] Implement PUT /api/v2/maintenance/{id} endpoint in internal/api/v2/v2.go
  - Extract user_id and roles from JWT context
  - Check HasRole(userRole, sd_creators or higher) permission
  - Only allow modification if event status is "pending review" (FR-006)
  - Only allow if current user is creator OR user is sd_admins/sd_operators (FR-006)
  - Validate contact email if being updated (uses T011)
  - Update modified timestamp
  - Return 200 OK with updated event
  - Return 403 Forbidden if user cannot modify (wrong role or wrong status)
  - Return 409 Conflict if concurrent modification attempted
  - Log modification attempt with user_id
  - **Definition of Done**: Unit tests for sd_creators modifying own pending event (success); tests for sd_creators modifying reviewed event (403); tests for non-creators modifying pending event (403); tests for sd_admins modifying any event (success); tests for email validation on update; unit tests for each role's modification permissions

- [ ] T016 [P] Implement DELETE /api/v2/maintenance/{id} endpoint in internal/api/v2/v2.go
  - Extract user_id and roles from JWT context
  - Check HasRole(userRole, sd_creators or higher) permission
  - Only allow deletion if event status is "pending review" (FR-007)
  - Only allow if current user is creator OR user is sd_admins/sd_operators (FR-007)
  - Return 204 No Content on successful deletion
  - Return 403 Forbidden if user cannot delete (wrong role or wrong status)
  - Log deletion attempt with user_id
  - **Definition of Done**: Unit tests for sd_creators deleting own pending event (success); tests for sd_creators deleting reviewed event (403); tests for non-creators deleting events (403); tests for sd_admins deleting any event (success); unit tests for each role's deletion permissions

### Tests for User Story 1

- [ ] T017 [P] Implement unit tests for sd_creators maintenance creation flow in tests/rbac_creators_test.go
  - Test: sd_creators can create maintenance event
  - Test: Created event has status "pending review"
  - Test: Creator user_id is correctly stored
  - Test: Contact email is correctly stored
  - Test: Invalid email format rejected (400)
  - Test: Email domain not in whitelist rejected (400)
  - Test: Unauthenticated user cannot create (401)
  - Test: sd_creators cannot create when not authenticated
  - Mocks: Database, JWT token parsing, email validation
  - **Definition of Done**: All 8 tests passing; 100% coverage of creator creation logic

- [ ] T018 [P] Implement unit tests for sd_creators modification permissions in tests/rbac_creators_test.go
  - Test: sd_creators can modify own pending event
  - Test: sd_creators cannot modify reviewed event (403)
  - Test: sd_creators cannot modify planned event (403)
  - Test: sd_creators cannot modify other creator's event (403)
  - Test: sd_admins can modify any event
  - Test: Invalid email on modification rejected (400)
  - Mocks: Database, context, status checks
  - **Definition of Done**: All 6 tests passing; 100% coverage of modification logic

- [ ] T019 [P] Implement unit tests for sd_creators deletion permissions in tests/rbac_creators_test.go
  - Test: sd_creators can delete own pending event
  - Test: sd_creators cannot delete reviewed event (403)
  - Test: sd_creators cannot delete planned event (403)
  - Test: sd_creators cannot delete other creator's event (403)
  - Test: sd_admins can delete any event
  - Mocks: Database, context, status checks
  - **Definition of Done**: All 5 tests passing; 100% coverage of deletion logic

**Checkpoint**: User Story 1 complete - sd_creators can create, modify pending, and delete pending maintenance events with full RBAC enforcement

---

## Phase 4: User Story 2 - Operator Reviews and Approves Maintenance (Priority: P2)

**Goal**: Allow sd_operators to see pending maintenance events with badge count, filter by "pending review" status, and approve events (transitioning status to "reviewed"). Support concurrent approval conflict resolution. Enable sd_admins to approve any status.

**Independent Test**: Can be tested by creating maintenance events in "pending review" status, querying as sd_operators user, and verifying badge count and approval endpoint changes status to "reviewed".

**Duration**: 16 hours

### API Endpoints for User Story 2

- [ ] T020 Implement GET /api/v2/maintenance/stats/pending-count endpoint in internal/api/v2/v2.go
  - Extract user_id and roles from JWT context
  - Check HasRole(userRole, sd_operators or higher) permission
  - Query count of maintenance events with status "pending review" (uses T010)
  - Return JSON with count field
  - Return 403 Forbidden for insufficient permissions
  - Return 401 Unauthorized for missing token
  - Cache result or implement efficient query
  - **Definition of Done**: Unit tests for sd_operators seeing count (success); tests for sd_creators requesting count (403); tests for unauthenticated users (401); tests for zero pending events; unit tests for each role's access to pending count

- [ ] T021 Enhance GET /api/v2/maintenance endpoint with status filtering in internal/api/v2/v2.go
  - Add optional query parameter ?status=pending_review
  - Extract user_id and roles from JWT context
  - Check HasRole(userRole, sd_operators or higher) permission for filtering
  - Query maintenance events filtered by status (uses T010)
  - Pagination support (limit, offset)
  - Return 200 OK with list of events
  - Return 403 Forbidden for insufficient permissions (if role cannot see pending)
  - Return 400 Bad Request for invalid status value
  - **Definition of Done**: Unit tests for sd_operators filtering pending events; tests for sd_creators filtering (403 if not allowed); tests for invalid status values; tests pagination; unit tests for each role's filtering abilities

- [ ] T022 Implement POST /api/v2/maintenance/{id}/approve endpoint in internal/api/v2/v2.go
  - Extract user_id and roles from JWT context
  - Check HasRole(userRole, sd_operators or higher) permission (FR-013)
  - Validate event exists and current status is "pending review" (FR-015)
  - Use database transaction to prevent race conditions (FR-015-1)
  - Update status from "pending review" to "reviewed"
  - Store approver user_id and approval timestamp
  - Return 200 OK with updated event
  - Return 403 Forbidden for insufficient permissions
  - Return 404 Not Found if event doesn't exist
  - Return 409 Conflict if event not in "pending review" status (FR-015-1)
  - Return error message for 409: "Event is no longer in pending review status"
  - Log approval with user_id
  - **Definition of Done**: Unit tests for sd_operators approving pending event (status=reviewed); tests for concurrent approvals returning 409 on second attempt; tests for approving non-pending event (409); tests for sd_creators approving (403); tests for unauthenticated users (401); unit tests for each role's approval permissions; tests for race condition handling

- [ ] T023 [P] Create approval request model and response in internal/api/v2/models.go
  - Create MaintenanceApprovalRequest struct (if additional fields needed, e.g., approval notes)
  - Create MaintenanceApprovalResponse struct with status, approver_id, approval_timestamp
  - **Definition of Done**: Models compile; JSON serialization works

### Authorization & Workflow for User Story 2

- [ ] T024 Implement concurrent approval conflict handling in internal/db/db.go
  - Create UpdateMaintenanceStatus(ctx, id, fromStatus, toStatus, approverID) → error function
  - Use UPDATE with WHERE clause checking current status (atomic operation)
  - Return conflict error if status doesn't match fromStatus (handles race conditions)
  - Store approver_id and approval_timestamp on status change
  - **Definition of Done**: Unit tests for successful approval; tests for concurrent update returning error; tests for missing approver_id; tests for timestamp precision

- [ ] T025 Extend maintenance event schema in db/migrations/000006_add_rbac_fields.up.sql (or 000007 if needed)
  - Add approver_id (TEXT) column to maintenance table (nullable, set on approval)
  - Add approval_timestamp (TIMESTAMP) column to maintenance table (nullable)
  - Create index on status='pending_review' for efficient query (FR-011a)
  - **Definition of Done**: Migration can rollback cleanly; schema added

- [ ] T026 Extend Maintenance model in internal/db/models.go
  - Add ApproverID *string field (nullable, set on approval)
  - Add ApprovalTimestamp *time.Time field (nullable)
  - Add JSON tags for API serialization
  - **Definition of Done**: Model compiles; serialization verified

### Tests for User Story 2

- [ ] T027 [P] Implement unit tests for sd_operators approval flow in tests/rbac_operators_test.go
  - Test: sd_operators can see pending count
  - Test: sd_operators can filter by pending status
  - Test: sd_operators can approve pending event
  - Test: Approved event status changes to "reviewed"
  - Test: Approver ID is correctly stored
  - Test: Approval timestamp is correctly stored
  - Mocks: Database, context, status checks
  - **Definition of Done**: All 6 tests passing; 100% coverage of operator approval logic

- [ ] T028 [P] Implement unit tests for concurrent approval handling in tests/rbac_operators_test.go
  - Test: First approval succeeds (status → reviewed)
  - Test: Concurrent second approval returns 409 Conflict
  - Test: 409 response includes clear error message
  - Test: Non-operator attempting approval gets 403
  - Mocks: Database with transaction simulation, concurrent requests
  - **Definition of Done**: All 4 tests passing; concurrent approval conflict resolution verified; unit tests for each role's approval permissions

- [ ] T029 [P] Implement unit tests for sd_operators access controls in tests/rbac_operators_test.go
  - Test: sd_operators can view all maintenance events
  - Test: sd_creators cannot filter pending events (403)
  - Test: sd_admins can approve any status (bypasses pending check)
  - Test: Unauthenticated users get 401
  - Mocks: Database, context, permissions
  - **Definition of Done**: All 4 tests passing; unit tests for each role's access control

**Checkpoint**: User Story 2 complete - sd_operators can review pending events and approve them with concurrent conflict handling

---

## Phase 5: User Story 3 - Permission Enforcement for Status-Based Actions (Priority: P3)

**Goal**: Enforce role-based permissions throughout maintenance lifecycle. Prevent unauthorized status transitions. Ensure creators cannot modify reviewed events, operators cannot approve non-pending events, and only sd_admins can bypass restrictions. Implement sd_admins unrestricted access.

**Independent Test**: Can be tested by attempting unauthorized actions (creator modifying reviewed event, operator approving planned event) and verifying all are rejected with appropriate HTTP status codes and error messages.

**Duration**: 12 hours

### Authorization & Policy Enforcement for User Story 3

- [ ] T030 Create permission policy engine in internal/api/auth/policy.go
  - Implement CanCreateMaintenance(role) → bool
  - Implement CanModifyMaintenance(role, eventStatus, userID, creatorID) → bool
  - Implement CanDeleteMaintenance(role, eventStatus, userID, creatorID) → bool
  - Implement CanApproveMaintenance(role, eventStatus) → bool
  - Implement CanViewPendingMaintenance(role) → bool
  - Apply hierarchy: sd_admins bypass all restrictions, sd_operators restricted by status, sd_creators restricted by creator/status
  - **Definition of Done**: Unit tests for each policy function with each role; tests for edge cases (owner vs non-owner); tests for role precedence; unit tests for sd_admins bypassing all restrictions

- [ ] T031 [P] Implement permission checks at endpoint level in internal/api/v2/v2.go
  - Add permission check before every modification endpoint (PUT, DELETE, POST /approve)
  - Use policy engine from T030
  - Return 403 Forbidden with detailed error message for denied actions
  - Log permission denials with user_id and attempted action
  - Ensure consistent error messages across all endpoints
  - **Definition of Done**: All modification endpoints have permission checks; unit tests for permission denials; error message consistency verified

- [ ] T032 [P] Add status transition validation in internal/db/db.go
  - Implement ValidateStatusTransition(fromStatus, toStatus, role) → error function
  - Prevent invalid transitions (e.g., planned → pending review, except for sd_admins)
  - Support status flow: pending review → reviewed → planned (for sd_creators)
  - Support direct planned status (for sd_operators, sd_admins)
  - Allow sd_admins to transition to any status (FR-024)
  - Return 409 Conflict for invalid transitions
  - **Definition of Done**: Unit tests for valid transitions per role; tests for invalid transitions; tests for sd_admins bypassing; unit tests for each role's transition permissions

- [ ] T033 Create sd_admins unrestricted access layer in internal/api/auth/admin.go
  - Implement IsAdminRole(role) → bool function
  - Document admin permissions: create/modify/delete/approve ANY maintenance regardless of status
  - Document backward compatibility: existing admin-group maps to sd_admins
  - **Definition of Done**: Unit tests for admin detection; tests for admin permissions; integration tests showing admin bypassing all restrictions

### Audit & Compliance for User Story 3

- [ ] T034 [P] Create audit logging for authorization checks in internal/api/middleware.go
  - Log all authorization decisions (permit/deny)
  - Include user_id, role, requested action, resource, result
  - Use structured logging format
  - Enable debug-level detailed logging
  - **Definition of Done**: Unit tests for log output; tests for each role's authorization attempts; logs verified for audit trail

- [ ] T035 Extend database schema for audit trail in db/migrations/000008_add_audit_trail.up.sql
  - Add status_change_history table with: id, maintenance_id, old_status, new_status, changed_by_user_id, changed_at_timestamp
  - Create foreign key to maintenance table
  - Create index on maintenance_id for historical queries
  - **Definition of Done**: Migration can rollback; schema validated

### Tests for User Story 3

- [ ] T036 [P] Implement unit tests for permission enforcement in tests/rbac_permissions_test.go
  - Test: sd_creators cannot modify reviewed event (403)
  - Test: sd_creators cannot modify planned event (403)
  - Test: sd_creators cannot delete reviewed event (403)
  - Test: sd_creators cannot approve any event (403)
  - Test: sd_operators cannot create maintenance
  - Test: sd_operators cannot approve non-pending event (409)
  - Test: sd_admins can create any maintenance
  - Test: sd_admins can modify any maintenance
  - Test: sd_admins can delete any maintenance
  - Test: sd_admins can approve any status
  - Mocks: Database, context, status values
  - **Definition of Done**: All 10+ tests passing; 100% coverage of permission enforcement; unit tests for each role's restricted actions

- [ ] T037 [P] Implement unit tests for status transition validation in tests/rbac_permissions_test.go
  - Test: Valid transition pending→reviewed allowed (operators)
  - Test: Invalid transition planned→pending rejected (409)
  - Test: Invalid transition reviewed→pending rejected (409)
  - Test: sd_admins can transition to any status
  - Test: Creator created events have correct initial status
  - Test: Operator created events have correct initial status
  - Test: Admin created events have correct initial status
  - Mocks: Database, status values
  - **Definition of Done**: All 7 tests passing; transition validation verified; unit tests for each role's status transitions

- [ ] T038 [P] Implement integration tests for complete RBAC flow in tests/rbac_integration_test.go
  - Test complete User Story 1 flow: creator creates → pending review status
  - Test complete User Story 2 flow: operator approves → reviewed status
  - Test complete User Story 3 flow: creator attempt to modify reviewed → 403
  - Test complete flow with sd_admins: admin creates, modifies, deletes any event
  - Test role precedence: user with multiple roles uses highest privilege
  - Test concurrent operations: multiple users operating on same event
  - Mocks: Full API stack with database, JWT tokens, multiple roles
  - **Definition of Done**: All integration tests passing; end-to-end workflows verified; concurrent operation handling verified

- [ ] T039 [P] Implement unit tests for error responses in tests/rbac_errors_test.go
  - Test: 401 Unauthorized responses include clear error message
  - Test: 403 Forbidden responses include reason for denial
  - Test: 409 Conflict responses on concurrent approvals include status message
  - Test: 400 Bad Request on validation errors (email, required fields)
  - Test: Consistent error response format across all endpoints
  - Mocks: API handlers, various error conditions
  - **Definition of Done**: All error response tests passing; error message clarity verified; format consistency checked

**Checkpoint**: User Story 3 complete - Comprehensive RBAC permission enforcement across entire maintenance lifecycle

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final improvements, documentation, and validation

**Duration**: 8 hours

- [ ] T040 [P] Create comprehensive API documentation in docs/API_RBAC.md
  - Document all RBAC endpoints with examples
  - Document error codes and meanings (401, 403, 409, 400)
  - Document role permissions matrix
  - Document JWT token requirements and claims
  - Document environment variable configuration
  - **Definition of Done**: Documentation complete and accurate; examples match implementation

- [ ] T041 [P] Create RBAC configuration guide in docs/RBAC_SETUP.md
  - Document environment variables (SD_ADMINS_GROUP, SD_OPERATORS_GROUP, SD_CREATORS_GROUP, ALLOWED_EMAIL_DOMAINS)
  - Provide example .env configuration
  - Document role precedence logic
  - Document troubleshooting guide
  - **Definition of Done**: Guide is complete and reproducible from guide alone

- [ ] T042 Create database migration rollback verification script in db/verify_migrations.sh
  - Test all migrations can be applied in order
  - Test all migrations can be rolled back cleanly
  - Verify schema at each step
  - **Definition of Done**: Script runs without errors; all migrations verified forward and backward

- [ ] T043 Update existing API route registration in internal/api/routes.go
  - Register all new RBAC endpoints
  - Apply RBAC middleware to all maintenance endpoints
  - Ensure consistent middleware ordering
  - **Definition of Done**: Routes registered; middleware applied; tested via startup

- [ ] T044 Create RBAC initialization checklist in docs/IMPLEMENTATION_CHECKLIST.md
  - Step-by-step verification of all RBAC components
  - Database schema verification
  - Environment variable verification
  - API endpoint verification (201 create, 200 modify, 204 delete, 200 approve)
  - Authorization verification per role
  - End-to-end workflow verification
  - **Definition of Done**: Checklist complete; all items verified during implementation

- [ ] T045 [P] Run full test suite and verify coverage in tests/
  - Execute all unit tests for RBAC (T017-T019, T027-T029, T036-T039)
  - Verify code coverage ≥ 80% for all RBAC modules
  - Generate coverage report
  - Document any coverage gaps
  - **Definition of Done**: All tests passing; coverage report shows ≥ 80%; gaps documented

- [ ] T046 Create backend configuration example in docs/backend-config-example.env
  - Example environment variables for local development
  - Example environment variables for staging
  - Example environment variables for production
  - Comment explaining each variable
  - **Definition of Done**: Examples complete; values are sensible defaults

---

## Dependencies & Execution Order

### Phase Dependencies

1. **Phase 1 (Setup)** → No dependencies - START HERE
2. **Phase 2 (Foundational)** → Depends on Phase 1 - BLOCKS all user stories
3. **Phase 3 (User Story 1)** → Depends on Phase 2 - INDEPENDENT of US2, US3
4. **Phase 4 (User Story 2)** → Depends on Phase 2 - INDEPENDENT of US1, US3
5. **Phase 5 (User Story 3)** → Depends on Phase 2 - INDEPENDENT of US1, US2
6. **Phase 6 (Polish)** → Depends on desired stories completion

### Critical Path (MVP)

1. ✅ Phase 1: Setup (T001-T004) - 8 hours
2. ✅ Phase 2: Foundational (T005-T007) - 12 hours
3. ✅ Phase 3: User Story 1 (T008-T019) - 20 hours
4. **TOTAL MVP**: 40 hours = 1 developer week

### Full Implementation

1. Phase 1: Setup - 8 hours
2. Phase 2: Foundational - 12 hours
3. Phase 3: User Story 1 - 20 hours (parallel: T008-T010, T017-T019)
4. Phase 4: User Story 2 - 16 hours (parallel: T020-T023, T027-T029)
5. Phase 5: User Story 3 - 12 hours (parallel: T030-T032, T036-T039)
6. Phase 6: Polish - 8 hours (parallel: T040-T041, T045-T046)
7. **TOTAL**: 76 hours ≈ 2 developer weeks

### Parallel Execution Example (Team of 3 Developers)

**Week 1:**
- All 3 developers: Phase 1 Setup (4 hours total, complete T001-T004)
- All 3 developers: Phase 2 Foundational (6 hours total, complete T005-T007)

**Week 2:**
- Developer A: Phase 3 (US1) - T008-T019 (20 hours)
- Developer B: Phase 4 (US2) - T020-T029 (16 hours, starts after Phase 2 complete)
- Developer C: Phase 5 (US3) - T030-T039 (12 hours, starts after Phase 2 complete)

**Week 3:**
- All 3 developers: Phase 6 Polish (2 hours each, complete T040-T046)

### Parallel Opportunities Within Phases

**Phase 1 Setup**: T001, T002, T003, T004 can run in parallel (separate concerns)

**Phase 2 Foundational**: T006, T007 can run in parallel with each other (after T005)

**Phase 3 User Story 1**:
- Data Model: T008, T009, T010 run in parallel (separate database concerns)
- Authorization: T011, T012 run in parallel (separate validation concerns)
- Endpoints: T013 completes first (depends on prior), then T014, T015, T016 run in parallel
- Tests: T017, T018, T019 run in parallel (separate test concerns)

**Phase 4 User Story 2**:
- API Endpoints: T020, T021, T022 run in parallel (separate endpoints)
- Authorization: T024, T025, T026 run in parallel (separate database concerns)
- Tests: T027, T028, T029 run in parallel (separate test concerns)

**Phase 5 User Story 3**:
- Authorization: T030, T031, T032 can start in parallel, T033 depends on T030
- Audit: T034, T035 run in parallel
- Tests: T036, T037, T038, T039 run in parallel (separate test concerns)

---

## Implementation Strategy

### MVP Scope (Recommended Start)

**Deliver User Story 1 Only** - Creates foundation for other stories

1. Complete Phase 1: Setup (T001-T004) - 8 hours
2. Complete Phase 2: Foundational (T005-T007) - 12 hours
3. Complete Phase 3: User Story 1 (T008-T019) - 20 hours
4. **Subtotal**: 40 hours (1 week for single developer)
5. **Verification**: sd_creators can create, modify pending, and delete pending maintenance events
6. **Deploy/Demo**: Share with stakeholders, verify API works end-to-end

### Incremental Delivery

**After MVP approved:**

1. Add Phase 4: User Story 2 (T020-T029) - 16 hours
   - **Verification**: sd_operators can review and approve pending events
   - **Deploy/Demo**: Share operator workflow
   
2. Add Phase 5: User Story 3 (T030-T039) - 12 hours
   - **Verification**: All unauthorized actions properly blocked
   - **Deploy/Demo**: Complete RBAC system ready for production

3. Add Phase 6: Polish (T040-T046) - 8 hours
   - **Documentation complete**
   - **All tests passing**
   - **Configuration examples ready**

### Single Developer Strategy (4 Weeks)

- **Week 1**: Phase 1 + Phase 2 (20 hours)
- **Week 2**: Phase 3 US1 (20 hours) - MVP Complete
- **Week 3**: Phase 4 US2 (16 hours)
- **Week 4**: Phase 5 US3 (12 hours) + Phase 6 Polish (8 hours)

### Team Strategy (2 Developers, 3 Weeks)

- **Week 1**: Phase 1 + Phase 2 together (20 hours total = 10 each)
- **Week 2**: Dev A does Phase 3 (US1), Dev B does Phase 4 (US2) - run in parallel
- **Week 3**: Dev A does Phase 5 (US3), Dev B does Phase 6 Polish - then switch and verify each other's work

### Code Quality Requirements

- **Unit Test Coverage**: Minimum 80% across all RBAC modules
- **Integration Test Coverage**: Critical workflows (create → approve → verify) 100% covered
- **Code Review**: All changes reviewed for security (authorization, SQL injection, token validation)
- **Error Handling**: All error paths tested and return appropriate HTTP status codes
- **Concurrent Operations**: Race conditions in approval workflow tested and handled

### Definition of Done for Each Task

**For API Endpoint Tasks** (T013-T016, T020-T022):
- [ ] Unit tests pass (authorization, validation, status checks)
- [ ] Response format matches spec (status codes, JSON structure)
- [ ] Error handling covers all failure modes
- [ ] Endpoint registered in routes.go
- [ ] Middleware applied (authentication, RBAC)
- [ ] Logs generated for audit trail

**For Database Tasks** (T008-T010, T024-T026, T035):
- [ ] Migration can be applied cleanly
- [ ] Migration can be rolled back cleanly
- [ ] Models updated to match schema
- [ ] Queries tested with empty, single, and multiple records
- [ ] Indexes created for performance-critical queries

**For Validation Tasks** (T011-T012, T023):
- [ ] All validation rules tested (valid and invalid cases)
- [ ] Error messages clear and actionable
- [ ] Unit tests for each rule
- [ ] Integration with API endpoints verified

**For Authorization Tasks** (T005-T007, T030-T034):
- [ ] Each role tested separately (sd_admins, sd_operators, sd_creators)
- [ ] Role combinations tested (precedence verified)
- [ ] Unauthorized access blocked (401, 403 responses)
- [ ] Logs show authorization attempts and results
- [ ] Unit tests pass for each role

**For Test Tasks** (T017-T019, T027-T029, T036-T039):
- [ ] All tests pass
- [ ] Mocks are realistic and complete
- [ ] Coverage ≥ 80% of tested module
- [ ] Tests are independent (can run in any order)
- [ ] Tests document expected behavior
- [ ] Each role (sd_admins, sd_operators, sd_creators) tested

---

## Success Criteria

- **Functionality**: All three user stories independently deployable and testable
- **Authorization**: 100% of unauthorized actions rejected with appropriate error codes
- **Concurrency**: Approval workflow handles concurrent requests without race conditions
- **Performance**: All endpoints respond within 500ms (normal load)
- **Testing**: ≥80% code coverage for RBAC modules
- **Documentation**: API, configuration, and implementation guide complete and accurate
- **Maintainability**: Code follows project conventions, is reviewable, and uses clear abstractions

