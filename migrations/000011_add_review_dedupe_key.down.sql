DROP INDEX IF EXISTS idx_reviews_dedupe_key;

ALTER TABLE reviews
  DROP COLUMN IF EXISTS dedupe_key;
