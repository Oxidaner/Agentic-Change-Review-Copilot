package testflow

import (
	"strconv"
	"strings"
)

func collectFailureEvidenceRefs(results []ExecutionResult, preferredArtifactTypes ...string) []string {
	refs := make([]string, 0, len(results)*3)
	seen := map[string]struct{}{}

	for _, result := range results {
		if result.Status != ExecutionFailed {
			continue
		}

		refs = addFailureRef(refs, seen, "result:"+result.CaseID)

		addedPreferred := false
		for _, artifactType := range preferredArtifactTypes {
			if _, ok := artifactByType(result.Artifacts, artifactType); !ok {
				continue
			}
			refs = addFailureRef(refs, seen, failureArtifactRef(result.CaseID, artifactType))
			addedPreferred = true
		}
		if addedPreferred {
			continue
		}
		for _, artifact := range result.Artifacts {
			refs = addFailureRef(refs, seen, failureArtifactRef(result.CaseID, artifact.ArtifactType))
		}
	}

	return refs
}

func addFailureRef(refs []string, seen map[string]struct{}, ref string) []string {
	if ref == "" {
		return refs
	}
	if _, ok := seen[ref]; ok {
		return refs
	}
	seen[ref] = struct{}{}
	return append(refs, ref)
}

func failureArtifactRef(caseID, artifactType string) string {
	return "artifact:" + caseID + ":" + artifactType
}

func artifactByType(artifacts []ExecutionArtifact, artifactType string) (ExecutionArtifact, bool) {
	for _, artifact := range artifacts {
		if artifact.ArtifactType == artifactType {
			return artifact, true
		}
	}
	return ExecutionArtifact{}, false
}

func failedArtifactSnippetContains(results []ExecutionResult, artifactType, fragment string) bool {
	if fragment == "" {
		return false
	}
	fragment = strings.ToLower(fragment)
	for _, result := range results {
		if result.Status != ExecutionFailed {
			continue
		}
		artifact, ok := artifactByType(result.Artifacts, artifactType)
		if !ok {
			continue
		}
		if strings.Contains(strings.ToLower(artifact.Snippet), fragment) {
			return true
		}
	}
	return false
}

func failedHTTPStatusMatches(results []ExecutionResult, codes ...int) bool {
	for _, result := range results {
		if result.Status != ExecutionFailed {
			continue
		}
		status, ok := failedHTTPResponseStatus(result)
		if !ok {
			continue
		}
		for _, code := range codes {
			if status == code {
				return true
			}
		}
	}
	return false
}

func failedHTTPStatusRange(results []ExecutionResult, min, max int) bool {
	for _, result := range results {
		if result.Status != ExecutionFailed {
			continue
		}
		status, ok := failedHTTPResponseStatus(result)
		if ok && status >= min && status <= max {
			return true
		}
	}
	return false
}

func failedHTTPResponseStatus(result ExecutionResult) (int, bool) {
	artifact, ok := artifactByType(result.Artifacts, "http_response")
	if !ok {
		return 0, false
	}
	return statusFromResponseSnippet(artifact.Snippet)
}

func statusFromResponseSnippet(snippet string) (int, bool) {
	for _, part := range strings.Fields(snippet) {
		if !strings.HasPrefix(part, "status=") {
			continue
		}
		raw := strings.TrimPrefix(part, "status=")
		if raw == "" || raw == "unavailable" {
			return 0, false
		}
		status, err := strconv.Atoi(raw)
		if err != nil {
			return 0, false
		}
		return status, true
	}
	return 0, false
}
