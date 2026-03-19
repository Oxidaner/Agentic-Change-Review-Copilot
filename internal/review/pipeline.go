package review

import (
	"fmt"
	"strings"
	"time"
)

// ChangeBundle is the normalized internal representation of an incoming change.
//
// It intentionally flattens request metadata into fields that later pipeline
// stages can consume without knowing the original webhook or source format.
type ChangeBundle struct {
	ChangeID           string
	SourceType         string
	ChangeType         string
	Repo               string
	Service            string
	Environment        string
	Author             string
	SubmitTime         time.Time
	Title              string
	FileList           []string
	ImpactedComponents []string
	DiffSummary        string
	ReleaseTarget      string
}

// ChangeUnderstanding stores the semantic interpretation derived from the bundle.
type ChangeUnderstanding struct {
	Summary               string
	SemanticTags          []string
	RiskHints             []string
	ImpactedServices      []string
	PotentialFailureModes []string
}

// EvidencePack groups evidence items collected for the current review run.
type EvidencePack struct {
	Items []EvidenceItem
}

// ScoreResult is the structured outcome of the risk scoring step.
type ScoreResult struct {
	Score               int
	RiskLevel           RiskLevel
	Confidence          float64
	HumanReviewRequired bool
}

// normalizeChange maps the external request into the internal ChangeBundle model.
func normalizeChange(req CreateReviewRequest, now time.Time) ChangeBundle {
	fileList := metadataStringSlice(req.Payload.Metadata, "file_list")
	changeType := req.SourceType
	if changeType == "" {
		changeType = "pull_request"
	}

	return ChangeBundle{
		ChangeID:           req.SourceID,
		SourceType:         req.SourceType,
		ChangeType:         changeType,
		Repo:               req.Repo,
		Service:            req.Service,
		Environment:        req.Environment,
		Author:             req.Payload.Author,
		SubmitTime:         now,
		Title:              req.Payload.Title,
		FileList:           fileList,
		ImpactedComponents: inferImpactedComponents(req, fileList),
		DiffSummary:        summarizeDiff(req, fileList),
		ReleaseTarget:      req.Environment,
	}
}

// understandChange derives semantic tags, risk hints, and failure modes from the
// normalized bundle.
//
// The current implementation is heuristic, but the shape mirrors the future
// dedicated "change understanding" stage described in the design docs.
func understandChange(bundle ChangeBundle) ChangeUnderstanding {
	tags := []string{bundle.ChangeType}
	riskHints := make([]string, 0, 4)
	failureModes := make([]string, 0, 4)

	switch bundle.SourceType {
	case "sql_migration":
		tags = append(tags, "schema_change")
		riskHints = append(riskHints, "migration affects persistent state")
		failureModes = append(failureModes, "locking or irreversible migration failure")
	case "k8s_diff":
		tags = append(tags, "runtime_config_changed")
		riskHints = append(riskHints, "runtime configuration drift")
		failureModes = append(failureModes, "resource or readiness regression")
	case "gateway_config":
		tags = append(tags, "traffic_routing_changed")
		riskHints = append(riskHints, "entry routing path modified")
		failureModes = append(failureModes, "traffic misroute or auth regression")
	default:
		tags = append(tags, "application_code_change")
		failureModes = append(failureModes, "service logic regression")
	}

	if containsCriticalPath(bundle) {
		tags = append(tags, "core_path_modified")
		riskHints = append(riskHints, "critical business path detected")
	}
	if containsAuthScope(bundle) {
		tags = append(tags, "auth_logic_changed")
		riskHints = append(riskHints, "auth-related component touched")
	}
	if containsSensitiveArtifact(bundle.FileList) {
		tags = append(tags, "sensitive_artifact_changed")
		riskHints = append(riskHints, "config or migration artifact changed")
	}

	target := firstNonEmpty(bundle.Service, bundle.Repo, bundle.ChangeID)
	summary := fmt.Sprintf("%s change targets %s in %s.", bundle.SourceType, target, bundle.Environment)
	if bundle.DiffSummary != "" {
		summary = fmt.Sprintf("%s %s", summary, bundle.DiffSummary)
	}

	return ChangeUnderstanding{
		Summary:               summary,
		SemanticTags:          dedupeStrings(tags),
		RiskHints:             dedupeStrings(riskHints),
		ImpactedServices:      dedupeStrings([]string{target}),
		PotentialFailureModes: dedupeStrings(failureModes),
	}
}

// collectEvidence builds the MVP evidence set used by downstream signal and
// recommendation logic.
//
// Right now evidence is synthetic and local; later this function is the natural
// seam for Git, incident, runbook, metrics, and dependency retrieval.
func collectEvidence(bundle ChangeBundle, understanding ChangeUnderstanding, nextID func(string) string) EvidencePack {
	items := []EvidenceItem{
		{
			EvidenceID:     nextID("ev"),
			Type:           "change_bundle",
			Source:         "ingestion",
			Title:          "Normalized change bundle",
			ContentSnippet: bundle.DiffSummary,
			Confidence:     0.92,
			Metadata: map[string]any{
				"change_type":         bundle.ChangeType,
				"impacted_components": bundle.ImpactedComponents,
				"semantic_tags":       understanding.SemanticTags,
			},
		},
		{
			EvidenceID:     nextID("ev"),
			Type:           "risk_context",
			Source:         "context_collector",
			Title:          "MVP contextual evidence",
			ContentSnippet: strings.Join(understanding.RiskHints, "; "),
			Confidence:     0.86,
			Metadata: map[string]any{
				"environment":             bundle.Environment,
				"potential_failure_modes": understanding.PotentialFailureModes,
			},
		},
	}

	return EvidencePack{Items: items}
}

// extractRiskSignals translates the bundle plus evidence into explicit risk
// signals, each of which references supporting evidence IDs.
func extractRiskSignals(bundle ChangeBundle, understanding ChangeUnderstanding, pack EvidencePack, nextID func(string) string) ([]RiskSignal, int) {
	signals := make([]RiskSignal, 0, 6)
	score := 10

	evidenceRefs := make([]string, 0, len(pack.Items))
	for _, item := range pack.Items {
		evidenceRefs = append(evidenceRefs, item.EvidenceID)
	}

	addSignal := func(name string, severity RiskLevel, delta int, explanation string) {
		signals = append(signals, RiskSignal{
			SignalID:     nextID("sig"),
			SignalName:   name,
			Severity:     severity,
			ScoreDelta:   delta,
			Explanation:  explanation,
			EvidenceRefs: evidenceRefs,
		})
		score += delta
	}

	switch bundle.SourceType {
	case "sql_migration":
		addSignal("irreversible_operation", RiskHigh, 30, "SQL migration changes are treated as high-impact in the MVP rule set.")
	case "k8s_diff":
		addSignal("runtime_config_changed", RiskMedium, 18, "Kubernetes configuration changes can affect runtime stability.")
	case "terraform_plan":
		addSignal("infrastructure_change", RiskHigh, 24, "Infrastructure plan changes can have broad blast radius.")
	case "gateway_config":
		addSignal("traffic_routing_changed", RiskHigh, 26, "Gateway routing and auth changes affect entry traffic.")
	default:
		addSignal("application_code_change", RiskMedium, 14, "Code changes require standard review and rollout checks.")
	}

	if strings.EqualFold(bundle.Environment, "prod") || strings.Contains(strings.ToLower(bundle.Environment), "prod") {
		addSignal("production_release", RiskHigh, 20, "Production deployments receive additional risk weighting.")
	}
	if containsCriticalPath(bundle) {
		addSignal("core_path_modified", RiskHigh, 18, "Critical service path detected in change metadata.")
	}
	if containsAuthScope(bundle) {
		addSignal("auth_logic_changed", RiskHigh, 18, "Auth-related code or config is included in the change scope.")
	}
	if containsSensitiveArtifact(bundle.FileList) {
		addSignal("sensitive_artifact_changed", RiskMedium, 12, "Sensitive config or migration artifact found in the change set.")
	}
	if len(understanding.PotentialFailureModes) == 0 {
		addSignal("missing_failure_mode_analysis", RiskMedium, 8, "No potential failure mode was inferred for this change.")
	}

	return signals, score
}

// scoreReview collapses the extracted signals into score, risk level, and human
// review gating information.
func scoreReview(signals []RiskSignal) ScoreResult {
	score := 0
	for _, signal := range signals {
		score += signal.ScoreDelta
	}
	score += 10
	if score > 100 {
		score = 100
	}

	level := scoreToRiskLevel(score)
	confidence := confidenceFor(level)
	humanReviewRequired := level == RiskHigh || level == RiskCritical || confidence < 0.75

	return ScoreResult{
		Score:               score,
		RiskLevel:           level,
		Confidence:          confidence,
		HumanReviewRequired: humanReviewRequired,
	}
}

// buildRecommendation turns scoring output into release guidance.
func buildRecommendation(bundle ChangeBundle, understanding ChangeUnderstanding, result ScoreResult) Recommendation {
	reviewers := []string{"release-manager"}
	if result.HumanReviewRequired {
		reviewers = append(reviewers, "service-owner")
	}
	if containsAuthScope(bundle) {
		reviewers = append(reviewers, "security-reviewer")
	}

	window := "business hours"
	if strings.Contains(strings.ToLower(bundle.Environment), "prod") {
		window = "low-traffic release window"
	}

	observabilityPlan := []string{"error_rate", "latency_p95", "success_rate"}
	if containsAuthScope(bundle) {
		observabilityPlan = append(observabilityPlan, "auth_failure_rate")
	}

	return Recommendation{
		RequiredReviewers: dedupeStrings(reviewers),
		ReleaseWindow:     window,
		RolloutStrategy: RolloutStrategy{
			Steps: []string{
				"Deploy to canary slice",
				"Observe key indicators for 10 minutes",
				"Expand to 25%, then 100% if stable",
			},
			IntervalMinutes: 10,
		},
		ObservabilityPlan: dedupeStrings(observabilityPlan),
	}
}

// buildRollbackPlan creates the MVP rollback template associated with the review.
func buildRollbackPlan(req CreateReviewRequest, bundle ChangeBundle) RollbackPlan {
	metrics := []string{"error_rate", "latency_p95", "success_rate"}
	if containsAuthScope(bundle) {
		metrics = append(metrics, "auth_failure_rate")
	}

	return RollbackPlan{
		TriggerCondition:       "error rate or latency regression exceeds threshold",
		Steps:                  []string{"Stop rollout progression", "Restore previous stable artifact or config", "Verify health checks and core business flow"},
		VersionToRestore:       req.Payload.BaseCommit,
		VerificationMetrics:    dedupeStrings(metrics),
		RollbackDeadlineSecond: 900,
	}
}

// buildReviewSummary creates the top-level summary returned in the Review object.
func buildReviewSummary(bundle ChangeBundle, understanding ChangeUnderstanding, result ScoreResult, signals []RiskSignal) string {
	if len(signals) == 0 {
		return "No meaningful risk signals detected."
	}
	return fmt.Sprintf(
		"%s %s produced %d signal(s); highest assessed risk is %s.",
		understanding.Summary,
		firstNonEmpty(bundle.DiffSummary, "Change summary unavailable."),
		len(signals),
		result.RiskLevel,
	)
}

// inferImpactedComponents extracts a lightweight component list from request
// metadata and file paths.
func inferImpactedComponents(req CreateReviewRequest, fileList []string) []string {
	components := make([]string, 0, len(fileList)+2)
	lowerJoined := strings.ToLower(req.Service + " " + req.Repo + " " + req.Payload.Title)
	for _, token := range []string{"auth", "gateway", "billing", "routing", "migration"} {
		if strings.Contains(lowerJoined, token) {
			components = append(components, token)
		}
	}
	for _, file := range fileList {
		lower := strings.ToLower(file)
		switch {
		case strings.Contains(lower, "route"), strings.Contains(lower, "ingress"):
			components = append(components, "routing")
		case strings.Contains(lower, "auth"):
			components = append(components, "auth")
		case strings.Contains(lower, "migration"), strings.Contains(lower, ".sql"):
			components = append(components, "schema")
		}
	}
	return dedupeStrings(components)
}

// summarizeDiff generates a compact textual summary of the current change.
func summarizeDiff(req CreateReviewRequest, fileList []string) string {
	if len(fileList) > 0 {
		return fmt.Sprintf("Touched %d file(s), including %s.", len(fileList), fileList[0])
	}
	if req.Payload.Title != "" {
		return req.Payload.Title
	}
	return ""
}

// containsCriticalPath checks whether the change appears to touch a critical
// service or path according to simple keyword heuristics.
func containsCriticalPath(bundle ChangeBundle) bool {
	scope := strings.ToLower(strings.Join([]string{bundle.Service, bundle.Repo, bundle.Title, bundle.DiffSummary}, " "))
	for _, token := range []string{"auth", "payment", "gateway", "billing"} {
		if strings.Contains(scope, token) {
			return true
		}
	}
	return false
}

// containsAuthScope checks whether auth-related scope is part of the change.
func containsAuthScope(bundle ChangeBundle) bool {
	scope := strings.ToLower(strings.Join(append([]string{bundle.Service, bundle.Repo, bundle.Title}, bundle.FileList...), " "))
	return strings.Contains(scope, "auth")
}

// containsSensitiveArtifact identifies file names that deserve additional review.
func containsSensitiveArtifact(fileList []string) bool {
	for _, item := range fileList {
		lower := strings.ToLower(item)
		if strings.Contains(lower, "migration") || strings.Contains(lower, "ingress") || strings.Contains(lower, "route") || strings.Contains(lower, ".sql") {
			return true
		}
	}
	return false
}

// dedupeStrings keeps insertion order while removing duplicates and empty values.
func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// metadataStringSlice reads a string slice from the loose metadata payload while
// tolerating either []string or []any shapes produced by JSON decoding.
func metadataStringSlice(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	value, ok := metadata[key]
	if !ok {
		return nil
	}

	switch raw := value.(type) {
	case []string:
		return dedupeStrings(raw)
	case []any:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			text, ok := item.(string)
			if ok {
				out = append(out, text)
			}
		}
		return dedupeStrings(out)
	default:
		return nil
	}
}
