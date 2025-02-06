# Time Limits and Status Rules

## Incidents

### General Rules
- All incidents must have a start date
- End date remains null until incident is resolved
- End date can be modified after "resolved" status via "End date" field

### Valid Statuses
- analyzing
- fixing
- observing
- impact changed
- resolved
- changed
- reopened

### Time Validations
1. Incident Creation:
   - start_date must be <= current time

2. Status Updates:
   - entered_date must be <= current time
   - entered_date must be > incident start_date
   - entered_date must be > any previous status_timestamp

### Status-Specific Rules
- "changed":
  - end_date = entered_date
  - status_date = current time

- "analyzing", "fixing", "observing", "impact changed":
  - status_date = current time

- "resolved":
  - entered_date is optional
  - If entered_date is None: end_date = current time

- "reopened":
  - Sets end_date to None
  - status_date = current time

## Maintenance

### Valid Statuses
- completed
- modified
- in progress

### General Rules
- start_date is mandatory
- start_date must be < end_date
- end_date is mandatory

### Status-Specific Rules

#### "completed":
- start_date must be < current time
- Updates status to completed
- Sets end_date to current time

#### "in progress":
- update_date must be <= current time
- Only one status update allowed
- update_date must be < maintenance end_date

#### "modified":
- Used for modifying existing maintenance
- status_date = current time

### Validation Rules

#### Start Date Validations:
- Cannot be null
- Must be earlier than end_date
- Must be earlier than any status updates

#### End Date Validations:
- Cannot be null
- Must be later than start_date
- Must be later than any status updates