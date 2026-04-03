package review

import (
	"fmt"
	"strings"
)

type GitHubPullRequestEvent struct {
	Action      string            `json:"action"`
	Number      int               `json:"number,omitempty"`
	Repository  GitHubRepository  `json:"repository"`
	PullRequest GitHubPullRequest `json:"pull_request"`
	Sender      GitHubUser        `json:"sender"`
}

type GitHubRepository struct {
	Name     string `json:"name"`
	FullName string `json:"full_name"`
	HTMLURL  string `json:"html_url"`
}

type GitHubPullRequest struct {
	Number  int                  `json:"number"`
	Title   string               `json:"title"`
	Body    string               `json:"body"`
	HTMLURL string               `json:"html_url"`
	DiffURL string               `json:"diff_url"`
	State   string               `json:"state"`
	Draft   bool                 `json:"draft"`
	User    GitHubUser           `json:"user"`
	Head    GitHubPullRequestRef `json:"head"`
	Base    GitHubPullRequestRef `json:"base"`
	Labels  []GitHubLabel        `json:"labels,omitempty"`
}

type GitHubUser struct {
	Login string `json:"login"`
}

type GitHubPullRequestRef struct {
	Ref string `json:"ref"`
	SHA string `json:"sha"`
}

type GitHubLabel struct {
	Name string `json:"name"`
}

func GitHubPullRequestReviewRequest(event GitHubPullRequestEvent) (CreateReviewRequest, bool, string, error) {
	if strings.TrimSpace(event.Repository.FullName) == "" {
		return CreateReviewRequest{}, false, "", fmt.Errorf("repository.full_name is required")
	}
	prNumber := event.PullRequest.Number
	if prNumber == 0 {
		prNumber = event.Number
	}
	if prNumber == 0 {
		return CreateReviewRequest{}, false, "", fmt.Errorf("pull_request.number is required")
	}
	if strings.TrimSpace(event.PullRequest.Head.SHA) == "" {
		return CreateReviewRequest{}, false, "", fmt.Errorf("pull_request.head.sha is required")
	}

	action := strings.TrimSpace(event.Action)
	if !supportsGitHubPullRequestAction(action) {
		return CreateReviewRequest{}, true, fmt.Sprintf("github pull_request action %q does not create a review", action), nil
	}

	repoFullName := strings.TrimSpace(event.Repository.FullName)
	repoName := firstNonEmpty(strings.TrimSpace(event.Repository.Name), repoFullName)
	sourceID := fmt.Sprintf("%s#%d", repoFullName, prNumber)
	environment := inferEnvironmentFromBaseRef(event.PullRequest.Base.Ref)

	return CreateReviewRequest{
		SourceType:  "pull_request",
		SourceID:    sourceID,
		Repo:        repoFullName,
		Service:     repoName,
		Environment: environment,
		DedupeKey:   strings.ToLower(fmt.Sprintf("github:%s:pr-%d:%s:%s", repoFullName, prNumber, event.PullRequest.Head.SHA, environment)),
		TriggeredBy: "github_webhook",
		Payload: ReviewPayload{
			Title:      event.PullRequest.Title,
			Author:     firstNonEmpty(strings.TrimSpace(event.PullRequest.User.Login), strings.TrimSpace(event.Sender.Login)),
			BaseCommit: strings.TrimSpace(event.PullRequest.Base.SHA),
			HeadCommit: strings.TrimSpace(event.PullRequest.Head.SHA),
			DiffURL:    firstNonEmpty(strings.TrimSpace(event.PullRequest.DiffURL), strings.TrimSpace(event.PullRequest.HTMLURL)),
			Metadata: map[string]any{
				"provider":             "github",
				"webhook_action":       action,
				"pull_request_number":  prNumber,
				"pull_request_url":     event.PullRequest.HTMLURL,
				"repository_url":       event.Repository.HTMLURL,
				"repository_full_name": repoFullName,
				"head_ref":             event.PullRequest.Head.Ref,
				"base_ref":             event.PullRequest.Base.Ref,
				"draft":                event.PullRequest.Draft,
				"labels":               githubLabelNames(event.PullRequest.Labels),
				"git": map[string]any{
					"provider":    "github",
					"diff_url":    firstNonEmpty(strings.TrimSpace(event.PullRequest.DiffURL), strings.TrimSpace(event.PullRequest.HTMLURL)),
					"repository":  repoFullName,
					"base_commit": strings.TrimSpace(event.PullRequest.Base.SHA),
					"head_commit": strings.TrimSpace(event.PullRequest.Head.SHA),
					"head_ref":    strings.TrimSpace(event.PullRequest.Head.Ref),
					"base_ref":    strings.TrimSpace(event.PullRequest.Base.Ref),
				},
			},
		},
	}, false, "", nil
}

func supportsGitHubPullRequestAction(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "opened", "reopened", "synchronize", "ready_for_review":
		return true
	default:
		return false
	}
}

func githubLabelNames(labels []GitHubLabel) []string {
	if len(labels) == 0 {
		return nil
	}
	names := make([]string, 0, len(labels))
	for _, label := range labels {
		if name := strings.TrimSpace(label.Name); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	return names
}

func inferEnvironmentFromBaseRef(baseRef string) string {
	switch strings.ToLower(strings.TrimSpace(baseRef)) {
	case "main", "master", "release":
		return "prod"
	case "develop", "development", "staging", "stage":
		return "staging"
	default:
		return "unknown"
	}
}
