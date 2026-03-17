CREATE TABLE change_snapshots (
  id BIGSERIAL PRIMARY KEY,
  change_id VARCHAR(64) NOT NULL,
  raw_payload_ref TEXT NOT NULL,
  normalized_payload JSONB NOT NULL,
  semantic_tags JSONB,
  file_list JSONB,
  diff_stats JSONB,
  created_at TIMESTAMP NOT NULL DEFAULT NOW()
);
