package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/task"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	execKindStart  = "start"
	execKindStdin  = "stdin"
	execKindResize = "resize"
	execKindClose  = "close"

	execKindReady  = "ready"
	execKindOutput = "output"
	execKindError  = "error"
	execKindClosed = "closed"

	execAttachTokenTTL  = time.Minute
	execWebOriginHeader = "X-Composia-Web-Origin"
)

type execTunnelManager struct {
	mu           sync.Mutex
	tunnels      map[string]*agentExecTunnel
	sessions     map[string]*execSession
	attachTokens map[string]string
	upgrader     websocket.Upgrader
}

type agentExecTunnel struct {
	nodeID string
	sendCh chan *agentv1.OpenExecTunnelResponse
}

type execSession struct {
	id               string
	nodeID           string
	containerID      string
	command          []string
	rows             uint32
	cols             uint32
	incoming         chan *agentv1.OpenExecTunnelRequest
	createdAt        time.Time
	mu               sync.Mutex
	browserTaken     bool
	browserAttaching bool
	closed           bool
	attachToken      string
	attachExpiresAt  time.Time
	allowedOrigin    string
	createdBy        string
}

type execWSControlMessage struct {
	Type string `json:"type"`
	Rows uint32 `json:"rows,omitempty"`
	Cols uint32 `json:"cols,omitempty"`
}

type execWSEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
	Session string `json:"session_id,omitempty"`
}

type execOriginContextKey struct{}

type execTunnelTarget struct {
	session *execSession
	tunnel  *agentExecTunnel
}

func newExecTunnelManager() *execTunnelManager {
	manager := &execTunnelManager{
		tunnels:      make(map[string]*agentExecTunnel),
		sessions:     make(map[string]*execSession),
		attachTokens: make(map[string]string),
	}
	manager.upgrader = websocket.Upgrader{CheckOrigin: manager.checkOrigin}
	return manager
}

func (manager *execTunnelManager) registerTunnel(nodeID string) *agentExecTunnel {
	tunnel := &agentExecTunnel{
		nodeID: nodeID,
		sendCh: make(chan *agentv1.OpenExecTunnelResponse, 256),
	}
	manager.mu.Lock()
	manager.tunnels[nodeID] = tunnel
	manager.mu.Unlock()
	return tunnel
}

func (manager *execTunnelManager) unregisterTunnel(nodeID string, tunnel *agentExecTunnel) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	current := manager.tunnels[nodeID]
	if current == tunnel {
		delete(manager.tunnels, nodeID)
		close(tunnel.sendCh)
	}
	for _, session := range manager.sessions {
		if session.nodeID == nodeID {
			manager.closeSessionLocked(session.id)
		}
	}
}

func (manager *execTunnelManager) hasTunnel(nodeID string) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepExpiredSessionsLocked(time.Now().UTC())
	_, ok := manager.tunnels[nodeID]
	return ok
}

func (manager *execTunnelManager) openSession(nodeID, containerID string, command []string, rows, cols uint32, allowedOrigin, createdBy string) (*execSession, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepExpiredSessionsLocked(time.Now().UTC())
	tunnel := manager.tunnels[nodeID]
	if tunnel == nil {
		return nil, fmt.Errorf("node %q has no active exec tunnel", nodeID)
	}
	attachToken := uuid.NewString()
	now := time.Now().UTC()
	session := &execSession{
		id:              uuid.NewString(),
		nodeID:          nodeID,
		containerID:     containerID,
		command:         append([]string(nil), command...),
		rows:            rows,
		cols:            cols,
		incoming:        make(chan *agentv1.OpenExecTunnelRequest, 256),
		createdAt:       now,
		attachToken:     attachToken,
		attachExpiresAt: now.Add(execAttachTokenTTL),
		allowedOrigin:   allowedOrigin,
		createdBy:       createdBy,
	}
	manager.sessions[session.id] = session
	manager.attachTokens[attachToken] = session.id
	tunnel.sendCh <- &agentv1.OpenExecTunnelResponse{
		SessionId:   session.id,
		Kind:        execKindStart,
		ContainerId: containerID,
		Command:     append([]string(nil), command...),
		Rows:        rows,
		Cols:        cols,
	}
	return session, nil
}

func (manager *execTunnelManager) sendToSessionNode(sessionID string, message *agentv1.OpenExecTunnelResponse) error {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	target, err := manager.sessionTargetLocked(sessionID)
	if err != nil {
		return err
	}
	target.tunnel.sendCh <- message
	return nil
}

func (manager *execTunnelManager) sessionTargetLocked(sessionID string) (execTunnelTarget, error) {
	manager.sweepExpiredSessionsLocked(time.Now().UTC())
	session := manager.sessions[sessionID]
	if session == nil || session.closed {
		return execTunnelTarget{}, fmt.Errorf("session %q is closed", sessionID)
	}
	tunnel := manager.tunnels[session.nodeID]
	if tunnel == nil {
		return execTunnelTarget{}, fmt.Errorf("node %q tunnel is unavailable", session.nodeID)
	}
	return execTunnelTarget{session: session, tunnel: tunnel}, nil
}

func (manager *execTunnelManager) deliverFromAgent(message *agentv1.OpenExecTunnelRequest) {
	session := manager.lookupSession(message.GetSessionId())
	if session == nil {
		return
	}
	select {
	case session.incoming <- message:
	default:
		manager.closeSession(message.GetSessionId())
	}
}

func (manager *execTunnelManager) lookupSession(sessionID string) *execSession {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepExpiredSessionsLocked(time.Now().UTC())
	return manager.sessions[sessionID]
}

func (manager *execTunnelManager) reserveSessionAttach(attachToken string) (*execSession, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepExpiredSessionsLocked(time.Now().UTC())
	session, err := manager.sessionForAttachTokenLocked(attachToken)
	if err != nil {
		return nil, err
	}
	if session.browserTaken || session.browserAttaching {
		return nil, fmt.Errorf("session %q is already attached", session.id)
	}
	session.browserAttaching = true
	return session, nil
}

func (manager *execTunnelManager) confirmSessionAttach(attachToken string) (*execSession, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepExpiredSessionsLocked(time.Now().UTC())
	session, err := manager.sessionForAttachTokenLocked(attachToken)
	if err != nil {
		return nil, err
	}
	if session.browserTaken {
		return nil, fmt.Errorf("session %q is already attached", session.id)
	}
	if !session.browserAttaching {
		return nil, fmt.Errorf("session %q is not awaiting attach", session.id)
	}
	session.browserAttaching = false
	session.browserTaken = true
	delete(manager.attachTokens, attachToken)
	session.attachToken = ""
	return session, nil
}

func (manager *execTunnelManager) releaseSessionAttach(attachToken string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepExpiredSessionsLocked(time.Now().UTC())
	session, err := manager.sessionForAttachTokenLocked(attachToken)
	if err != nil || session.browserTaken {
		return
	}
	session.browserAttaching = false
}

func (manager *execTunnelManager) closeSession(sessionID string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.closeSessionLocked(sessionID)
}

func (manager *execTunnelManager) closeSessionLocked(sessionID string) {
	session := manager.sessions[sessionID]
	if session == nil || session.closed {
		return
	}
	session.closed = true
	session.browserAttaching = false
	if session.attachToken != "" {
		delete(manager.attachTokens, session.attachToken)
		session.attachToken = ""
	}
	close(session.incoming)
	delete(manager.sessions, sessionID)
	if tunnel := manager.tunnels[session.nodeID]; tunnel != nil {
		select {
		case tunnel.sendCh <- &agentv1.OpenExecTunnelResponse{SessionId: sessionID, Kind: execKindClose}:
		default:
		}
	}
}

func (manager *execTunnelManager) handleWebsocket(w http.ResponseWriter, r *http.Request) {
	attachToken := execAttachTokenFromPath(r.URL.Path)
	if attachToken == "" {
		http.NotFound(w, r)
		return
	}
	_, err := manager.reserveSessionAttach(attachToken)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	conn, err := manager.upgrader.Upgrade(w, r, nil)
	if err != nil {
		manager.releaseSessionAttach(attachToken)
		return
	}
	session, err := manager.confirmSessionAttach(attachToken)
	if err != nil {
		_ = conn.Close()
		return
	}
	defer conn.Close()
	defer manager.closeSession(session.id)

	if err := writeExecWSEvent(conn, execKindReady, session.id, ""); err != nil {
		return
	}

	readErrCh := make(chan error, 1)
	go func() {
		readErrCh <- manager.readBrowserMessages(conn, session)
	}()

	for {
		select {
		case err := <-readErrCh:
			if err != nil && !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				_ = writeExecWSEvent(conn, execKindError, session.id, err.Error())
			}
			return
		case message, ok := <-session.incoming:
			if !manager.forwardAgentMessageToBrowser(conn, session.id, message, ok) {
				return
			}
		}
	}
}

func (manager *execTunnelManager) checkOrigin(r *http.Request) bool {
	origin, err := normalizeOrigin(r.Header.Get("Origin"))
	if err != nil {
		return false
	}
	attachToken := execAttachTokenFromPath(r.URL.Path)
	if attachToken == "" {
		return false
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	manager.sweepExpiredSessionsLocked(time.Now().UTC())
	session, err := manager.sessionForAttachTokenLocked(attachToken)
	if err != nil {
		return false
	}
	return origin == session.allowedOrigin
}

func (manager *execTunnelManager) forwardAgentMessageToBrowser(conn *websocket.Conn, sessionID string, message *agentv1.OpenExecTunnelRequest, ok bool) bool {
	if !ok {
		_ = writeExecWSEvent(conn, execKindClosed, sessionID, "")
		return false
	}
	switch message.GetKind() {
	case execKindOutput:
		if err := conn.WriteMessage(websocket.BinaryMessage, append([]byte(nil), message.GetPayload()...)); err != nil {
			return false
		}
	case execKindError:
		if err := writeExecWSEvent(conn, execKindError, sessionID, string(message.GetPayload())); err != nil {
			return false
		}
	case execKindClosed:
		_ = writeExecWSEvent(conn, execKindClosed, sessionID, string(message.GetPayload()))
		return false
	}
	return true
}

func (manager *execTunnelManager) readBrowserMessages(conn *websocket.Conn, session *execSession) error {
	for {
		messageType, payload, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		switch messageType {
		case websocket.BinaryMessage:
			if err := manager.sendBrowserPayload(session.id, execKindStdin, payload); err != nil {
				return err
			}
		case websocket.TextMessage:
			var control execWSControlMessage
			if err := json.Unmarshal(payload, &control); err == nil && control.Type != "" {
				switch control.Type {
				case execKindResize:
					if err := manager.sendBrowserResize(session.id, control.Rows, control.Cols); err != nil {
						return err
					}
				case execKindClose:
					return nil
				default:
					if err := manager.sendBrowserPayload(session.id, execKindStdin, payload); err != nil {
						return err
					}
				}
				continue
			}
			if err := manager.sendBrowserPayload(session.id, execKindStdin, payload); err != nil {
				return err
			}
		}
	}
}

func (manager *execTunnelManager) sendBrowserPayload(sessionID, kind string, payload []byte) error {
	return manager.sendToSessionNode(sessionID, &agentv1.OpenExecTunnelResponse{SessionId: sessionID, Kind: kind, Payload: payload})
}

func (manager *execTunnelManager) sendBrowserResize(sessionID string, rows, cols uint32) error {
	return manager.sendToSessionNode(sessionID, &agentv1.OpenExecTunnelResponse{SessionId: sessionID, Kind: execKindResize, Rows: rows, Cols: cols})
}

func (manager *execTunnelManager) sweepExpiredSessionsLocked(now time.Time) {
	for _, session := range manager.sessions {
		if session.closed || session.browserTaken || session.browserAttaching || session.attachToken == "" {
			continue
		}
		if !now.Before(session.attachExpiresAt) {
			manager.closeSessionLocked(session.id)
		}
	}
}

func (manager *execTunnelManager) sessionForAttachTokenLocked(attachToken string) (*execSession, error) {
	sessionID := manager.attachTokens[attachToken]
	if sessionID == "" {
		return nil, fmt.Errorf("exec attach token %q not found", attachToken)
	}
	session := manager.sessions[sessionID]
	if session == nil || session.closed {
		delete(manager.attachTokens, attachToken)
		return nil, fmt.Errorf("session %q is closed", sessionID)
	}
	if session.attachToken != attachToken {
		return nil, fmt.Errorf("session %q attach token mismatch", sessionID)
	}
	if !session.attachExpiresAt.After(time.Now().UTC()) {
		manager.closeSessionLocked(session.id)
		return nil, fmt.Errorf("session %q attach token expired", session.id)
	}
	return session, nil
}

func execAttachTokenFromPath(path string) string {
	attachToken := strings.Trim(strings.TrimPrefix(path, "/ws/container-exec/"), "/")
	if strings.TrimSpace(attachToken) == "" {
		return ""
	}
	return attachToken
}

func normalizeOrigin(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("origin must include scheme and host")
	}
	if parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("origin must not include path, query, or fragment")
	}
	return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host), nil
}

func writeExecWSEvent(conn *websocket.Conn, kind, sessionID, message string) error {
	return conn.WriteJSON(execWSEvent{Type: kind, Message: message, Session: sessionID})
}

func (server *agentReportServer) OpenExecTunnel(ctx context.Context, stream *connect.BidiStream[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse]) error {
	nodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || strings.TrimSpace(nodeID) == "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer subject"))
	}
	manager := server.execManager
	if manager == nil {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("exec manager is not configured"))
	}
	tunnel := manager.registerTunnel(nodeID)
	defer manager.unregisterTunnel(nodeID, tunnel)

	recvErrCh := make(chan error, 1)
	go func() {
		for {
			message, recvErr := stream.Receive()
			if recvErr != nil {
				recvErrCh <- recvErr
				return
			}
			manager.deliverFromAgent(message)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case recvErr := <-recvErrCh:
			if errors.Is(recvErr, context.Canceled) {
				return nil
			}
			return recvErr
		case message, ok := <-tunnel.sendCh:
			if !ok {
				return nil
			}
			if err := stream.Send(message); err != nil {
				return err
			}
		}
	}
}

func (server *containerServer) openExecSession(ctx context.Context, nodeID, containerID string, command []string, rows, cols uint32) (*execSession, error) {
	allowedOrigin, _ := ctx.Value(execOriginContextKey{}).(string)
	allowedOrigin = strings.TrimSpace(allowedOrigin)
	if allowedOrigin == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s is required", execWebOriginHeader))
	}
	createdBy, ok := rpcutil.BearerSubject(ctx)
	if !ok || strings.TrimSpace(createdBy) == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer subject"))
	}
	if server.execManager == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("exec manager is not configured"))
	}
	if !server.execManager.hasTunnel(nodeID) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q has no active exec tunnel", nodeID))
	}
	return server.execManager.openSession(nodeID, containerID, command, rows, cols, allowedOrigin, createdBy)
}

func (server *containerServer) OpenContainerExec(ctx context.Context, req *connect.Request[controllerv1.OpenContainerExecRequest]) (*connect.Response[controllerv1.OpenContainerExecResponse], error) {
	if req.Msg == nil || req.Msg.GetNodeId() == "" || req.Msg.GetContainerId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("node_id and container_id are required"))
	}
	allowedOrigin, err := normalizeOrigin(req.Header().Get(execWebOriginHeader))
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("%s: %w", execWebOriginHeader, err))
	}
	ctx = context.WithValue(ctx, execOriginContextKey{}, allowedOrigin)
	if err := validateTaskTargetNode(ctx, server.db, server.cfg, req.Msg.GetNodeId(), task.TypeDockerStart); err != nil {
		return nil, err
	}
	command := req.Msg.GetCommand()
	if len(command) == 0 {
		command = []string{"/bin/sh"}
	}
	session, err := server.openExecSession(ctx, req.Msg.GetNodeId(), req.Msg.GetContainerId(), command, req.Msg.GetRows(), req.Msg.GetCols())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&controllerv1.OpenContainerExecResponse{
		SessionId:     session.id,
		WebsocketPath: "/ws/container-exec/" + session.attachToken,
	}), nil
}
