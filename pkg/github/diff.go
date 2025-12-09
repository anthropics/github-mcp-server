package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/github/github-mcp-server/pkg/translations"
	"github.com/google/go-github/v69/github"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// GetPullRequestDiff creates a tool to get the diff of a pull request.
func GetPullRequestDiff(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_pull_request_diff",
			mcp.WithDescription(t("TOOL_GET_PULL_REQUEST_DIFF_DESCRIPTION", "Get the diff of a pull request in unified diff format. This shows all changes made in the PR.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_PULL_REQUEST_DIFF_USER_TITLE", "Get pull request diff"),
				ReadOnlyHint: true,
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithNumber("pullNumber",
				mcp.Required(),
				mcp.Description("Pull request number"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			pullNumber, err := RequiredInt(request, "pullNumber")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Get the raw diff using the diff media type
			opts := github.RawOptions{Type: github.Diff}
			diff, resp, err := client.PullRequests.GetRaw(ctx, owner, repo, pullNumber, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to get pull request diff: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get pull request diff: %s", string(body))), nil
			}

			return mcp.NewToolResultText(diff), nil
		}
}

// GetCommitDiff creates a tool to get the diff of a specific commit.
func GetCommitDiff(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("get_commit_diff",
			mcp.WithDescription(t("TOOL_GET_COMMIT_DIFF_DESCRIPTION", "Get the diff of a specific commit in a GitHub repository. Shows changes introduced by the commit.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_GET_COMMIT_DIFF_USER_TITLE", "Get commit diff"),
				ReadOnlyHint: true,
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("sha",
				mcp.Required(),
				mcp.Description("Commit SHA"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			sha, err := requiredParam[string](request, "sha")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			// Use the compare API to get the diff for a single commit
			// by comparing the commit with its parent
			// First get the commit to find its parent
			commit, resp, err := client.Repositories.GetCommit(ctx, owner, repo, sha, nil)
			if err != nil {
				return nil, fmt.Errorf("failed to get commit: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to get commit: %s", string(body))), nil
			}

			// Build the diff from the files in the commit
			var diffContent string
			if commit.Files != nil {
				for _, file := range commit.Files {
					if file.Patch != nil {
						diffContent += fmt.Sprintf("diff --git a/%s b/%s\n", file.GetFilename(), file.GetFilename())
						diffContent += fmt.Sprintf("--- a/%s\n", file.GetFilename())
						diffContent += fmt.Sprintf("+++ b/%s\n", file.GetFilename())
						diffContent += *file.Patch + "\n"
					}
				}
			}

			if diffContent == "" {
				return mcp.NewToolResultText("No changes in this commit"), nil
			}

			return mcp.NewToolResultText(diffContent), nil
		}
}

// CompareCommits creates a tool to compare two commits, branches, or tags.
func CompareCommits(getClient GetClientFn, t translations.TranslationHelperFunc) (tool mcp.Tool, handler server.ToolHandlerFunc) {
	return mcp.NewTool("compare_commits",
			mcp.WithDescription(t("TOOL_COMPARE_COMMITS_DESCRIPTION", "Compare two commits, branches, or tags in a GitHub repository. Shows the diff and commits between the base and head references.")),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				Title:        t("TOOL_COMPARE_COMMITS_USER_TITLE", "Compare commits"),
				ReadOnlyHint: true,
			}),
			mcp.WithString("owner",
				mcp.Required(),
				mcp.Description("Repository owner"),
			),
			mcp.WithString("repo",
				mcp.Required(),
				mcp.Description("Repository name"),
			),
			mcp.WithString("base",
				mcp.Required(),
				mcp.Description("Base branch, tag, or commit SHA"),
			),
			mcp.WithString("head",
				mcp.Required(),
				mcp.Description("Head branch, tag, or commit SHA"),
			),
			mcp.WithNumber("page",
				mcp.Description("Page number for pagination (min 1)"),
				mcp.Min(1),
			),
			mcp.WithNumber("perPage",
				mcp.Description("Results per page for pagination (min 1, max 100)"),
				mcp.Min(1),
				mcp.Max(100),
			),
			mcp.WithString("format",
				mcp.Description("Output format: 'diff' for unified diff, 'json' for structured data with commits and files"),
				mcp.Enum("diff", "json"),
			),
		),
		func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			owner, err := requiredParam[string](request, "owner")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			repo, err := requiredParam[string](request, "repo")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			base, err := requiredParam[string](request, "base")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			head, err := requiredParam[string](request, "head")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			format, err := OptionalParam[string](request, "format")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			if format == "" {
				format = "diff"
			}

			page, err := OptionalIntParamWithDefault(request, "page", 1)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			perPage, err := OptionalIntParamWithDefault(request, "perPage", 30)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			client, err := getClient(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get GitHub client: %w", err)
			}

			opts := &github.ListOptions{
				Page:    page,
				PerPage: perPage,
			}

			comparison, resp, err := client.Repositories.CompareCommits(ctx, owner, repo, base, head, opts)
			if err != nil {
				return nil, fmt.Errorf("failed to compare commits: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != http.StatusOK {
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return nil, fmt.Errorf("failed to read response body: %w", err)
				}
				return mcp.NewToolResultError(fmt.Sprintf("failed to compare commits: %s", string(body))), nil
			}

			if format == "json" {
				r, err := json.Marshal(comparison)
				if err != nil {
					return nil, fmt.Errorf("failed to marshal response: %w", err)
				}
				return mcp.NewToolResultText(string(r)), nil
			}

			// Build unified diff format
			var diffContent string

			// Add a header with comparison summary
			diffContent += fmt.Sprintf("Comparing %s...%s\n", base, head)
			diffContent += fmt.Sprintf("Status: %s\n", comparison.GetStatus())
			diffContent += fmt.Sprintf("Ahead by: %d commits\n", comparison.GetAheadBy())
			diffContent += fmt.Sprintf("Behind by: %d commits\n", comparison.GetBehindBy())
			diffContent += fmt.Sprintf("Total commits: %d\n", comparison.GetTotalCommits())
			diffContent += "\n"

			// List commits
			if len(comparison.Commits) > 0 {
				diffContent += "Commits:\n"
				for _, commit := range comparison.Commits {
					msg := ""
					if commit.Commit != nil && commit.Commit.Message != nil {
						// Get first line of commit message
						msg = *commit.Commit.Message
						for i, c := range msg {
							if c == '\n' {
								msg = msg[:i]
								break
							}
						}
					}
					sha := commit.GetSHA()
					if len(sha) > 7 {
						sha = sha[:7]
					}
					diffContent += fmt.Sprintf("  %s %s\n", sha, msg)
				}
				diffContent += "\n"
			}

			// Build the diff from files
			if comparison.Files != nil {
				diffContent += "Files changed:\n"
				for _, file := range comparison.Files {
					diffContent += fmt.Sprintf("  %s: +%d -%d\n", file.GetFilename(), file.GetAdditions(), file.GetDeletions())
				}
				diffContent += "\n"

				diffContent += "Diff:\n"
				for _, file := range comparison.Files {
					if file.Patch != nil {
						diffContent += fmt.Sprintf("diff --git a/%s b/%s\n", file.GetFilename(), file.GetFilename())
						diffContent += fmt.Sprintf("--- a/%s\n", file.GetFilename())
						diffContent += fmt.Sprintf("+++ b/%s\n", file.GetFilename())
						diffContent += *file.Patch + "\n"
					}
				}
			}

			return mcp.NewToolResultText(diffContent), nil
		}
}
