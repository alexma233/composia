//go:build !unix

package repo

import "os/exec"

// Controller synchronization runs on Linux; other builds retain standard process cancellation.
func configureGitCancellation(_ *exec.Cmd) {}
