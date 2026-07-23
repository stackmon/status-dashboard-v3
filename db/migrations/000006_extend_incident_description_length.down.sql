-- Revert description length limit for incidents
UPDATE incident
SET description = LEFT(description, 500)
WHERE description IS NOT NULL AND LENGTH(description) > 500;

ALTER TABLE incident
ALTER COLUMN description TYPE VARCHAR(500);
