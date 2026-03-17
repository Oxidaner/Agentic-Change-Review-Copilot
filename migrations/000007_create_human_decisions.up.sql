CREATE TABLE human_decisions (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) NOT NULL,
  reviewer VARCHAR(128) NOT NULL,
  decision VARCHAR(32) NOT NULL,
  reason TEXT,
  override_flag BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_human_decisions_review_id
    FOREIGN KEY (review_id) REFERENCES reviews(review_id) ON DELETE CASCADE
);
