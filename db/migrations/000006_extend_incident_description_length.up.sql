-- Increase description length limit for incidents
ALTER TABLE incident
ALTER COLUMN description TYPE VARCHAR(1500);
