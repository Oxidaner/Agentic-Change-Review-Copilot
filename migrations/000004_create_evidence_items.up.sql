CREATE TABLE evidence_items (
  id BIGSERIAL PRIMARY KEY,
  evidence_id VARCHAR(64) UNIQUE NOT NULL,
  review_id VARCHAR(64) NOT NULL,
  evidence_type VARCHAR(64) NOT NULL,
  source VARCHAR(128) NOT NULL,
  title TEXT,
  content_snippet TEXT,
  reference_url TEXT,
  confidence NUMERIC(5,4),
  metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_evidence_items_review_id
    FOREIGN KEY (review_id) REFERENCES reviews(review_id) ON DELETE CASCADE
);
