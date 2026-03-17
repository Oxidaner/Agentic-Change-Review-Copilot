DROP INDEX IF EXISTS idx_evaluation_records_incident_flag_created_at;
DROP INDEX IF EXISTS idx_evaluation_records_review_id;

DROP INDEX IF EXISTS idx_audit_events_trace_id;
DROP INDEX IF EXISTS idx_audit_events_review_id_created_at;
DROP INDEX IF EXISTS idx_human_decisions_review_id_created_at;
DROP INDEX IF EXISTS idx_recommendations_review_id;

DROP INDEX IF EXISTS idx_risk_signals_name_severity;
DROP INDEX IF EXISTS idx_risk_signals_review_id;
DROP INDEX IF EXISTS idx_risk_signals_signal_id;

DROP INDEX IF EXISTS idx_evidence_items_metadata_gin;
DROP INDEX IF EXISTS idx_evidence_items_type_source;
DROP INDEX IF EXISTS idx_evidence_items_review_id;
DROP INDEX IF EXISTS idx_evidence_items_evidence_id;

DROP INDEX IF EXISTS idx_change_snapshots_file_list_gin;
DROP INDEX IF EXISTS idx_change_snapshots_tags_gin;
DROP INDEX IF EXISTS idx_change_snapshots_change_id;

DROP INDEX IF EXISTS idx_review_tasks_state_updated_at;
DROP INDEX IF EXISTS idx_review_tasks_review_id;
DROP INDEX IF EXISTS idx_review_tasks_task_id;

DROP INDEX IF EXISTS idx_reviews_risk_level_created_at;
DROP INDEX IF EXISTS idx_reviews_service_env_created_at;
DROP INDEX IF EXISTS idx_reviews_status_created_at;
DROP INDEX IF EXISTS idx_reviews_review_id;
