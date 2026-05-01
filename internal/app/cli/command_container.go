package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"github.com/gorilla/websocket"
	"golang.org/x/sys/unix"
)

func (application *app) runContainer(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia container <list|get|logs|start|stop|restart|remove|exec>")
	}
	switch args[0] {
	case "list":
		return application.runContainerList(args[1:])
	case "get":
		return application.runContainerGet(args[1:])
	case "logs":
		return application.runContainerLogs(args[1:])
	case "start", "stop", "restart":
		return application.runContainerAction(args[0], args[1:])
	case "remove":
		return application.runContainerRemove(args[1:])
	case "exec":
		return application.runContainerExec(args[1:])
	default:
		return fmt.Errorf("unknown container command %q", args[0])
	}
}

func (application *app) runContainerList(args []string) error {
	fs := newCommandFlagSet("container list")
	nodeID := fs.String("node", "", "node ID")
	search := fs.String("search", "", "search text")
	sortBy := fs.String("sort-by", "", "sort field")
	sortDesc := fs.Bool("desc", false, "sort descending")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 0, "composia container list --node node [--search text] [--sort-by field] [--desc]"); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", "composia container list --node node [--search text] [--sort-by field] [--desc]")
	}
	pageSize, page := pageValues()
	response, err := application.client.docker.ListNodeContainers(application.ctx, newRequest(&controllerv1.ListNodeContainersRequest{
		NodeId:   strings.TrimSpace(*nodeID),
		PageSize: pageSize,
		Page:     page,
		Search:   *search,
		SortBy:   *sortBy,
		SortDesc: *sortDesc,
	}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	rows := make([][]string, 0, len(response.Msg.GetContainers()))
	for _, container := range response.Msg.GetContainers() {
		rows = append(rows, []string{
			container.GetId(),
			container.GetName(),
			container.GetImage(),
			container.GetState(),
			container.GetStatus(),
			strings.Join(container.GetPorts(), ","),
			strings.Join(container.GetNetworks(), ","),
		})
	}
	if err := application.writeTable([]string{"CONTAINER", "NAME", "IMAGE", "STATE", "STATUS", "PORTS", "NETWORKS"}, rows); err != nil {
		return err
	}
	return application.writeCount("total_count", response.Msg.GetTotalCount())
}

func (application *app) runContainerGet(args []string) error {
	fs := newCommandFlagSet("container get")
	nodeID := fs.String("node", "", "node ID")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia container get --node node <container>"); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", "composia container get --node node <container>")
	}
	response, err := application.client.docker.InspectNodeContainer(application.ctx, newRequest(&controllerv1.InspectNodeContainerRequest{NodeId: strings.TrimSpace(*nodeID), ContainerId: fs.Arg(0)}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	_, err = fmt.Fprintln(application.out, response.Msg.GetRawJson())
	return err
}

func (application *app) runContainerLogs(args []string) error {
	fs := newCommandFlagSet("container logs")
	nodeID := fs.String("node", "", "node ID")
	tail := fs.String("tail", "100", "number of lines or all")
	timestamps := fs.Bool("timestamps", false, "include timestamps")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia container logs --node node [--tail n|all] [--timestamps] <container>"); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", "composia container logs --node node [--tail n|all] [--timestamps] <container>")
	}
	stream, err := application.client.containers.GetContainerLogs(application.ctx, newRequest(&controllerv1.GetContainerLogsRequest{NodeId: strings.TrimSpace(*nodeID), ContainerId: fs.Arg(0), Tail: *tail, Timestamps: *timestamps}))
	if err != nil {
		return err
	}
	for stream.Receive() {
		if application.isJSONOutput() {
			if err := application.printMessage(stream.Msg()); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprint(application.out, stream.Msg().GetContent()); err != nil {
			return err
		}
	}
	return stream.Err()
}

func (application *app) runContainerAction(actionName string, args []string) error {
	action, err := containerActionFromName(actionName)
	if err != nil {
		return err
	}
	fs := newCommandFlagSet("container " + actionName)
	nodeID := fs.String("node", "", "node ID")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := fmt.Sprintf("composia container %s [--wait] [--follow] [--timeout duration] --node node <container>", actionName)
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", usage)
	}
	response, err := application.client.containers.RunContainerAction(application.ctx, newRequest(&controllerv1.RunContainerActionRequest{NodeId: strings.TrimSpace(*nodeID), ContainerId: fs.Arg(0), Action: action}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runContainerRemove(args []string) error {
	fs := newCommandFlagSet("container remove")
	nodeID := fs.String("node", "", "node ID")
	force := fs.Bool("force", false, "force remove")
	volumes := fs.Bool("volumes", false, "remove anonymous volumes")
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := "composia container remove [--wait] [--follow] [--timeout duration] --node node [--force] [--volumes] <container>"
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", usage)
	}
	response, err := application.client.containers.RemoveContainer(application.ctx, newRequest(&controllerv1.RemoveContainerRequest{NodeId: strings.TrimSpace(*nodeID), ContainerId: fs.Arg(0), Force: *force, RemoveVolumes: *volumes}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runContainerExec(args []string) error {
	fs := newCommandFlagSet("container exec")
	nodeID := fs.String("node", "", "node ID")
	tty := fs.Bool("tty", false, "attach an interactive terminal")
	fs.BoolVar(tty, "t", false, "attach an interactive terminal")
	stdinFile := fs.String("stdin-file", "", "file to send to stdin; use - for standard input")
	timeout := fs.Duration("timeout", 30*time.Second, "non-interactive exec timeout")
	maxOutput := fs.Uint64("max-output", 1024*1024, "maximum bytes per output stream")
	rows := fs.Uint("rows", 24, "terminal rows for --tty")
	cols := fs.Uint("cols", 80, "terminal columns for --tty")
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := "composia container exec [--tty] [--stdin-file file] [--timeout duration] [--max-output bytes] --node node <container> -- <command> [args...]"
	if len(fs.Args()) < 2 {
		return fmt.Errorf("usage: %s", usage)
	}
	if strings.TrimSpace(*nodeID) == "" {
		return errorsWithUsage("node is required", usage)
	}
	if *tty {
		if strings.TrimSpace(*stdinFile) != "" {
			return errorsWithUsage("stdin-file cannot be used with --tty", usage)
		}
		return application.runContainerExecTTY(strings.TrimSpace(*nodeID), fs.Arg(0), fs.Args()[1:], uint32(*rows), uint32(*cols))
	}
	stdin, err := readExecStdin(*stdinFile)
	if err != nil {
		return err
	}
	response, err := application.client.containers.RunContainerExec(application.ctx, newRequest(&controllerv1.RunContainerExecRequest{
		NodeId:         strings.TrimSpace(*nodeID),
		ContainerId:    fs.Arg(0),
		Command:        fs.Args()[1:],
		Stdin:          stdin,
		TimeoutSeconds: durationSeconds(*timeout),
		MaxOutputBytes: *maxOutput,
	}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		if err := application.printMessage(response.Msg); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprint(application.out, response.Msg.GetStdout()); err != nil {
			return err
		}
		if _, err := fmt.Fprint(application.errOut, response.Msg.GetStderr()); err != nil {
			return err
		}
	}
	if response.Msg.GetTimedOut() {
		return fmt.Errorf("container exec timed out after %s", *timeout)
	}
	if response.Msg.GetExitCode() != 0 {
		return fmt.Errorf("container exec exited with code %d", response.Msg.GetExitCode())
	}
	return nil
}

func (application *app) runContainerExecTTY(nodeID, containerID string, command []string, rows, cols uint32) error {
	if termRows, termCols, ok := terminalSize(os.Stdout.Fd()); ok {
		rows = termRows
		cols = termCols
	}
	origin, err := controllerOrigin(application.cfg.addr)
	if err != nil {
		return err
	}
	request := newRequest(&controllerv1.OpenContainerExecRequest{
		NodeId:      strings.TrimSpace(nodeID),
		ContainerId: containerID,
		Command:     command,
		Rows:        rows,
		Cols:        cols,
	})
	request.Header().Set("X-Composia-Web-Origin", origin)
	response, err := application.client.containers.OpenContainerExec(application.ctx, request)
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	wsURL, err := containerExecWebsocketURL(application.cfg.addr, response.Msg.GetWebsocketPath())
	if err != nil {
		return err
	}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{"Origin": []string{origin}})
	if err != nil {
		return fmt.Errorf("attach container exec websocket: %w", err)
	}
	defer func() { _ = conn.Close() }()
	return application.attachContainerExecWebsocket(conn)
}

func (application *app) attachContainerExecWebsocket(conn *websocket.Conn) error {
	fd := os.Stdin.Fd()
	state, rawErr := makeTerminalRaw(fd)
	if rawErr == nil {
		defer restoreTerminal(fd, state)
	}
	resizeCh := make(chan os.Signal, 1)
	signal.Notify(resizeCh, syscall.SIGWINCH)
	defer signal.Stop(resizeCh)

	var writeMu sync.Mutex
	writeMessage := func(messageType int, payload []byte) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.WriteMessage(messageType, payload)
	}

	errCh := make(chan error, 2)
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buffer)
			if n > 0 {
				if writeErr := writeMessage(websocket.BinaryMessage, append([]byte(nil), buffer[:n]...)); writeErr != nil {
					errCh <- writeErr
					return
				}
			}
			if err != nil {
				if err == io.EOF {
					_ = writeMessage(websocket.TextMessage, []byte(`{"type":"close"}`))
					errCh <- nil
					return
				}
				errCh <- err
				return
			}
		}
	}()
	go func() {
		for {
			messageType, payload, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					errCh <- nil
					return
				}
				errCh <- err
				return
			}
			switch messageType {
			case websocket.BinaryMessage:
				if _, err := application.out.Write(payload); err != nil {
					errCh <- err
					return
				}
			case websocket.TextMessage:
				if done, err := handleExecWebsocketEvent(payload); done || err != nil {
					errCh <- err
					return
				}
			}
		}
	}()
	for {
		select {
		case <-application.ctx.Done():
			return nil
		case <-resizeCh:
			if rows, cols, ok := terminalSize(os.Stdout.Fd()); ok {
				payload, _ := json.Marshal(execResizeMessage{Type: "resize", Rows: rows, Cols: cols})
				_ = writeMessage(websocket.TextMessage, payload)
			}
		case err := <-errCh:
			return err
		}
	}
}

type execResizeMessage struct {
	Type string `json:"type"`
	Rows uint32 `json:"rows"`
	Cols uint32 `json:"cols"`
}

type execWebsocketEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type terminalState struct {
	termios *unix.Termios
}

func handleExecWebsocketEvent(payload []byte) (bool, error) {
	var event execWebsocketEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return false, nil
	}
	switch event.Type {
	case "ready":
		return false, nil
	case "closed":
		return true, nil
	case "error":
		if event.Message == "" {
			return true, fmt.Errorf("container exec websocket error")
		}
		return true, fmt.Errorf("container exec websocket error: %s", event.Message)
	default:
		return false, nil
	}
}

func readExecStdin(path string) ([]byte, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	if path == "-" {
		return io.ReadAll(os.Stdin)
	}
	return os.ReadFile(path)
}

func durationSeconds(duration time.Duration) uint32 {
	if duration <= 0 {
		return 0
	}
	seconds := duration / time.Second
	if duration%time.Second != 0 {
		seconds++
	}
	return uint32(seconds)
}

func controllerOrigin(addr string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(addr))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("controller address must include scheme and host")
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func containerExecWebsocketURL(addr, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(addr))
	if err != nil {
		return "", err
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported controller scheme %q", parsed.Scheme)
	}
	parsed.Path = path
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String(), nil
}

func makeTerminalRaw(fd uintptr) (*terminalState, error) {
	oldState, err := unix.IoctlGetTermios(int(fd), unix.TCGETS)
	if err != nil {
		return nil, err
	}
	raw := *oldState
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(int(fd), unix.TCSETS, &raw); err != nil {
		return nil, err
	}
	return &terminalState{termios: oldState}, nil
}

func restoreTerminal(fd uintptr, state *terminalState) {
	if state == nil || state.termios == nil {
		return
	}
	_ = unix.IoctlSetTermios(int(fd), unix.TCSETS, state.termios)
}

func terminalSize(fd uintptr) (uint32, uint32, bool) {
	size, err := unix.IoctlGetWinsize(int(fd), unix.TIOCGWINSZ)
	if err != nil || size == nil || size.Row == 0 || size.Col == 0 {
		return 0, 0, false
	}
	return uint32(size.Row), uint32(size.Col), true
}

func containerActionFromName(name string) (controllerv1.ContainerAction, error) {
	switch name {
	case "start":
		return controllerv1.ContainerAction_CONTAINER_ACTION_START, nil
	case "stop":
		return controllerv1.ContainerAction_CONTAINER_ACTION_STOP, nil
	case "restart":
		return controllerv1.ContainerAction_CONTAINER_ACTION_RESTART, nil
	default:
		return controllerv1.ContainerAction_CONTAINER_ACTION_UNSPECIFIED, fmt.Errorf("unknown container action %q", name)
	}
}
