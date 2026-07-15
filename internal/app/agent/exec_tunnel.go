package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"github.com/moby/moby/client"
)

type execTunnelClient struct {
	nodeID string
	client agentv1connect.AgentReportServiceClient
}

type runningExecSession struct {
	id     string
	execID string
	client *DockerClient
	attach client.HijackedResponse
	mu     sync.Mutex
	closed bool
}

type execTunnelSendStream interface {
	Send(*agentv1.OpenExecTunnelRequest) error
}

type execTunnelSender struct {
	ctx      context.Context
	cancel   context.CancelFunc
	stream   execTunnelSendStream
	messages chan *agentv1.OpenExecTunnelRequest
	done     chan struct{}
	errMu    sync.Mutex
	err      error
}

type runningExecSessions struct {
	mu       sync.Mutex
	sessions map[string]*runningExecSession
}

func startExecTunnelLoop(ctx context.Context, client agentv1connect.AgentReportServiceClient, nodeID string) {
	tunnel := &execTunnelClient{nodeID: nodeID, client: client}
	go tunnel.run(ctx)
}

func (tunnel *execTunnelClient) run(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		if err := tunnel.runStream(ctx); err != nil && ctx.Err() == nil {
			log.Printf("exec tunnel disconnected for node %s: %v", tunnel.nodeID, err)
		}
		if !sleepWithContext(ctx, 1*time.Second) {
			return
		}
	}
}

func (tunnel *execTunnelClient) runStream(ctx context.Context) error {
	stream := tunnel.client.OpenExecTunnel(ctx)
	sender := newExecTunnelSender(ctx, stream)
	defer func() { _ = sender.close() }()
	sessions := &runningExecSessions{sessions: make(map[string]*runningExecSession)}
	defer sessions.closeAll()
	if err := sender.send(&agentv1.OpenExecTunnelRequest{Kind: execKindReady}); err != nil {
		return err
	}

	for {
		message, err := stream.Receive()
		if err != nil {
			return err
		}
		if err := tunnel.handleMessage(ctx, sender, sessions, message); err != nil {
			return err
		}
	}
}

func (tunnel *execTunnelClient) handleMessage(ctx context.Context, sender *execTunnelSender, sessions *runningExecSessions, message *agentv1.OpenExecTunnelResponse) error {
	switch message.GetKind() {
	case execKindStart:
		return tunnel.handleStartMessage(ctx, sender, sessions, message)
	case execKindStdin:
		tunnel.handleStdinMessage(sender, sessions.get(message.GetSessionId()), message)
	case execKindResize:
		tunnel.handleResizeMessage(ctx, sender, sessions.get(message.GetSessionId()), message)
	case execKindClose:
		tunnel.handleCloseMessage(sessions.take(message.GetSessionId()))
	}
	return nil
}

func (tunnel *execTunnelClient) handleStartMessage(ctx context.Context, sender *execTunnelSender, sessions *runningExecSessions, message *agentv1.OpenExecTunnelResponse) error {
	session, err := startDockerExecSession(ctx, message)
	if err != nil {
		sendExecTunnelError(sender, message.GetSessionId(), err)
		sendExecTunnelClosed(sender, message.GetSessionId())
		return nil
	}
	sessions.set(session)
	if err := sender.send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindReady}); err != nil {
		sessions.deleteIfCurrent(session)
		closeRunningExecSession(session)
		return err
	}
	go pumpExecOutput(ctx, sender, session, func() {
		sessions.deleteIfCurrent(session)
	})
	return nil
}

func (tunnel *execTunnelClient) handleStdinMessage(sender *execTunnelSender, session *runningExecSession, message *agentv1.OpenExecTunnelResponse) {
	if session == nil {
		return
	}
	if _, err := session.attach.Conn.Write(message.GetPayload()); err != nil {
		sendExecTunnelError(sender, session.id, err)
		closeRunningExecSession(session)
	}
}

func (tunnel *execTunnelClient) handleResizeMessage(ctx context.Context, sender *execTunnelSender, session *runningExecSession, message *agentv1.OpenExecTunnelResponse) {
	if session == nil {
		return
	}
	if err := resizeRunningExecSession(ctx, session, message.GetRows(), message.GetCols()); err != nil {
		sendExecTunnelError(sender, session.id, err)
	}
}

func (tunnel *execTunnelClient) handleCloseMessage(session *runningExecSession) {
	if session == nil {
		return
	}
	closeRunningExecSession(session)
}

func (sessions *runningExecSessions) get(sessionID string) *runningExecSession {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	return sessions.sessions[sessionID]
}

func (sessions *runningExecSessions) set(session *runningExecSession) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if current := sessions.sessions[session.id]; current != nil && current != session {
		closeRunningExecSession(current)
	}
	sessions.sessions[session.id] = session
}

func (sessions *runningExecSessions) deleteIfCurrent(session *runningExecSession) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	if sessions.sessions[session.id] == session {
		delete(sessions.sessions, session.id)
	}
}

func (sessions *runningExecSessions) take(sessionID string) *runningExecSession {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	session := sessions.sessions[sessionID]
	delete(sessions.sessions, sessionID)
	return session
}

func (sessions *runningExecSessions) closeAll() {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	for sessionID, session := range sessions.sessions {
		closeRunningExecSession(session)
		delete(sessions.sessions, sessionID)
	}
}

func startDockerExecSession(ctx context.Context, message *agentv1.OpenExecTunnelResponse) (*runningExecSession, error) {
	dockerClient, err := NewDockerClient()
	if err != nil {
		return nil, err
	}
	createResult, err := dockerClient.cli.ExecCreate(ctx, message.GetContainerId(), client.ExecCreateOptions{
		TTY:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          append([]string(nil), message.GetCommand()...),
		ConsoleSize:  client.ConsoleSize{Height: uint(message.GetRows()), Width: uint(message.GetCols())},
	})
	if err != nil {
		_ = dockerClient.Close()
		return nil, fmt.Errorf("create docker exec: %w", err)
	}
	attachResult, err := dockerClient.cli.ExecAttach(ctx, createResult.ID, client.ExecAttachOptions{
		TTY:         true,
		ConsoleSize: client.ConsoleSize{Height: uint(message.GetRows()), Width: uint(message.GetCols())},
	})
	if err != nil {
		_ = dockerClient.Close()
		return nil, fmt.Errorf("attach docker exec: %w", err)
	}
	return &runningExecSession{id: message.GetSessionId(), execID: createResult.ID, client: dockerClient, attach: attachResult.HijackedResponse}, nil
}

func resizeRunningExecSession(ctx context.Context, session *runningExecSession, rows, cols uint32) error {
	dockerClient, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer func() { _ = dockerClient.Close() }()
	_, err = dockerClient.cli.ExecResize(ctx, session.execID, client.ExecResizeOptions{Height: uint(rows), Width: uint(cols)})
	if err != nil {
		return fmt.Errorf("resize docker exec: %w", err)
	}
	return nil
}

func pumpExecOutput(ctx context.Context, sender *execTunnelSender, session *runningExecSession, onClose func()) {
	defer onClose()
	defer closeRunningExecSession(session)
	buffer := make([]byte, 4096)
	for {
		n, err := session.attach.Reader.Read(buffer)
		if n > 0 {
			payload := append([]byte(nil), buffer[:n]...)
			if sendErr := sender.send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindOutput, Payload: payload}); sendErr != nil {
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				sendExecTunnelError(sender, session.id, err)
			}
			sendExecTunnelClosed(sender, session.id)
			return
		}
		if ctx.Err() != nil {
			sendExecTunnelClosed(sender, session.id)
			return
		}
	}
}

func newExecTunnelSender(ctx context.Context, stream execTunnelSendStream) *execTunnelSender {
	senderCtx, cancel := context.WithCancel(ctx)
	sender := &execTunnelSender{
		ctx:      senderCtx,
		cancel:   cancel,
		stream:   stream,
		messages: make(chan *agentv1.OpenExecTunnelRequest, 32),
		done:     make(chan struct{}),
	}
	go sender.run()
	return sender
}

func (sender *execTunnelSender) run() {
	defer close(sender.done)
	for {
		select {
		case <-sender.ctx.Done():
			return
		case message := <-sender.messages:
			if err := sender.stream.Send(message); err != nil {
				sender.setErr(err)
				sender.cancel()
				return
			}
		}
	}
}

func (sender *execTunnelSender) send(message *agentv1.OpenExecTunnelRequest) error {
	select {
	case <-sender.ctx.Done():
		return sender.errOrContext()
	case sender.messages <- message:
		return nil
	}
}

func (sender *execTunnelSender) close() error {
	sender.cancel()
	<-sender.done
	return sender.sendErr()
}

func (sender *execTunnelSender) setErr(err error) {
	sender.errMu.Lock()
	defer sender.errMu.Unlock()
	if sender.err == nil {
		sender.err = err
	}
}

func (sender *execTunnelSender) errOrContext() error {
	sender.errMu.Lock()
	defer sender.errMu.Unlock()
	if sender.err != nil {
		return sender.err
	}
	return sender.ctx.Err()
}

func (sender *execTunnelSender) sendErr() error {
	sender.errMu.Lock()
	defer sender.errMu.Unlock()
	return sender.err
}

func sendExecTunnelError(sender *execTunnelSender, sessionID string, err error) {
	_ = sender.send(&agentv1.OpenExecTunnelRequest{SessionId: sessionID, Kind: execKindError, Payload: []byte(err.Error())})
}

func sendExecTunnelClosed(sender *execTunnelSender, sessionID string) {
	_ = sender.send(&agentv1.OpenExecTunnelRequest{SessionId: sessionID, Kind: execKindClosed})
}

func closeRunningExecSession(session *runningExecSession) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return
	}
	session.closed = true
	session.attach.Close()
	if session.client != nil {
		_ = session.client.Close()
	}
}
