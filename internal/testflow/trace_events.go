package testflow

import (
	"sort"
	"strconv"
	"strings"
	"time"
)

func traceEventsForRecord(record Record) []TraceEvent {
	events := make([]TraceEvent, 0, len(record.Timeline)+len(record.TestCases)+len(record.ExecutionResults)+3)
	plan := executionPlanForRecord(record)
	sequence := 1

	for i, event := range record.Timeline {
		events = append(events, TraceEvent{
			EventID:  traceEventID(record.Task.TaskID, "timeline", i+1),
			At:       event.At,
			State:    event.State,
			Kind:     "state_transition",
			Message:  event.Detail,
			Sequence: sequence,
		})
		sequence++
	}

	planAt := firstTimelineAt(record.Timeline, StatusGenerateTestCases)
	for i, step := range plan.Steps {
		events = append(events, TraceEvent{
			EventID:       traceEventID(record.Task.TaskID, "plan", i+1),
			At:            stepTime(planAt, i),
			State:         StatusGenerateTestCases,
			Kind:          "plan_step",
			Message:       step.Name,
			StepID:        step.StepID,
			CaseID:        caseIDFromStepID(step.StepID),
			ToolName:      step.ToolName,
			ArtifactTypes: append([]string(nil), step.ExpectedArtifacts...),
			Sequence:      sequence,
		})
		sequence++
	}

	executeAt := firstTimelineAt(record.Timeline, StatusExecuteTools)
	for i, result := range record.ExecutionResults {
		events = append(events, TraceEvent{
			EventID:       traceEventID(record.Task.TaskID, "result", i+1),
			At:            stepTime(executeAt, i),
			State:         StatusExecuteTools,
			Kind:          "tool_result",
			Message:       result.Summary,
			CaseID:        result.CaseID,
			ToolName:      result.ToolName,
			Status:        result.Status,
			ArtifactTypes: artifactTypes(result.Artifacts),
			Sequence:      sequence,
		})
		sequence++
	}

	events = append(events, TraceEvent{
		EventID:  traceEventID(record.Task.TaskID, "assertion", 1),
		At:       firstTimelineAt(record.Timeline, StatusSmartAssert),
		State:    StatusSmartAssert,
		Kind:     "assertion",
		Message:  record.AssertionResult.Summary,
		Status:   record.AssertionResult.Status,
		Sequence: sequence,
	})
	sequence++

	events = append(events, TraceEvent{
		EventID:  traceEventID(record.Task.TaskID, "failure", 1),
		At:       firstTimelineAt(record.Timeline, StatusRootCauseAnalyze),
		State:    StatusRootCauseAnalyze,
		Kind:     "failure_analysis",
		Message:  record.FailureAnalysis.ProbableRootCause,
		Sequence: sequence,
	})
	sequence++

	events = append(events, TraceEvent{
		EventID:  traceEventID(record.Task.TaskID, "report", 1),
		At:       firstTimelineAt(record.Timeline, StatusGenerateReport),
		State:    StatusGenerateReport,
		Kind:     "report",
		Message:  record.Report.Summary,
		Status:   record.Report.OverallStatus,
		Sequence: sequence,
	})

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].At.Equal(events[j].At) {
			return events[i].Sequence < events[j].Sequence
		}
		if events[i].At.IsZero() {
			return false
		}
		if events[j].At.IsZero() {
			return true
		}
		return events[i].At.Before(events[j].At)
	})
	return events
}

func firstTimelineAt(events []TimelineEvent, state TaskStatus) time.Time {
	for _, event := range events {
		if event.State == state {
			return event.At
		}
	}
	if len(events) > 0 {
		return events[len(events)-1].At
	}
	return time.Time{}
}

func stepTime(base time.Time, offset int) time.Time {
	if base.IsZero() {
		return base
	}
	return base.Add(time.Duration(offset) * time.Millisecond)
}

func artifactTypes(artifacts []ExecutionArtifact) []string {
	if len(artifacts) == 0 {
		return nil
	}
	out := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		out = append(out, artifact.ArtifactType)
	}
	sort.Strings(out)
	return out
}

func traceEventID(taskID, kind string, index int) string {
	return taskID + ":" + kind + ":" + strconv.Itoa(index)
}

func caseIDFromStepID(stepID string) string {
	return strings.TrimPrefix(stepID, "step_")
}
