package cli

type repoWriteResult struct {
	commitID             string
	syncStatus           string
	pushError            string
	lastSuccessfulPullAt string
}

func (application *app) printRepoWriteResult(result repoWriteResult) error {
	return writeKV(application.out, [][2]string{
		{"commit_id", result.commitID},
		{"sync_status", result.syncStatus},
		{"push_error", result.pushError},
		{"last_successful_pull_at", result.lastSuccessfulPullAt},
	})
}
