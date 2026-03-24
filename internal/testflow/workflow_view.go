package testflow

import (
	"sort"
	"strconv"
)

func workflowViewForRecord(record Record) WorkflowView {
	return WorkflowView{
		WorkflowID: "wf_" + record.Task.TaskID,
		Name:       record.Task.Scenario + "_workflow",
		State:      record.Task.Status,
		Steps:      workflowStepsForRecord(record),
		Tools:      workflowToolsForRecord(record),
	}
}

func workflowStepsForRecord(record Record) []WorkflowStepView {
	steps := make([]WorkflowStepView, 0, len(record.Timeline))
	for i, event := range record.Timeline {
		status := "completed"
		if i == len(record.Timeline)-1 {
			status = "current"
		}
		steps = append(steps, WorkflowStepView{
			StepID: "workflow_step_" + record.Task.TaskID + "_" + strconv.Itoa(i+1),
			State:  event.State,
			Title:  workflowStepTitle(event.State),
			Detail: event.Detail,
			Status: status,
			At:     event.At,
			Order:  i + 1,
		})
	}
	return steps
}

func workflowToolsForRecord(record Record) []WorkflowToolView {
	plan := executionPlanForRecord(record)
	byTool := map[string]*WorkflowToolView{}

	for _, step := range plan.Steps {
		view := byTool[step.ToolName]
		if view == nil {
			view = &WorkflowToolView{ToolName: step.ToolName}
			byTool[step.ToolName] = view
		}
		view.PlannedCount++
		view.ExpectedArtifacts = unionStrings(view.ExpectedArtifacts, step.ExpectedArtifacts)
	}

	for _, result := range record.ExecutionResults {
		view := byTool[result.ToolName]
		if view == nil {
			view = &WorkflowToolView{ToolName: result.ToolName}
			byTool[result.ToolName] = view
		}
		view.ExecutedCount++
		view.LastStatus = result.Status
		view.ExpectedArtifacts = unionStrings(view.ExpectedArtifacts, artifactTypes(result.Artifacts))
	}

	tools := make([]WorkflowToolView, 0, len(byTool))
	for _, view := range byTool {
		tools = append(tools, *view)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].ToolName < tools[j].ToolName
	})
	return tools
}

func unionStrings(left, right []string) []string {
	set := map[string]struct{}{}
	for _, item := range left {
		if item != "" {
			set[item] = struct{}{}
		}
	}
	for _, item := range right {
		if item != "" {
			set[item] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for item := range set {
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func workflowStepTitle(state TaskStatus) string {
	switch state {
	case StatusInit:
		return "Task Accepted"
	case StatusParseChange:
		return "Parse Change"
	case StatusExtractTestPoints:
		return "Extract Test Points"
	case StatusGenerateTestCases:
		return "Generate Test Cases"
	case StatusPrepareEnv:
		return "Prepare Environment"
	case StatusExecuteTools:
		return "Execute Tools"
	case StatusSmartAssert:
		return "Smart Assertion"
	case StatusRootCauseAnalyze:
		return "Analyze Root Cause"
	case StatusGenerateReport:
		return "Generate Report"
	case StatusHumanReviewRequired:
		return "Manual Triage"
	case StatusDone:
		return "Workflow Complete"
	case StatusFailed:
		return "Workflow Failed"
	default:
		return string(state)
	}
}
