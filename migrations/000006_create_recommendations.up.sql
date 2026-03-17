CREATE TABLE recommendations (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) NOT NULL,
  recommendation JSONB NOT NULL,
  rollback_plan JSONB,
  rollout_strategy JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_recommendations_review_id
    FOREIGN KEY (review_id) REFERENCES reviews(review_id) ON DELETE CASCADE
);
