-- Add new status column to the incident table
ALTER TABLE incident ADD COLUMN IF NOT EXISTS status VARCHAR(50);

-- Populate the new status column with latest non-SYSTEM and non-description status
UPDATE incident
SET status = latest_status.status
FROM (
    SELECT DISTINCT ON (incident_id) 
        incident_id,
        status
    FROM incident_status
    WHERE status NOT IN ('SYSTEM', 'description')
        AND status IS NOT NULL 
        AND status != ''
    ORDER BY incident_id, timestamp DESC
) latest_status
WHERE incident.id = latest_status.incident_id;
