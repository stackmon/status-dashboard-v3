-- Rollback: remove audit user fields and contact_email

ALTER TABLE incident
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS contact_email,
    DROP COLUMN IF EXISTS version;

ALTER TABLE incident_status
    DROP COLUMN IF EXISTS created_by,
    DROP COLUMN IF EXISTS modified_by;
