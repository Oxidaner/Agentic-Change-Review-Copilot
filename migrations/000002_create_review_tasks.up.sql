CREATE TABLE review_tasks (
  id BIGSERIAL PRIMARY KEY,
  task_id VARCHAR(64) UNIQUE NOT NULL,
  review_id VARCHAR(64) NOT NULL,
  current_state VARCHAR(64) NOT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  last_error TEXT,
  started_at TIMESTAMP,
  finished_at TIMESTAMP,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
  CONSTRAINT fk_review_tasks_review_id
    FOREIGN KEY (review_id) REFERENCES reviews(review_id) ON DELETE CASCADE
);
