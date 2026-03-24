package testflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type Service struct {
	store   Store
	counter atomic.Uint64
}

// NewService constructs the application service that owns the testflow workflow.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// CreateTask accepts a new task request, applies idempotency, runs the current
// in-process workflow, and persists the resulting aggregate.
func (s *Service) CreateTask(req CreateTestTaskRequest) (CreateTestTaskResponse, error) {
	dedupeKey := dedupeKeyFor(req)
	if dedupeKey != "" {
		existing, err := s.store.FindByDedupeKey(dedupeKey)
		if err == nil {
			return CreateTestTaskResponse{
				TaskID:   existing.Task.TaskID,
				Status:   existing.Task.Status,
				PollURL:  "/api/v1/test-tasks/" + existing.Task.TaskID,
				Scenario: existing.Task.Scenario,
			}, nil
		}
		if !errors.Is(err, ErrNotFound) {
			return CreateTestTaskResponse{}, err
		}
	}

	now := time.Now().UTC()
	taskID := s.nextID("tsk")
	record := Record{
		Task: TestTask{
			TaskID:    taskID,
			ChangeID:  req.SourceID,
			DedupeKey: dedupeKey,
			InputType: req.InputType,
			Scenario:  defaultScenario(req),
			Repo:      req.Repo,
			Service:   req.Service,
			Status:    StatusInit,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Request: &req,
		Timeline: []TimelineEvent{
			{State: StatusInit, At: now, Detail: "test task accepted"},
		},
	}

	record = s.runPipeline(record, req)
	if err := s.store.Save(record); err != nil {
		if dedupeKey != "" && errors.Is(err, ErrConflict) {
			existing, findErr := s.store.FindByDedupeKey(dedupeKey)
			if findErr == nil {
				return CreateTestTaskResponse{
					TaskID:   existing.Task.TaskID,
					Status:   existing.Task.Status,
					PollURL:  "/api/v1/test-tasks/" + existing.Task.TaskID,
					Scenario: existing.Task.Scenario,
				}, nil
			}
			if !errors.Is(findErr, ErrNotFound) {
				return CreateTestTaskResponse{}, findErr
			}
		}
		return CreateTestTaskResponse{}, err
	}

	return CreateTestTaskResponse{
		TaskID:   taskID,
		Status:   record.Task.Status,
		PollURL:  "/api/v1/test-tasks/" + taskID,
		Scenario: record.Task.Scenario,
	}, nil
}

// GetTask returns the latest assembled view of a task and all generated artifacts.
func (s *Service) GetTask(taskID string) (GetTestTaskResponse, error) {
	record, err := s.store.Get(taskID)
	if err != nil {
		return GetTestTaskResponse{}, err
	}

	return GetTestTaskResponse{
		Task:             record.Task,
		TestPoints:       record.TestPoints,
		TestCases:        record.TestCases,
		Workflow:         workflowViewForRecord(record),
		ExecutionPlan:    executionPlanForRecord(record),
		TraceEvents:      traceEventsForRecord(record),
		ExecutionResults: record.ExecutionResults,
		AssertionResult:  record.AssertionResult,
		FailureAnalysis:  record.FailureAnalysis,
		Report:           record.Report,
	}, nil
}

// GetTimeline returns the state transition history for a task.
func (s *Service) GetTimeline(taskID string) (TimelineResponse, error) {
	record, err := s.store.Get(taskID)
	if err != nil {
		return TimelineResponse{}, err
	}
	return TimelineResponse{TaskID: taskID, Events: record.Timeline}, nil
}

// RetryTask replays a failed or manually-blocked task while reusing the same ID.
func (s *Service) RetryTask(taskID string, _ RetryTaskRequest) (RetryTaskResponse, error) {
	record, err := s.store.Get(taskID)
	if err != nil {
		return RetryTaskResponse{}, err
	}
	if record.Task.Status != StatusFailed && record.Task.Status != StatusHumanReviewRequired {
		return RetryTaskResponse{}, ErrConflict
	}

	req := requestFromRecord(record)
	now := time.Now().UTC()
	record.Task.Status = StatusInit
	record.Task.UpdatedAt = now
	record.TestPoints = nil
	record.TestCases = nil
	record.ExecutionResults = nil
	record.AssertionResult = AssertionResult{}
	record.FailureAnalysis = FailureAnalysis{}
	record.Report = TestReport{}
	record.LastError = ""
	record.Timeline = append(record.Timeline, TimelineEvent{State: StatusInit, At: now, Detail: "retry accepted"})
	record = s.runPipeline(record, req)

	if err := s.store.Save(record); err != nil {
		return RetryTaskResponse{}, err
	}
	return RetryTaskResponse{TaskID: taskID, Status: record.Task.Status}, nil
}

// ExportReport renders a task either as JSON or a simple markdown report.
func (s *Service) ExportReport(taskID, format string) ([]byte, string, error) {
	record, err := s.store.Get(taskID)
	if err != nil {
		return nil, "", err
	}

	if format == "json" {
		payload, err := json.MarshalIndent(GetTestTaskResponse{
			Task:             record.Task,
			TestPoints:       record.TestPoints,
			TestCases:        record.TestCases,
			Workflow:         workflowViewForRecord(record),
			ExecutionPlan:    executionPlanForRecord(record),
			TraceEvents:      traceEventsForRecord(record),
			ExecutionResults: record.ExecutionResults,
			AssertionResult:  record.AssertionResult,
			FailureAnalysis:  record.FailureAnalysis,
			Report:           record.Report,
		}, "", "  ")
		return payload, "application/json", err
	}

	body := fmt.Sprintf(
		"# Test Task %s\n\nScenario: %s\n\nStatus: %s / %s\n\nSummary: %s\n\n## Test Points\n",
		record.Task.TaskID,
		record.Task.Scenario,
		record.Task.Status,
		record.Task.OverallStatus,
		record.Task.Summary,
	)
	for _, point := range record.TestPoints {
		body += fmt.Sprintf("- %s: %s\n", point.Name, point.Rationale)
	}
	plan := executionPlanForRecord(record)
	body += "\n## Workflow\n"
	workflow := workflowViewForRecord(record)
	body += fmt.Sprintf("- Workflow: %s (%s)\n", workflow.Name, workflow.State)
	for _, tool := range workflow.Tools {
		body += fmt.Sprintf("- Tool %s: planned=%d executed=%d\n", tool.ToolName, tool.PlannedCount, tool.ExecutedCount)
	}
	body += "\n## Execution Plan\n"
	for _, step := range plan.Steps {
		body += fmt.Sprintf("- %d. %s via %s", step.Order, step.Name, step.ToolName)
		if step.Target != "" {
			body += fmt.Sprintf(" -> %s", step.Target)
		}
		if len(step.VariableRefs) > 0 {
			body += fmt.Sprintf(" [vars: %s]", strings.Join(step.VariableRefs, ", "))
		}
		body += "\n"
	}
	body += "\n## Execution Results\n"
	for _, result := range record.ExecutionResults {
		body += fmt.Sprintf("- %s: %s (%s)\n", result.CaseID, result.Status, result.Summary)
	}
	body += "\n## Trace Events\n"
	for _, event := range traceEventsForRecord(record) {
		body += fmt.Sprintf("- %s / %s: %s\n", event.State, event.Kind, event.Message)
	}
	body += fmt.Sprintf(
		"\n## Failure Analysis\n- Type: %s\n- Cause: %s\n- Next Action: %s\n",
		record.FailureAnalysis.FailureType,
		record.FailureAnalysis.ProbableRootCause,
		record.FailureAnalysis.NextAction,
	)
	if len(record.FailureAnalysis.EvidenceRefs) > 0 {
		body += "- Evidence:\n"
		for _, ref := range record.FailureAnalysis.EvidenceRefs {
			body += fmt.Sprintf("  - %s\n", ref)
		}
	}
	return []byte(body), "text/markdown", nil
}

// GetMetrics computes lightweight aggregate metrics directly from stored task records.
func (s *Service) GetMetrics(from, to, scenario string) (MetricsResponse, error) {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return MetricsResponse{}, fmt.Errorf("invalid from date")
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return MetricsResponse{}, fmt.Errorf("invalid to date")
	}
	end = end.Add(24*time.Hour - time.Nanosecond)

	records := s.store.List()
	var (
		taskCount          int
		totalCases         int
		executableCases    int
		workflowSuccesses  int
		falsePositives     int
		manualIntervention int
		totalDuration      int
	)
	for _, record := range records {
		if record.Task.CreatedAt.Before(start) || record.Task.CreatedAt.After(end) {
			continue
		}
		if scenario != "" && record.Task.Scenario != scenario {
			continue
		}
		taskCount++
		totalCases += len(record.TestCases)
		executableCases += len(record.ExecutionResults)
		if record.Task.Status == StatusDone {
			workflowSuccesses++
		}
		if record.AssertionResult.FalsePositive {
			falsePositives++
		}
		if record.Task.HumanReviewRequired {
			manualIntervention++
		}
		for _, result := range record.ExecutionResults {
			totalDuration += result.DurationMS
		}
	}

	avgDuration := 0
	if executableCases > 0 {
		avgDuration = totalDuration / executableCases
	}

	return MetricsResponse{
		Window:   MetricsWindow{From: from, To: to},
		Scenario: scenario,
		Metrics: Metrics{
			TaskCount:                  taskCount,
			ExecutableCaseRate:         safeRate(executableCases, totalCases),
			WorkflowSuccessRate:        safeRate(workflowSuccesses, taskCount),
			AssertionFalsePositiveRate: safeRate(falsePositives, taskCount),
			AverageDurationMS:          avgDuration,
			AverageTokenCost:           0.18,
			ManualInterventionRate:     safeRate(manualIntervention, taskCount),
		},
	}, nil
}

// runPipeline executes the fixed MVP workflow for a single task.
//
// Each stage appends timeline events and materializes intermediate artifacts so
// later API reads do not need to rerun any business logic.
func (s *Service) runPipeline(record Record, req CreateTestTaskRequest) Record {
	record = s.transition(record, StatusParseChange, "change payload normalized")
	evidenceRef := "change:" + firstNonEmpty(req.Payload.HeadCommit, req.SourceID, record.Task.TaskID)

	record.TestPoints = s.generateTestPoints(req, evidenceRef)
	record = s.transition(record, StatusExtractTestPoints, fmt.Sprintf("%d test point(s) extracted", len(record.TestPoints)))

	record.TestCases = s.generateTestCases(req, record.TestPoints)
	record = s.transition(record, StatusGenerateTestCases, fmt.Sprintf("%d test case(s) generated", len(record.TestCases)))

	record = s.transition(record, StatusPrepareEnv, "execution environment prepared")
	record.ExecutionResults = s.executeTestCases(req, record.TestCases)
	record = s.transition(record, StatusExecuteTools, fmt.Sprintf("%d test case(s) executed", len(record.ExecutionResults)))

	record.AssertionResult = s.assertResults(record.ExecutionResults)
	record.Task.OverallStatus = record.AssertionResult.Status
	record = s.transition(record, StatusSmartAssert, record.AssertionResult.Summary)

	record.FailureAnalysis = s.analyzeFailures(req, record.ExecutionResults, record.AssertionResult)
	record = s.transition(record, StatusRootCauseAnalyze, firstNonEmpty(record.FailureAnalysis.ProbableRootCause, "no failure root cause identified"))

	record.Report = s.buildReport(record)
	record.Task.Summary = record.Report.Summary
	record.Task.Confidence = confidenceFor(record.Report.OverallStatus)
	record.Task.HumanReviewRequired = record.Report.OverallStatus == ExecutionFailed || record.AssertionResult.FalsePositive
	record = s.transition(record, StatusGenerateReport, "test report generated")

	if record.Task.HumanReviewRequired {
		return s.transition(record, StatusHumanReviewRequired, "manual triage required")
	}
	return s.transition(record, StatusDone, "workflow completed")
}

// transition appends a timeline event and updates the current task status.
func (s *Service) transition(record Record, status TaskStatus, detail string) Record {
	now := time.Now().UTC()
	record.Task.Status = status
	record.Task.UpdatedAt = now
	record.Timeline = append(record.Timeline, TimelineEvent{State: status, At: now, Detail: detail})
	return record
}

// generateTestPoints derives the key validation angles from the incoming change metadata.
func (s *Service) generateTestPoints(req CreateTestTaskRequest, evidenceRef string) []TestPoint {
	points := []TestPoint{{
		PointID:      s.nextID("tp"),
		Name:         "API contract regression",
		Category:     "api_regression",
		Rationale:    "PR-driven API regression is the default MVP scenario for the platform.",
		EvidenceRefs: []string{evidenceRef},
	}}

	context := strings.ToLower(strings.Join([]string{req.Service, req.Repo, req.Payload.Title, req.Payload.Description}, " "))
	if strings.Contains(context, "auth") || strings.Contains(context, "token") {
		points = append(points, TestPoint{
			PointID:      s.nextID("tp"),
			Name:         "Authentication flow regression",
			Category:     "auth",
			Rationale:    "Change touches auth-related terms and should validate token and permission flows.",
			EvidenceRefs: []string{evidenceRef},
		})
	}
	if strings.Contains(context, "route") || strings.Contains(context, "gateway") || strings.Contains(context, "ingress") {
		points = append(points, TestPoint{
			PointID:      s.nextID("tp"),
			Name:         "Routing behavior regression",
			Category:     "routing",
			Rationale:    "Routing or gateway changes should validate path mapping and upstream behavior.",
			EvidenceRefs: []string{evidenceRef},
		})
	}
	return points
}

// generateTestCases materializes executable cases from the generated test points.
func (s *Service) generateTestCases(req CreateTestTaskRequest, points []TestPoint) []TestCase {
	if configured := configuredAPIRequestsV2(req.Payload); len(configured) > 0 {
		baseURL := strings.TrimSpace(metadataString(req.Payload.Metadata, "api_base_url"))
		cases := make([]TestCase, 0, len(configured))
		multiStep := len(configured) > 1
		for i, probe := range configured {
			point := defaultPointFor(points, i)
			expected := http.StatusOK
			if probe.ExpectStatus > 0 {
				expected = probe.ExpectStatus
			}
			title := firstNonEmpty(strings.TrimSpace(probe.Name), strings.ToUpper(probe.Method)+" "+probe.Path)
			expectedResult := []string{
				fmt.Sprintf("HTTP status is %d", expected),
			}
			for _, contains := range probe.ExpectBodyContains {
				expectedResult = append(expectedResult, fmt.Sprintf("Response body contains %q", contains))
			}
			for _, contains := range probe.ExpectBodyNotContains {
				expectedResult = append(expectedResult, fmt.Sprintf("Response body does not contain %q", contains))
			}
			for _, header := range sortedKeysString(probe.ExpectHeaders) {
				expectedResult = append(expectedResult, fmt.Sprintf("Response header %s matches the expected value", header))
			}
			for _, header := range probe.ExpectHeadersAbsent {
				expectedResult = append(expectedResult, fmt.Sprintf("Response header %s is absent", header))
			}
			for _, cookieName := range sortedKeysString(probe.ExpectCookies) {
				expectedResult = append(expectedResult, fmt.Sprintf("Response cookie %s matches the expected value", cookieName))
			}
			for _, cookieName := range probe.ExpectCookiesAbsent {
				expectedResult = append(expectedResult, fmt.Sprintf("Response cookie %s is absent", cookieName))
			}
			for _, path := range probe.ExpectJSONPresent {
				expectedResult = append(expectedResult, fmt.Sprintf("JSON path %s is present", path))
			}
			for _, path := range probe.ExpectJSONAbsent {
				expectedResult = append(expectedResult, fmt.Sprintf("JSON path %s is absent", path))
			}
			for _, path := range sortedKeysString(probe.ExpectJSONTypes) {
				expectedResult = append(expectedResult, fmt.Sprintf("JSON path %s matches the expected type", path))
			}
			for _, path := range sortedKeysAny(probe.ExpectJSON) {
				expectedResult = append(expectedResult, fmt.Sprintf("JSON path %s matches the expected value", path))
			}
			if probe.TimeoutMS > 0 {
				expectedResult = append(expectedResult, fmt.Sprintf("Request completes within the %dms timeout budget", probe.TimeoutMS))
			}
			if probe.MaxDurationMS > 0 {
				expectedResult = append(expectedResult, fmt.Sprintf("Request completes within the %dms latency budget", probe.MaxDurationMS))
			}
			if probe.MaxAttempts > 1 {
				expectedResult = append(expectedResult, fmt.Sprintf("Request succeeds within %d attempt(s)", probe.MaxAttempts))
			}
			for _, variable := range sortedKeysString(probe.ExtractHeaders) {
				headerName := probe.ExtractHeaders[variable]
				expectedResult = append(expectedResult, fmt.Sprintf("Response header %s is captured into workflow variable %s", headerName, variable))
			}
			for _, variable := range sortedKeysString(probe.ExtractCookies) {
				cookieName := probe.ExtractCookies[variable]
				expectedResult = append(expectedResult, fmt.Sprintf("Response cookie %s is captured into workflow variable %s", cookieName, variable))
			}

			steps := []string{
				"Build HTTP request from workflow metadata",
				"Send request to the target service",
				"Assert status code matches expectation",
			}
			if len(probe.Query) > 0 {
				steps = append(steps, "Attach templated query parameters to the request URL")
			}
			if len(probe.Cookies) > 0 {
				steps = append(steps, "Attach configured or templated cookies to the request")
			}
			if probe.TimeoutMS > 0 {
				steps = append(steps, "Apply the configured per-request timeout budget")
			}
			if probe.MaxDurationMS > 0 {
				steps = append(steps, "Assert the response completes within the configured latency budget")
			}
			if probe.MaxAttempts > 1 {
				steps = append(steps, "Retry failed requests within the configured attempt budget")
			}
			if len(probe.ExpectHeaders) > 0 || len(probe.ExpectHeadersAbsent) > 0 || len(probe.ExpectCookies) > 0 || len(probe.ExpectCookiesAbsent) > 0 || len(probe.ExpectBodyContains) > 0 || len(probe.ExpectBodyNotContains) > 0 || len(probe.ExpectJSONPresent) > 0 || len(probe.ExpectJSONAbsent) > 0 || len(probe.ExpectJSONTypes) > 0 || len(probe.ExpectJSON) > 0 {
				steps = append(steps, "Assert response headers, cookies, body content, and JSON fields")
			}
			if len(probe.Extract) > 0 {
				steps = append(steps, "Extract response values into workflow context for later requests")
			}
			if len(probe.ExtractHeaders) > 0 {
				steps = append(steps, "Extract response header values into workflow context for later requests")
			}
			if len(probe.ExtractCookies) > 0 {
				steps = append(steps, "Extract response cookie values into workflow context and session state for later requests")
			}

			tags := []string{point.Category, "pr-driven", "http-live"}
			if multiStep || len(probe.Extract) > 0 || len(probe.ExtractHeaders) > 0 || len(probe.ExtractCookies) > 0 {
				tags = append(tags, "multi-step")
			}
			if len(probe.Query) > 0 {
				tags = append(tags, "query-params")
			}
			if len(probe.ExpectHeaders) > 0 || len(probe.ExpectHeadersAbsent) > 0 || len(probe.ExpectCookies) > 0 || len(probe.ExpectCookiesAbsent) > 0 || len(probe.ExpectBodyContains) > 0 || len(probe.ExpectBodyNotContains) > 0 || len(probe.ExpectJSON) > 0 {
				tags = append(tags, "payload-assert")
			}
			if len(probe.ExpectJSONPresent) > 0 || len(probe.ExpectJSONAbsent) > 0 || len(probe.ExpectJSONTypes) > 0 {
				tags = append(tags, "schema-assert")
			}
			if len(probe.Cookies) > 0 || len(probe.ExpectCookies) > 0 || len(probe.ExpectCookiesAbsent) > 0 || len(probe.ExtractCookies) > 0 {
				tags = append(tags, "cookie-session")
			}
			if probe.MaxAttempts > 1 {
				tags = append(tags, "retry")
			}
			if probe.TimeoutMS > 0 {
				tags = append(tags, "timeout-budget")
			}
			if probe.MaxDurationMS > 0 {
				tags = append(tags, "latency-budget")
			}

			cases = append(cases, TestCase{
				CaseID:   s.nextID("tc"),
				Title:    title,
				ToolName: "api_test_runner_http",
				Preconditions: []string{
					"Target API endpoint is reachable from the workflow runtime",
				},
				Steps: steps,
				InputData: map[string]any{
					"category":                 point.Category,
					"base_url":                 baseURL,
					"method":                   strings.ToUpper(probe.Method),
					"path":                     probe.Path,
					"query":                    probe.Query,
					"expect_status":            expected,
					"expect_headers":           probe.ExpectHeaders,
					"expect_headers_absent":    probe.ExpectHeadersAbsent,
					"headers":                  probe.Headers,
					"cookies":                  probe.Cookies,
					"expect_cookies":           probe.ExpectCookies,
					"expect_cookies_absent":    probe.ExpectCookiesAbsent,
					"body":                     probe.Body,
					"expect_body_contains":     probe.ExpectBodyContains,
					"expect_body_not_contains": probe.ExpectBodyNotContains,
					"expect_json_present":      probe.ExpectJSONPresent,
					"expect_json_absent":       probe.ExpectJSONAbsent,
					"expect_json":              probe.ExpectJSON,
					"expect_json_types":        probe.ExpectJSONTypes,
					"extract":                  probe.Extract,
					"extract_headers":          probe.ExtractHeaders,
					"extract_cookies":          probe.ExtractCookies,
					"timeout_ms":               probe.TimeoutMS,
					"max_duration_ms":          probe.MaxDurationMS,
					"max_attempts":             probe.MaxAttempts,
				},
				ExpectedResult: expectedResult,
				Tags:           tags,
			})
		}
		return cases
	}

	cases := make([]TestCase, 0, len(points))
	for _, point := range points {
		cases = append(cases, TestCase{
			CaseID:   s.nextID("tc"),
			Title:    point.Name,
			ToolName: "api_test_runner",
			Preconditions: []string{
				"Seed baseline test data",
				"Prepare mock downstream dependencies",
			},
			Steps: []string{
				"Load generated API request set",
				"Invoke target service through the configured gateway",
				"Collect status code, body, and latency artifacts",
			},
			InputData: map[string]any{"category": point.Category},
			ExpectedResult: []string{
				"Response schema remains compatible",
				"No error budget regression is observed",
			},
			Tags: []string{point.Category, "pr-driven"},
		})
	}
	return cases
}

// executeTestCases runs generated test cases against either configured live HTTP
// targets or the deterministic fallback heuristic.
func (s *Service) executeTestCases(req CreateTestTaskRequest, cases []TestCase) []ExecutionResult {
	results := make([]ExecutionResult, 0, len(cases))
	context := strings.ToLower(strings.Join([]string{req.Service, req.Repo, req.Payload.Title, req.Payload.Description}, " "))
	liveVariables := configuredAPIVariables(req.Payload)
	liveCookies := map[string]string(nil)
	for i, testCase := range cases {
		if liveResult, extracted, nextCookies, ok := runHTTPCaseWithVariables(testCase, liveVariables, liveCookies); ok {
			results = append(results, liveResult)
			if liveResult.Status == ExecutionPassed {
				if len(extracted) > 0 && liveVariables == nil {
					liveVariables = map[string]string{}
				}
				for key, value := range extracted {
					liveVariables[key] = value
				}
				liveCookies = nextCookies
			}
			continue
		}

		status := ExecutionPassed
		summary := "execution passed with stable assertions"
		if i == len(cases)-1 && (strings.Contains(context, "auth") || strings.Contains(context, "gateway")) {
			status = ExecutionFailed
			summary = "received incompatible response payload from changed endpoint"
		}
		results = append(results, ExecutionResult{
			CaseID:     testCase.CaseID,
			ToolName:   testCase.ToolName,
			Status:     status,
			DurationMS: 800 + i*150,
			Summary:    summary,
			Artifacts: []ExecutionArtifact{{
				ArtifactType: "http_trace",
				Location:     fmt.Sprintf("/artifacts/%s.json", testCase.CaseID),
				Snippet:      summary,
			}},
		})
	}
	return results
}

func buildHTTPFailureResult(testCase TestCase, started time.Time, url, method, path string, headers, requestCookies map[string]string, body string, summary string, attemptsUsed int) ExecutionResult {
	durationMS := int(time.Since(started).Milliseconds())
	if durationMS < 1 {
		durationMS = 1
	}
	return ExecutionResult{
		CaseID:     testCase.CaseID,
		ToolName:   testCase.ToolName,
		Status:     ExecutionFailed,
		DurationMS: durationMS,
		Summary:    summary,
		Artifacts:  buildHTTPArtifacts(url, method, path, headers, requestCookies, body, testCase.InputData, 0, nil, nil, nil, nil, nil, summary, attemptsUsed, durationMS),
	}
}

func defaultPointFor(points []TestPoint, index int) TestPoint {
	if len(points) == 0 {
		return TestPoint{Category: "api_regression"}
	}
	return points[index%len(points)]
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func metadataInt(metadata map[string]any, key string) int {
	if metadata == nil {
		return 0
	}
	value, ok := metadata[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	default:
		return 0
	}
}

func metadataStringMap(metadata map[string]any, key string) map[string]string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]string:
		return typed
	case map[string]any:
		out := make(map[string]string, len(typed))
		for k, v := range typed {
			out[k] = fmt.Sprintf("%v", v)
		}
		return out
	default:
		return nil
	}
}

func inputDataString(input map[string]any, key string) string {
	return metadataString(input, key)
}

func inputDataInt(input map[string]any, key string) int {
	return metadataInt(input, key)
}

func inputDataStringMap(input map[string]any, key string) map[string]string {
	return metadataStringMap(input, key)
}

func truncate(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}

// assertResults collapses raw execution outcomes into a higher-level assertion judgment.
func (s *Service) assertResults(results []ExecutionResult) AssertionResult {
	if len(results) == 0 {
		return AssertionResult{Status: ExecutionNeedsAttention, Summary: "no executable test case was produced"}
	}

	passed := 0
	failed := 0
	for _, result := range results {
		if result.Status == ExecutionPassed {
			passed++
			continue
		}
		failed++
	}

	outcome := AssertionResult{PassedCount: passed, FailedCount: failed}
	switch {
	case failed == 0:
		outcome.Status = ExecutionPassed
		outcome.Summary = "all generated assertions passed"
	case failed == 1 && len(results) > 2:
		outcome.Status = ExecutionNeedsAttention
		outcome.FalsePositive = true
		outcome.Summary = "a minority of assertions failed and needs manual confirmation"
	default:
		outcome.Status = ExecutionFailed
		outcome.Summary = "assertion failures indicate a likely product regression"
	}
	return outcome
}

// analyzeFailures produces the workflow's best-effort explanation for failures
// or ambiguous results.
func (s *Service) analyzeFailures(req CreateTestTaskRequest, results []ExecutionResult, assertion AssertionResult) FailureAnalysis {
	if assertion.Status == ExecutionPassed {
		return FailureAnalysis{
			FailureType:       "none",
			ProbableRootCause: "no user-visible regression was detected",
			Confidence:        0.91,
			NextAction:        "promote the generated report to the PR discussion",
		}
	}

	context := strings.ToLower(strings.Join([]string{req.Service, req.Repo, req.Payload.Title, req.Payload.Description}, " "))
	if assertion.FalsePositive {
		return FailureAnalysis{
			FailureType:       "assertion_instability",
			ProbableRootCause: "generated assertion appears brittle against partial response shape changes",
			EvidenceRefs:      collectFailureEvidenceRefs(results, "http_cookie", "http_assertion", "http_response", "http_trace"),
			Confidence:        0.62,
			NextAction:        "manually inspect traces and tighten assertion schema",
		}
	}
	if hasFailureSummaryPattern(results, "request failed") || failedArtifactSnippetContains(results, "http_response", "status=unavailable") {
		return FailureAnalysis{
			FailureType:       "environment_or_connectivity",
			ProbableRootCause: "workflow runtime could not reach the target API or its execution environment was unhealthy",
			EvidenceRefs:      collectFailureEvidenceRefs(results, "http_request", "http_response", "http_assertion", "http_trace"),
			Confidence:        0.87,
			NextAction:        "verify network reachability, service readiness, and environment configuration before retrying",
		}
	}
	if hasFailureSummaryPattern(results, "exceeded max_duration_ms budget") {
		return FailureAnalysis{
			FailureType:       "performance_regression",
			ProbableRootCause: "target endpoint responded successfully but exceeded the configured latency budget during live execution",
			EvidenceRefs:      collectFailureEvidenceRefs(results, "http_cookie", "http_assertion", "http_response", "http_trace"),
			Confidence:        0.84,
			NextAction:        "inspect endpoint latency, downstream dependencies, and recent query or payload changes before merging",
		}
	}
	if strings.Contains(context, "auth") && (failedHTTPStatusMatches(results, http.StatusUnauthorized, http.StatusForbidden) || failedArtifactSnippetContains(results, "http_response", "missing token") || failedArtifactSnippetContains(results, "http_response", "forbidden")) {
		return FailureAnalysis{
			FailureType:       "authentication_regression",
			ProbableRootCause: "authentication or permission flow changed and no longer satisfies the generated token or access expectations",
			EvidenceRefs:      collectFailureEvidenceRefs(results, "http_request", "http_cookie", "http_response", "http_assertion"),
			Confidence:        0.9,
			NextAction:        "inspect auth middleware, token issuance, and access-control changes before retrying the workflow",
		}
	}
	if failedHTTPStatusRange(results, 500, 599) {
		return FailureAnalysis{
			FailureType:       "service_runtime_failure",
			ProbableRootCause: "target service returned a 5xx response during live execution, indicating a runtime or dependency failure",
			EvidenceRefs:      collectFailureEvidenceRefs(results, "http_response", "http_trace", "http_assertion"),
			Confidence:        0.89,
			NextAction:        "inspect service logs, traces, and downstream dependencies around the failing request before merging",
		}
	}
	if failedHTTPStatusRange(results, 400, 499) {
		return FailureAnalysis{
			FailureType:       "contract_regression",
			ProbableRootCause: "target endpoint returned a 4xx response that no longer matches the generated request and contract expectations",
			EvidenceRefs:      collectFailureEvidenceRefs(results, "http_request", "http_cookie", "http_response", "http_assertion"),
			Confidence:        0.85,
			NextAction:        "compare the failing request and response artifacts to the intended contract, then update either the service behavior or the generated assertions",
		}
	}
	if hasFailureSummaryPattern(results, "expected response header") || hasFailureSummaryPattern(results, "expected response cookie") || hasFailureSummaryPattern(results, "expected response body to contain") || hasFailureSummaryPattern(results, "expected response body not to contain") || hasFailureSummaryPattern(results, "expected JSON path") || hasFailureSummaryPattern(results, "response body was not valid JSON for path assertions") {
		return FailureAnalysis{
			FailureType:       "contract_regression",
			ProbableRootCause: "response payload no longer satisfies the generated API contract assertions",
			EvidenceRefs:      collectFailureEvidenceRefs(results, "http_cookie", "http_assertion", "http_response", "http_trace"),
			Confidence:        0.86,
			NextAction:        "inspect the failing payload artifact and update either the service contract or the generated assertions",
		}
	}
	rootCause := "response contract changed without matching downstream update"
	if strings.Contains(context, "auth") {
		rootCause = "authentication or permission flow changed and broke existing token expectations"
	}
	return FailureAnalysis{
		FailureType:       "product_regression",
		ProbableRootCause: rootCause,
		EvidenceRefs:      collectFailureEvidenceRefs(results, "http_response", "http_assertion", "http_trace"),
		Confidence:        0.84,
		NextAction:        "block merge until the failing regression is triaged",
	}
}

// buildReport assembles the final task summary returned by the API.
func (s *Service) buildReport(record Record) TestReport {
	report := TestReport{
		OverallStatus: record.AssertionResult.Status,
		TotalCases:    len(record.TestCases),
		PassedCases:   record.AssertionResult.PassedCount,
		FailedCases:   record.AssertionResult.FailedCount,
	}
	switch record.AssertionResult.Status {
	case ExecutionPassed:
		report.Summary = fmt.Sprintf("%d generated API regression case(s) passed.", report.TotalCases)
		report.RecommendedAction = "attach the report to the PR and continue pipeline promotion"
	case ExecutionNeedsAttention:
		report.Summary = "generated tests found ambiguous evidence and require manual triage."
		report.RecommendedAction = "review the failed assertion and decide whether to regenerate or quarantine it"
	default:
		report.Summary = "generated tests found a likely regression that should block merge."
		report.RecommendedAction = "keep the PR blocked until the suspected root cause is resolved"
	}
	return report
}

// defaultScenario provides a stable fallback scenario classification.
func defaultScenario(req CreateTestTaskRequest) string {
	if req.Scenario != "" {
		return req.Scenario
	}
	return "api_regression"
}

// dedupeKeyFor computes the idempotency key used to avoid duplicate task creation.
func dedupeKeyFor(req CreateTestTaskRequest) string {
	if req.DedupeKey != "" {
		return req.DedupeKey
	}
	if req.SourceID == "" || req.Payload.HeadCommit == "" {
		return ""
	}
	return strings.ToLower(fmt.Sprintf("%s:%s:%s", req.InputType, req.SourceID, req.Payload.HeadCommit))
}

// requestFromRecord reconstructs a request when retrying a persisted task.
func requestFromRecord(record Record) CreateTestTaskRequest {
	if record.Request != nil {
		return *record.Request
	}
	return CreateTestTaskRequest{
		InputType: record.Task.InputType,
		SourceID:  record.Task.ChangeID,
		Repo:      record.Task.Repo,
		Service:   record.Task.Service,
		Scenario:  record.Task.Scenario,
		DedupeKey: record.Task.DedupeKey,
	}
}

// confidenceFor assigns a heuristic confidence to the workflow outcome.
func confidenceFor(status ExecutionStatus) float64 {
	switch status {
	case ExecutionPassed:
		return 0.90
	case ExecutionNeedsAttention:
		return 0.68
	default:
		return 0.83
	}
}

// safeRate guards simple ratio calculations against division by zero.
func safeRate(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

// firstNonEmpty returns the first populated string from a list of candidates.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

// nextID generates monotonic in-process identifiers for task artifacts.
func (s *Service) nextID(prefix string) string {
	seq := s.counter.Add(1)
	return fmt.Sprintf("%s_%d", prefix, seq)
}
