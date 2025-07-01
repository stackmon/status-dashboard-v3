-- Add new description column to the incident table
ALTER TABLE incident ADD COLUMN IF NOT EXISTS description VARCHAR(500);

-- Populate the new description column for maintenance incidents (impact=0)
-- from the corresponding incident_status update with status='description'.
UPDATE incident
SET description = s.text
FROM incident_status AS s
WHERE incident.id = s.incident_id AND incident.impact = 0 AND s.status = 'description' AND s.text IS NOT NULL AND s.text <> '';
