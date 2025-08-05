-- Remove the description column from the incident table
ALTER TABLE incident DROP COLUMN IF EXISTS actual_status;