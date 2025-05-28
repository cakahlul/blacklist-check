-- Enable pg_trgm extension
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Add GIN indexes for fuzzy matching
CREATE INDEX IF NOT EXISTS idx_blacklist_name_trgm ON blacklist USING gin (name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_blacklist_birth_place_trgm ON blacklist USING gin (birth_place gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_blacklist_birth_date_btree ON blacklist USING btree (birth_date);

-- Add composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_blacklist_name_birth_date ON blacklist (name, birth_date);
CREATE INDEX IF NOT EXISTS idx_blacklist_name_birth_place ON blacklist (name, birth_place); 