# Components availability V2

## Overview

The `GetComponentsAvailabilityHandler` is an HTTP handler function
that returns a JSON response containing the availability data of
system components. It supports the following features:

## Handler Function: `GetComponentsAvailabilityHandler`

### Description

This handler performs the following operations:

- **Data Retrieval**: Fetches components along with their associated incidents from the database.
- **Availability Calculation**: Computes monthly availability percentages for each component.
- **Data Sorting**: Orders the availability data in descending order by year and month.
- **Response Delivery**: Returns a JSON response containing the compiled availability data.

## Request

- **Method**: `GET`
- **Endpoint**: `v2/availability`
- **Query Parameters**: None
- **Headers**:
  - `Content-Type: application/json`

## Response

- **Status Code**: `200 OK`
- **Content-Type**: `application/json`
- **Body**:

## JSON Response Structure

The handler returns a JSON object with the following structure:

```json
{
  "data": [
    {
      "id": 218,
      "name": "Auto Scaling",
      "region": "EU-DE",
      "availability": [
        {
          "year": 2024,
          "month": 5,
          "percentage": 99.999666
        }
      ]
    }
  ]
}
```

## Function: `calculateAvailability`

### Description

Calculates the monthly availability of a component over the past year, expressed as a percentage.   
Availability is defined as the proportion of time a component was operational within a given month.

### Workflow

1. **Input Validation**:
   - Returns an error if the component is `nil`.
   - Returns `nil` if the component has no incidents.

2. **Defining the Calculation Period**:
   - Sets the end date to the current date.
   - Sets the start date to 11 months prior, covering a 12-month period.

3. **Initializing Downtime Array**:
   - Creates an array to record downtime for each month.

4. **Processing Incidents**:
   - Filters incidents to include only those with an end date and a specific impact level.
   - Adjusts incident periods to fit within the calculation timeframe.
   - Allocates downtime across relevant months, accounting for month boundaries.

5. **Calculating Monthly Availability**:
   - For each month:
     - Determines total hours in the month.
     - Calculates availability using the formula:

       ```markdown
       Availability (%) = 100% - (Downtime Hours / Total Hours in Month) × 100%
       ```

     - Rounds the result to five decimal places.

6. **Returning the Result**:
   - Returns an array of `MonthlyAvailability` objects with year, month, and availability percentage.

### Handling Month Boundaries

The function accounts for incidents spanning multiple months by:

- **Adjusting Incident Periods**: Constrains incident times to the calculation period.
- **Distributing Downtime**: Calculates overlap with each month to allocate downtime accurately.
