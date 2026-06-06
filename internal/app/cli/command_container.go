package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"github.com/gorilla/websocket"
)

func (application *app) runContainer(args []string) error {
	if len(args) < 2 {
		return errors.New("usage: composia container <node> <list|get|logs|start|stop|restart|remove|exec>")
	}
	nodeID := strings.TrimSpace(args[0])
	if nodeID == "" {
		return errors.New("node is required")
	}
	switch args[1] {
	case "list": //nolint:goconst
		return application.runContainerList(nodeID, args[2:])
	case "get": //nolint:goconst
		return application.runContainerGet(nodeID, args[2:])
	case "logs":
		return application.runContainerLogs(nodeID, args[2:])
	case "start", "stop", "restart":
		return application.runContainerAction(nodeID, args[1], args[2:])
	case "remove":
		return application.runContainerRemove(nodeID, args[2:])
	case "exec":
		return application.runContainerExec(nodeID, args[2:])
	default:
		return fmt.Errorf("unknown container command %q", args[1])
	}
}

func (application *app) runContainerList(nodeID string, args []string) error {
	fs := newCommandFlagSet("container list")
	search := fs.String("search", "", "search text")
	sortBy := fs.String("sort-by", "", "sort field")
	sortDesc := fs.Bool("desc", false, "sort descending")
	pageValues, _ := parsePageFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := "composia container <node> list [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]"
	if err := requireArgs(fs.Args(), 0, usage); err != nil {
		return err
	}
	pageSize, page, err := pageValues()
	if err != nil {
		return err
	}
	response, err := application.client.docker.ListNodeContainers(application.ctx, newRequest(&controllerv1.ListNodeContainersRequest{
		NodeId:   nodeID,
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

func (application *app) runContainerGet(nodeID string, args []string) error {
	fs := newCommandFlagSet("container get")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia container <node> get <container>"); err != nil {
		return err
	}
	response, err := application.client.docker.InspectNodeContainer(application.ctx, newRequest(&controllerv1.InspectNodeContainerRequest{NodeId: nodeID, ContainerId: fs.Arg(0)}))
	if err != nil {
		return err
	}
	if application.isJSONOutput() {
		return application.printMessage(response.Msg)
	}
	_, err = fmt.Fprintln(application.out, response.Msg.GetRawJson())
	return err
}

func (application *app) runContainerLogs(nodeID string, args []string) error {
	fs := newCommandFlagSet("container logs")
	tail := fs.String("tail", "100", "number of lines or all")
	timestamps := fs.Bool("timestamps", false, "include timestamps")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := requireArgs(fs.Args(), 1, "composia container <node> logs [--tail n|all] [--timestamps] <container>"); err != nil {
		return err
	}
	stream, err := application.client.dockerCommands.GetContainerLogs(application.ctx, newRequest(&controllerv1.GetContainerLogsRequest{NodeId: nodeID, ContainerId: fs.Arg(0), Tail: *tail, Timestamps: *timestamps}))
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

func (application *app) runContainerAction(nodeID string, actionName string, args []string) error {
	action, err := containerActionFromName(actionName)
	if err != nil {
		return err
	}
	fs := newCommandFlagSet("container " + actionName)
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := fmt.Sprintf("composia container <node> %s [--wait] [--follow] [--timeout duration] <container>", actionName)
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	response, err := application.client.dockerCommands.RunContainerAction(application.ctx, newRequest(&controllerv1.RunContainerActionRequest{NodeId: nodeID, ContainerId: fs.Arg(0), Action: action}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runContainerRemove(nodeID string, args []string) error {
	fs := newCommandFlagSet("container remove")
	force := fs.Bool("force", false, "force remove")
	volumes := fs.Bool("volumes", false, "remove anonymous volumes")
	yes := addYesFlag(fs)
	waitOptions := addWaitFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	usage := "composia container <node> remove [--yes] [--wait] [--follow] [--timeout duration] [--force] [--volumes] <container>"
	if err := requireArgs(fs.Args(), 1, usage); err != nil {
		return err
	}
	if err := application.confirmDestructive(fmt.Sprintf("This will remove container %q on node %q.", fs.Arg(0), nodeID), yes); err != nil {
		return err
	}
	response, err := application.client.dockerCommands.RemoveContainer(application.ctx, newRequest(&controllerv1.RemoveContainerRequest{NodeId: nodeID, ContainerId: fs.Arg(0), Force: *force, RemoveVolumes: *volumes}))
	if err != nil {
		return err
	}
	return application.printTaskActionWithWait(response.Msg, waitOptions)
}

func (application *app) runContainerExec(nodeID string, args []string) error {
	fs := newCommandFlagSet("container exec")
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
	usage := "composia container <node> exec [--tty] [--stdin-file file] [--timeout duration] [--max-output bytes] <container> <command> [args...]"
	if len(fs.Args()) < 2 {
		return fmt.Errorf("usage: %s", usage)
	}
	if *tty {
		if strings.TrimSpace(*stdinFile) != "" {
			return errorsWithUsage("stdin-file cannot be used with --tty", usage)
		}
		rowCount, err := uint32FlagValue("rows", *rows)
		if err != nil {
			return err
		}
		colCount, err := uint32FlagValue("cols", *cols)
		if err != nil {
			return err
		}
		return application.runContainerExecTTY(nodeID, fs.Arg(0), fs.Args()[1:], rowCount, colCount)
	}
	stdin, err := readExecStdin(*stdinFile)
	if err != nil {
		return err
	}
	timeoutSeconds, err := durationSeconds(*timeout)
	if err != nil {
		return err
	}
	response, err := application.client.dockerCommands.RunContainerExec(application.ctx, newRequest(&controllerv1.RunContainerExecRequest{
		NodeId:         nodeID,
		ContainerId:    fs.Arg(0),
		Command:        fs.Args()[1:],
		Stdin:          stdin,
		TimeoutSeconds: timeoutSeconds,
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
	response, err := application.client.dockerCommands.OpenContainerExec(application.ctx, request)
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
	dialHeaders := staticHTTPHeaders(application.cfg.headers)
	dialHeaders.Set("Origin", origin)
	conn, dialResponse, err := websocket.DefaultDialer.Dial(wsURL, dialHeaders)
	if dialResponse != nil && dialResponse.Body != nil {
		defer func() { _ = dialResponse.Body.Close() }()
	}
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
	resizeCh, stopResize := subscribeTerminalResize()
	defer stopResize()

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
			return application.ctx.Err()
		case <-resizeCh:
			if rows, cols, ok := terminalSize(os.Stdout.Fd()); ok {
				payload, err := json.Marshal(execResizeMessage{Type: "resize", Rows: rows, Cols: cols})
				if err != nil {
					return fmt.Errorf("marshal resize message: %w", err)
				}
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

func handleExecWebsocketEvent(payload []byte) (bool, error) {
	var event execWebsocketEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		return false, fmt.Errorf("invalid container exec websocket event JSON: %w", err)
	}
	switch event.Type {
	case "ready":
		return false, nil
	case "closed":
		return true, nil
	case "error":
		if event.Message == "" {
			return true, errors.New("container exec websocket error")
		}
		return true, fmt.Errorf("container exec websocket error: %s", event.Message)
	default:
		return false, fmt.Errorf("unknown container exec websocket event type %q", event.Type)
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
	return os.ReadFile(path) //nolint:gosec
}

func durationSeconds(duration time.Duration) (uint32, error) {
	if duration <= 0 {
		return 0, nil
	}
	seconds := duration / time.Second
	if duration%time.Second != 0 {
		seconds++
	}
	if seconds > time.Duration(^uint32(0)) {
		return 0, fmt.Errorf("timeout exceeds maximum uint32 seconds value %d", uint64(^uint32(0)))
	}
	return uint32(seconds), nil
}

func controllerOrigin(addr string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(addr))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("controller address must include scheme and host")
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
