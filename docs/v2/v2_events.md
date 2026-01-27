# Events V2

## Overview

The `/v2/events` endpoint provides full event management capabilities, including listing events with pagination, creating new events, updating existing events, and extracting components. This endpoint supports incidents, maintenances, and informational events.

The existing `/v2/incidents` endpoint remains for backward compatibility but is **deprecated**. All operations available on `/v2/incidents` are now also available on `/v2/events`:

| Operation | Deprecated Endpoint | New Endpoint |
|-----------|-------------------|--------------|
| List events | `GET /v2/incidents` | `GET /v2/events` (with pagination) |
| Create event | `POST /v2/incidents` | `POST /v2/events` |
| Get event | `GET /v2/incidents/:eventID` | `GET /v2/events/:eventID` |
| Update event | `PATCH /v2/incidents/:eventID` | `PATCH /v2/events/:eventID` |
| Extract components | `POST /v2/incidents/:eventID/extract` | `POST /v2/events/:eventID/extract` |
| Update event text | `PATCH /v2/incidents/:eventID/updates/:updateID` | `PATCH /v2/events/:eventID/updates/:updateID` |

> **Note**: We recommend using `/v2/events` for all new integrations. The `/v2/incidents` endpoints will be removed in a future version.

## Handler Function: `GetEventsHandler`

### Description

This handler is responsible for fetching events from the database. It is used by both `/v2/events` (with pagination) and `/v2/incidents` (without pagination, deprecated).

- **Parameter Parsing**: It parses and validates query parameters for filtering and pagination.
- **Data Retrieval**: Fetches a list of incidents from the database based on the provided filters. For paginated requests, it also retrieves the total count of matching records.
- **Response Formatting**: Formats the retrieved events into the appropriate JSON structure. For the `/v2/events` endpoint, it includes a `pagination` object in the response.

## Endpoint: `/v2/events`

### Request

- **Method**: `GET`
- **Endpoint**: `/v2/events`
- **Headers**:
  - `Content-Type: application/json`

### Query Parameters

#### Filtering

- `type` (string): Filters events by type. Can be a single type or a comma-separated list (e.g., `incident,maintenance`).
- `active` (boolean): If `true`, returns only active events.
- `status` (string): Filters events by their current status (e.g., `resolved`, `in progress`).
- `start_date` (string): Filters events that start on or after this date (RFC3339 format: `YYYY-MM-DDTHH:MM:SSZ`).
- `end_date` (string): Filters events that end on or before this date (RFC3339 format: `YYYY-MM-DDTHH:MM:SSZ`).
- `impact` (integer): Filters events by impact level (0-3).
- `system` (boolean): Filters system-generated events.
- `components` (string): Filters events affecting specific components. Provide a comma-separated list of component IDs.

#### Pagination

- `page` (integer): The page number to retrieve.
  - **Default**: `1`
  - **Constraint**: Must be `gte=1`.
- `limit` (integer): The number of items per page.
  - **Default**: `50`
  - **Allowed Values**: `10`, `20`, `50`.


### Example Request

```bash
curl -X GET "http://localhost:8000/v2/events?page=1&limit=10&type=incident"
```

### Response

- **Status Code**: `200 OK`
- **Content-Type**: `application/json`

### JSON Response Structure

The handler returns a JSON object containing the list of events and pagination details.

```json
{
    "data": [
        {
            "id": 200,
            "title": "OpenStack problem in regions EU-DE/EU-NL",
            "description": "The service is partially unavailable or its performance has decreased.",
            "impact": 1,
            "components": [
                218,
                254
            ],
            "start_date": "2025-05-20T10:00:00Z",
            "end_date": null,
            "system": false,
            "type": "incident",
            "updates": [
                {
                    "id": 0,
                    "status": "detected",
                    "text": "The incident has been detected.",
                    "timestamp": "2025-05-20T10:00:00Z"
                },
                {
                    "id": 1,
                    "status": "in progress",
                    "text": "update message",
                    "timestamp": "2025-05-20T11:00:00Z"
                }
            ],
            "status": "in progress"
        },
        ...
    ],
    "pagination": {
        "pageIndex": 1,
        "recordsPerPage": 10,
        "totalRecords": 500,
        "totalPages": 50
    }
}
```

### Pagination Object Details

- `pageIndex`: The current page number.
- `recordsPerPage`: The number of records on the current page.
- `totalRecords`: The total number of records matching the query.
- `totalPages`: The total number of pages available.

## Endpoint: `POST /v2/events`

Creates a new event (incident, maintenance, or info).

### Request

- **Method**: `POST`
- **Endpoint**: `/v2/events`
- **Headers**:
  - `Content-Type: application/json`
  - `Authorization: Bearer <token>` (required)

### Request Body

```json
{
  "title": "OpenStack Upgrade in regions EU-DE/EU-NL",
  "description": "Scheduled maintenance for OpenStack upgrade",
  "impact": 0,
  "components": [218, 254],
  "start_date": "2025-05-20T10:00:00Z",
  "end_date": "2025-05-20T14:00:00Z",
  "system": false,
  "type": "maintenance"
}
```

See [v2_incident_creation.md](v2_incident_creation.md) for detailed documentation on event creation.

## Endpoint: `GET /v2/events/:eventID`

Retrieves a single event by its ID.

### Request

- **Method**: `GET`
- **Endpoint**: `/v2/events/:eventID`

### Response

Returns the event object with all its details.

## Endpoint: `PATCH /v2/events/:eventID`

Updates an existing event.

### Request

- **Method**: `PATCH`
- **Endpoint**: `/v2/events/:eventID`
- **Headers**:
  - `Content-Type: application/json`
  - `Authorization: Bearer <token>` (required)

### Request Body

```json
{
  "title": "Updated title",
  "description": "Updated description",
  "impact": 2,
  "status": "analysing",
  "message": "Update message",
  "update_date": "2025-05-20T11:00:00Z"
}
```

## Endpoint: `POST /v2/events/:eventID/extract`

Extracts components from an existing event into a new event.

### Request

- **Method**: `POST`
- **Endpoint**: `/v2/events/:eventID/extract`
- **Headers**:
  - `Content-Type: application/json`
  - `Authorization: Bearer <token>` (required)

### Request Body

```json
{
  "components": [254]
}
```

## Endpoint: `PATCH /v2/events/:eventID/updates/:updateID`

Updates the text of a specific event update.

### Request

- **Method**: `PATCH`
- **Endpoint**: `/v2/events/:eventID/updates/:updateID`
- **Headers**:
  - `Content-Type: application/json`
  - `Authorization: Bearer <token>` (required)

### Request Body

```json
{
  "text": "Updated status message"
}
```
