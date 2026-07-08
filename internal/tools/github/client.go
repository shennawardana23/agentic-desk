// Package github wraps google/go-github behind a minimal, testable
// interface — the domain layer for the GitHub tools registered on
// Genkit in tools.go. This file never imports genkit/ai, mirroring
// internal/secondbrain's separation of domain from framework wiring.
package github

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/go-github/v72/github"
)

// Issue is the subset of a GitHub issue these tools expose.
type Issue struct {
	Number  int
	Title   string
	Body    string
	State   string
	HTMLURL string
}

// Comment is a single issue comment.
type Comment struct {
	ID      int64
	Body    string
	HTMLURL string
}

// PullRequest is the subset of a GitHub pull request these tools
// expose.
type PullRequest struct {
	Number  int
	Title   string
	Body    string
	State   string
	Head    string
	Base    string
	Merged  bool
	HTMLURL string
}

// MergeResult reports the outcome of a merge attempt.
type MergeResult struct {
	SHA     string
	Merged  bool
	Message string
}

// Client is the subset of GitHub operations these tools need — an
// interface so tests substitute a fake instead of calling the real
// API. realClient (wrapping go-github) is the only production
// implementation.
type Client interface {
	ListIssues(ctx context.Context, owner, repo, state string) ([]Issue, error)
	GetIssue(ctx context.Context, owner, repo string, number int) (Issue, error)
	CreateIssue(ctx context.Context, owner, repo, title, body string) (Issue, error)
	CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) (Comment, error)
	ListPullRequests(ctx context.Context, owner, repo, state string) ([]PullRequest, error)
	CreatePullRequest(ctx context.Context, owner, repo, title, head, base, body string) (PullRequest, error)
	MergePullRequest(ctx context.Context, owner, repo string, number int, commitMessage string) (MergeResult, error)
}

// realClient wraps *github.Client. Use NewClient to construct one.
type realClient struct {
	gh *github.Client
}

var _ Client = (*realClient)(nil)

// NewClient returns a Client authenticated with a GitHub personal
// access token. token must be non-empty — this package never falls
// back to unauthenticated, rate-limited-to-60-req/hour requests
// silently.
func NewClient(token string) (Client, error) {
	if token == "" {
		return nil, errors.New("github: token must not be empty")
	}
	return &realClient{gh: github.NewClient(nil).WithAuthToken(token)}, nil
}

func toIssue(i *github.Issue) Issue {
	return Issue{
		Number:  i.GetNumber(),
		Title:   i.GetTitle(),
		Body:    i.GetBody(),
		State:   i.GetState(),
		HTMLURL: i.GetHTMLURL(),
	}
}

func (c *realClient) ListIssues(ctx context.Context, owner, repo, state string) ([]Issue, error) {
	if state == "" {
		state = "open"
	}
	issues, _, err := c.gh.Issues.ListByRepo(ctx, owner, repo, &github.IssueListByRepoOptions{State: state})
	if err != nil {
		return nil, fmt.Errorf("list issues: %w", err)
	}
	out := make([]Issue, 0, len(issues))
	for _, i := range issues {
		// GitHub's REST API returns pull requests from this same
		// endpoint (a PR is an issue under the hood) — skip them here
		// since ListPullRequests is the dedicated path for PRs.
		if i.IsPullRequest() {
			continue
		}
		out = append(out, toIssue(i))
	}
	return out, nil
}

func (c *realClient) GetIssue(ctx context.Context, owner, repo string, number int) (Issue, error) {
	issue, _, err := c.gh.Issues.Get(ctx, owner, repo, number)
	if err != nil {
		return Issue{}, fmt.Errorf("get issue: %w", err)
	}
	return toIssue(issue), nil
}

func (c *realClient) CreateIssue(ctx context.Context, owner, repo, title, body string) (Issue, error) {
	issue, _, err := c.gh.Issues.Create(ctx, owner, repo, &github.IssueRequest{
		Title: github.Ptr(title),
		Body:  github.Ptr(body),
	})
	if err != nil {
		return Issue{}, fmt.Errorf("create issue: %w", err)
	}
	return toIssue(issue), nil
}

func (c *realClient) CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) (Comment, error) {
	comment, _, err := c.gh.Issues.CreateComment(ctx, owner, repo, number, &github.IssueComment{Body: github.Ptr(body)})
	if err != nil {
		return Comment{}, fmt.Errorf("comment on issue: %w", err)
	}
	return Comment{ID: comment.GetID(), Body: comment.GetBody(), HTMLURL: comment.GetHTMLURL()}, nil
}

func toPullRequest(pr *github.PullRequest) PullRequest {
	return PullRequest{
		Number:  pr.GetNumber(),
		Title:   pr.GetTitle(),
		Body:    pr.GetBody(),
		State:   pr.GetState(),
		Head:    pr.GetHead().GetRef(),
		Base:    pr.GetBase().GetRef(),
		Merged:  pr.GetMerged(),
		HTMLURL: pr.GetHTMLURL(),
	}
}

func (c *realClient) ListPullRequests(ctx context.Context, owner, repo, state string) ([]PullRequest, error) {
	if state == "" {
		state = "open"
	}
	prs, _, err := c.gh.PullRequests.List(ctx, owner, repo, &github.PullRequestListOptions{State: state})
	if err != nil {
		return nil, fmt.Errorf("list pull requests: %w", err)
	}
	out := make([]PullRequest, len(prs))
	for i, pr := range prs {
		out[i] = toPullRequest(pr)
	}
	return out, nil
}

func (c *realClient) CreatePullRequest(ctx context.Context, owner, repo, title, head, base, body string) (PullRequest, error) {
	pr, _, err := c.gh.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: github.Ptr(title),
		Head:  github.Ptr(head),
		Base:  github.Ptr(base),
		Body:  github.Ptr(body),
	})
	if err != nil {
		return PullRequest{}, fmt.Errorf("create pull request: %w", err)
	}
	return toPullRequest(pr), nil
}

func (c *realClient) MergePullRequest(ctx context.Context, owner, repo string, number int, commitMessage string) (MergeResult, error) {
	result, _, err := c.gh.PullRequests.Merge(ctx, owner, repo, number, commitMessage, nil)
	if err != nil {
		return MergeResult{}, fmt.Errorf("merge pull request: %w", err)
	}
	return MergeResult{SHA: result.GetSHA(), Merged: result.GetMerged(), Message: result.GetMessage()}, nil
}
