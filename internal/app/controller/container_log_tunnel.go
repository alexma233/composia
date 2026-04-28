package controller

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"github.com/google/uuid"
)

const (
	containerLogKindReady  = "ready"
	containerLogKindStart  = "start"
	containerLogKindClose  = "close"
	containerLogKindChunk  = "chunk"
	containerLogKindError  = "error"
	containerLogKindClosed = "closed"
)

type containerLogTunnelManager struct {
	mu       sync.Mutex
	tunnels  map[string]*agentContainerLogTunnel
	sessions map[string]*containerLogSession
}

type agentContainerLogTunnel struct {
	nodeID string
	sendCh chan *agentv1.OpenContainerLogTunnelResponse
}

type containerLogSession struct {
	id       string
	nodeID   string
	incoming chan *agentv1.OpenContainerLogTunnelRequest
	created  time.Time

	mu     sync.Mutex
	closed bool
}

func newContainerLogTunnelManager() *containerLogTunnelManager {
	return &containerLogTunnelManager{
		tunnels:  make(map[string]*agentContainerLogTunnel),
		sessions: make(map[string]*containerLogSession),
	}
}

func (manager *containerLogTunnelManager) registerTunnel(nodeID string) *agentContainerLogTunnel {
	tunnel := &agentContainerLogTunnel{
		nodeID: nodeID,
		sendCh: make(chan *agentv1.OpenContainerLogTunnelResponse, 256),
	}
	manager.mu.Lock()
	manager.tunnels[nodeID] = tunnel
	manager.mu.Unlock()
	return tunnel
}

func (manager *containerLogTunnelManager) unregisterTunnel(nodeID string, tunnel *agentContainerLogTunnel) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	if manager.tunnels[nodeID] == tunnel {
		delete(manager.tunnels, nodeID)
		close(tunnel.sendCh)
	}
	for sessionID, session := range manager.sessions {
		if session.nodeID != nodeID {
			continue
		}
		delete(manager.sessions, sessionID)
		session.close()
	}
}

func (manager *containerLogTunnelManager) hasTunnel(nodeID string) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	_, ok := manager.tunnels[nodeID]
	return ok
}

func (manager *containerLogTunnelManager) openSession(nodeID, containerID, tail string, timestamps bool) (*containerLogSession, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	tunnel := manager.tunnels[nodeID]
	if tunnel == nil {
		return nil, fmt.Errorf("node %q has no active container log tunnel", nodeID)
	}
	session := &containerLogSession{
		id:       uuid.NewString(),
		nodeID:   nodeID,
		incoming: make(chan *agentv1.OpenContainerLogTunnelRequest, 256),
		created:  time.Now().UTC(),
	}
	manager.sessions[session.id] = session
	tunnel.sendCh <- &agentv1.OpenContainerLogTunnelResponse{
		SessionId:   session.id,
		Kind:        containerLogKindStart,
		ContainerId: containerID,
		Tail:        tail,
		Timestamps:  timestamps,
	}
	return session, nil
}

func (manager *containerLogTunnelManager) closeSession(sessionID string) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	session := manager.sessions[sessionID]
	if session == nil {
		return
	}
	delete(manager.sessions, sessionID)
	session.close()
	if tunnel := manager.tunnels[session.nodeID]; tunnel != nil {
		tunnel.sendCh <- &agentv1.OpenContainerLogTunnelResponse{SessionId: sessionID, Kind: containerLogKindClose}
	}
}

func (manager *containerLogTunnelManager) deliverFromAgent(message *agentv1.OpenContainerLogTunnelRequest) {
	if message != nil && message.GetKind() == containerLogKindReady {
		return
	}
	if message == nil || message.GetSessionId() == "" {
		return
	}
	manager.mu.Lock()
	session := manager.sessions[message.GetSessionId()]
	if message.GetKind() == containerLogKindError || message.GetKind() == containerLogKindClosed {
		delete(manager.sessions, message.GetSessionId())
	}
	manager.mu.Unlock()
	if session == nil {
		return
	}
	if !session.deliver(message) {
		manager.closeSession(message.GetSessionId())
		return
	}
	if message.GetKind() == containerLogKindError || message.GetKind() == containerLogKindClosed {
		session.close()
	}
}

func (session *containerLogSession) deliver(message *agentv1.OpenContainerLogTunnelRequest) bool {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return false
	}
	select {
	case session.incoming <- message:
		return true
	default:
		return false
	}
}

func (session *containerLogSession) close() {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return
	}
	session.closed = true
	close(session.incoming)
}

func (server *agentReportServer) OpenContainerLogTunnel(ctx context.Context, stream *connect.BidiStream[agentv1.OpenContainerLogTunnelRequest, agentv1.OpenContainerLogTunnelResponse]) error {
	nodeID, ok := rpcutil.BearerSubject(ctx)
	if !ok || nodeID == "" {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer subject"))
	}
	manager := server.logManager
	if manager == nil {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("container log manager is not configured"))
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

func (server *containerServer) openContainerLogSession(nodeID, containerID, tail string, timestamps bool) (*containerLogSession, error) {
	if server.logManager == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("container log manager is not configured"))
	}
	if !server.logManager.hasTunnel(nodeID) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("node %q has no active container log tunnel", nodeID))
	}
	session, err := server.logManager.openSession(nodeID, containerID, tail, timestamps)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return session, nil
}

func containerLogStreamError(message *agentv1.OpenContainerLogTunnelRequest) error {
	if message == nil {
		return connect.NewError(connect.CodeInternal, errors.New("container log stream failed"))
	}
	text := message.GetErrorMessage()
	if text == "" {
		text = "container log stream failed"
	}
	return connect.NewError(dockerQueryConnectCode(message.GetErrorCode()), errors.New(text))
}
