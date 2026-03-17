CREATE UNIQUE INDEX idx_reviews_review_id ON reviews(review_id);
CREATE INDEX idx_reviews_status_created_at ON reviews(status, created_at DESC);
CREATE INDEX idx_reviews_service_env_created_at ON reviews(service, environment, created_at DESC);
CREATE INDEX idx_reviews_risk_level_created_at ON reviews(risk_level, created_at DESC);

CREATE UNIQUE INDEX idx_review_tasks_task_id ON review_tasks(task_id);
CREATE INDEX idx_review_tasks_review_id ON review_tasks(review_id);
CREATE INDEX idx_review_tasks_state_updated_at ON review_tasks(current_state, updated_at DESC);

CREATE INDEX idx_change_snapshots_change_id ON change_snapshots(change_id);
CREATE INDEX idx_change_snapshots_tags_gin ON change_snapshots USING GIN (semantic_tags);
CREATE INDEX idx_change_snapshots_file_list_gin ON change_snapshots USING GIN (file_list);

CREATE UNIQUE INDEX idx_evidence_items_evidence_id ON evidence_items(evidence_id);
CREATE INDEX idx_evidence_items_review_id ON evidence_items(review_id);
CREATE INDEX idx_evidence_items_type_source ON evidence_items(evidence_type, source);
CREATE INDEX idx_evidence_items_metadata_gin ON evidence_items USING GIN (metadata);

CREATE UNIQUE INDEX idx_risk_signals_signal_id ON risk_signals(signal_id);
CREATE INDEX idx_risk_signals_review_id ON risk_signals(review_id);
CREATE INDEX idx_risk_signals_name_severity ON risk_signals(signal_name, severity);

CREATE INDEX idx_recommendations_review_id ON recommendations(review_id);
CREATE INDEX idx_human_decisions_review_id_created_at ON human_decisions(review_id, created_at DESC);
CREATE INDEX idx_audit_events_review_id_created_at ON audit_events(review_id, created_at DESC);
CREATE INDEX idx_audit_events_trace_id ON audit_events(trace_id);

CREATE INDEX idx_evaluation_records_review_id ON evaluation_records(review_id);
CREATE INDEX idx_evaluation_records_incident_flag_created_at ON evaluation_records(incident_flag, created_at DESC);
