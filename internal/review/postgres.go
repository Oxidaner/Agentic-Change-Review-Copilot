package review

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db}
}

func RunMigrations(db *sql.DB, dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)

	for _, file := range files {
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		if _, err := db.Exec(string(sqlBytes)); err != nil && !isAlreadyExistsError(err) {
			return fmt.Errorf("%s: %w", filepath.Base(file), err)
		}
	}
	return nil
}

func (s *PostgresStore) Save(record Record) error {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := upsertReview(ctx, tx, record); err != nil {
		return err
	}
	if err := replaceTask(ctx, tx, record); err != nil {
		return err
	}
	if err := replaceEvidence(ctx, tx, record); err != nil {
		return err
	}
	if err := replaceSignals(ctx, tx, record); err != nil {
		return err
	}
	if err := replaceRecommendation(ctx, tx, record); err != nil {
		return err
	}
	if err := replaceAuditEvents(ctx, tx, record); err != nil {
		return err
	}
	if err := replaceHumanDecisions(ctx, tx, record); err != nil {
		return err
	}
	if err := replaceEvaluation(ctx, tx, record); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *PostgresStore) Get(reviewID string) (Record, error) {
	ctx := context.Background()
	var record Record
	var riskLevel string

	err := s.db.QueryRowContext(ctx, `
		SELECT review_id, change_id, source_type, COALESCE(repo, ''), COALESCE(service, ''),
		       COALESCE(environment, ''), status, COALESCE(risk_level, ''), COALESCE(score, 0),
		       COALESCE(confidence, 0), COALESCE(summary, ''), human_review_required, created_at, updated_at
		FROM reviews
		WHERE review_id = $1
	`, reviewID).Scan(
		&record.Review.ReviewID,
		&record.Review.ChangeID,
		&record.Review.SourceType,
		&record.Review.Repo,
		&record.Review.Service,
		&record.Review.Environment,
		&record.Review.Status,
		&riskLevel,
		&record.Review.Score,
		&record.Review.Confidence,
		&record.Review.Summary,
		&record.Review.HumanReviewRequired,
		&record.Review.CreatedAt,
		&record.Review.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	record.Review.RiskLevel = RiskLevel(riskLevel)

	_ = s.db.QueryRowContext(ctx, `
		SELECT task_id FROM review_tasks WHERE review_id = $1 ORDER BY created_at DESC LIMIT 1
	`, reviewID).Scan(&record.TaskID)

	signalsRows, err := s.db.QueryContext(ctx, `
		SELECT signal_id, signal_name, severity, score_delta, COALESCE(explanation, ''), COALESCE(evidence_refs, '[]'::jsonb)
		FROM risk_signals WHERE review_id = $1 ORDER BY id ASC
	`, reviewID)
	if err != nil {
		return Record{}, err
	}
	defer signalsRows.Close()

	for signalsRows.Next() {
		var signal RiskSignal
		var refs []byte
		if err := signalsRows.Scan(&signal.SignalID, &signal.SignalName, &signal.Severity, &signal.ScoreDelta, &signal.Explanation, &refs); err != nil {
			return Record{}, err
		}
		_ = json.Unmarshal(refs, &signal.EvidenceRefs)
		record.Signals = append(record.Signals, signal)
	}
	if err := signalsRows.Err(); err != nil {
		return Record{}, err
	}

	evidenceRows, err := s.db.QueryContext(ctx, `
		SELECT evidence_id, evidence_type, source, COALESCE(title, ''), COALESCE(content_snippet, ''),
		       COALESCE(reference_url, ''), COALESCE(confidence, 0), COALESCE(metadata, '{}'::jsonb)
		FROM evidence_items WHERE review_id = $1 ORDER BY id ASC
	`, reviewID)
	if err != nil {
		return Record{}, err
	}
	defer evidenceRows.Close()

	for evidenceRows.Next() {
		var item EvidenceItem
		var metadata []byte
		if err := evidenceRows.Scan(&item.EvidenceID, &item.Type, &item.Source, &item.Title, &item.ContentSnippet, &item.ReferenceURL, &item.Confidence, &metadata); err != nil {
			return Record{}, err
		}
		_ = json.Unmarshal(metadata, &item.Metadata)
		record.Evidence = append(record.Evidence, item)
	}
	if err := evidenceRows.Err(); err != nil {
		return Record{}, err
	}

	var recommendationJSON []byte
	var rollbackJSON []byte
	err = s.db.QueryRowContext(ctx, `
		SELECT recommendation, COALESCE(rollback_plan, '{}'::jsonb)
		FROM recommendations WHERE review_id = $1 ORDER BY id DESC LIMIT 1
	`, reviewID).Scan(&recommendationJSON, &rollbackJSON)
	if err != nil && err != sql.ErrNoRows {
		return Record{}, err
	}
	if err == nil {
		_ = json.Unmarshal(recommendationJSON, &record.Recommendation)
		_ = json.Unmarshal(rollbackJSON, &record.RollbackPlan)
	}

	timelineRows, err := s.db.QueryContext(ctx, `
		SELECT payload FROM audit_events WHERE review_id = $1 ORDER BY created_at ASC, id ASC
	`, reviewID)
	if err != nil {
		return Record{}, err
	}
	defer timelineRows.Close()

	for timelineRows.Next() {
		var payload []byte
		if err := timelineRows.Scan(&payload); err != nil {
			return Record{}, err
		}
		var event TimelineEvent
		if err := json.Unmarshal(payload, &event); err != nil {
			return Record{}, err
		}
		record.Timeline = append(record.Timeline, event)
	}
	if err := timelineRows.Err(); err != nil {
		return Record{}, err
	}

	decisionRows, err := s.db.QueryContext(ctx, `
		SELECT reviewer, decision, COALESCE(reason, ''), override_flag, created_at
		FROM human_decisions WHERE review_id = $1 ORDER BY created_at ASC, id ASC
	`, reviewID)
	if err != nil {
		return Record{}, err
	}
	defer decisionRows.Close()

	for decisionRows.Next() {
		var decision HumanDecision
		if err := decisionRows.Scan(&decision.Reviewer, &decision.Decision, &decision.Reason, &decision.OverrideFlag, &decision.CreatedAt); err != nil {
			return Record{}, err
		}
		record.HumanDecisions = append(record.HumanDecisions, decision)
	}
	if err := decisionRows.Err(); err != nil {
		return Record{}, err
	}

	var finalDecision sql.NullString
	var releaseOutcome sql.NullString
	var incidentFlag sql.NullBool
	var outcomeMetadata []byte
	var createdAt time.Time
	var updatedAt time.Time
	err = s.db.QueryRowContext(ctx, `
		SELECT final_human_decision, release_outcome, incident_flag, COALESCE(outcome_metadata, '{}'::jsonb), created_at, updated_at
		FROM evaluation_records WHERE review_id = $1 ORDER BY id DESC LIMIT 1
	`, reviewID).Scan(&finalDecision, &releaseOutcome, &incidentFlag, &outcomeMetadata, &createdAt, &updatedAt)
	if err != nil && err != sql.ErrNoRows {
		return Record{}, err
	}
	if err == nil {
		record.Evaluation = &EvaluationRecord{CreatedAt: createdAt, UpdatedAt: updatedAt}
		if finalDecision.Valid {
			record.Evaluation.FinalHumanDecision = stringPtr(finalDecision.String)
		}
		if releaseOutcome.Valid {
			record.Evaluation.ReleaseOutcome = stringPtr(releaseOutcome.String)
		}
		if incidentFlag.Valid {
			value := incidentFlag.Bool
			record.Evaluation.IncidentFlag = &value
		}
		_ = json.Unmarshal(outcomeMetadata, &record.Evaluation.OutcomeMetadata)
	}

	return record, nil
}

func (s *PostgresStore) List() []Record {
	ctx := context.Background()
	rows, err := s.db.QueryContext(ctx, `SELECT review_id FROM reviews ORDER BY created_at DESC`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var reviewID string
		if err := rows.Scan(&reviewID); err != nil {
			continue
		}
		record, err := s.Get(reviewID)
		if err == nil {
			out = append(out, record)
		}
	}
	return out
}

func upsertReview(ctx context.Context, tx *sql.Tx, record Record) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO reviews (
			review_id, change_id, source_type, repo, service, environment, author,
			status, risk_level, score, confidence, summary, can_release,
			human_review_required, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13,
			$14, $15, $16
		)
		ON CONFLICT (review_id) DO UPDATE SET
			change_id = EXCLUDED.change_id,
			source_type = EXCLUDED.source_type,
			repo = EXCLUDED.repo,
			service = EXCLUDED.service,
			environment = EXCLUDED.environment,
			status = EXCLUDED.status,
			risk_level = EXCLUDED.risk_level,
			score = EXCLUDED.score,
			confidence = EXCLUDED.confidence,
			summary = EXCLUDED.summary,
			can_release = EXCLUDED.can_release,
			human_review_required = EXCLUDED.human_review_required,
			updated_at = EXCLUDED.updated_at
	`,
		record.Review.ReviewID,
		record.Review.ChangeID,
		record.Review.SourceType,
		nullIfEmpty(record.Review.Repo),
		nullIfEmpty(record.Review.Service),
		nullIfEmpty(record.Review.Environment),
		nil,
		record.Review.Status,
		nullIfEmpty(string(record.Review.RiskLevel)),
		record.Review.Score,
		record.Review.Confidence,
		nullIfEmpty(record.Review.Summary),
		canRelease(record.Review.Status),
		record.Review.HumanReviewRequired,
		record.Review.CreatedAt,
		record.Review.UpdatedAt,
	)
	return err
}

func replaceTask(ctx context.Context, tx *sql.Tx, record Record) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO review_tasks (
			task_id, review_id, current_state, retry_count, last_error, started_at, finished_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, 0, $4, $5, $6, $7, $8
		)
		ON CONFLICT (task_id) DO UPDATE SET
			current_state = EXCLUDED.current_state,
			last_error = EXCLUDED.last_error,
			finished_at = EXCLUDED.finished_at,
			updated_at = EXCLUDED.updated_at
	`,
		record.TaskID,
		record.Review.ReviewID,
		record.Review.Status,
		nullIfEmpty(record.LastError),
		record.Review.CreatedAt,
		finishedAtFor(record.Review.Status, record.Review.UpdatedAt),
		record.Review.CreatedAt,
		record.Review.UpdatedAt,
	)
	return err
}

func replaceEvidence(ctx context.Context, tx *sql.Tx, record Record) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM evidence_items WHERE review_id = $1`, record.Review.ReviewID); err != nil {
		return err
	}
	for _, item := range record.Evidence {
		metadata, err := json.Marshal(item.Metadata)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO evidence_items (
				evidence_id, review_id, evidence_type, source, title, content_snippet, reference_url, confidence, metadata
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`,
			item.EvidenceID,
			record.Review.ReviewID,
			item.Type,
			item.Source,
			nullIfEmpty(item.Title),
			nullIfEmpty(item.ContentSnippet),
			nullIfEmpty(item.ReferenceURL),
			item.Confidence,
			jsonOrObject(metadata),
		); err != nil {
			return err
		}
	}
	return nil
}

func replaceSignals(ctx context.Context, tx *sql.Tx, record Record) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM risk_signals WHERE review_id = $1`, record.Review.ReviewID); err != nil {
		return err
	}
	for _, signal := range record.Signals {
		refs, err := json.Marshal(signal.EvidenceRefs)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO risk_signals (
				signal_id, review_id, signal_name, severity, score_delta, explanation, evidence_refs
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`,
			signal.SignalID,
			record.Review.ReviewID,
			signal.SignalName,
			signal.Severity,
			signal.ScoreDelta,
			nullIfEmpty(signal.Explanation),
			jsonOrArray(refs),
		); err != nil {
			return err
		}
	}
	return nil
}

func replaceRecommendation(ctx context.Context, tx *sql.Tx, record Record) error {
	recommendationJSON, err := json.Marshal(record.Recommendation)
	if err != nil {
		return err
	}
	rollbackJSON, err := json.Marshal(record.RollbackPlan)
	if err != nil {
		return err
	}
	rolloutJSON, err := json.Marshal(record.Recommendation.RolloutStrategy)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM recommendations WHERE review_id = $1`, record.Review.ReviewID); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO recommendations (
			review_id, recommendation, rollback_plan, rollout_strategy, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6)
	`,
		record.Review.ReviewID,
		recommendationJSON,
		rollbackJSON,
		rolloutJSON,
		record.Review.CreatedAt,
		record.Review.UpdatedAt,
	)
	return err
}

func replaceAuditEvents(ctx context.Context, tx *sql.Tx, record Record) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM audit_events WHERE review_id = $1`, record.Review.ReviewID); err != nil {
		return err
	}
	for _, event := range record.Timeline {
		payload, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO audit_events (review_id, event_type, payload, trace_id, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, record.Review.ReviewID, event.State, payload, record.TaskID, event.At); err != nil {
			return err
		}
	}
	return nil
}

func replaceHumanDecisions(ctx context.Context, tx *sql.Tx, record Record) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM human_decisions WHERE review_id = $1`, record.Review.ReviewID); err != nil {
		return err
	}
	for _, decision := range record.HumanDecisions {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO human_decisions (review_id, reviewer, decision, reason, override_flag, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`,
			record.Review.ReviewID,
			decision.Reviewer,
			decision.Decision,
			nullIfEmpty(decision.Reason),
			decision.OverrideFlag,
			decision.CreatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

func replaceEvaluation(ctx context.Context, tx *sql.Tx, record Record) error {
	if record.Evaluation == nil {
		return nil
	}
	metadata, err := json.Marshal(record.Evaluation.OutcomeMetadata)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO evaluation_records (
			review_id, final_human_decision, release_outcome, incident_flag, outcome_metadata, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT DO NOTHING
	`,
		record.Review.ReviewID,
		nullStringPtr(record.Evaluation.FinalHumanDecision),
		nullStringPtr(record.Evaluation.ReleaseOutcome),
		nullBoolPtr(record.Evaluation.IncidentFlag),
		jsonOrObject(metadata),
		record.Evaluation.CreatedAt,
		record.Evaluation.UpdatedAt,
	)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE evaluation_records
		SET final_human_decision = $2,
		    release_outcome = $3,
		    incident_flag = $4,
		    outcome_metadata = $5,
		    updated_at = $6
		WHERE review_id = $1
	`,
		record.Review.ReviewID,
		nullStringPtr(record.Evaluation.FinalHumanDecision),
		nullStringPtr(record.Evaluation.ReleaseOutcome),
		nullBoolPtr(record.Evaluation.IncidentFlag),
		jsonOrObject(metadata),
		record.Evaluation.UpdatedAt,
	)
	return err
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullStringPtr(value *string) any {
	if value == nil || *value == "" {
		return nil
	}
	return *value
}

func nullBoolPtr(value *bool) any {
	if value == nil {
		return nil
	}
	return *value
}

func jsonOrObject(value []byte) []byte {
	if len(value) == 0 || string(value) == "null" {
		return []byte("{}")
	}
	return value
}

func jsonOrArray(value []byte) []byte {
	if len(value) == 0 || string(value) == "null" {
		return []byte("[]")
	}
	return value
}

func finishedAtFor(status ReviewStatus, at time.Time) any {
	switch status {
	case StatusApproved, StatusRejected, StatusOverridden, StatusFailed, StatusRecommended, StatusWaitingHumanReview:
		return at
	default:
		return nil
	}
}

func canRelease(status ReviewStatus) bool {
	return status == StatusApproved || status == StatusRecommended || status == StatusOverridden
}

func isAlreadyExistsError(err error) bool {
	if err == nil {
		return false
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "already exists")
}
