CREATE TABLE evaluation_records (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) NOT NULL,
  final_human_decision VARCHAR(32),
  release_outcome VARCHAR(32),
  incident_flag BOOLEAN,
  outcome_metadata JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_evaluation_records_review_id
    FOREIGN KEY (review_id) REFERENCES reviews(review_id) ON DELETE CASCADE
);
