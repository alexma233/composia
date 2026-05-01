package cli

import (
	"os"
	"os/signal"

	"golang.org/x/term"
)

type terminalState struct {
	state *term.State
}

func makeTerminalRaw(fd uintptr) (*terminalState, error) {
	state, err := term.MakeRaw(int(fd))
	if err != nil {
		return nil, err
	}
	return &terminalState{state: state}, nil
}

func restoreTerminal(fd uintptr, state *terminalState) {
	if state == nil || state.state == nil {
		return
	}
	_ = term.Restore(int(fd), state.state)
}

func terminalSize(fd uintptr) (uint32, uint32, bool) {
	cols, rows, err := term.GetSize(int(fd))
	if err != nil || rows <= 0 || cols <= 0 {
		return 0, 0, false
	}
	return uint32(rows), uint32(cols), true
}

func subscribeTerminalResize() (<-chan os.Signal, func()) {
	resizeSignal := terminalResizeSignal()
	if resizeSignal == nil {
		return nil, func() {}
	}
	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, resizeSignal)
	return resizeCh, func() {
		signal.Stop(resizeCh)
	}
}
