package testflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

type liveAPIProbeRequest struct {
	Name                  string
	Method                string
	Path                  string
	Query                 map[string]string
	ExpectStatus          int
	ExpectHeaders         map[string]string
	ExpectHeadersAbsent   []string
	Headers               map[string]string
	Cookies               map[string]string
	Body                  string
	ExpectBodyContains    []string
	ExpectBodyNotContains []string
	ExpectCookies         map[string]string
	ExpectCookiesAbsent   []string
	ExpectJSONPresent     []string
	ExpectJSONAbsent      []string
	ExpectJSON            map[string]any
	ExpectJSONTypes       map[string]string
	Extract               map[string]string
	ExtractHeaders        map[string]string
	ExtractCookies        map[string]string
	TimeoutMS             int
	MaxDurationMS         int
	MaxAttempts           int
}

func configuredAPIRequestsV2(payload ChangeInputPayload) []liveAPIProbeRequest {
	if len(payload.Metadata) == 0 {
		return nil
	}
	rawRequests, ok := payload.Metadata["requests"]
	if !ok {
		return nil
	}
	items, ok := rawRequests.([]any)
	if !ok {
		return nil
	}
	defaultHeaders := metadataStringMap(payload.Metadata, "default_headers")
	defaultCookies := metadataStringMap(payload.Metadata, "default_cookies")

	out := make([]liveAPIProbeRequest, 0, len(items))
	for _, item := range items {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}

		method := strings.ToUpper(firstNonEmpty(metadataString(entry, "method"), "GET"))
		path := strings.TrimSpace(metadataString(entry, "path"))
		if path == "" {
			continue
		}
		out = append(out, liveAPIProbeRequest{
			Name:                  strings.TrimSpace(metadataString(entry, "name")),
			Method:                method,
			Path:                  path,
			Query:                 metadataStringMap(entry, "query"),
			ExpectStatus:          metadataInt(entry, "expect_status"),
			ExpectHeaders:         metadataStringMap(entry, "expect_headers"),
			ExpectHeadersAbsent:   metadataStringSlice(entry, "expect_headers_absent"),
			Headers:               mergeStringMaps(defaultHeaders, metadataStringMap(entry, "headers")),
			Cookies:               mergeStringMaps(defaultCookies, metadataStringMap(entry, "cookies")),
			Body:                  metadataRequestBody(entry, "body"),
			ExpectBodyContains:    metadataStringSlice(entry, "expect_body_contains"),
			ExpectBodyNotContains: metadataStringSlice(entry, "expect_body_not_contains"),
			ExpectCookies:         metadataStringMap(entry, "expect_cookies"),
			ExpectCookiesAbsent:   metadataStringSlice(entry, "expect_cookies_absent"),
			ExpectJSONPresent:     metadataStringSlice(entry, "expect_json_present"),
			ExpectJSONAbsent:      metadataStringSlice(entry, "expect_json_absent"),
			ExpectJSON:            metadataAnyMap(entry, "expect_json"),
			ExpectJSONTypes:       metadataStringMap(entry, "expect_json_types"),
			Extract:               metadataStringMap(entry, "extract"),
			ExtractHeaders:        metadataStringMap(entry, "extract_headers"),
			ExtractCookies:        metadataStringMap(entry, "extract_cookies"),
			TimeoutMS:             metadataInt(entry, "timeout_ms"),
			MaxDurationMS:         metadataInt(entry, "max_duration_ms"),
			MaxAttempts:           metadataInt(entry, "max_attempts"),
		})
	}
	return out
}

func configuredAPIVariables(payload ChangeInputPayload) map[string]string {
	return metadataStringMap(payload.Metadata, "variables")
}

func runHTTPCaseWithVariables(testCase TestCase, variables, sessionCookies map[string]string) (ExecutionResult, map[string]string, map[string]string, bool) {
	baseURL := strings.TrimSpace(resolveTemplate(inputDataString(testCase.InputData, "base_url"), variables))
	path := strings.TrimSpace(resolveTemplate(inputDataString(testCase.InputData, "path"), variables))
	requestURL := joinRequestURL(baseURL, path)
	if requestURL == "" {
		return ExecutionResult{}, nil, nil, false
	}
	query := resolveTemplateMap(inputDataStringMap(testCase.InputData, "query"), variables)
	requestURL = appendRequestQuery(requestURL, query)
	displayPath := appendRequestQuery(path, query)

	method := strings.ToUpper(firstNonEmpty(resolveTemplate(inputDataString(testCase.InputData, "method"), variables), "GET"))
	expectedStatus := inputDataInt(testCase.InputData, "expect_status")
	if expectedStatus == 0 {
		expectedStatus = http.StatusOK
	}
	timeoutMS := inputDataInt(testCase.InputData, "timeout_ms")
	if timeoutMS <= 0 {
		timeoutMS = 8000
	}
	maxAttempts := inputDataInt(testCase.InputData, "max_attempts")
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	maxDurationMS := inputDataInt(testCase.InputData, "max_duration_ms")
	headers := resolveTemplateMap(inputDataStringMap(testCase.InputData, "headers"), variables)
	requestCookies := mergeStringMaps(sessionCookies, resolveTemplateMap(inputDataStringMap(testCase.InputData, "cookies"), variables))
	body := resolveTemplate(inputDataString(testCase.InputData, "body"), variables)
	client := &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	started := time.Now()

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		httpReq, err := http.NewRequest(method, requestURL, bytes.NewBufferString(body))
		if err != nil {
			summary := "failed to build request: " + err.Error()
			return buildHTTPFailureResult(testCase, started, requestURL, method, displayPath, headers, requestCookies, body, summary, attempt), nil, nil, true
		}
		for key, value := range headers {
			httpReq.Header.Set(key, value)
		}
		applyRequestCookies(httpReq, requestCookies)

		resp, err := client.Do(httpReq)
		if err != nil {
			if attempt < maxAttempts {
				continue
			}
			summary := fmt.Sprintf("request failed after %d attempt(s): %s", attempt, err.Error())
			return buildHTTPFailureResult(testCase, started, requestURL, method, displayPath, headers, requestCookies, body, summary, attempt), nil, nil, true
		}

		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		responsePayload, responseJSONOK := decodeJSONBody(responseBody)
		responseCookies := responseCookieValues(resp.Cookies())
		nextSessionCookies := mergeSessionCookies(sessionCookies, resp.Cookies())
		extractedCookies := extractResponseCookieVariables(responseCookies, inputDataStringMap(testCase.InputData, "extract_cookies"))
		extracted := mergeStringMaps(
			extractResponseVariables(responsePayload, responseJSONOK, inputDataStringMap(testCase.InputData, "extract")),
			extractResponseHeaderVariables(resp.Header, inputDataStringMap(testCase.InputData, "extract_headers")),
		)
		extracted = mergeStringMaps(extracted, extractedCookies)

		status := ExecutionPassed
		summary := withAttemptsSuffix(fmt.Sprintf("received expected status %d from %s %s", expectedStatus, method, displayPath), attempt)
		if resp.StatusCode != expectedStatus {
			status = ExecutionFailed
			summary = withAttemptsSuffix(fmt.Sprintf("expected status %d, got %d from %s %s", expectedStatus, resp.StatusCode, method, displayPath), attempt)
		}
		if status == ExecutionPassed {
			if reason := validateHTTPExpectations(testCase.InputData, resp.Header, responseCookies, responseBody, responsePayload, responseJSONOK); reason != "" {
				status = ExecutionFailed
				summary = withAttemptsSuffix(reason, attempt)
			}
		}

		durationMS := int(time.Since(started).Milliseconds())
		if durationMS < 1 {
			durationMS = 1
		}
		if status == ExecutionPassed && maxDurationMS > 0 && durationMS > maxDurationMS {
			status = ExecutionFailed
			summary = withAttemptsSuffix(fmt.Sprintf("request exceeded max_duration_ms budget: duration_ms=%d max_duration_ms=%d for %s %s", durationMS, maxDurationMS, method, displayPath), attempt)
		}
		artifacts := buildHTTPArtifacts(requestURL, method, displayPath, headers, requestCookies, body, testCase.InputData, resp.StatusCode, responseBody, extracted, responseCookies, extractedCookies, nextSessionCookies, summary, attempt, durationMS)
		if status == ExecutionPassed {
			return ExecutionResult{
				CaseID:     testCase.CaseID,
				ToolName:   testCase.ToolName,
				Status:     status,
				DurationMS: durationMS,
				Summary:    summary,
				Artifacts:  artifacts,
			}, extracted, nextSessionCookies, true
		}
		if attempt == maxAttempts {
			return ExecutionResult{
				CaseID:     testCase.CaseID,
				ToolName:   testCase.ToolName,
				Status:     status,
				DurationMS: durationMS,
				Summary:    summary,
				Artifacts:  artifacts,
			}, nil, nil, true
		}
	}

	return ExecutionResult{}, nil, nil, true
}

func metadataRequestBody(metadata map[string]any, key string) string {
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
		payload, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(payload)
	}
}

func metadataStringSlice(metadata map[string]any, key string) []string {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if item == nil {
				continue
			}
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				out = append(out, text)
			}
		}
		return out
	default:
		return nil
	}
}

func metadataAnyMap(metadata map[string]any, key string) map[string]any {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = v
		}
		return out
	case map[string]string:
		out := make(map[string]any, len(typed))
		for k, v := range typed {
			out[k] = v
		}
		return out
	default:
		return nil
	}
}

func inputDataStringSlice(input map[string]any, key string) []string {
	return metadataStringSlice(input, key)
}

func inputDataAnyMap(input map[string]any, key string) map[string]any {
	return metadataAnyMap(input, key)
}

func mergeStringMaps(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(override))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range override {
		out[key] = value
	}
	return out
}

func resolveTemplate(value string, variables map[string]string) string {
	if value == "" || len(variables) == 0 {
		return value
	}
	resolved := value
	for key, variable := range variables {
		resolved = strings.ReplaceAll(resolved, "{{"+key+"}}", variable)
	}
	return resolved
}

func resolveTemplateMap(values map[string]string, variables map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		out[key] = resolveTemplate(value, variables)
	}
	return out
}

func validateHTTPExpectations(input map[string]any, responseHeaders http.Header, responseCookies map[string]string, responseBody []byte, responsePayload any, responseJSONOK bool) string {
	for key, expected := range inputDataStringMap(input, "expect_headers") {
		actual := responseHeaders.Get(key)
		if actual != expected {
			return fmt.Sprintf("expected response header %s=%q, got %q", key, expected, actual)
		}
	}
	for _, key := range inputDataStringSlice(input, "expect_headers_absent") {
		if actual := responseHeaders.Get(key); actual != "" {
			return fmt.Sprintf("expected response header %s to be absent, got %q", key, actual)
		}
	}
	for key, expected := range inputDataStringMap(input, "expect_cookies") {
		actual := responseCookies[key]
		if actual != expected {
			return fmt.Sprintf("expected response cookie %s=%q, got %q", key, expected, actual)
		}
	}
	for _, key := range inputDataStringSlice(input, "expect_cookies_absent") {
		if actual, ok := responseCookies[key]; ok {
			return fmt.Sprintf("expected response cookie %s to be absent, got %q", key, actual)
		}
	}

	responseText := string(responseBody)
	for _, contains := range inputDataStringSlice(input, "expect_body_contains") {
		if !strings.Contains(responseText, contains) {
			return fmt.Sprintf("expected response body to contain %q", contains)
		}
	}
	for _, contains := range inputDataStringSlice(input, "expect_body_not_contains") {
		if strings.Contains(responseText, contains) {
			return fmt.Sprintf("expected response body not to contain %q", contains)
		}
	}

	expectJSON := inputDataAnyMap(input, "expect_json")
	expectJSONPresent := inputDataStringSlice(input, "expect_json_present")
	expectJSONAbsent := inputDataStringSlice(input, "expect_json_absent")
	expectJSONTypes := inputDataStringMap(input, "expect_json_types")
	if len(expectJSON) == 0 && len(expectJSONPresent) == 0 && len(expectJSONAbsent) == 0 && len(expectJSONTypes) == 0 {
		return ""
	}
	if !responseJSONOK {
		return "response body was not valid JSON for path assertions"
	}

	for _, path := range expectJSONPresent {
		if _, ok := lookupJSONPath(responsePayload, path); !ok {
			return fmt.Sprintf("expected JSON path %s to be present", path)
		}
	}
	for _, path := range expectJSONAbsent {
		if _, ok := lookupJSONPath(responsePayload, path); ok {
			return fmt.Sprintf("expected JSON path %s to be absent", path)
		}
	}

	for _, path := range sortedKeysString(expectJSONTypes) {
		actual, ok := lookupJSONPath(responsePayload, path)
		if !ok {
			return fmt.Sprintf("expected JSON path %s to be present for type assertion", path)
		}
		actualType := jsonTypeName(actual)
		expectedType := strings.ToLower(strings.TrimSpace(expectJSONTypes[path]))
		if actualType != expectedType {
			return fmt.Sprintf("expected JSON path %s type=%s, got %s", path, expectedType, actualType)
		}
	}

	for _, path := range sortedKeysAny(expectJSON) {
		actual, ok := lookupJSONPath(responsePayload, path)
		if !ok {
			return fmt.Sprintf("expected JSON path %s to be present", path)
		}
		if !valuesMatch(expectJSON[path], actual) {
			return fmt.Sprintf("expected JSON path %s=%s, got %s", path, formatValue(expectJSON[path]), formatValue(actual))
		}
	}
	return ""
}

func withAttemptsSuffix(message string, attemptsUsed int) string {
	if attemptsUsed <= 1 {
		return message
	}
	return fmt.Sprintf("%s after %d attempt(s)", message, attemptsUsed)
}

func decodeJSONBody(body []byte) (any, bool) {
	if len(bytes.TrimSpace(body)) == 0 {
		return nil, false
	}
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	return payload, true
}

func extractResponseVariables(responsePayload any, responseJSONOK bool, paths map[string]string) map[string]string {
	if !responseJSONOK || len(paths) == 0 {
		return nil
	}
	out := make(map[string]string, len(paths))
	for key, path := range paths {
		value, ok := lookupJSONPath(responsePayload, path)
		if !ok {
			continue
		}
		out[key] = stringValue(value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func extractResponseHeaderVariables(responseHeaders http.Header, headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := make(map[string]string, len(headers))
	for key, headerName := range headers {
		value := strings.TrimSpace(responseHeaders.Get(headerName))
		if value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func extractResponseCookieVariables(responseCookies, cookies map[string]string) map[string]string {
	if len(cookies) == 0 {
		return nil
	}
	out := make(map[string]string, len(cookies))
	for key, cookieName := range cookies {
		value := responseCookies[cookieName]
		if value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func applyRequestCookies(req *http.Request, cookies map[string]string) {
	for _, name := range sortedKeysString(cookies) {
		if strings.TrimSpace(name) == "" {
			continue
		}
		req.AddCookie(&http.Cookie{Name: name, Value: cookies[name]})
	}
}

func responseCookieValues(cookies []*http.Cookie) map[string]string {
	if len(cookies) == 0 {
		return nil
	}
	out := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		out[cookie.Name] = cookie.Value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergeSessionCookies(existing map[string]string, cookies []*http.Cookie) map[string]string {
	if len(existing) == 0 && len(cookies) == 0 {
		return nil
	}
	out := make(map[string]string, len(existing)+len(cookies))
	for key, value := range existing {
		out[key] = value
	}
	now := time.Now()
	for _, cookie := range cookies {
		if cookie == nil || strings.TrimSpace(cookie.Name) == "" {
			continue
		}
		if cookie.MaxAge < 0 || (!cookie.Expires.IsZero() && cookie.Expires.Before(now)) {
			delete(out, cookie.Name)
			continue
		}
		out[cookie.Name] = cookie.Value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func lookupJSONPath(value any, path string) (any, bool) {
	if strings.TrimSpace(path) == "" {
		return nil, false
	}
	current := value
	for _, segment := range strings.Split(path, ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return nil, false
		}
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func valuesMatch(expected, actual any) bool {
	if expectedNumber, ok := numericValue(expected); ok {
		actualNumber, actualOK := numericValue(actual)
		return actualOK && expectedNumber == actualNumber
	}

	expectedJSON, expectedErr := json.Marshal(expected)
	actualJSON, actualErr := json.Marshal(actual)
	if expectedErr == nil && actualErr == nil {
		return string(expectedJSON) == string(actualJSON)
	}
	return fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", actual)
}

func numericValue(value any) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case float32:
		return float64(typed), true
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	default:
		return 0, false
	}
}

func formatValue(value any) string {
	payload, err := json.Marshal(value)
	if err == nil {
		return string(payload)
	}
	return fmt.Sprintf("%v", value)
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case int, int32, int64, float32, float64, bool, json.Number:
		return fmt.Sprintf("%v", typed)
	default:
		return formatValue(value)
	}
}

func sortedKeysAny(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedKeysString(values map[string]string) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func joinRequestURL(baseURL, path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}
	if baseURL == "" || path == "" {
		return ""
	}
	return strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(path, "/")
}

func appendRequestQuery(rawURL string, query map[string]string) string {
	if rawURL == "" || len(query) == 0 {
		return rawURL
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	values := parsed.Query()
	for key, value := range query {
		if strings.TrimSpace(key) == "" {
			continue
		}
		values.Set(key, value)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String()
}

func buildHTTPArtifacts(url, method, path string, headers, requestCookies map[string]string, body string, input map[string]any, responseStatus int, responseBody []byte, extracted, responseCookies, extractedCookies, sessionCookies map[string]string, summary string, attemptsUsed int, durationMS int) []ExecutionArtifact {
	responseSummary := fmt.Sprintf("attempts=%d duration_ms=%d status=%d body=%s", attemptsUsed, durationMS, responseStatus, string(responseBody))
	if responseStatus == 0 {
		responseSummary = fmt.Sprintf("attempts=%d duration_ms=%d status=unavailable body=%s", attemptsUsed, durationMS, string(responseBody))
	}

	artifacts := []ExecutionArtifact{
		{
			ArtifactType: "http_request",
			Location:     url,
			Snippet: truncate(
				fmt.Sprintf("%s %s headers=%s body=%s", method, path, formatStringMap(redactSensitiveMap(headers)), truncate(body, 120)),
				220,
			),
		},
		{
			ArtifactType: "http_response",
			Location:     url,
			Snippet:      truncate(responseSummary, 220),
		},
		{
			ArtifactType: "http_assertion",
			Location:     url,
			Snippet:      truncate(describeHTTPAssertions(input, responseStatus, summary, attemptsUsed, durationMS), 220),
		},
	}

	if len(requestCookies) > 0 || len(responseCookies) > 0 || len(extractedCookies) > 0 || len(sessionCookies) > 0 {
		parts := []string{}
		if len(requestCookies) > 0 {
			parts = append(parts, "request="+formatStringMap(redactAllValuesMap(requestCookies)))
		}
		if len(responseCookies) > 0 {
			parts = append(parts, "response="+formatStringMap(redactAllValuesMap(responseCookies)))
		}
		if len(extractedCookies) > 0 {
			parts = append(parts, "extracted="+formatStringMap(redactAllValuesMap(extractedCookies)))
		}
		if len(sessionCookies) > 0 {
			parts = append(parts, "session="+formatStringMap(redactAllValuesMap(sessionCookies)))
		}
		artifacts = append(artifacts, ExecutionArtifact{
			ArtifactType: "http_cookie",
			Location:     url,
			Snippet:      truncate(strings.Join(parts, " "), 220),
		})
	}

	if len(extracted) > 0 {
		artifacts = append(artifacts, ExecutionArtifact{
			ArtifactType: "http_extract",
			Location:     url,
			Snippet:      truncate("variables="+formatStringMap(redactSensitiveMap(extracted)), 220),
		})
	}

	artifacts = append(artifacts, ExecutionArtifact{
		ArtifactType: "http_trace",
		Location:     url,
		Snippet:      truncate(fmt.Sprintf("attempts=%d duration_ms=%d summary=%s body=%s", attemptsUsed, durationMS, summary, string(responseBody)), 220),
	})
	return artifacts
}

func describeHTTPAssertions(input map[string]any, responseStatus int, summary string, attemptsUsed int, durationMS int) string {
	parts := []string{
		fmt.Sprintf("expected_status=%d", inputDataInt(input, "expect_status")),
		fmt.Sprintf("actual_status=%d", responseStatus),
		fmt.Sprintf("attempts=%d", attemptsUsed),
		fmt.Sprintf("duration_ms=%d", durationMS),
	}

	if timeoutMS := inputDataInt(input, "timeout_ms"); timeoutMS > 0 {
		parts = append(parts, fmt.Sprintf("timeout_ms=%d", timeoutMS))
	}
	if maxDurationMS := inputDataInt(input, "max_duration_ms"); maxDurationMS > 0 {
		parts = append(parts, fmt.Sprintf("max_duration_ms=%d", maxDurationMS))
	}
	if maxAttempts := inputDataInt(input, "max_attempts"); maxAttempts > 1 {
		parts = append(parts, fmt.Sprintf("max_attempts=%d", maxAttempts))
	}

	expectHeaders := inputDataStringMap(input, "expect_headers")
	if len(expectHeaders) > 0 {
		parts = append(parts, "headers="+strings.Join(sortedKeysString(expectHeaders), ","))
	}
	expectHeadersAbsent := inputDataStringSlice(input, "expect_headers_absent")
	if len(expectHeadersAbsent) > 0 {
		parts = append(parts, "headers_absent="+strings.Join(expectHeadersAbsent, ","))
	}
	expectCookies := inputDataStringMap(input, "expect_cookies")
	if len(expectCookies) > 0 {
		parts = append(parts, "cookies="+strings.Join(sortedKeysString(expectCookies), ","))
	}
	expectCookiesAbsent := inputDataStringSlice(input, "expect_cookies_absent")
	if len(expectCookiesAbsent) > 0 {
		parts = append(parts, "cookies_absent="+strings.Join(expectCookiesAbsent, ","))
	}

	query := inputDataStringMap(input, "query")
	if len(query) > 0 {
		parts = append(parts, "query="+strings.Join(sortedKeysString(query), ","))
	}

	bodyContains := inputDataStringSlice(input, "expect_body_contains")
	if len(bodyContains) > 0 {
		parts = append(parts, "body_contains="+strings.Join(bodyContains, ","))
	}
	bodyNotContains := inputDataStringSlice(input, "expect_body_not_contains")
	if len(bodyNotContains) > 0 {
		parts = append(parts, "body_not_contains="+strings.Join(bodyNotContains, ","))
	}

	expectJSONPresent := inputDataStringSlice(input, "expect_json_present")
	if len(expectJSONPresent) > 0 {
		parts = append(parts, "json_present="+strings.Join(expectJSONPresent, ","))
	}
	expectJSONAbsent := inputDataStringSlice(input, "expect_json_absent")
	if len(expectJSONAbsent) > 0 {
		parts = append(parts, "json_absent="+strings.Join(expectJSONAbsent, ","))
	}

	expectJSONTypes := inputDataStringMap(input, "expect_json_types")
	if len(expectJSONTypes) > 0 {
		parts = append(parts, "json_types="+strings.Join(sortedKeysString(expectJSONTypes), ","))
	}

	expectJSON := inputDataAnyMap(input, "expect_json")
	if len(expectJSON) > 0 {
		parts = append(parts, "json_paths="+strings.Join(sortedKeysAny(expectJSON), ","))
	}

	extractHeaders := inputDataStringMap(input, "extract_headers")
	if len(extractHeaders) > 0 {
		parts = append(parts, "extract_headers="+strings.Join(sortedKeysString(extractHeaders), ","))
	}
	extractCookies := inputDataStringMap(input, "extract_cookies")
	if len(extractCookies) > 0 {
		parts = append(parts, "extract_cookies="+strings.Join(sortedKeysString(extractCookies), ","))
	}

	parts = append(parts, summary)
	return strings.Join(parts, " | ")
}

func redactAllValuesMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key := range values {
		out[key] = "***redacted***"
	}
	return out
}

func redactSensitiveMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		if isSensitiveKey(key) {
			out[key] = "***redacted***"
			continue
		}
		out[key] = value
	}
	return out
}

func formatStringMap(values map[string]string) string {
	if len(values) == 0 {
		return "{}"
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, values[key]))
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

func isSensitiveKey(key string) bool {
	key = strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(key, "authorization") ||
		strings.Contains(key, "token") ||
		strings.Contains(key, "secret") ||
		strings.Contains(key, "password") ||
		strings.Contains(key, "cookie")
}

func jsonTypeName(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case int, int32, int64:
		return "integer"
	case float32:
		return "number"
	case float64:
		if typed == float64(int64(typed)) {
			return "integer"
		}
		return "number"
	case json.Number:
		if _, err := typed.Int64(); err == nil {
			return "integer"
		}
		return "number"
	default:
		return "unknown"
	}
}

func hasFailureSummaryPattern(results []ExecutionResult, pattern string) bool {
	for _, result := range results {
		if result.Status != ExecutionFailed {
			continue
		}
		if strings.Contains(result.Summary, pattern) {
			return true
		}
	}
	return false
}
