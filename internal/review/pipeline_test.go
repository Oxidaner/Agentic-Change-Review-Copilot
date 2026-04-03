package review

import "testing"

func TestCollectExternalContextEvidenceRedactsUnexpectedFields(t *testing.T) {
	items := collectExternalContextEvidence(ChangeBundle{
		Metadata: map[string]any{
			"git": map[string]any{
				"provider":    "github",
				"repository":  "octo/gateway-service",
				"diff_url":    "https://github.com/octo/gateway-service/pull/42.diff",
				"head_commit": "def456",
				"token":       "secret-token",
				"headers":     map[string]any{"Authorization": "Bearer secret"},
			},
			"metrics": map[string]any{
				"summary":      "error rate stable",
				"window":       "15m",
				"raw_payload":  "{\"series\":[]}",
				"internal_url": "https://metrics.internal/query",
			},
			"runbook": map[string]any{
				"title":  "Gateway rollback",
				"url":    "https://runbooks.example.com/gateway/rollback",
				"secret": "should-not-leak",
			},
		},
	}, func(prefix string) string { return prefix + "_1" })

	if len(items) != 3 {
		t.Fatalf("item count = %d, want 3", len(items))
	}

	for _, item := range items {
		if _, ok := item.Metadata["token"]; ok {
			t.Fatalf("unexpected token leakage in metadata: %+v", item.Metadata)
		}
		if _, ok := item.Metadata["headers"]; ok {
			t.Fatalf("unexpected headers leakage in metadata: %+v", item.Metadata)
		}
		if _, ok := item.Metadata["raw_payload"]; ok {
			t.Fatalf("unexpected raw payload leakage in metadata: %+v", item.Metadata)
		}
		if _, ok := item.Metadata["internal_url"]; ok {
			t.Fatalf("unexpected internal url leakage in metadata: %+v", item.Metadata)
		}
		if _, ok := item.Metadata["secret"]; ok {
			t.Fatalf("unexpected secret leakage in metadata: %+v", item.Metadata)
		}
	}
}
