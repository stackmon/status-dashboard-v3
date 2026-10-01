ALTER TABLE incident
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS contact_email VARCHAR(255),
    ADD COLUMN IF NOT EXISTS version INTEGER;

UPDATE incident SET version = 1 WHERE version IS NULL;

-- Applied separately so the constraint is enforced even if the column already existed as nullable.
ALTER TABLE incident
    ALTER COLUMN version SET DEFAULT 1,
    ALTER COLUMN version SET NOT NULL;

ALTER TABLE incident_status
    ADD COLUMN IF NOT EXISTS created_by VARCHAR(255),
    ADD COLUMN IF NOT EXISTS modified_by VARCHAR(255);
