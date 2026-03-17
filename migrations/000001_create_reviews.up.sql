CREATE TABLE reviews (
  id BIGSERIAL PRIMARY KEY,
  review_id VARCHAR(64) UNIQUE NOT NULL,
  change_id VARCHAR(64) NOT NULL,
  source_type VARCHAR(32) NOT NULL,
  repo VARCHAR(255),
  service VARCHAR(255),
  environment VARCHAR(64),
  author VARCHAR(128),
  status VARCHAR(64) NOT NULL,
  risk_level VARCHAR(32),
  score INT,
  confidence NUMERIC(5,4),
  summary TEXT,
  can_release BOOLEAN,
  human_review_required BOOLEAN NOT NULL DEFAULT FALSE,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
