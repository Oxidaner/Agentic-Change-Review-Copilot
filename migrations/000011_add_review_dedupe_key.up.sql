ALTER TABLE reviews
  ADD COLUMN IF NOT EXISTS dedupe_key VARCHAR(255);

CREATE UNIQUE INDEX IF NOT EXISTS idx_reviews_dedupe_key
  ON reviews(dedupe_key)
  WHERE dedupe_key IS NOT NULL;
