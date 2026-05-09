package agent

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"
)

type commandLogWriter struct {
	mu         sync.Mutex
	uploadLog  func(string) error
	output     bytes.Buffer
	captureOut bool
}

func newCommandLogWriter(uploadLog func(string) error, captureOut bool) *commandLogWriter {
	return &commandLogWriter{uploadLog: uploadLog, captureOut: captureOut}
}

func (writer *commandLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()

	if writer.captureOut {
		if _, err := writer.output.Write(p); err != nil {
			return 0, err
		}
	}
	if writer.uploadLog != nil {
		if err := writer.uploadLog(string(p)); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (writer *commandLogWriter) String() string {
	writer.mu.Lock()
	defer writer.mu.Unlock()
	return writer.output.String()
}

func prepareCommandForTerminalLog(command *exec.Cmd) {
	if command == nil {
		return
	}

	env := command.Environ()
	env = setCommandEnv(env, "TERM", "xterm-256color")
	env = setCommandEnv(env, "CLICOLOR_FORCE", "1")
	env = setCommandEnv(env, "FORCE_COLOR", "1")
	env = setCommandEnv(env, "COMPOSE_ANSI", "always")
	env = setCommandEnv(env, "COMPOSE_STATUS_STDOUT", "1")
	env = setCommandEnv(env, "COMPOSE_PROGRESS", "tty")
	command.Env = env
}

func setCommandEnv(env []string, key, value string) []string {
	prefix := key + "="
	for index, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			env[index] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func runCommandWithLiveLogs(command *exec.Cmd, uploadLog func(string) error) error {
	writer := newCommandLogWriter(uploadLog, false)
	prepareCommandForTerminalLog(command)
	command.Stdout = writer
	command.Stderr = writer
	return command.Run()
}

func runCommandWithLiveLogsAndCapture(command *exec.Cmd, uploadLog func(string) error) (string, error) {
	writer := newCommandLogWriter(uploadLog, true)
	prepareCommandForTerminalLog(command)
	command.Stdout = writer
	command.Stderr = writer
	err := command.Run()
	return writer.String(), err
}
