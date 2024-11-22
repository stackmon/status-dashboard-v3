# Incident management V2

This document is described the business logic schema for incident management. All actions require authorisation.

## Incident creation

For creation an incident, sent a POST request to endpoint `v2/incidents`.

The example:

```json
{
  "title": "Incident",
  "description": "any description",
  "components": [1,2],
  "impact": 1,
  "start_date": "2006-01-01T12:00:00Z",
  "end_date": "2006-01-02T12:00:00Z",
  "system": false
}
```

Fields `title`, `impact`, `components`, `start_date` are required.
The field `end_date` can be not nil only if `impact` is `0`.
The field `description` only valid for maintenance incidents (will be changed in the next releases).

### Business logic

The logic mostly based on v1:

<details><summary>Documentation</summary>

Update component status

Process component status update and open new incident if required:

- current active maintenance for the component - do nothing
- current active incident for the component - do nothing
- current active incident NOT for the component - add component into
  the list of affected components
- no active incidents - create new one
- current active incident for the component and requested
  impact > current impact - run handling:

  If a component exists in an incident, but the requested
  impact is higher than the current one, then the component
  will be moved to another incident if it exists with the
  requested impact, otherwise a new incident will be created
  and the component will be moved to the new incident.
  If there is only one component in an incident, and an
  incident with the requested impact does not exist,
  then the impact of the incident will be changed to a higher
  one, otherwise the component will be moved to an existing
  incident with the requested impact, and the current incident
  will be closed by the system.
  The movement of a component and the closure of an incident
  will be reflected in the incident statuses.

This method requires authorization to be used.
</details>
