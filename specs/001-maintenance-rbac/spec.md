# Feature Specification: Maintenance Management RBAC

**Feature Branch**: `001-maintenance-rbac`  
**Created**: 2025-01-21  
**Status**: Draft  
**Input**: User description: "Create a feature specification for RBAC (Role-Based Access Control) for maintenance management in a Go backend Status Dashboard project."

## Clarifications

### Session 2026-01-20

- Q: How should the existing "admin-group" middleware integrate with the new RBAC roles (sd_creators, sd_operators)? → A: admin-group is the future sd_admins role, that will have unrestricted access to all system settings, events, etc.
- Q: How should maintenance events be removed or cancelled? → A: No DELETE method exists; use status "cancelled" to remove/cancel events
- Q: When an sd_admins user performs operations (create/modify/delete/approve), should the system apply normal status workflow rules or allow bypassing status transitions entirely? → A: By default, then the roles sd_admins or sd_operators create maintenance, the status should be "planned". That's the current flow.
- Q: When a user has multiple roles (e.g., both sd_creators and sd_operators), how should the system determine which permissions apply? → A: Highest privilege role takes precedence (sd_admins > sd_operators > sd_creators)
- Q: What is the JWT token claim structure for roles? How are sd_admins, sd_creators, and sd_operators represented in the token? → A: Single 'groups' claim with array of role names
- Q: When sd_operators or sd_admins users create maintenance events (which go directly to 'planned' status), should the system still capture and display their user_id as the creator? → A: Always store creator user_id regardless of role
- Q: How should IdP group names be mapped to application roles (sd_creators, sd_operators, sd_admins) to avoid hardcoding group names in the codebase? → A: Environment variables map IdP group names to application roles (e.g., SD_CREATORS_GROUP, SD_OPERATORS_GROUP, SD_ADMINS_GROUP). Application reads JWT 'groups' claim and checks against configured env var values.
- Q: Role configuration mechanism: How are environment variables (SD_CREATORS_GROUP, SD_OPERATORS_GROUP, SD_ADMINS_GROUP) mapped to IdP groups? → A: Environment variables (SD_CREATORS_GROUP, SD_OPERATORS_GROUP, SD_ADMINS_GROUP) store IdP group names. Application reads JWT 'groups' claim and matches against these configured values.
- Q: Status transition automation: Which component handles the "reviewed" → "planned" status change? → A: Internal checker goroutine in existing "checker" module performs the status transition from "reviewed" to "planned"
- Q: Notification mechanism for pending reviews: How should sd_operators be notified about maintenance events in "pending review" status? → A: Badge count + status filter in list view - Operators see a count badge and can filter the maintenance list to show only "pending review" events (no separate notification endpoint)
- Q: Contact email validation rules: What validation should apply to the contact email field? → A: RFC 5322 format validation only - Email must pass RFC 5322 format validation. Any valid email address is allowed (no domain restrictions).
- Q: Concurrent approval handling: What happens when multiple sd_operators attempt to approve the same maintenance event simultaneously? → A: First approval wins, subsequent get error - Later approval requests receive 409 Conflict indicating the event is no longer in "pending review" status
- Q: How should maintenance events be distinguished from other event types (incidents, alerts) in the system? → A: Use explicit `event_type` field (e.g., "maintenance", "incident") to categorize events

- Q: Audit trail table structure: Should the system create a new audit trail table or use an existing table to track maintenance event status changes? → A: Use existing `incident_status` table as-is for audit trail functionality - no new table needed
- Q: Checker validation logic: What specific checks should the checker goroutine perform when transitioning "reviewed" events to "planned"? → A: Checker should NOT perform any validation when transitioning from "reviewed" → "planned". All validation checks are applied BEFORE this transition (at submission time). The checker simply transitions the state without additional validation.
- Q: Validation timing: When should validation checks (time window in future, required fields present, email format) be performed? → A: Validate at submission time - All validation checks (time window, required fields, email format) must be performed when the maintenance event is initially created. Reject invalid submissions immediately with 400 Bad Request.
- Q: Should validated events that later exceed their time window still be allowed to transition from "reviewed" to "planned"? → A: Allow transition to "planned" - validation was done at submission time, workflow continues. Time window validation only applies at submission time; once approved, events complete their workflow regardless of elapsed time.
- Q: What minimum time threshold should be enforced for scheduling maintenance events in the future (e.g., must be at least 1 hour, 24 hours ahead)? → A: No minimum future threshold - any start time > current timestamp is acceptable. Maximum flexibility for urgent maintenance while preventing backdating.
- Q: Concurrent modification conflicts: If an operator approves a maintenance event based on version X, but the event was updated to version X+1 by another user before approval is processed, how should the system handle this race condition? → A: Last write wins with version conflict detection - Use version/timestamp field. If operator approves based on outdated data, return 409 Conflict and require re-review of updated event.
- Q: What level of observability (logging, metrics, tracing) should be implemented for maintenance event status transitions and approval operations? → A: Basic logging only - Log state transitions with timestamp and user_id to application log.
- Q: How should the system handle errors and exceptional conditions specific to maintenance events (e.g., validation failures, status transition errors, approval conflicts)? → A: The checker module's existing error handling applies to maintenance events as well. Reuse proven error handling patterns from incident handling.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Creator Initiates Maintenance Event (Priority: P1)

A user with sd_creators role needs to schedule a maintenance window for their service. They create a new maintenance event with all necessary details (service, time window, description, contact email), which automatically enters a pending review state awaiting operator approval.

**Why this priority**: This is the foundation of the maintenance workflow. Without the ability to create maintenance events, the entire feature is non-functional. This story delivers immediate value by allowing service owners to request maintenance windows.

**Independent Test**: Can be fully tested by authenticating as an sd_creators user, submitting a maintenance creation request via API, and verifying the event is stored with "pending review" status and the creator's user ID is captured.

**Acceptance Scenarios**:

1. **Given** a user with sd_creators role is authenticated, **When** they submit a valid maintenance event with all required fields including a valid email and future time window, **Then** the system creates the event with status "pending review" and stores the creator's user_id from the JWT token
2. **Given** an sd_creators user submits a maintenance event with an invalid email format, **When** the system validates the submission, **Then** the system rejects the request with 400 Bad Request error at submission time
3. **Given** an sd_creators user submits a maintenance event with a time window in the past, **When** the system validates the submission, **Then** the system rejects the request with 400 Bad Request error at submission time
4. **Given** an sd_creators user submits a maintenance event with missing required fields, **When** the system validates the submission, **Then** the system rejects the request with 400 Bad Request error at submission time
5. **Given** an sd_creators user creates a maintenance event, **When** the event is created, **Then** the system stores the provided contact email and makes it visible as "Contact Email" to logged-in users
6. **Given** an sd_creators user submits a maintenance event, **When** viewing the event details, **Then** the creator's user_id is displayed as "Creator or Author" to logged-in users
7. **Given** an sd_creators user has a maintenance event in "pending review" status, **When** they request to modify the event, **Then** the system allows the modification
8. **Given** an sd_creators user has a maintenance event in "pending review" status, **When** they request to cancel the event (change status to "cancelled"), **Then** the system allows the cancellation

---

### User Story 2 - Operator Reviews and Approves Maintenance (Priority: P2)

A user with sd_operators role monitors pending maintenance requests. They see notifications about events awaiting review, examine the details, and approve legitimate requests. Upon approval, the event status changes to "reviewed" and awaits external validation before becoming "planned".

**Why this priority**: This completes the approval workflow and provides the control mechanism for maintenance requests. Without operator approval, all maintenance events would remain in limbo. This story enables governance over maintenance scheduling.

**Independent Test**: Can be fully tested by creating maintenance events in "pending review" status, authenticating as an sd_operators user, and verifying they can see pending notifications and approve events, changing their status to "reviewed".

**Acceptance Scenarios**:

1. **Given** there are maintenance events in "pending review" status, **When** an sd_operators user views the dashboard, **Then** they see a badge count indicating the number of pending reviews and can filter the list to show only "pending review" events
2. **Given** an sd_operators user is viewing a maintenance event with "pending review" status, **When** they click the "approve" button, **Then** the system changes the event status from "pending review" to "reviewed"
3. **Given** a maintenance event is in "reviewed" status, **When** the internal checker goroutine in the "checker" module processes it, **Then** the status automatically changes from "reviewed" to "planned" without additional validation

---

### User Story 3 - Permission Enforcement for Status-Based Actions (Priority: P3)

The system enforces role-based permissions throughout the maintenance lifecycle. Creators cannot modify events once they're under review, and operators cannot approve events that aren't in the correct status. All unauthorized actions are rejected with clear error messages.

**Why this priority**: This ensures data integrity and proper workflow enforcement. While critical for production use, the basic create/approve flow (P1 and P2) can function without complete permission enforcement during initial testing.

**Independent Test**: Can be fully tested by attempting various unauthorized actions (creator modifying reviewed event, operator approving planned event, wrong role accessing protected endpoints) and verifying all are rejected with appropriate HTTP status codes and error messages.

**Acceptance Scenarios**:

1. **Given** an sd_creators user has a maintenance event with status "reviewed", **When** they attempt to modify the event, **Then** the system rejects the request with a 403 Forbidden error
2. **Given** an sd_creators user has a maintenance event with status "reviewed", **When** they attempt to cancel the event (status change to "cancelled"), **Then** the system rejects the request with a 403 Forbidden error
3. **Given** an sd_operators user views a maintenance event with status "planned", **When** they attempt to approve it again, **Then** the system rejects the request indicating the event is not in "pending review" status
4. **Given** an sd_creators user attempts to approve a maintenance event, **When** they submit the approval request, **Then** the system rejects the request with a 403 Forbidden error indicating insufficient permissions
5. **Given** an unauthenticated user, **When** they attempt to access any maintenance management endpoint, **Then** the system rejects the request with a 401 Unauthorized error

---

### Edge Cases

- What happens when a user's JWT token contains a user_id that doesn't exist in the system?
- When a sd_creators user provides an email with invalid format, the system rejects creation with a 400 Bad Request error at submission time
- When a sd_creators user submits a maintenance event with a time window in the past, the system rejects creation with a 400 Bad Request error at submission time
- When a sd_creators user submits a maintenance event with missing required fields (service, description, contact email, time window), the system rejects creation with a 400 Bad Request error at submission time
- When multiple sd_operators users attempt to approve the same maintenance event simultaneously, the first approval succeeds (status → "reviewed") and subsequent attempts receive 409 Conflict error
- When an operator loads a maintenance event for review, and a creator modifies the event before the operator submits approval, the system returns 409 Conflict due to version mismatch, forcing the operator to reload and re-review the updated event
- How does the system handle a maintenance event that remains in "pending review" status for an extended period?
- When a user has multiple roles (sd_creators, sd_operators, sd_admins), the system applies the highest privilege role (sd_admins > sd_operators > sd_creators)
- What happens when a user has sd_admins role along with sd_creators or sd_operators roles?
- How does the system behave when a user transitions from admin-group to explicit sd_admins role assignment?
- What happens when a user's role is revoked while they have active maintenance events?


## Requirements *(mandatory)*

### Functional Requirements

#### Role Management

- **FR-001**: System MUST support three distinct roles: sd_admins (backward compatible with existing admin-group), sd_creators, and sd_operators
- **FR-002**: System MUST extract user roles from the 'groups' claim in the JWT token, which contains an array of role names (e.g., ["admin-group", "sd_creators"])
- **FR-002a**: System MUST map IdP group names to application roles using environment variables (SD_CREATORS_GROUP, SD_OPERATORS_GROUP, SD_ADMINS_GROUP). The application reads the JWT 'groups' claim and checks if any value matches the configured environment variable values to determine role membership.
- **FR-002b**: System MUST support dynamic IdP group name changes through environment variable updates without requiring code modifications
- **FR-002c**: System MUST implement CRUD operations for maintenance events using the `/v2/events` endpoint (POST for creation with type="maintenance") and `/v2/events/:eventID` endpoint (GET for retrieval, PATCH for modification), mirroring the structure of `/v2/incidents/*` endpoints. DELETE method MUST NOT be exposed for event removal.
- **FR-002d**: System MUST accept a "type" field in POST requests to `/v2/events` that determines the event type (e.g., "maintenance" for maintenance events, "incident" for incident events)
- **FR-003**: System MUST extract user_id from JWT token and store it with maintenance events
- **FR-003a**: System MUST recognize existing "admin-group" membership as equivalent to sd_admins role for backward compatibility (configurable via SD_ADMINS_GROUP environment variable)
- **FR-003b**: When a user has multiple roles, system MUST apply permissions from the highest privilege role using the precedence order: sd_admins > sd_operators > sd_creators

#### sd_creators Role Permissions

- **FR-004**: Users with sd_creators role MUST be able to create new maintenance events
- **FR-005**: When an sd_creators user creates a maintenance event via POST to the `/v2/events` endpoint with type="maintenance", the system MUST automatically set its status to "pending review"
- **FR-005a**: When an sd_operators user creates a maintenance event via POST to the `/v2/events` endpoint with type="maintenance", the system MUST automatically set its status to "planned" (bypassing the review workflow)
- **FR-005b**: When an sd_admins user creates a maintenance event via POST to the `/v2/events` endpoint with type="maintenance", the system MUST automatically set its status to "planned" (bypassing the review workflow)
- **FR-006**: Users with sd_creators role MUST be able to modify maintenance events ONLY when the event status is "pending review" using the PATCH method for endpoint `/v2/events/:eventID`
- **FR-007**: Users with sd_creators role MUST NOT delete maintenance events; instead they MUST set the event status to "cancelled" to remove events, and this operation is ONLY allowed when current status is "pending review"
- **FR-008**: System MUST require a valid email address during maintenance event creation
- **FR-008a**: System MUST validate contact email against RFC 5322 format specifications at submission time (during initial event creation)
- **FR-008b**: System MUST validate that the time window (start/end timestamps) is in the future relative to the submission time during initial event creation
- **FR-008c**: System MUST validate that all required fields are present (service, description, contact email, time window) at submission time during initial event creation
- **FR-008d**: System MUST reject maintenance creation requests that fail any validation check (email format, time window, required fields), returning a 400 Bad Request with a clear error message
- **FR-009**: System MUST reject modification attempts by sd_creators users when event status is not "pending review"
- **FR-010**: System MUST reject cancellation attempts (status change to "cancelled") by sd_creators users when event status is not "pending review"
- **FR-010a**: System MUST NOT expose a DELETE method on `/v2/events/:eventID` for maintenance events; event removal MUST be accomplished by changing status to "cancelled"

#### sd_operators Role Permissions

- **FR-011**: Users with sd_operators role MUST see a badge count in the UI indicating the number of maintenance events in "pending review" status
- **FR-011a**: Users with sd_operators role MUST be able to filter the maintenance event list to show only events with "pending review" status
- **FR-012**: Users with sd_operators role MUST be able to view all maintenance events regardless of status
- **FR-013**: Users with sd_operators role MUST be able to approve maintenance events with "pending review" status
- **FR-014**: When an sd_operators user approves a maintenance event, the system MUST change its status from "pending review" to "reviewed"
- **FR-015**: System MUST prevent sd_operators users from approving events that are not in "pending review" status
- **FR-015-1**: When multiple sd_operators users attempt to approve the same event simultaneously, the system MUST allow the first approval to succeed and return 409 Conflict for subsequent attempts with a message indicating the event is no longer in "pending review" status
- **FR-015-2**: System MUST implement version conflict detection using a version or timestamp field on maintenance events. When an operator attempts to approve an event based on outdated data (event was modified after the operator loaded it), the system MUST return 409 Conflict and require the operator to re-review the updated event before approving

#### sd_admins Role Permissions

- **FR-015a**: Users with sd_admins role (including existing admin-group members) MUST have unrestricted access to all maintenance management operations via the `/v2/events` endpoint and its sub-resources (POST `/v2/events`, PATCH `/v2/events/:eventID`, GET, etc.), regardless of event status
- **FR-015b**: System MUST bypass all status-based permission restrictions for sd_admins users
- **FR-015c**: System MUST NOT require sd_admins users to have sd_creators or sd_operators roles to perform any maintenance operation

#### Data Storage and Display

- **FR-016**: System MUST store the creator's user_id (from JWT token) in the maintenance event record for all users regardless of their role (sd_creators, sd_operators, or sd_admins)
- **FR-017**: System MUST expose the creator's user_id in API responses using the field name "creator"
- **FR-018**: System MUST display the creator information as "Creator or Author" in the UI for logged-in users
- **FR-019**: System MUST store the contact email provided during maintenance creation
- **FR-020**: System MUST display the contact email as "Contact Email" in the UI for logged-in users
- **FR-021**: System MUST NOT send actual email notifications (email field is for display purposes only)

#### Status Workflow

- **FR-022**: System MUST support the following status flow for sd_creators: pending review → reviewed → planned → [existing statuses]
- **FR-022a**: System MUST support direct "planned" status for events created by sd_operators and sd_admins users (bypassing pending review and reviewed statuses)
- **FR-022b**: System MUST support "cancelled" as a terminal status reachable from any other status, representing event removal/cancellation
- **FR-023**: The internal checker goroutine in the existing "checker" module MUST automatically change status from "reviewed" to "planned" without performing additional validation
- **FR-023a**: The checker goroutine MUST NOT perform any validation checks (including time window validation) when transitioning from "reviewed" to "planned"; all validation is completed at submission time before the event enters "pending review" status
- **FR-023b**: The checker MUST transition events from "reviewed" to "planned" status regardless of elapsed time since initial submission, as time window validation is enforced only at creation time
- **FR-023c**: The checker MUST transition events from "reviewed" to "planned" status as its sole responsibility during this state change
- **FR-023d**: System MUST reuse existing error handling patterns from the checker module's incident handling for all maintenance event errors, including validation failures, status transition errors, and approval conflicts
- **FR-024**: System MUST prevent manual status changes that skip steps in the workflow, except for sd_admins users who can transition to any status (including "cancelled")
- **FR-025**: System MUST maintain an audit trail of status changes including timestamp and user who initiated the change using the existing `incident_status` table (no new table creation required). Status transition logging (timestamp, user_id) is handled by the existing audit trail mechanism in the incident_status table.

#### Authorization and Security

- **FR-026**: System MUST validate JWT tokens on all maintenance management endpoints
- **FR-027**: System MUST return 401 Unauthorized for requests without valid JWT tokens
- **FR-028**: System MUST return 403 Forbidden when users attempt actions not permitted for their role
- **FR-029**: System MUST validate that the user_id in the JWT token matches the creator's user_id when enforcing creator-specific permissions
- **FR-030**: System MUST validate email format (RFC 5322) during maintenance event creation
- **FR-031**: System MUST return 409 Conflict when users attempt status transitions that conflict with the current state (e.g., approving an event not in "pending review" status)
- **FR-032**: System MUST return 409 Conflict when an approval attempt is based on an outdated version of the maintenance event (event was modified after being loaded by the operator), requiring the operator to reload and re-review the updated event

### Key Entities

- **Maintenance Event**: Represents a scheduled or planned maintenance window for a service. Created via POST to `/v2/events` endpoint with type="maintenance" and modified via PATCH to `/v2/events/:eventID`. Core attributes include unique identifier, type (set to "maintenance" to distinguish from incidents), service identifier, time window (start/end), description, status (pending review/reviewed/planned/cancelled/etc.), creator (user_id from JWT), contact email, version (or timestamp field for optimistic locking), created timestamp, updated timestamp, and audit trail of status changes (stored in the existing `incident_status` table). The version/timestamp field enables conflict detection to prevent approval based on outdated event data. Events are never deleted via DELETE method; removal is accomplished by transitioning to "cancelled" status.

- **User**: Represents an authenticated user in the system. Attributes include user_id (extracted from JWT token), roles (sd_admins, sd_creators, sd_operators, or combinations thereof), and authentication details. Users with existing "admin-group" membership are automatically granted sd_admins privileges. Users are related to Maintenance Events through the creator field.

- **Role**: Represents permission sets assigned to users. Three roles exist: sd_admins (unrestricted access to all maintenance operations via `/v2/events` endpoints, backward compatible with admin-group), sd_creators (can create and modify pending events), and sd_operators (can review and approve events). Roles determine which API endpoints and actions are accessible.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: sd_creators users can successfully create a maintenance event and see it in "pending review" status within 2 seconds
- **SC-002**: sd_operators users see the updated badge count when new maintenance events enter "pending review" status within 5 seconds of page refresh
- **SC-003**: Unauthorized modification attempts (wrong role or wrong status) are rejected with appropriate error codes (401/403) 100% of the time
- **SC-004**: The approval workflow (pending review → reviewed → planned) completes successfully for 100% of valid requests
- **SC-005**: Creator information (user_id) and contact email are accurately captured and displayed for 100% of maintenance events
- **SC-006**: System enforces status-based permissions correctly, preventing 100% of invalid state transitions
- **SC-007**: All maintenance management API endpoints respond within 500ms under normal load (up to 100 concurrent users)
