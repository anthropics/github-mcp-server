package github

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// GetReposPullsDiffByOwnerByRepoByPullNumber is a custom endpoint pattern for getting PR diffs
var GetReposPullsDiffByOwnerByRepoByPullNumber = mock.EndpointPattern{
	Pattern: "/repos/{owner}/{repo}/pulls/{pull_number}",
	Method:  "GET",
}

// GetReposCompareByOwnerByRepoByBasehead is a custom endpoint pattern for comparing commits
var GetReposCompareByOwnerByRepoByBasehead = mock.EndpointPattern{
	Pattern: "/repos/{owner}/{repo}/compare/{basehead}",
	Method:  "GET",
}

func Test_GetPullRequestDiff(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetPullRequestDiff(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_pull_request_diff", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "pullNumber")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "pullNumber"})

	// Test the diff output
	mockDiff := `diff --git a/file.txt b/file.txt
index abc1234..def5678 100644
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,4 @@
 line 1
+added line
 line 2
 line 3
`

	tests := []struct {
		name           string
		mockedClient   *http.Client
		requestArgs    map[string]interface{}
		expectError    bool
		expectedDiff   string
		expectedErrMsg string
	}{
		{
			name: "successful PR diff fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetReposPullsDiffByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						// Check Accept header for diff media type
						w.Header().Set("Content-Type", "text/plain")
						w.WriteHeader(http.StatusOK)
						_, _ = w.Write([]byte(mockDiff))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(42),
			},
			expectError:  false,
			expectedDiff: mockDiff,
		},
		{
			name: "PR diff fetch fails - not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetReposPullsDiffByOwnerByRepoByPullNumber,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner":      "owner",
				"repo":       "repo",
				"pullNumber": float64(999),
			},
			expectError:    true,
			expectedErrMsg: "failed to get pull request diff",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := GetPullRequestDiff(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			assert.Equal(t, tc.expectedDiff, textContent.Text)
		})
	}
}

func Test_GetCommitDiff(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := GetCommitDiff(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "get_commit_diff", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "sha")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "sha"})

	// Test with commit that has files with patches
	patchContent := "@@ -1,3 +1,4 @@\n line 1\n+added line\n line 2\n line 3"
	mockCommit := &github.RepositoryCommit{
		SHA: github.Ptr("abc123def456"),
		Commit: &github.Commit{
			Message: github.Ptr("Test commit message"),
		},
		Files: []*github.CommitFile{
			{
				SHA:       github.Ptr("file123"),
				Filename:  github.Ptr("file.txt"),
				Status:    github.Ptr("modified"),
				Additions: github.Ptr(1),
				Deletions: github.Ptr(0),
				Changes:   github.Ptr(1),
				Patch:     github.Ptr(patchContent),
			},
		},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedContain string
		expectedErrMsg  string
	}{
		{
			name: "successful commit diff fetch",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposCommitsByOwnerByRepoByRef,
					mockCommit,
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"sha":   "abc123def456",
			},
			expectError:     false,
			expectedContain: "file.txt",
		},
		{
			name: "commit diff fetch fails - not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					mock.GetReposCommitsByOwnerByRepoByRef,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"sha":   "nonexistent",
			},
			expectError:    true,
			expectedErrMsg: "failed to get commit",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := GetCommitDiff(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			assert.Contains(t, textContent.Text, tc.expectedContain)
		})
	}
}

func Test_CompareCommits(t *testing.T) {
	// Verify tool definition once
	mockClient := github.NewClient(nil)
	tool, _ := CompareCommits(stubGetClientFn(mockClient), translations.NullTranslationHelper)

	assert.Equal(t, "compare_commits", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Contains(t, tool.InputSchema.Properties, "owner")
	assert.Contains(t, tool.InputSchema.Properties, "repo")
	assert.Contains(t, tool.InputSchema.Properties, "base")
	assert.Contains(t, tool.InputSchema.Properties, "head")
	assert.Contains(t, tool.InputSchema.Properties, "page")
	assert.Contains(t, tool.InputSchema.Properties, "perPage")
	assert.Contains(t, tool.InputSchema.Properties, "format")
	assert.ElementsMatch(t, tool.InputSchema.Required, []string{"owner", "repo", "base", "head"})

	patchContent := "@@ -1,3 +1,4 @@\n line 1\n+added line\n line 2\n line 3"
	mockComparison := &github.CommitsComparison{
		Status:       github.Ptr("ahead"),
		AheadBy:      github.Ptr(2),
		BehindBy:     github.Ptr(0),
		TotalCommits: github.Ptr(2),
		Commits: []*github.RepositoryCommit{
			{
				SHA: github.Ptr("abc123"),
				Commit: &github.Commit{
					Message: github.Ptr("First commit"),
				},
			},
			{
				SHA: github.Ptr("def456"),
				Commit: &github.Commit{
					Message: github.Ptr("Second commit"),
				},
			},
		},
		Files: []*github.CommitFile{
			{
				Filename:  github.Ptr("file.txt"),
				Status:    github.Ptr("modified"),
				Additions: github.Ptr(1),
				Deletions: github.Ptr(0),
				Changes:   github.Ptr(1),
				Patch:     github.Ptr(patchContent),
			},
		},
	}

	tests := []struct {
		name            string
		mockedClient    *http.Client
		requestArgs     map[string]interface{}
		expectError     bool
		expectedContain string
		expectedErrMsg  string
	}{
		{
			name: "successful comparison - diff format",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					GetReposCompareByOwnerByRepoByBasehead,
					mockComparison,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"base":   "main",
				"head":   "feature",
				"format": "diff",
			},
			expectError:     false,
			expectedContain: "file.txt",
		},
		{
			name: "successful comparison - json format",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					GetReposCompareByOwnerByRepoByBasehead,
					mockComparison,
				),
			),
			requestArgs: map[string]interface{}{
				"owner":  "owner",
				"repo":   "repo",
				"base":   "main",
				"head":   "feature",
				"format": "json",
			},
			expectError:     false,
			expectedContain: `"status":"ahead"`,
		},
		{
			name: "successful comparison - default format",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					GetReposCompareByOwnerByRepoByBasehead,
					mockComparison,
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"base":  "main",
				"head":  "feature",
			},
			expectError:     false,
			expectedContain: "Comparing main...feature",
		},
		{
			name: "comparison fails - not found",
			mockedClient: mock.NewMockedHTTPClient(
				mock.WithRequestMatchHandler(
					GetReposCompareByOwnerByRepoByBasehead,
					http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
						w.WriteHeader(http.StatusNotFound)
						_, _ = w.Write([]byte(`{"message": "Not Found"}`))
					}),
				),
			),
			requestArgs: map[string]interface{}{
				"owner": "owner",
				"repo":  "repo",
				"base":  "nonexistent",
				"head":  "feature",
			},
			expectError:    true,
			expectedErrMsg: "failed to compare commits",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := github.NewClient(tc.mockedClient)
			_, handler := CompareCommits(stubGetClientFn(client), translations.NullTranslationHelper)

			request := createMCPRequest(tc.requestArgs)
			result, err := handler(context.Background(), request)

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErrMsg)
				return
			}

			require.NoError(t, err)
			textContent := getTextResult(t, result)
			assert.Contains(t, textContent.Text, tc.expectedContain)
		})
	}
}

func Test_CompareCommits_JSONOutput(t *testing.T) {
	patchContent := "@@ -1 +1 @@\n-old\n+new"
	mockComparison := &github.CommitsComparison{
		Status:       github.Ptr("ahead"),
		AheadBy:      github.Ptr(1),
		BehindBy:     github.Ptr(0),
		TotalCommits: github.Ptr(1),
		Commits: []*github.RepositoryCommit{
			{
				SHA: github.Ptr("abc123"),
				Commit: &github.Commit{
					Message: github.Ptr("Test commit"),
				},
			},
		},
		Files: []*github.CommitFile{
			{
				Filename:  github.Ptr("test.txt"),
				Additions: github.Ptr(1),
				Deletions: github.Ptr(1),
				Patch:     github.Ptr(patchContent),
			},
		},
	}

	mockedClient := mock.NewMockedHTTPClient(
		mock.WithRequestMatch(
			GetReposCompareByOwnerByRepoByBasehead,
			mockComparison,
		),
	)

	client := github.NewClient(mockedClient)
	_, handler := CompareCommits(stubGetClientFn(client), translations.NullTranslationHelper)

	request := createMCPRequest(map[string]interface{}{
		"owner":  "owner",
		"repo":   "repo",
		"base":   "main",
		"head":   "feature",
		"format": "json",
	})

	result, err := handler(context.Background(), request)
	require.NoError(t, err)

	textContent := getTextResult(t, result)

	// Verify JSON structure
	var comparison github.CommitsComparison
	err = json.Unmarshal([]byte(textContent.Text), &comparison)
	require.NoError(t, err)

	assert.Equal(t, "ahead", *comparison.Status)
	assert.Equal(t, 1, *comparison.AheadBy)
	assert.Equal(t, 1, *comparison.TotalCommits)
	require.Len(t, comparison.Commits, 1)
	require.Len(t, comparison.Files, 1)
	assert.Equal(t, "test.txt", *comparison.Files[0].Filename)
}
