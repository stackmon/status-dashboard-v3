ALTER TABLE incident
    ADD COLUMN created_by VARCHAR(255),
    ADD COLUMN contact_email VARCHAR(255),
    ADD COLUMN version INTEGER NOT NULL DEFAULT 1;

UPDATE incident SET version = 1 WHERE version IS NULL;

ALTER TABLE incident_status
    ADD COLUMN created_by VARCHAR(255),
    ADD COLUMN modified_by VARCHAR(255);
