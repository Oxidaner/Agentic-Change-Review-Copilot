CREATE TABLE risk_signals (
  id BIGSERIAL PRIMARY KEY,
  signal_id VARCHAR(64) UNIQUE NOT NULL,
  review_id VARCHAR(64) NOT NULL,
  signal_name VARCHAR(128) NOT NULL,
  severity VARCHAR(32) NOT NULL,
  score_delta INT NOT NULL,
  explanation TEXT,
  evidence_refs JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_risk_signals_review_id
    FOREIGN KEY (review_id) REFERENCES reviews(review_id) ON DELETE CASCADE
);
