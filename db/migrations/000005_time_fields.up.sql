-- Add timestamp fields to incident table
ALTER TABLE incident ADD COLUMN IF NOT EXISTS created_at timestamp without time zone;
ALTER TABLE incident ADD COLUMN IF NOT EXISTS modified_at timestamp without time zone;
ALTER TABLE incident ADD COLUMN IF NOT EXISTS deleted_at timestamp without time zone;

-- Add timestamp fields to component table
ALTER TABLE component ADD COLUMN IF NOT EXISTS created_at timestamp without time zone;
ALTER TABLE component ADD COLUMN IF NOT EXISTS modified_at timestamp without time zone;
ALTER TABLE component ADD COLUMN IF NOT EXISTS deleted_at timestamp without time zone;

-- Add timestamp fields to incident_status table
ALTER TABLE incident_status ADD COLUMN IF NOT EXISTS created_at timestamp without time zone;
ALTER TABLE incident_status ADD COLUMN IF NOT EXISTS modified_at timestamp without time zone;
ALTER TABLE incident_status ADD COLUMN IF NOT EXISTS deleted_at timestamp without time zone;

-- Populate created_at and modified_at fields for existing records
-- For incident table: set created_at equal to start_date
-- and modified_at equal to the timestamp from the last incident_status
UPDATE incident
SET created_at = start_date,
    modified_at = COALESCE(
        (SELECT MAX("timestamp")
         FROM incident_status
         WHERE incident_status.incident_id = incident.id),
        start_date
    )
WHERE created_at IS NULL;

-- For component table: set created_at to predefined time '01-01-2024' (no date field exists)
UPDATE component
SET created_at = '01-01-2024',
    modified_at = '01-01-2024'
WHERE created_at IS NULL;

-- For incident_status table: set created_at equal to timestamp field
UPDATE incident_status
SET created_at = "timestamp",
    modified_at = "timestamp"
WHERE created_at IS NULL;

