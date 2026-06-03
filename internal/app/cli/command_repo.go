package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runRepoMkdir(args []string) error {
	fs := newCommandFlagSet("repo mkdir")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia repo mkdir [--message text] <path>"); err != nil {
		return err
	}
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	response, err := application.client.repoCommands.CreateRepoDirectory(application.ctx, newRequest(&controllerv1.CreateRepoDirectoryRequest{Path: fs.Arg(0), BaseRevision: baseRevision, CommitMessage: *message}))
	if err != nil {
		return repoWriteError(err)
	}
	return application.printRepoWriteMessage(response.Msg)
}

func (application *app) runRepoMove(args []string) error {
	fs := newCommandFlagSet("repo mv")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 2, "composia repo mv [--message text] <source> <destination>"); err != nil {
		return err
	}
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	response, err := application.client.repoCommands.MoveRepoPath(application.ctx, newRequest(&controllerv1.MoveRepoPathRequest{SourcePath: fs.Arg(0), DestinationPath: fs.Arg(1), BaseRevision: baseRevision, CommitMessage: *message}))
	if err != nil {
		return repoWriteError(err)
	}
	return application.printRepoWriteMessage(response.Msg)
}

func (application *app) runRepoRemove(args []string) error {
	fs := newCommandFlagSet("repo rm")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia repo rm [--message text] <path>"); err != nil {
		return err
	}
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	response, err := application.client.repoCommands.DeleteRepoPath(application.ctx, newRequest(&controllerv1.DeleteRepoPathRequest{Path: fs.Arg(0), BaseRevision: baseRevision, CommitMessage: *message}))
	if err != nil {
		return repoWriteError(err)
	}
	return application.printRepoWriteMessage(response.Msg)
}

func (application *app) runRepoEdit(args []string) error {
	fs := newCommandFlagSet("repo edit")
	create := fs.Bool("create", false, "create the file when it does not exist")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia repo edit [--create] [--message text] <path>"); err != nil {
		return err
	}
	repoPath := fs.Arg(0)
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	content := ""
	fileResponse, err := application.client.repos.GetRepoFile(application.ctx, newRequest(&controllerv1.GetRepoFileRequest{Path: repoPath}))
	if err != nil {
		if !*create || connect.CodeOf(err) != connect.CodeNotFound {
			return err
		}
	} else {
		content = fileResponse.Msg.GetContent()
	}
	updatedContent, changed, err := editText(application.ctx, content, "composia-repo-*", 0o600)
	if err != nil {
		return err
	}
	if !changed {
		_, err := fmt.Fprintln(application.out, "unchanged")
		return err
	}
	response, err := application.client.repoCommands.UpdateRepoFile(application.ctx, newRequest(&controllerv1.UpdateRepoFileRequest{Path: repoPath, Content: updatedContent, BaseRevision: baseRevision, CommitMessage: *message}))
	if err != nil {
		return repoWriteError(err)
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	return application.printRepoWriteResult(repoWriteResult{
		commitID:             response.Msg.GetCommitId(),
		syncStatus:           response.Msg.GetSyncStatus(),
		pushError:            response.Msg.GetPushError(),
		lastSuccessfulPullAt: response.Msg.GetLastSuccessfulPullAt(),
	})
}

func (application *app) runRepoUpdate(args []string) error {
	fs := newCommandFlagSet("repo update")
	filePath := fs.String("file", "", "local file to read; use - for stdin")
	message := fs.String("message", "", "commit message")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia repo update --file file [--message text] <path>"); err != nil {
		return err
	}
	if *filePath == "" {
		return errorsWithUsage("file is required", "composia repo update --file file [--message text] <path>")
	}
	content, err := readContentSource(*filePath)
	if err != nil {
		return err
	}
	baseRevision, err := application.currentRepoRevision()
	if err != nil {
		return err
	}
	response, err := application.client.repoCommands.UpdateRepoFile(application.ctx, newRequest(&controllerv1.UpdateRepoFileRequest{Path: fs.Arg(0), Content: content, BaseRevision: baseRevision, CommitMessage: *message}))
	if err != nil {
		return repoWriteError(err)
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	return application.printRepoWriteResult(repoWriteResult{
		commitID:             response.Msg.GetCommitId(),
		syncStatus:           response.Msg.GetSyncStatus(),
		pushError:            response.Msg.GetPushError(),
		lastSuccessfulPullAt: response.Msg.GetLastSuccessfulPullAt(),
	})
}

func (application *app) printRepoWriteMessage(message *controllerv1.RepoWriteResult) error {
	if application.isJSONOutput() {
		return application.printMessage(message)
	}
	return application.printRepoWriteResult(repoWriteResult{
		commitID:             message.GetCommitId(),
		syncStatus:           message.GetSyncStatus(),
		pushError:            message.GetPushError(),
		lastSuccessfulPullAt: message.GetLastSuccessfulPullAt(),
	})
}

func (application *app) runRepoHead(args []string) error {
	if err := requireArgs(args, 0, "composia repo head"); err != nil {
		return err
	}
	response, err := application.client.repos.GetRepoHead(application.ctx, newRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	head := response.Msg
	return application.writeKV([][2]string{
		{"head_revision", head.GetHeadRevision()},
		{"branch", head.GetBranch()},
		{"has_remote", boolText(head.GetHasRemote())},
		{"clean_worktree", boolText(head.GetCleanWorktree())},
		{"sync_status", head.GetSyncStatus()},
		{"last_sync_error", head.GetLastSyncError()},
		{"last_successful_pull_at", head.GetLastSuccessfulPullAt()},
	})
}

func (application *app) runRepoFiles(args []string) error {
	fs := newCommandFlagSet("repo files")
	recursive := fs.Bool("recursive", false, "include descendants")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if len(fs.Args()) > 1 {
		return errors.New("usage: composia repo files [--recursive] [path]")
	}
	path := ""
	if len(fs.Args()) == 1 {
		path = fs.Arg(0)
	}
	response, err := application.client.repos.ListRepoFiles(application.ctx, newRequest(&controllerv1.ListRepoFilesRequest{Path: path, Recursive: *recursive}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetEntries()))
	for _, entry := range response.Msg.GetEntries() {
		kind := "file"
		if entry.GetIsDir() {
			kind = "dir"
		}
		rows = append(rows, []string{kind, entry.GetPath(), entry.GetName(), int64Text(entry.GetSize())})
	}
	return application.writeTable([]string{"TYPE", "PATH", "NAME", "SIZE"}, rows)
}

func (application *app) runRepoGet(args []string) error {
	if err := requireArgs(args, 1, "composia repo get <path>"); err != nil {
		return err
	}
	response, err := application.client.repos.GetRepoFile(application.ctx, newRequest(&controllerv1.GetRepoFileRequest{Path: args[0]}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	_, err = fmt.Fprint(application.out, response.Msg.GetContent())
	return err
}

func (application *app) runRepoHistory(args []string) error {
	fs := newCommandFlagSet("repo history")
	pageSize := fs.Uint("page-size", 20, "page size")
	cursor := fs.String("cursor", "", "pagination cursor")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia repo history [--page-size n] [--cursor cursor]"); err != nil {
		return err
	}
	pageSizeValue, err := uint32FlagValue("page-size", *pageSize)
	if err != nil {
		return err
	}
	response, err := application.client.repos.ListRepoCommits(application.ctx, newRequest(&controllerv1.ListRepoCommitsRequest{PageSize: pageSizeValue, Cursor: *cursor}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetCommits()))
	for _, commit := range response.Msg.GetCommits() {
		rows = append(rows, []string{commit.GetCommitId(), commit.GetSubject(), commit.GetCommittedAt()})
	}
	if err := application.writeTable([]string{"COMMIT", "SUBJECT", "COMMITTED"}, rows); err != nil {
		return err
	}
	return application.writeCursor(response.Msg.GetNextCursor())
}

func (application *app) runRepoSync(args []string) error {
	if err := requireArgs(args, 0, "composia repo sync"); err != nil {
		return err
	}
	response, err := application.client.repoCommands.SyncRepo(application.ctx, newRequest(&controllerv1.SyncRepoRequest{}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	sync := response.Msg
	return application.writeKV([][2]string{
		{"head_revision", sync.GetHeadRevision()},
		{"branch", sync.GetBranch()},
		{"sync_status", sync.GetSyncStatus()},
		{"last_sync_error", sync.GetLastSyncError()},
		{"last_successful_pull_at", sync.GetLastSuccessfulPullAt()},
	})
}

func (application *app) runRepoValidate(args []string) error {
	if err := requireArgs(args, 0, "composia repo validate"); err != nil {
		return err
	}
	response, err := application.client.repos.ValidateRepo(application.ctx, newRequest(&controllerv1.ValidateRepoRequest{}))
	if err != nil {
		return err
	}
	errors := response.Msg.GetErrors()
	if application.isJSONOutput() {
		if err := application.printMessage(response.Msg); err != nil {
			return err
		}
		if len(errors) > 0 {
			return fmt.Errorf("repo validation failed with %d error(s)", len(errors))
		}
		return nil
	}
	if len(errors) == 0 {
		_, err := fmt.Fprintln(application.out, "OK")
		return err
	}
	rows := make([][]string, 0, len(errors))
	for _, validationError := range errors {
		rows = append(rows, []string{validationError.GetPath(), uintText(validationError.GetLine()), validationError.GetMessage()})
	}
	if err := application.writeTable([]string{"PATH", "LINE", "MESSAGE"}, rows); err != nil {
		return err
	}
	return fmt.Errorf("repo validation failed with %d error(s)", len(errors))
}

func (application *app) currentRepoRevision() (string, error) {
	response, err := application.client.repos.GetRepoHead(application.ctx, newRequest(&controllerv1.GetRepoHeadRequest{}))
	if err != nil {
		return "", err
	}
	if response.Msg.GetHeadRevision() == "" {
		return "", errors.New("controller repo has no HEAD revision")
	}
	return response.Msg.GetHeadRevision(), nil
}

func readContentSource(path string) (string, error) {
	var content []byte
	var err error
	if path == "-" {
		content, err = io.ReadAll(os.Stdin)
	} else {
		content, err = os.ReadFile(path) //nolint:gosec
	}
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	return string(content), nil
}
