-- Drop unnecessary timestamp columns from posts
BEGIN;
ALTER TABLE posts DROP COLUMN IF EXISTS created_at;
ALTER TABLE posts DROP COLUMN IF EXISTS updated_at;
COMMIT;

