package cli

import (
	"fmt"
	"io"
	"os"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
)

func (application *app) runRepo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia repo <head|files|get|edit|update|history|sync|validate>")
	}
	switch args[0] {
	case "head":
		return application.runRepoHead(args[1:])
	case "files":
		return application.runRepoFiles(args[1:])
	case "get":
		return application.runRepoGet(args[1:])
	case "edit":
		return application.runRepoEdit(args[1:])
	case "update":
		return application.runRepoUpdate(args[1:])
	case "history":
		return application.runRepoHistory(args[1:])
	case "sync":
		return application.runRepoSync(args[1:])
	case "validate":
		return application.runRepoValidate(args[1:])
	default:
		return fmt.Errorf("unknown repo command %q", args[0])
	}
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
	if application.cfg.json {
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
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	return application.printRepoWriteResult(repoWriteResult{
		commitID:             response.Msg.GetCommitId(),
		syncStatus:           response.Msg.GetSyncStatus(),
		pushError:            response.Msg.GetPushError(),
		lastSuccessfulPullAt: response.Msg.GetLastSuccessfulPullAt(),
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
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	head := response.Msg
	return writeKV(application.out, [][2]string{
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
		return fmt.Errorf("usage: composia repo files [--recursive] [path]")
	}
	path := ""
	if len(fs.Args()) == 1 {
		path = fs.Arg(0)
	}
	response, err := application.client.repos.ListRepoFiles(application.ctx, newRequest(&controllerv1.ListRepoFilesRequest{Path: path, Recursive: *recursive}))
	if err != nil {
		return err
	}
	if application.cfg.json {
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
	return writeTable(application.out, []string{"TYPE", "PATH", "NAME", "SIZE"}, rows)
}

func (application *app) runRepoGet(args []string) error {
	if err := requireArgs(args, 1, "composia repo get <path>"); err != nil {
		return err
	}
	response, err := application.client.repos.GetRepoFile(application.ctx, newRequest(&controllerv1.GetRepoFileRequest{Path: args[0]}))
	if err != nil {
		return err
	}
	if application.cfg.json {
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
	response, err := application.client.repos.ListRepoCommits(application.ctx, newRequest(&controllerv1.ListRepoCommitsRequest{PageSize: uint32(*pageSize), Cursor: *cursor}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetCommits()))
	for _, commit := range response.Msg.GetCommits() {
		rows = append(rows, []string{commit.GetCommitId(), commit.GetSubject(), commit.GetCommittedAt()})
	}
	if err := writeTable(application.out, []string{"COMMIT", "SUBJECT", "COMMITTED"}, rows); err != nil {
		return err
	}
	if response.Msg.GetNextCursor() != "" {
		_, err = fmt.Fprintf(application.out, "next_cursor: %s\n", response.Msg.GetNextCursor())
		return err
	}
	return nil
}

func (application *app) runRepoSync(args []string) error {
	if err := requireArgs(args, 0, "composia repo sync"); err != nil {
		return err
	}
	response, err := application.client.repoCommands.SyncRepo(application.ctx, newRequest(&controllerv1.SyncRepoRequest{}))
	if err != nil {
		return err
	}
	if application.cfg.json {
		return application.printMessage(response.Msg)
	}
	sync := response.Msg
	return writeKV(application.out, [][2]string{
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
	if application.cfg.json {
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
	if err := writeTable(application.out, []string{"PATH", "LINE", "MESSAGE"}, rows); err != nil {
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
		return "", fmt.Errorf("controller repo has no HEAD revision")
	}
	return response.Msg.GetHeadRevision(), nil
}

func readContentSource(path string) (string, error) {
	var content []byte
	var err error
	if path == "-" {
		content, err = io.ReadAll(os.Stdin)
	} else {
		content, err = os.ReadFile(path)
	}
	if err != nil {
		return "", fmt.Errorf("read %q: %w", path, err)
	}
	return string(content), nil
}
