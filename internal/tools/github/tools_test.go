package github

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/firebase/genkit/go/genkit"
)

// fakeClient implements Client with an in-memory map, enough to
// exercise every tool without calling the real GitHub API.
type fakeClient struct {
	issues       map[int]Issue
	pullRequests map[int]PullRequest
	nextComment  int64
	failWith     error
}

func newFakeClient() *fakeClient {
	return &fakeClient{issues: map[int]Issue{}, pullRequests: map[int]PullRequest{}}
}

var _ Client = (*fakeClient)(nil)

func (f *fakeClient) ListIssues(context.Context, string, string, string) ([]Issue, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	var out []Issue
	for _, i := range f.issues {
		out = append(out, i)
	}
	return out, nil
}

func (f *fakeClient) GetIssue(_ context.Context, _, _ string, number int) (Issue, error) {
	if f.failWith != nil {
		return Issue{}, f.failWith
	}
	issue, ok := f.issues[number]
	if !ok {
		return Issue{}, errors.New("not found")
	}
	return issue, nil
}

func (f *fakeClient) CreateIssue(_ context.Context, _, _, title, body string) (Issue, error) {
	if f.failWith != nil {
		return Issue{}, f.failWith
	}
	number := len(f.issues) + 1
	issue := Issue{Number: number, Title: title, Body: body, State: "open"}
	f.issues[number] = issue
	return issue, nil
}

func (f *fakeClient) CommentOnIssue(_ context.Context, _, _ string, _ int, body string) (Comment, error) {
	if f.failWith != nil {
		return Comment{}, f.failWith
	}
	f.nextComment++
	return Comment{ID: f.nextComment, Body: body}, nil
}

func (f *fakeClient) ListPullRequests(context.Context, string, string, string) ([]PullRequest, error) {
	if f.failWith != nil {
		return nil, f.failWith
	}
	var out []PullRequest
	for _, pr := range f.pullRequests {
		out = append(out, pr)
	}
	return out, nil
}

func (f *fakeClient) CreatePullRequest(_ context.Context, _, _, title, head, base, body string) (PullRequest, error) {
	if f.failWith != nil {
		return PullRequest{}, f.failWith
	}
	number := len(f.pullRequests) + 1
	pr := PullRequest{Number: number, Title: title, Head: head, Base: base, Body: body, State: "open"}
	f.pullRequests[number] = pr
	return pr, nil
}

func (f *fakeClient) MergePullRequest(_ context.Context, _, _ string, number int, _ string) (MergeResult, error) {
	if f.failWith != nil {
		return MergeResult{}, f.failWith
	}
	pr, ok := f.pullRequests[number]
	if !ok {
		return MergeResult{}, errors.New("not found")
	}
	pr.Merged = true
	f.pullRequests[number] = pr
	return MergeResult{SHA: "deadbeef", Merged: true}, nil
}

func newTestTools(client *fakeClient) Tools {
	g := genkit.Init(context.Background())
	return DefineTools(g, Deps{Client: client})
}

// runTool calls a tool exactly like an external MCP client would — JSON
// in, JSON out — then decodes the result into Out for assertions.
// Mirrors internal/mcp/server/tools_test.go's helper of the same name.
func runTool[Out any](t *testing.T, tool interface {
	RunRaw(context.Context, any) (any, error)
}, input any) Out {
	t.Helper()
	var zero Out

	raw, err := tool.RunRaw(context.Background(), input)
	if err != nil {
		t.Fatalf("RunRaw: %v", err)
		return zero
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("marshal tool output: %v", err)
		return zero
	}
	var out Out
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal tool output: %v", err)
		return zero
	}
	return out
}

func TestCreateIssue_ThenGetIssue(t *testing.T) {
	client := newFakeClient()
	tools := newTestTools(client)

	created := runTool[issueView](t, tools.CreateIssue, createIssueInput{
		repoRef: repoRef{Owner: "acme", Repo: "widgets"}, Title: "bug", Body: "it's broken",
	})
	if created.Number != 1 || created.Title != "bug" {
		t.Fatalf("unexpected created issue: %+v", created)
	}

	got := runTool[issueView](t, tools.GetIssue, getIssueInput{
		repoRef: repoRef{Owner: "acme", Repo: "widgets"}, Number: 1,
	})
	if got.Title != "bug" || got.Body != "it's broken" {
		t.Fatalf("unexpected fetched issue: %+v", got)
	}
}

func TestListIssues_ReturnsAll(t *testing.T) {
	client := newFakeClient()
	client.issues[1] = Issue{Number: 1, Title: "a"}
	client.issues[2] = Issue{Number: 2, Title: "b"}
	tools := newTestTools(client)

	out := runTool[listIssuesOutput](t, tools.ListIssues, listIssuesInput{
		repoRef: repoRef{Owner: "acme", Repo: "widgets"},
	})
	if len(out.Issues) != 2 {
		t.Fatalf("expected 2 issues, got %d", len(out.Issues))
	}
}

func TestCommentOnIssue_AssignsID(t *testing.T) {
	client := newFakeClient()
	tools := newTestTools(client)

	comment := runTool[commentView](t, tools.CommentOnIssue, commentOnIssueInput{
		repoRef: repoRef{Owner: "acme", Repo: "widgets"}, Number: 1, Body: "looks good",
	})
	if comment.ID == 0 || comment.Body != "looks good" {
		t.Fatalf("unexpected comment: %+v", comment)
	}
}

func TestCreatePullRequest_ThenMerge(t *testing.T) {
	client := newFakeClient()
	tools := newTestTools(client)

	pr := runTool[pullRequestView](t, tools.CreatePullRequest, createPullRequestInput{
		repoRef: repoRef{Owner: "acme", Repo: "widgets"}, Title: "fix bug", Head: "fix-branch", Base: "main",
	})
	if pr.Number != 1 || pr.Merged {
		t.Fatalf("unexpected created PR: %+v", pr)
	}

	result := runTool[mergeResultView](t, tools.MergePullRequest, mergePullRequestInput{
		repoRef: repoRef{Owner: "acme", Repo: "widgets"}, Number: 1,
	})
	if !result.Merged || result.SHA == "" {
		t.Fatalf("unexpected merge result: %+v", result)
	}
}

func TestListPullRequests_ReturnsAll(t *testing.T) {
	client := newFakeClient()
	client.pullRequests[1] = PullRequest{Number: 1, Title: "a"}
	tools := newTestTools(client)

	out := runTool[listPullRequestsOutput](t, tools.ListPullRequests, listPullRequestsInput{
		repoRef: repoRef{Owner: "acme", Repo: "widgets"},
	})
	if len(out.PullRequests) != 1 {
		t.Fatalf("expected 1 pull request, got %d", len(out.PullRequests))
	}
}

// TestToolErr_DoesNotLeakInnerError mirrors the same sanitization
// contract established in internal/mcp/server/tools_test.go — this
// package registers tools the same way, so it must not leak a
// GitHub API error's raw body to an external caller either.
func TestToolErr_DoesNotLeakInnerError(t *testing.T) {
	inner := errors.New("GET https://api.github.com/repos/acme/widgets/issues/1: 404 Not Found []")
	got := toolErr("github.get_issue", inner).Error()
	if strings.Contains(got, "api.github.com") || strings.Contains(got, "404") {
		t.Fatalf("inner error leaked to caller: %q", got)
	}
}

func TestCreateIssue_PropagatesSanitizedError(t *testing.T) {
	client := newFakeClient()
	client.failWith = errors.New("GET https://api.github.com/repos/acme/widgets: 500 Internal Server Error")
	tools := newTestTools(client)

	_, err := tools.CreateIssue.RunRaw(context.Background(), createIssueInput{
		repoRef: repoRef{Owner: "acme", Repo: "widgets"}, Title: "bug",
	})
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "api.github.com") {
		t.Fatalf("inner error leaked through tool call: %q", err.Error())
	}
}
