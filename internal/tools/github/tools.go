// This file registers Client as Genkit tools so an agent (or the MCP
// server, if these are ever exposed the same way as
// internal/mcp/server's Second Brain tools) can read/write GitHub
// issues and pull requests.
package github

import (
	"fmt"
	"log/slog"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

// Deps is what this package needs to back the GitHub tools —
// Client is an interface, never the concrete go-github wrapper
// directly, so tests substitute a fake.
type Deps struct {
	Client Client
}

// Tools holds every registered tool, so callers (tests, in
// particular) can invoke one directly without going through a
// model's tool-call loop.
type Tools struct {
	ListIssues        *ai.ToolDef[listIssuesInput, listIssuesOutput]
	GetIssue          *ai.ToolDef[getIssueInput, issueView]
	CreateIssue       *ai.ToolDef[createIssueInput, issueView]
	CommentOnIssue    *ai.ToolDef[commentOnIssueInput, commentView]
	ListPullRequests  *ai.ToolDef[listPullRequestsInput, listPullRequestsOutput]
	CreatePullRequest *ai.ToolDef[createPullRequestInput, pullRequestView]
	MergePullRequest  *ai.ToolDef[mergePullRequestInput, mergeResultView]
}

// toolErr logs the real error server-side and returns a fixed,
// opaque message. Genkit's MCP server plugin (verified in
// plugins/mcp/server.go v1.10.0) forwards a tool's error text
// verbatim to an external caller — GitHub API error bodies must
// never reach that path unfiltered.
func toolErr(tool string, err error) error {
	slog.Error("github tool failed", "tool", tool, "err", err)
	return fmt.Errorf("%s: internal error", tool)
}

type repoRef struct {
	Owner string `json:"owner" jsonschema:"description=Repository owner (user or org)"`
	Repo  string `json:"repo" jsonschema:"description=Repository name"`
}

type issueView struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	HTMLURL string `json:"htmlUrl"`
}

func newIssueView(i Issue) issueView {
	return issueView{Number: i.Number, Title: i.Title, Body: i.Body, State: i.State, HTMLURL: i.HTMLURL}
}

type commentView struct {
	ID      int64  `json:"id"`
	Body    string `json:"body"`
	HTMLURL string `json:"htmlUrl"`
}

type pullRequestView struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	Body    string `json:"body"`
	State   string `json:"state"`
	Head    string `json:"head"`
	Base    string `json:"base"`
	Merged  bool   `json:"merged"`
	HTMLURL string `json:"htmlUrl"`
}

func newPullRequestView(pr PullRequest) pullRequestView {
	return pullRequestView{
		Number: pr.Number, Title: pr.Title, Body: pr.Body, State: pr.State,
		Head: pr.Head, Base: pr.Base, Merged: pr.Merged, HTMLURL: pr.HTMLURL,
	}
}

type mergeResultView struct {
	SHA     string `json:"sha"`
	Merged  bool   `json:"merged"`
	Message string `json:"message"`
}

type listIssuesInput struct {
	repoRef
	State string `json:"state,omitempty" jsonschema:"description=open, closed, or all (defaults to open)"`
}

type listIssuesOutput struct {
	Issues []issueView `json:"issues"`
}

type getIssueInput struct {
	repoRef
	Number int `json:"number" jsonschema:"description=Issue number"`
}

type createIssueInput struct {
	repoRef
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
}

type commentOnIssueInput struct {
	repoRef
	Number int    `json:"number" jsonschema:"description=Issue number to comment on"`
	Body   string `json:"body"`
}

type listPullRequestsInput struct {
	repoRef
	State string `json:"state,omitempty" jsonschema:"description=open, closed, or all (defaults to open)"`
}

type listPullRequestsOutput struct {
	PullRequests []pullRequestView `json:"pullRequests"`
}

type createPullRequestInput struct {
	repoRef
	Title string `json:"title"`
	Head  string `json:"head" jsonschema:"description=Branch containing the changes"`
	Base  string `json:"base" jsonschema:"description=Branch the changes merge into"`
	Body  string `json:"body,omitempty"`
}

type mergePullRequestInput struct {
	repoRef
	Number        int    `json:"number" jsonschema:"description=Pull request number to merge"`
	CommitMessage string `json:"commitMessage,omitempty" jsonschema:"description=Extra detail appended to the automatic merge commit message"`
}

// DefineTools registers the GitHub tools on g, backed by deps, and
// returns them for direct invocation (tests) alongside normal model
// tool-calling.
func DefineTools(g *genkit.Genkit, deps Deps) Tools {
	return Tools{
		ListIssues: genkit.DefineTool(g, "github.list_issues",
			"Lists issues (not pull requests) in a GitHub repository.",
			func(ctx *ai.ToolContext, in listIssuesInput) (listIssuesOutput, error) {
				issues, err := deps.Client.ListIssues(ctx, in.Owner, in.Repo, in.State)
				if err != nil {
					return listIssuesOutput{}, toolErr("github.list_issues", err)
				}
				out := make([]issueView, len(issues))
				for i, iss := range issues {
					out[i] = newIssueView(iss)
				}
				return listIssuesOutput{Issues: out}, nil
			},
		),

		GetIssue: genkit.DefineTool(g, "github.get_issue",
			"Returns a single GitHub issue by number.",
			func(ctx *ai.ToolContext, in getIssueInput) (issueView, error) {
				issue, err := deps.Client.GetIssue(ctx, in.Owner, in.Repo, in.Number)
				if err != nil {
					return issueView{}, toolErr("github.get_issue", err)
				}
				return newIssueView(issue), nil
			},
		),

		CreateIssue: genkit.DefineTool(g, "github.create_issue",
			"Creates a new issue in a GitHub repository.",
			func(ctx *ai.ToolContext, in createIssueInput) (issueView, error) {
				issue, err := deps.Client.CreateIssue(ctx, in.Owner, in.Repo, in.Title, in.Body)
				if err != nil {
					return issueView{}, toolErr("github.create_issue", err)
				}
				return newIssueView(issue), nil
			},
		),

		CommentOnIssue: genkit.DefineTool(g, "github.comment_on_issue",
			"Adds a comment to an existing GitHub issue.",
			func(ctx *ai.ToolContext, in commentOnIssueInput) (commentView, error) {
				comment, err := deps.Client.CommentOnIssue(ctx, in.Owner, in.Repo, in.Number, in.Body)
				if err != nil {
					return commentView{}, toolErr("github.comment_on_issue", err)
				}
				return commentView{ID: comment.ID, Body: comment.Body, HTMLURL: comment.HTMLURL}, nil
			},
		),

		ListPullRequests: genkit.DefineTool(g, "github.list_pull_requests",
			"Lists pull requests in a GitHub repository.",
			func(ctx *ai.ToolContext, in listPullRequestsInput) (listPullRequestsOutput, error) {
				prs, err := deps.Client.ListPullRequests(ctx, in.Owner, in.Repo, in.State)
				if err != nil {
					return listPullRequestsOutput{}, toolErr("github.list_pull_requests", err)
				}
				out := make([]pullRequestView, len(prs))
				for i, pr := range prs {
					out[i] = newPullRequestView(pr)
				}
				return listPullRequestsOutput{PullRequests: out}, nil
			},
		),

		CreatePullRequest: genkit.DefineTool(g, "github.create_pull_request",
			"Creates a new pull request in a GitHub repository.",
			func(ctx *ai.ToolContext, in createPullRequestInput) (pullRequestView, error) {
				pr, err := deps.Client.CreatePullRequest(ctx, in.Owner, in.Repo, in.Title, in.Head, in.Base, in.Body)
				if err != nil {
					return pullRequestView{}, toolErr("github.create_pull_request", err)
				}
				return newPullRequestView(pr), nil
			},
		),

		MergePullRequest: genkit.DefineTool(g, "github.merge_pull_request",
			"Merges an existing pull request in a GitHub repository.",
			func(ctx *ai.ToolContext, in mergePullRequestInput) (mergeResultView, error) {
				result, err := deps.Client.MergePullRequest(ctx, in.Owner, in.Repo, in.Number, in.CommitMessage)
				if err != nil {
					return mergeResultView{}, toolErr("github.merge_pull_request", err)
				}
				return mergeResultView{SHA: result.SHA, Merged: result.Merged, Message: result.Message}, nil
			},
		),
	}
}
