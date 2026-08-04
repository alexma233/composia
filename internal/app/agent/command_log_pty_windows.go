//go:build windows

package agent

import "os/exec"

func runCommandWithPTYLiveLogs(command *exec.Cmd, uploadLog func(string) error) error {
	return runCommandWithLiveLogs(command, uploadLog)
}
