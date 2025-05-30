-- Add new column
ALTER TABLE incident ADD COLUMN IF NOT EXISTS "type" character varying;

-- Set default values based on impact
UPDATE incident 
SET "type" = CASE 
    WHEN impact = 0 THEN 'maintenance'
    ELSE 'incident'
END;

-- Make the column NOT NULL after setting values
ALTER TABLE incident ALTER COLUMN "type" SET NOT NULL;
