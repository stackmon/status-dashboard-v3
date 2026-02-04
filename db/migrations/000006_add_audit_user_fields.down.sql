-- Rollback: remove audit user fields

ALTER TABLE incident
    DROP COLUMN IF EXISTS created_by;

ALTER TABLE incident_status
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS modified_by;
