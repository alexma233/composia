package repo

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateWorkingTreeAcceptsGitRepo(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if output, err := exec.CommandContext(context.Background(), "git", "-C", repoDir, "init").CombinedOutput(); err != nil { //nolint:gosec
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
	if err := os.WriteFile(filePath, []byte("not a directory"), 0o600); err != nil {
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
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o600); err != nil {
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

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("dirty\n"), 0o600); err != nil {
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
		if err := os.WriteFile(filepath.Join(repoDir, filename), []byte(content), 0o600); err != nil {
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
	firstCommit := writeAndCommit("one.txt", "one\n", "first")
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

	commits, nextCursor, err = ListCommits(repoDir, firstCommit, 2)
	if err != nil {
		t.Fatalf("list after root cursor: %v", err)
	}
	if len(commits) != 0 || nextCursor != "" {
		t.Fatalf("expected empty page after root cursor, got commits=%+v cursor=%q", commits, nextCursor)
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

func TestFetchAndFastForwardUsesGitConfigForAuthHeaders(t *testing.T) {
	repoDir := t.TempDir()
	remoteDir := t.TempDir()
	gitRun(t, remoteDir, "init", "--bare")
	gitRun(t, repoDir, "init")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "initial")
	gitRun(t, repoDir, "remote", "add", "origin", remoteDir)
	gitRun(t, repoDir, "push", "origin", "HEAD:refs/heads/main")

	spyDir := t.TempDir()
	logPath := filepath.Join(spyDir, "git-invocations.log")
	spyPath := filepath.Join(spyDir, "git")
	realGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("find git: %v", err)
	}
	t.Setenv("TEST_REAL_GIT", realGit)
	spyScript := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> \"" + logPath + "\"\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"fetch\" ]; then\n    echo 'forced fetch failure' >&2\n    exit 1\n  fi\ndone\nexec \"$TEST_REAL_GIT\" \"$@\"\n"
	if err := os.WriteFile(spyPath, []byte(spyScript), 0o755); err != nil { //nolint:gosec
		t.Fatalf("write git spy: %v", err)
	}

	originalPath := os.Getenv("PATH")
	t.Setenv("PATH", spyDir+string(os.PathListSeparator)+originalPath)

	cloneDir := t.TempDir()
	gitRun(t, cloneDir, "clone", remoteDir, ".")

	logBefore, err := os.ReadFile(logPath) //nolint:gosec
	if err != nil {
		t.Fatalf("read initial spy log: %v", err)
	}
	beforeEntries := len(strings.Split(strings.TrimSpace(string(logBefore)), "\n"))

	err = FetchAndFastForward(cloneDir, "https://example.com/repo.git", "main", "octocat", "secret-token")
	if err == nil || !strings.Contains(err.Error(), "forced fetch failure") {
		t.Fatalf("expected forced fetch failure, got %v", err)
	}

	logAfter, err := os.ReadFile(logPath) //nolint:gosec
	if err != nil {
		t.Fatalf("read final spy log: %v", err)
	}
	entries := strings.Split(strings.TrimSpace(string(logAfter)), "\n")
	if len(entries) <= beforeEntries {
		t.Fatalf("expected new git invocation, got log %q", string(logAfter))
	}
	fetchCommand := entries[len(entries)-1]
	expectedHeader := "-c http.extraHeader=Authorization: Basic " + base64.StdEncoding.EncodeToString([]byte("octocat:secret-token"))
	if !strings.Contains(fetchCommand, expectedHeader) {
		t.Fatalf("expected fetch command %q to contain %q", fetchCommand, expectedHeader)
	}
	if !strings.Contains(fetchCommand, "fetch https://example.com/repo.git main") {
		t.Fatalf("expected fetch command, got %q", fetchCommand)
	}
}

func TestCommitPathsAndRevisionHelpers(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "initial")
	initialRevision, err := CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("current revision: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "service"), 0o750); err != nil {
		t.Fatalf("create service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "service", "compose.yaml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("write compose: %v", err)
	}

	changed, err := HasPathsChanges(repoDir, "service/compose.yaml")
	if err != nil {
		t.Fatalf("has path changes: %v", err)
	}
	if !changed {
		t.Fatalf("expected uncommitted path changes")
	}
	commitID, err := CommitPaths(repoDir, []string{"service/compose.yaml"}, "add compose", "Tester", "tester@example.com")
	if err != nil {
		t.Fatalf("commit paths: %v", err)
	}
	if commitID == "" || commitID == initialRevision {
		t.Fatalf("unexpected commit id %q initial %q", commitID, initialRevision)
	}
	content, err := ReadFileAtRevision(repoDir, commitID, "service/compose.yaml")
	if err != nil {
		t.Fatalf("read file at revision: %v", err)
	}
	if content != "services: {}\n" {
		t.Fatalf("content = %q", content)
	}
	files, err := ListFilesAtRevision(repoDir, commitID, "service")
	if err != nil {
		t.Fatalf("list files at revision: %v", err)
	}
	if len(files) != 1 || files[0] != "compose.yaml" {
		t.Fatalf("files = %+v", files)
	}
	changedFiles, err := DiffChangedFiles(repoDir, initialRevision, commitID)
	if err != nil {
		t.Fatalf("diff changed files: %v", err)
	}
	if len(changedFiles) != 1 || changedFiles[0] != "service/compose.yaml" {
		t.Fatalf("changed files = %+v", changedFiles)
	}
	if _, err := CommitPaths(repoDir, []string{"service/compose.yaml"}, "no changes", "", ""); !errors.Is(err, ErrNoGitChanges) {
		t.Fatalf("expected ErrNoGitChanges, got %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("updated\n"), 0o600); err != nil {
		t.Fatalf("update README: %v", err)
	}
	changed, err = HasPathChanges(repoDir, "README.md")
	if err != nil {
		t.Fatalf("has path changes wrapper: %v", err)
	}
	if !changed {
		t.Fatalf("expected README changes")
	}
	if _, err := CommitPath(repoDir, "README.md", "", "", ""); err != nil {
		t.Fatalf("commit path wrapper: %v", err)
	}
}

func TestCommitPathsScopesCommitAndRestoresIndexOnError(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	gitRun(t, repoDir, "init")
	gitRun(t, repoDir, "config", "commit.gpgsign", "false")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("hello\n"), 0o600); err != nil {
		t.Fatalf("write README: %v", err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "initial")
	initialRevision, err := CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("current revision: %v", err)
	}

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("staged\n"), 0o600); err != nil {
		t.Fatalf("update README: %v", err)
	}
	gitRun(t, repoDir, "add", "README.md")
	if _, err := CommitPaths(repoDir, []string{"missing.txt"}, "missing", "Tester", "tester@example.com"); err == nil {
		t.Fatalf("expected missing path commit to fail")
	}
	cached, err := gitOutput(repoDir, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatalf("read cached diff: %v", err)
	}
	if strings.TrimSpace(cached) != "README.md" {
		t.Fatalf("expected README.md to stay staged after failed scoped commit, got %q", cached)
	}

	serviceDir := filepath.Join(repoDir, "-service")
	if err := os.MkdirAll(serviceDir, 0o750); err != nil {
		t.Fatalf("create option-like service dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serviceDir, "compose.yaml"), []byte("services: {}\n"), 0o600); err != nil {
		t.Fatalf("write compose: %v", err)
	}
	commitID, err := CommitPaths(repoDir, []string{"-service/compose.yaml"}, "add option path", "Tester", "tester@example.com")
	if err != nil {
		t.Fatalf("commit option-like path: %v", err)
	}
	changedFiles, err := DiffChangedFiles(repoDir, initialRevision, commitID)
	if err != nil {
		t.Fatalf("diff changed files: %v", err)
	}
	if len(changedFiles) != 1 || changedFiles[0] != "-service/compose.yaml" {
		t.Fatalf("scoped commit changed files = %+v", changedFiles)
	}
	cached, err = gitOutput(repoDir, "diff", "--cached", "--name-only")
	if err != nil {
		t.Fatalf("read cached diff after commit: %v", err)
	}
	if strings.TrimSpace(cached) != "README.md" {
		t.Fatalf("expected unrelated README.md to remain staged, got %q", cached)
	}
}

func TestFindServiceAtRevision(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	gitRun(t, repoDir, "init")
	if err := os.MkdirAll(filepath.Join(repoDir, "app"), 0o750); err != nil {
		t.Fatalf("create app dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "app", MetaFileName), []byte("name: app\nnodes:\n  - main\n"), 0o600); err != nil {
		t.Fatalf("write app meta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, MetaFileName), []byte("name: root-rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o600); err != nil {
		t.Fatalf("write root meta: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "apps", "rustic"), 0o750); err != nil {
		t.Fatalf("create nested rustic dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "apps", "rustic", MetaFileName), []byte("name: nested-rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o600); err != nil {
		t.Fatalf("write nested rustic meta: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoDir, "rustic"), 0o750); err != nil {
		t.Fatalf("create rustic dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoDir, "rustic", MetaFileName), []byte("name: rustic\nnodes:\n  - main\ninfra:\n  rustic:\n    compose_service: rustic\n"), 0o600); err != nil {
		t.Fatalf("write rustic meta: %v", err)
	}
	gitRun(t, repoDir, "add", ".")
	gitRun(t, repoDir, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-m", "services")
	revision, err := CurrentRevision(repoDir)
	if err != nil {
		t.Fatalf("current revision: %v", err)
	}
	metaFiles, err := ListServiceMetaFilesAtRevision(repoDir, revision)
	if err != nil {
		t.Fatalf("list service meta files at revision: %v", err)
	}
	if len(metaFiles) != 2 || metaFiles[0] != "app/composia-meta.yaml" || metaFiles[1] != "rustic/composia-meta.yaml" {
		t.Fatalf("meta files = %+v", metaFiles)
	}
	available := map[string]struct{}{"main": {}}
	service, err := FindServiceAtRevision(repoDir, revision, "app", available)
	if err != nil {
		t.Fatalf("find service at revision: %v", err)
	}
	if service.Name != "app" || service.MetaPath != filepath.Join(repoDir, "app", MetaFileName) {
		t.Fatalf("unexpected service: %+v", service)
	}
	rustic, err := FindRusticInfraServiceAtRevision(repoDir, revision, available)
	if err != nil {
		t.Fatalf("find rustic at revision: %v", err)
	}
	if rustic.Name != "rustic" {
		t.Fatalf("rustic = %+v", rustic)
	}
	if _, err := FindServiceAtRevision(repoDir, "", "app", available); err == nil || !strings.Contains(err.Error(), "revision is required") {
		t.Fatalf("expected missing revision error, got %v", err)
	}
}

func TestPushCurrentBranchRequiresRemoteAndBranch(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	if err := PushCurrentBranch(repoDir, "", "main", "", ""); err == nil || !strings.Contains(err.Error(), "remote URL") {
		t.Fatalf("expected remote URL error, got %v", err)
	}
	if err := PushCurrentBranch(repoDir, "https://example.com/repo.git", "", "", ""); err == nil || !strings.Contains(err.Error(), "remote branch") {
		t.Fatalf("expected remote branch error, got %v", err)
	}
}

func gitRun(t *testing.T, repoDir string, args ...string) {
	t.Helper()
	commandArgs := append([]string{"-C", repoDir}, args...)
	output, err := exec.CommandContext(context.Background(), "git", commandArgs...).CombinedOutput() //nolint:gosec
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}
