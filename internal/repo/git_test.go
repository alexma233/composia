package repo

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateWorkingTreeAcceptsGitRepo(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if output, err := exec.Command("git", "-C", repoDir, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, string(output))
	}

	if err := ValidateWorkingTree(repoDir); err != nil {
		t.Fatalf("expected valid working tree, got %v", err)
	}
}

func TestValidateWorkingTreeRejectsPlainDirectory(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	err := ValidateWorkingTree(repoDir)
	if err == nil || !strings.Contains(err.Error(), "git working tree") {
		t.Fatalf("expected git working tree error, got %v", err)
	}
}

func TestValidateWorkingTreeRejectsFilePath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "repo.txt")
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	err := ValidateWorkingTree(filePath)
	if err == nil || !strings.Contains(err.Error(), "must be a directory") {
		t.Fatalf("expected directory error, got %v", err)
	}
}

func TestGitHeadHelpers(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	gitRun(t, repoDir, "init")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "initial")

	revision, err := CurrentRevision(repoDir)
	if err != nil || revision == "" {
		t.Fatalf("current revision failed: %v %q", err, revision)
	}

	branch, err := CurrentBranch(repoDir)
	if err != nil || branch == "" {
		t.Fatalf("current branch failed: %v %q", err, branch)
	}

	hasRemote, err := HasRemote(repoDir)
	if err != nil {
		t.Fatalf("has remote failed: %v", err)
	}
	if hasRemote {
		t.Fatalf("expected no remote")
	}

	clean, err := IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("clean worktree failed: %v", err)
	}
	if !clean {
		t.Fatalf("expected clean worktree")
	}

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatalf("rewrite README: %v", err)
	}
	clean, err = IsCleanWorkingTree(repoDir)
	if err != nil {
		t.Fatalf("dirty worktree check failed: %v", err)
	}
	if clean {
		t.Fatalf("expected dirty worktree")
	}
}

func TestListCommitsSupportsCursorPaging(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	gitRun(t, repoDir, "init")
	writeAndCommit := func(filename, content, message string) string {
		t.Helper()
		if err := os.WriteFile(filepath.Join(repoDir, filename), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", filename, err)
		}
		gitRun(t, repoDir, "add", ".")
		gitRun(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", message)
		revision, err := CurrentRevision(repoDir)
		if err != nil {
			t.Fatalf("current revision: %v", err)
		}
		return revision
	}
	_ = writeAndCommit("one.txt", "one\n", "first")
	secondCommit := writeAndCommit("two.txt", "two\n", "second")
	_ = writeAndCommit("three.txt", "three\n", "third")

	commits, nextCursor, err := ListCommits(repoDir, "", 2)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(commits) != 2 || commits[0].Subject != "third" || commits[1].Subject != "second" {
		t.Fatalf("unexpected first page: %+v", commits)
	}
	if nextCursor != commits[1].CommitID {
		t.Fatalf("expected next cursor %q, got %q", commits[1].CommitID, nextCursor)
	}

	commits, nextCursor, err = ListCommits(repoDir, secondCommit, 2)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(commits) != 1 || commits[0].Subject != "first" {
		t.Fatalf("unexpected second page: %+v", commits)
	}
	if nextCursor != "" {
		t.Fatalf("expected empty next cursor, got %q", nextCursor)
	}
}

func TestGitRemoteConfigUsesBearerTokenWithoutUsername(t *testing.T) {
	t.Parallel()

	config := gitRemoteConfig("https://example.com/repo.git", "", "secret-token")
	if len(config) != 1 {
		t.Fatalf("expected one config entry, got %v", config)
	}
	if config[0] != "http.extraHeader=Authorization: Bearer secret-token" {
		t.Fatalf("unexpected bearer config: %q", config[0])
	}
}

func TestGitRemoteConfigUsesBasicAuthWhenUsernameConfigured(t *testing.T) {
	t.Parallel()

	config := gitRemoteConfig("https://example.com/repo.git", "octocat", "secret-token")
	if len(config) != 1 {
		t.Fatalf("expected one config entry, got %v", config)
	}
	want := "http.extraHeader=Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("octocat:secret-token"))
	if config[0] != want {
		t.Fatalf("unexpected basic auth config: %q", config[0])
	}
}

func gitRun(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", repoDir}, args...)
	output, err := exec.Command("git", commandArgs...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}
