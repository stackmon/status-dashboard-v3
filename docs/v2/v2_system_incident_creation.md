# System Incident Creation Logic

**Last Updated:** November 5, 2025  
**Document Version:** 1.0  
**Author:** Auto-generated documentation with human review :)
**Related Files:**
- `internal/api/v2/v2.go` - Main implementation
- `internal/db/db.go` - Database operations
- `tests/v2_system_incident_test.go` - Test suite
- `internal/api/errors/incident.go` - Error definitions


## Overview

System incidents are automated incidents created by monitoring systems to track widespread service disruptions affecting multiple components. Unlike regular incidents created manually by operators, system incidents follow special rules for creation, merging, and impact management.

## Key Concepts

### System Incident vs Regular Incident

- **System Incident** (`system: true`): Automatically created by monitoring systems, can be merged and have impact-based priority rules
- **Regular Incident** (`system: false`): Manually created by operators, takes precedence over system incidents

### Incident Types

Only `type: "incident"` is allowed for system incidents. Other types (`"maintenance"`, `"info"`) will be rejected with `ErrIncidentSystemCreationWrongType`.

## Core Logic Flow

The system incident creation process is handled by `handleSystemIncidentCreation()` which processes each component independently through `processSystemIncidentComponent()`.

### Decision Tree

For each component in the request, the system follows this decision tree:

```
1. Validate incident type == "incident"
   └─ If not → Return ErrIncidentSystemCreationWrongType

2. Fetch component and find active events (incidents + maintenances)
   
3. IF no active events found:
   └─ Call addComponentToSystemIncident()
      ├─ Search for existing active system incident with matching impact
      │  ├─ If found → Add component to existing incident
      │  └─ If not found → Create new system incident
      └─ Return incident ID

4. IF active events found, prioritize by type:
   
   a. HIGHEST PRIORITY: Maintenance Event
      └─ Return maintenance incident ID with error message
         (Cannot create system incident during maintenance)
   
   b. SECOND PRIORITY: Non-System Incident
      └─ Return existing incident ID
         (Manual incidents always take precedence)
   
   c. LOWEST PRIORITY: System Incident(s)
      └─ Take first system incident and compare impact:
         
         i. IF existing.impact >= requested.impact:
            └─ Return existing incident ID
               (Keep higher or equal impact incident)
         
         ii. IF existing.impact < requested.impact:
             └─ Call moveComponentFromToSystemIncidents()
                ├─ Search for system incident with target impact
                │  ├─ If found → Move component to that incident
                │  └─ If not found:
                │     ├─ If old incident has only 1 component:
                │     │  └─ Update old incident's impact (IncreaseIncidentImpact)
                │     └─ If old incident has multiple components:
                │        └─ Extract component to new incident (ExtractComponentsToNewIncident)
                └─ Return new/updated incident ID
```

## Main Functions

### `handleSystemIncidentCreation()`

**Purpose:** Entry point for system incident creation.

**Parameters:**
- `dbInst *db.DB` - Database instance
- `log *zap.Logger` - Logger
- `incData IncidentData` - Incident data with components, impact, etc.

**Returns:** `[]*ProcessComponentResp` - Array of results for each component

**Logic:**
1. Validates incident type must be "incident"
2. Sets default description if not provided
3. Fetches all components from database
4. Processes each component independently via `processSystemIncidentComponent()`
5. Returns aggregated results

### `processSystemIncidentComponent()`

**Purpose:** Process a single component for system incident assignment.

**Key Steps:**
1. Fetch active events for component
2. If no events → route to `handleComponentWithNoEvents()`
3. If events exist → route to `handleComponentWithExistingEvents()`

### `handleComponentWithNoEvents()`

**Purpose:** Handle component with no active incidents or maintenances.

**Behavior:** Calls `addComponentToSystemIncident()` to either:
- Add component to existing system incident with matching impact
- Create new system incident if none exists

### `handleComponentWithExistingEvents()`

**Purpose:** Apply priority rules when component has existing events.

**Priority Order:**
1. **Maintenance** (highest) → Return maintenance ID with error
2. **Non-system incident** → Return existing incident ID
3. **System incident** → Apply impact comparison logic

### `addComponentToSystemIncident()`

**Purpose:** Add component to appropriate system incident.

**Search Logic:**
1. Query all active system incidents
2. Search for incident with matching impact
3. If found:
   - Add component to incident
   - Add status update: "{component} added to the incident by system"
   - Save and return
4. If not found:
   - Create new system incident with specified impact
   - Add component to it
   - Return new incident

### `moveComponentFromToSystemIncidents()`

**Purpose:** Move component from lower impact to higher impact system incident.

**Scenarios:**

#### Scenario 1: Target System Incident Exists
- Find active system incident with target impact
- Move component using `MoveComponentFromOldToAnotherIncident()`
- Close old incident if it becomes empty (1 component)

#### Scenario 2: Old Incident Has Single Component
- No need to create new incident
- Update old incident's impact using `IncreaseIncidentImpact()`
- Component stays in same incident with upgraded impact

#### Scenario 3: Old Incident Has Multiple Components
- Extract component using `ExtractComponentsToNewIncident()`
- Create new system incident with target impact
- Leave other components in old incident
- Add status updates to both incidents

### `handleSystemIncidentWithImpactComparison()`

**Purpose:** Compare impacts and decide whether to keep or move component.

**Impact Rules:**
- `existing >= requested` → Keep component in existing incident
- `existing < requested` → Move component to higher impact incident

## Database Operations

### `MoveComponentFromOldToAnotherIncident()`

**Location:** `internal/db/db.go`

**Purpose:** Move component between two existing incidents.

**Actions:**
1. Add component to new incident
2. Add status to new incident: "{component} moved from {old_incident}"
3. Add status to old incident: "{component} moved to {new_incident}"
4. If `closeOld=true`:
   - Set old incident status to "resolved"
   - Set old incident end_date to now
   - Remove component association
5. Execute in transaction

### `ExtractComponentsToNewIncident()`

**Location:** `internal/db/db.go

**Purpose:** Create new incident and extract components from existing one.

**Actions:**
1. Create new incident with specified impact and components
2. Add status to new incident: "{component} moved from {old_incident}"
3. Add status to old incident: "{component} moved to {new_incident}"
4. Remove components from old incident
5. Mark statuses with `OutDatedSystem` status
6. Execute in transaction

### `IncreaseIncidentImpact()`

**Location:** `internal/db/db.go`

**Purpose:** Upgrade impact level of existing incident.

**Actions:**
1. Update incident impact field
2. Add status update documenting impact change
3. Save incident

## Response Structure

### `ProcessComponentResp`

```go
type ProcessComponentResp struct {
    ComponentID int    `json:"component_id"`
    IncidentID  int    `json:"incident_id"`
    Error       string `json:"error,omitempty"`
}
```

**Fields:**
- `ComponentID`: ID of the processed component
- `IncidentID`: ID of the assigned/created incident
- `Error`: Optional error message (e.g., maintenance exists)

## Request Structure

### `IncidentData`

```go
type IncidentData struct {
    Title       string     `json:"title" binding:"required"`
    Description string     `json:"description,omitempty"`
    Impact      *int       `json:"impact" binding:"required,gte=0,lte=3"`
    Components  []int      `json:"components" binding:"required"`
    StartDate   time.Time  `json:"start_date" binding:"required"`
    EndDate     *time.Time `json:"end_date,omitempty"`
    System      *bool      `json:"system,omitempty"`
    Type        string     `json:"type" binding:"required,oneof=maintenance info incident"`
    Updates     []EventUpdateData `json:"updates,omitempty"`
}
```

**For System Incidents:**
- `System` must be `true`
- `Type` must be `"incident"`
- `Impact` must be 1-3
- `Components` array of component IDs to affect

## Error Cases

### `ErrIncidentSystemCreationWrongType`

**Trigger:** System incident with type != "incident"

**Response:** HTTP 400 Bad Request

**Reason:** System incidents can only track active incidents, not maintenance or info events.

### `ErrIncidentCreationMaintenanceExists`

**Trigger:** Component has active maintenance

**Response:** Component result includes error message, returns maintenance incident ID

**Reason:** Maintenance events take precedence; no new incidents can be created during maintenance.

### Component Not Found

**Trigger:** Invalid component ID in request

**Response:** HTTP 400/404 error

**Reason:** Cannot create incident for non-existent component.

## Impact Comparison Examples

### Example 1: Component with No Events

**Request:**
```json
{
  "title": "API Gateway Issue",
  "impact": 2,
  "components": [5],
  "system": true,
  "type": "incident",
  "start_date": "2025-11-04T10:00:00Z"
}
```

**Result:** Creates new system incident (ID: 101) with impact=2

### Example 2: Component with Lower Impact System Incident

**Existing:** System incident (ID: 101, impact=1) affecting component 5

**Request:**
```json
{
  "impact": 3,
  "components": [5],
  "system": true,
  "type": "incident"
}
```

**Result:** 
- If incident 101 has only component 5: Update impact to 3
- If incident 101 has multiple components: Extract component 5 to new incident with impact=3

### Example 3: Component with Higher Impact System Incident

**Existing:** System incident (ID: 102, impact=3) affecting component 6

**Request:**
```json
{
  "impact": 1,
  "components": [6],
  "system": true,
  "type": "incident"
}
```

**Result:** Returns existing incident ID 102 (higher impact takes precedence)

### Example 4: Component with Non-System Incident

**Existing:** Regular incident (ID: 103, system=false) affecting component 7

**Request:**
```json
{
  "impact": 3,
  "components": [7],
  "system": true,
  "type": "incident"
}
```

**Result:** Returns existing incident ID 103 (manual incidents always win)

### Example 5: Component with Maintenance

**Existing:** Maintenance (ID: 104, type="maintenance") affecting component 8

**Request:**
```json
{
  "impact": 2,
  "components": [8],
  "system": true,
  "type": "incident"
}
```

**Result:** Returns maintenance ID 104 with error message

### Example 6: Multiple Components Mixed Scenario

**Request:**
```json
{
  "impact": 2,
  "components": [1, 2, 3],
  "system": true,
  "type": "incident"
}
```

**Component States:**
- Component 1: No events → Creates/reuses system incident with impact=2
- Component 2: Has system incident impact=1 → Moves to higher impact incident
- Component 3: Has maintenance → Returns maintenance ID with error

**Result:**
```json
{
  "result": [
    {"component_id": 1, "incident_id": 105},
    {"component_id": 2, "incident_id": 105},
    {"component_id": 3, "incident_id": 104, "error": "maintenance exists"}
  ]
}
```

## Testing

Comprehensive tests are available in `tests/v2_system_incident_test.go`:

- `TestV2SystemIncidentCreationWrongType` - Validates type enforcement
- `TestV2SystemIncidentCreationNoActiveEvents` - Tests clean slate scenario
- `TestV2SystemIncidentCreationWithMaintenance` - Tests maintenance priority
- `TestV2SystemIncidentCreationWithNonSystemIncident` - Tests manual incident priority
- `TestV2SystemIncidentSameImpact` - Tests impact equality
- `TestV2SystemIncidentHigherImpact` - Tests higher impact precedence
- `TestV2SystemIncidentLowerImpactSingleComponent` - Tests impact upgrade
- `TestV2SystemIncidentLowerImpactMultiComponent` - Tests component extraction
- `TestV2SystemIncidentReuseExisting` - Tests incident reuse
- `TestV2SystemIncidentMultipleComponents` - Tests batch processing
- `TestV2SystemIncidentMixedScenarios` - Tests complex scenarios

## LLM Prompt Guidelines

When generating code or explanations for system incident creation:

### Key Points to Remember

1. **Priority Order:** Maintenance > Non-System Incident > System Incident
2. **Impact Direction:** Higher impact always takes precedence
3. **Incident Reuse:** Always search for existing system incidents with matching impact before creating new ones
4. **Component Independence:** Each component is processed independently with its own result
5. **Transaction Safety:** All database operations use transactions to ensure consistency
6. **Automatic Descriptions:** System incidents get default descriptions if not provided
7. **Type Restriction:** Only `"incident"` type is allowed for system incidents

### Common Patterns

**Pattern 1: Clean Component**
```
No events → Search existing system incidents → Add to matching or create new
```

**Pattern 2: Component in Maintenance**
```
Has maintenance → Return maintenance ID immediately → No new incident
```

**Pattern 3: Component in Manual Incident**
```
Has non-system incident → Return existing ID → No new incident
```

**Pattern 4: Component in Lower Impact System Incident**
```
Has system incident (low) → Search for higher impact incident
  → If found: Move component
  → If not found and single component: Upgrade impact
  → If not found and multi component: Extract to new incident
```

**Pattern 5: Component in Higher/Equal Impact System Incident**
```
Has system incident (high) → Return existing ID → No changes
```

### Code Generation Template

When generating system incident creation calls:

```go
// Prepare system incident request
impact := 2 // major incident
systemTrue := true
incData := v2.IncidentData{
    Title:       "Automated Alert: Service Degradation",
    Description: "Monitoring detected elevated error rates",
    Impact:      &impact,
    Components:  []int{componentID1, componentID2},
    StartDate:   time.Now().UTC(),
    System:      &systemTrue,
    Type:        event.TypeIncident,
}

// Send request
resp, err := createIncident(incData)
if err != nil {
    return err
}

// Process results per component
for _, result := range resp.Result {
    if result.Error != "" {
        // Component in special state (maintenance, etc.)
        handleSpecialCase(result.ComponentID, result.IncidentID, result.Error)
    } else {
        // Component successfully assigned to incident
        trackIncident(result.ComponentID, result.IncidentID)
    }
}
```

## Architectural Decisions

### Why Independent Component Processing?

Each component is processed independently because:
1. Components may have different existing incidents
2. Different components may end up in different incidents based on existing state
3. Allows partial success when some components are in maintenance
4. Provides granular error reporting per component

### Why Impact-Based Merging?

System incidents are merged by impact level to:
1. Avoid incident sprawl from automated systems
2. Group similar-severity issues together
3. Provide clear dashboard view of system health
4. Allow automatic escalation as issues worsen

### Why Manual Incidents Take Precedence?

Non-system incidents override system incidents because:
1. Operators have more context than automated systems
2. Manual incidents may have custom communication plans
3. Prevents automated systems from interfering with incident response
4. Maintains operator authority over incident management

## Future Considerations

### Potential Enhancements

1. **Auto-Resolution:** Automatically resolve system incidents when monitoring systems report recovery
2. **Impact Decay:** Automatically downgrade impact if issue persists at lower severity
3. **Component Groups:** Handle related components as units
4. **Regional Isolation:** Separate system incidents by region/datacenter
5. **Incident Merging:** Merge multiple system incidents when root cause is identified

### Backward Compatibility

This system incident logic is specific to API v2. Legacy v1 API behavior is preserved in `internal/api/v1/v1.go`.

