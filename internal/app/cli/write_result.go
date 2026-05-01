package cli

import (
	"fmt"
	"strings"

	"connectrpc.com/connect"
)

type repoWriteResult struct {
	commitID             string
	syncStatus           string
	pushError            string
	lastSuccessfulPullAt string
}

func (application *app) printRepoWriteResult(result repoWriteResult) error {
	return application.writeKV([][2]string{
		{"commit_id", result.commitID},
		{"sync_status", result.syncStatus},
		{"push_error", result.pushError},
		{"last_successful_pull_at", result.lastSuccessfulPullAt},
	})
}

func repoWriteError(err error) error {
	if err == nil {
		return nil
	}
	if connect.CodeOf(err) == connect.CodeFailedPrecondition && strings.Contains(err.Error(), "base_revision") {
		return fmt.Errorf("repo changed while preparing this write; run `composia repo sync` if needed and retry: %w", err)
	}
	return err
}
