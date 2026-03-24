package testflow

import (
	"fmt"
	"sort"
	"strings"
)

func executionPlanForRecord(record Record) ExecutionPlan {
	steps := make([]ExecutionPlanStep, 0, len(record.TestCases))
	artifactSet := map[string]struct{}{}
	mode := "heuristic"
	if hasLiveHTTPCase(record.TestCases) {
		mode = "live_http"
	}

	for i, testCase := range record.TestCases {
		target := executionTargetForCase(testCase)
		artifacts := expectedArtifactsForCase(testCase)
		for _, artifact := range artifacts {
			artifactSet[artifact] = struct{}{}
		}
		steps = append(steps, ExecutionPlanStep{
			StepID:            "step_" + testCase.CaseID,
			Order:             i + 1,
			Name:              testCase.Title,
			ToolName:          testCase.ToolName,
			Target:            target,
			Assertions:        append([]string(nil), testCase.ExpectedResult...),
			VariableRefs:      templateVariablesForCase(testCase),
			ExpectedArtifacts: artifacts,
		})
	}

	expectedArtifacts := make([]string, 0, len(artifactSet))
	for artifact := range artifactSet {
		expectedArtifacts = append(expectedArtifacts, artifact)
	}
	sort.Strings(expectedArtifacts)

	summary := fmt.Sprintf("%d execution step(s) prepared in %s mode.", len(steps), mode)
	if len(steps) == 0 {
		summary = "no execution step was generated"
	}

	return ExecutionPlan{
		PlanID:            "plan_" + record.Task.TaskID,
		Mode:              mode,
		Summary:           summary,
		Steps:             steps,
		ExpectedArtifacts: expectedArtifacts,
	}
}

func hasLiveHTTPCase(testCases []TestCase) bool {
	for _, testCase := range testCases {
		if testCase.ToolName == "api_test_runner_http" {
			return true
		}
	}
	return false
}

func executionTargetForCase(testCase TestCase) string {
	baseURL := strings.TrimSpace(inputDataString(testCase.InputData, "base_url"))
	path := strings.TrimSpace(inputDataString(testCase.InputData, "path"))
	query := inputDataStringMap(testCase.InputData, "query")
	if baseURL != "" && path != "" {
		return appendRequestQuery(joinRequestURL(baseURL, path), query)
	}
	if category := strings.TrimSpace(inputDataString(testCase.InputData, "category")); category != "" {
		return category
	}
	return ""
}

func expectedArtifactsForCase(testCase TestCase) []string {
	if testCase.ToolName != "api_test_runner_http" {
		return []string{"http_trace"}
	}

	artifacts := []string{"http_assertion", "http_request", "http_response", "http_trace"}
	if len(inputDataStringMap(testCase.InputData, "extract")) > 0 || len(inputDataStringMap(testCase.InputData, "extract_headers")) > 0 || len(inputDataStringMap(testCase.InputData, "extract_cookies")) > 0 {
		artifacts = append(artifacts, "http_extract")
	}
	if len(inputDataStringMap(testCase.InputData, "cookies")) > 0 || len(inputDataStringMap(testCase.InputData, "expect_cookies")) > 0 || len(inputDataStringSlice(testCase.InputData, "expect_cookies_absent")) > 0 || len(inputDataStringMap(testCase.InputData, "extract_cookies")) > 0 {
		artifacts = append(artifacts, "http_cookie")
	}
	return artifacts
}

func templateVariablesForCase(testCase TestCase) []string {
	values := []string{
		inputDataString(testCase.InputData, "base_url"),
		inputDataString(testCase.InputData, "path"),
		inputDataString(testCase.InputData, "body"),
		inputDataString(testCase.InputData, "method"),
	}
	for _, value := range inputDataStringMap(testCase.InputData, "headers") {
		values = append(values, value)
	}
	for _, value := range inputDataStringMap(testCase.InputData, "query") {
		values = append(values, value)
	}
	for _, value := range inputDataStringMap(testCase.InputData, "cookies") {
		values = append(values, value)
	}

	set := map[string]struct{}{}
	for _, value := range values {
		for _, variable := range templateVariables(value) {
			set[variable] = struct{}{}
		}
	}

	out := make([]string, 0, len(set))
	for variable := range set {
		out = append(out, variable)
	}
	sort.Strings(out)
	return out
}

func templateVariables(value string) []string {
	if value == "" {
		return nil
	}
	matches := []string{}
	remaining := value
	for {
		start := strings.Index(remaining, "{{")
		if start < 0 {
			break
		}
		remaining = remaining[start+2:]
		end := strings.Index(remaining, "}}")
		if end < 0 {
			break
		}
		variable := strings.TrimSpace(remaining[:end])
		if variable != "" {
			matches = append(matches, variable)
		}
		remaining = remaining[end+2:]
	}
	return matches
}
