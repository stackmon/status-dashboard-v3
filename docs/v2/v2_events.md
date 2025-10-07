# Events V2

## Overview

The `/v2/events` endpoint provides a paginated list of events, which can be incidents, maintenances, or informational events. This endpoint supports extensive filtering capabilities and pagination to allow clients to efficiently retrieve the data they need.

The existing `/v2/incidents` endpoint remains for backward compatibility and returns a complete, non-paginated list of incidents matching the filter criteria.

## Handler Function: `GetEventsHandler`

### Description

This handler is responsible for fetching events from the database. It is used by both `/v2/events` (with pagination) and `/v2/incidents` (without pagination).

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
  - **Allowed Values**: `10`, `20`, `50`, `100`.
  - **Special Case**: If `limit=0`, all matching records are returned without pagination.

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
