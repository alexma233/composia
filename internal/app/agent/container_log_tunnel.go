package agent

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
)

const (
	containerLogKindReady  = "ready"
	containerLogKindStart  = "start"
	containerLogKindClose  = "close"
	containerLogKindChunk  = "chunk"
	containerLogKindError  = "error"
	containerLogKindClosed = "closed"
)

type containerLogTunnelClient struct {
	nodeID string
	client agentv1connect.AgentReportServiceClient
}

type runningContainerLogSession struct {
	id     string
	cancel context.CancelFunc
}

type runningContainerLogSessions struct {
	mu       sync.Mutex
	sessions map[string]*runningContainerLogSession
}

func startContainerLogTunnelLoop(ctx context.Context, client agentv1connect.AgentReportServiceClient, nodeID string) {
	tunnel := &containerLogTunnelClient{nodeID: nodeID, client: client}
	go tunnel.run(ctx)
}

func (tunnel *containerLogTunnelClient) run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := tunnel.runStream(ctx); err != nil && ctx.Err() == nil {
			log.Printf("container log tunnel disconnected for node %s: %v", tunnel.nodeID, err)
		}
		if !sleepWithContext(ctx, time.Second) {
			return
		}
	}
}

func (tunnel *containerLogTunnelClient) runStream(ctx context.Context) error {
	stream := tunnel.client.OpenContainerLogTunnel(ctx)
	sendCh := make(chan *agentv1.OpenContainerLogTunnelRequest, 256)
	recvCh := make(chan *agentv1.OpenContainerLogTunnelResponse)
	errCh := make(chan error, 2)
	sessions := &runningContainerLogSessions{sessions: make(map[string]*runningContainerLogSession)}
	defer sessions.cancelAll()

	go tunnel.sendLoop(ctx, stream, sendCh, errCh)
	go tunnel.receiveLoop(stream, recvCh, errCh)
	if err := tunnel.sendMessage(ctx, sendCh, &agentv1.OpenContainerLogTunnelRequest{Kind: containerLogKindReady}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-errCh:
			if errors.Is(err, context.Canceled) && ctx.Err() != nil {
				return nil
			}
			return err
		case message, ok := <-recvCh:
			if !ok {
				return nil
			}
			if err := tunnel.handleMessage(ctx, sendCh, sessions, message); err != nil {
				return err
			}
		}
	}
}

func (tunnel *containerLogTunnelClient) sendLoop(ctx context.Context, stream *connect.BidiStreamForClient[agentv1.OpenContainerLogTunnelRequest, agentv1.OpenContainerLogTunnelResponse], sendCh <-chan *agentv1.OpenContainerLogTunnelRequest, errCh chan<- error) {
	for {
		select {
		case <-ctx.Done():
			return
		case message, ok := <-sendCh:
			if !ok {
				return
			}
			if err := stream.Send(message); err != nil {
				errCh <- err
				return
			}
		}
	}
}

func (tunnel *containerLogTunnelClient) receiveLoop(stream *connect.BidiStreamForClient[agentv1.OpenContainerLogTunnelRequest, agentv1.OpenContainerLogTunnelResponse], recvCh chan<- *agentv1.OpenContainerLogTunnelResponse, errCh chan<- error) {
	defer close(recvCh)
	for {
		message, err := stream.Receive()
		if err != nil {
			errCh <- err
			return
		}
		recvCh <- message
	}
}

func (tunnel *containerLogTunnelClient) handleMessage(ctx context.Context, sendCh chan<- *agentv1.OpenContainerLogTunnelRequest, sessions *runningContainerLogSessions, message *agentv1.OpenContainerLogTunnelResponse) error {
	switch message.GetKind() {
	case containerLogKindStart:
		tunnel.handleStartMessage(ctx, sendCh, sessions, message)
	case containerLogKindClose:
		sessions.cancel(message.GetSessionId())
	}
	return nil
}

func (tunnel *containerLogTunnelClient) handleStartMessage(ctx context.Context, sendCh chan<- *agentv1.OpenContainerLogTunnelRequest, sessions *runningContainerLogSessions, message *agentv1.OpenContainerLogTunnelResponse) {
	if message.GetSessionId() == "" || message.GetContainerId() == "" {
		return
	}
	sessionCtx, cancel := context.WithCancel(ctx)
	sessions.set(&runningContainerLogSession{id: message.GetSessionId(), cancel: cancel})
	go tunnel.streamContainerLogs(sessionCtx, sendCh, sessions, message)
}

func (tunnel *containerLogTunnelClient) streamContainerLogs(ctx context.Context, sendCh chan<- *agentv1.OpenContainerLogTunnelRequest, sessions *runningContainerLogSessions, message *agentv1.OpenContainerLogTunnelResponse) {
	defer sessions.delete(message.GetSessionId())
	server, err := newDockerServer()
	if err != nil {
		tunnel.sendError(ctx, sendCh, message.GetSessionId(), err)
		return
	}
	defer func() { _ = server.client.Close() }()

	err = server.streamContainerLogs(ctx, message.GetContainerId(), message.GetTail(), message.GetTimestamps(), true, func(content string) error {
		return tunnel.sendMessage(ctx, sendCh, &agentv1.OpenContainerLogTunnelRequest{
			SessionId: message.GetSessionId(),
			Kind:      containerLogKindChunk,
			Content:   content,
		})
	})
	if err != nil {
		tunnel.sendError(ctx, sendCh, message.GetSessionId(), err)
		return
	}
	if ctx.Err() == nil {
		_ = tunnel.sendMessage(ctx, sendCh, &agentv1.OpenContainerLogTunnelRequest{SessionId: message.GetSessionId(), Kind: containerLogKindClosed})
	}
}

func (tunnel *containerLogTunnelClient) sendError(ctx context.Context, sendCh chan<- *agentv1.OpenContainerLogTunnelRequest, sessionID string, err error) {
	if ctx.Err() != nil {
		return
	}
	_ = tunnel.sendMessage(ctx, sendCh, &agentv1.OpenContainerLogTunnelRequest{
		SessionId:    sessionID,
		Kind:         containerLogKindError,
		ErrorMessage: err.Error(),
		ErrorCode:    dockerQueryErrorCode(err),
	})
	_ = tunnel.sendMessage(ctx, sendCh, &agentv1.OpenContainerLogTunnelRequest{SessionId: sessionID, Kind: containerLogKindClosed})
}

func (tunnel *containerLogTunnelClient) sendMessage(ctx context.Context, sendCh chan<- *agentv1.OpenContainerLogTunnelRequest, message *agentv1.OpenContainerLogTunnelRequest) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case sendCh <- message:
		return nil
	}
}

func (sessions *runningContainerLogSessions) set(session *runningContainerLogSession) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if current := sessions.sessions[session.id]; current != nil {
		current.cancel()
	}
	sessions.sessions[session.id] = session
}

func (sessions *runningContainerLogSessions) delete(sessionID string) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	delete(sessions.sessions, sessionID)
}

func (sessions *runningContainerLogSessions) cancel(sessionID string) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	session := sessions.sessions[sessionID]
	if session == nil {
		return
	}
	delete(sessions.sessions, sessionID)
	session.cancel()
}

func (sessions *runningContainerLogSessions) cancelAll() {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	for sessionID, session := range sessions.sessions {
		session.cancel()
		delete(sessions.sessions, sessionID)
	}
}
