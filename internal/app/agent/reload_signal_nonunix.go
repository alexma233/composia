//go:build !unix

package agent

import "context"

func watchAgentReloadSignals(_ context.Context, _ chan<- agentReloadRequest) func() {
	return func() {}
}
