-- Add audit user fields for tracking who created/modified records
-- Incident: only created_by (modifications tracked via incident_status entries)
-- IncidentStatus: created_by + modified_by (for text edits)

ALTER TABLE incident
    ADD COLUMN created_by VARCHAR(255);

ALTER TABLE incident_status
    ADD COLUMN created_by VARCHAR(255),
    ADD COLUMN modified_by VARCHAR(255);

COMMENT ON COLUMN incident.created_by IS 'User ID (from JWT sub claim) who created this incident';
COMMENT ON COLUMN incident_status.created_by IS 'User ID (from JWT sub claim) who created this status entry';
COMMENT ON COLUMN incident_status.modified_by IS 'User ID (from JWT sub claim) who last modified this status entry text';
