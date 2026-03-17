CREATE TABLE audit_events (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) NOT NULL,
  event_type VARCHAR(64) NOT NULL,
  payload JSONB NOT NULL,
  trace_id VARCHAR(128),
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_audit_events_review_id
    FOREIGN KEY (review_id) REFERENCES reviews(review_id) ON DELETE CASCADE
);
