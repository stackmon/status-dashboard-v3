-- Remove timestamp fields from incident_status table
ALTER TABLE incident_status DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE incident_status DROP COLUMN IF EXISTS modified_at;
ALTER TABLE incident_status DROP COLUMN IF EXISTS created_at;

-- Remove timestamp fields from component table
ALTER TABLE component DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE component DROP COLUMN IF EXISTS modified_at;
ALTER TABLE component DROP COLUMN IF EXISTS created_at;

-- Remove timestamp fields from incident table
ALTER TABLE incident DROP COLUMN IF EXISTS deleted_at;
ALTER TABLE incident DROP COLUMN IF EXISTS modified_at;
ALTER TABLE incident DROP COLUMN IF EXISTS created_at;
