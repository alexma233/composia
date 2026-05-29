package repo

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ErrNoGitChanges = errors.New("no git changes")

const gitCommandTimeout = 2 * time.Minute

type CommitSummary struct {
	CommitID    string
	Subject     string
	CommittedAt string
}

func ValidateWorkingTree(repoDir string) error {
	stat, err := os.Stat(repoDir)
	if err != nil {
		return fmt.Errorf("check repo_dir %q: %w", repoDir, err)
	}
	if !stat.IsDir() {
		return fmt.Errorf("repo_dir %q must be a directory", repoDir)
	}

	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "rev-parse", "--is-inside-work-tree") //nolint:gosec
	output, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("repo_dir %q git working tree check timed out: %w", repoDir, ctx.Err())
		}
		var stderr bytes.Buffer
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			stderr.Write(exitErr.Stderr)
		}
		return fmt.Errorf("repo_dir %q must be a git working tree: %w %s", repoDir, err, strings.TrimSpace(stderr.String()))
	}
	if strings.TrimSpace(string(output)) != "true" {
		return fmt.Errorf("repo_dir %q must be a git working tree", repoDir)
	}
	return nil
}

func CurrentRevision(repoDir string) (string, error) {
	output, err := gitOutput(repoDir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("read git HEAD for %q: %w", repoDir, err)
	}
	return strings.TrimSpace(output), nil
}

func CurrentBranch(repoDir string) (string, error) {
	output, err := gitOutput(repoDir, "branch", "--show-current")
	if err != nil {
		return "", fmt.Errorf("read git branch for %q: %w", repoDir, err)
	}
	return strings.TrimSpace(output), nil
}

func HasRemote(repoDir string) (bool, error) {
	output, err := gitOutput(repoDir, "remote")
	if err != nil {
		return false, fmt.Errorf("read git remotes for %q: %w", repoDir, err)
	}
	return strings.TrimSpace(output) != "", nil
}

func IsCleanWorkingTree(repoDir string) (bool, error) {
	output, err := gitOutput(repoDir, "status", "--short")
	if err != nil {
		return false, fmt.Errorf("read git worktree status for %q: %w", repoDir, err)
	}
	return strings.TrimSpace(output) == "", nil
}

func ListCommits(repoDir, cursor string, limit uint32) ([]CommitSummary, string, error) {
	if limit == 0 {
		limit = 100
	}
	pageSize := int(limit) + 1
	args := []string{"log", "--format=%H%x00%s%x00%cI", "-n", strconv.Itoa(pageSize)}
	if cursor != "" {
		args = append(args, "--skip=1", cursor)
	}

	output, err := gitOutput(repoDir, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list git commits for %q: %w", repoDir, err)
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	commits := make([]CommitSummary, 0, limit)
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if len(commits) == pageSize {
			break
		}
		parts := strings.Split(line, "\x00")
		if len(parts) != 3 {
			return nil, "", fmt.Errorf("unexpected git log line %q", line)
		}
		commits = append(commits, CommitSummary{
			CommitID:    parts[0],
			Subject:     parts[1],
			CommittedAt: parts[2],
		})
	}

	nextCursor := ""
	if len(commits) > int(limit) {
		commits = commits[:limit]
		nextCursor = commits[len(commits)-1].CommitID
	}
	return commits, nextCursor, nil
}

func HasPathChanges(repoDir, relativePath string) (bool, error) {
	return HasPathsChanges(repoDir, relativePath)
}

func HasPathsChanges(repoDir string, relativePaths ...string) (bool, error) {
	args := []string{"status", "--short"}
	if len(relativePaths) > 0 {
		args = append(args, "--")
		args = append(args, relativePaths...)
	}
	output, err := gitOutput(repoDir, args...)
	if err != nil {
		return false, fmt.Errorf("read git status for %q: %w", strings.Join(relativePaths, ", "), err)
	}
	return strings.TrimSpace(output) != "", nil
}

func CommitPath(repoDir, relativePath, message, authorName, authorEmail string) (string, error) {
	return CommitPaths(repoDir, []string{relativePath}, message, authorName, authorEmail)
}

func CommitPaths(repoDir string, relativePaths []string, message, authorName, authorEmail string) (string, error) {
	if len(relativePaths) == 0 {
		return "", errors.New("at least one repo path is required")
	}
	addArgs := make([]string, 0, 3+len(relativePaths))
	addArgs = append(addArgs, "add", "-A", "--")
	addArgs = append(addArgs, relativePaths...)
	if _, err := gitOutput(repoDir, addArgs...); err != nil {
		return "", fmt.Errorf("stage repo paths %q: %w", strings.Join(relativePaths, ", "), err)
	}
	changed, err := HasPathsChanges(repoDir, relativePaths...)
	if err != nil {
		return "", err
	}
	if !changed {
		return "", ErrNoGitChanges
	}
	if message == "" {
		message = "update " + relativePaths[0]
	}
	if err := gitCommandWithOptions(repoDir, gitAuthorEnv(authorName, authorEmail), []string{"commit.gpgsign=false"}, "commit", "-m", message); err != nil {
		return "", fmt.Errorf("commit repo paths %q: %w", strings.Join(relativePaths, ", "), err)
	}
	commitID, err := CurrentRevision(repoDir)
	if err != nil {
		return "", err
	}
	return commitID, nil
}

func ReadFileAtRevision(repoDir, revision, relativePath string) (string, error) {
	output, err := gitOutput(repoDir, "show", revision+":"+filepath.ToSlash(relativePath))
	if err != nil {
		return "", fmt.Errorf("read git file %q at revision %q: %w", relativePath, revision, err)
	}
	return output, nil
}

func ListFilesAtRevision(repoDir, revision, relativePath string) ([]string, error) {
	output, err := gitOutput(repoDir, "ls-tree", "--name-only", "-r", revision+":"+filepath.ToSlash(relativePath))
	if err != nil {
		return nil, fmt.Errorf("list files at revision %q for %q: %w", revision, relativePath, err)
	}
	if output == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	return lines, nil
}

func DiffChangedFiles(repoDir, oldRevision, newRevision string) ([]string, error) {
	output, err := gitOutput(repoDir, "diff", "--name-only", oldRevision, newRevision)
	if err != nil {
		return nil, fmt.Errorf("diff changed files %s..%s: %w", oldRevision, newRevision, err)
	}
	if output == "" {
		return nil, nil
	}
	lines := strings.Split(strings.TrimSuffix(output, "\n"), "\n")
	return lines, nil
}

func FetchAndFastForward(repoDir, remoteURL, branch, authUsername, authToken string) error {
	if remoteURL == "" {
		return errors.New("remote URL is required")
	}
	if branch == "" {
		return errors.New("remote branch is required")
	}
	if _, err := gitOutputWithOptions(repoDir, nil, gitRemoteConfig(remoteURL, authUsername, authToken), "fetch", remoteURL, branch); err != nil {
		return fmt.Errorf("fetch remote branch %q: %w", branch, err)
	}
	if err := gitCommand(repoDir, nil, "merge", "--ff-only", "FETCH_HEAD"); err != nil {
		return fmt.Errorf("fast-forward to fetched HEAD: %w", err)
	}
	return nil
}

func PushCurrentBranch(repoDir, remoteURL, branch, authUsername, authToken string) error {
	if remoteURL == "" {
		return errors.New("remote URL is required")
	}
	if branch == "" {
		return errors.New("remote branch is required")
	}
	if err := gitCommandWithOptions(repoDir, nil, gitRemoteConfig(remoteURL, authUsername, authToken), "push", remoteURL, "HEAD:refs/heads/"+branch); err != nil {
		return fmt.Errorf("push HEAD to remote branch %q: %w", branch, err)
	}
	return nil
}

func gitAuthorEnv(authorName, authorEmail string) []string {
	if authorName == "" {
		authorName = "Composia"
	}
	if authorEmail == "" {
		authorEmail = "composia@localhost"
	}
	return []string{
		"GIT_AUTHOR_NAME=" + authorName,
		"GIT_AUTHOR_EMAIL=" + authorEmail,
		"GIT_COMMITTER_NAME=" + authorName,
		"GIT_COMMITTER_EMAIL=" + authorEmail,
	}
}

func gitCommand(repoDir string, extraEnv []string, args ...string) error {
	return gitCommandWithOptions(repoDir, extraEnv, nil, args...)
}

func gitCommandWithOptions(repoDir string, extraEnv, gitConfig []string, args ...string) error {
	commandArgs := make([]string, 0, len(args)+2)
	commandArgs = append(commandArgs, "-C", repoDir)
	for _, configValue := range gitConfig {
		commandArgs = append(commandArgs, "-c", configValue)
	}
	commandArgs = append(commandArgs, args...)

	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", commandArgs...) //nolint:gosec
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("git %s timed out: %w", strings.Join(args, " "), ctx.Err())
		}
		return fmt.Errorf("git %s: %w %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func gitOutput(repoDir string, args ...string) (string, error) {
	return gitOutputWithOptions(repoDir, nil, nil, args...)
}

func gitOutputWithOptions(repoDir string, extraEnv, gitConfig []string, args ...string) (string, error) {
	commandArgs := make([]string, 0, len(args)+2)
	commandArgs = append(commandArgs, "-C", repoDir)
	for _, configValue := range gitConfig {
		commandArgs = append(commandArgs, "-c", configValue)
	}
	commandArgs = append(commandArgs, args...)

	ctx, cancel := context.WithTimeout(context.Background(), gitCommandTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", commandArgs...) //nolint:gosec
	if len(extraEnv) > 0 {
		cmd.Env = append(os.Environ(), extraEnv...)
	}
	output, err := cmd.Output()
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("git %s timed out: %w", strings.Join(args, " "), ctx.Err())
		}
		var stderr bytes.Buffer
		exitErr := &exec.ExitError{}
		if errors.As(err, &exitErr) {
			stderr.Write(exitErr.Stderr)
		}
		return "", fmt.Errorf("git %s: %w %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return string(output), nil
}

func gitRemoteConfig(remoteURL, authUsername, authToken string) []string {
	if authToken == "" {
		return nil
	}
	parsed, err := url.Parse(remoteURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil
	}
	if strings.TrimSpace(authUsername) != "" {
		credentials := base64.StdEncoding.EncodeToString([]byte(strings.TrimSpace(authUsername) + ":" + authToken))
		return []string{"http.extraHeader=Authorization: Basic " + credentials}
	}
	return []string{"http.extraHeader=Authorization: Bearer " + authToken}
}
