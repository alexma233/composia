package agent

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"

	"connectrpc.com/connect"
	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1/agentv1connect"
	"github.com/moby/moby/client"
)

type execTunnelClient struct {
	client agentv1connect.AgentReportServiceClient
}

type runningExecSession struct {
	id     string
	execID string
	attach client.HijackedResponse
	mu     sync.Mutex
	closed bool
}

type runningExecSessions struct {
	mu       sync.Mutex
	sessions map[string]*runningExecSession
}

func startExecTunnelLoop(ctx context.Context, client agentv1connect.AgentReportServiceClient, nodeID string) {
	_ = nodeID
	tunnel := &execTunnelClient{client: client}
	go tunnel.run(ctx)
}

func (tunnel *execTunnelClient) run(ctx context.Context) {
	stream := tunnel.client.OpenExecTunnel(ctx)
	sessions := &runningExecSessions{sessions: make(map[string]*runningExecSession)}

	for {
		message, err := stream.Receive()
		if err != nil {
			return
		}
		if err := tunnel.handleMessage(ctx, stream, sessions, message); err != nil {
			return
		}
	}
}

func (tunnel *execTunnelClient) handleMessage(ctx context.Context, stream *connect.BidiStreamForClient[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse], sessions *runningExecSessions, message *agentv1.OpenExecTunnelResponse) error {
	switch message.GetKind() {
	case execKindStart:
		return tunnel.handleStartMessage(ctx, stream, sessions, message)
	case execKindStdin:
		tunnel.handleStdinMessage(stream, sessions.get(message.GetSessionId()), message)
	case execKindResize:
		tunnel.handleResizeMessage(ctx, stream, sessions.get(message.GetSessionId()), message)
	case execKindClose:
		tunnel.handleCloseMessage(sessions.take(message.GetSessionId()))
	}
	return nil
}

func (tunnel *execTunnelClient) handleStartMessage(ctx context.Context, stream *connect.BidiStreamForClient[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse], sessions *runningExecSessions, message *agentv1.OpenExecTunnelResponse) error {
	session, err := startDockerExecSession(ctx, message)
	if err != nil {
		sendExecTunnelError(stream, message.GetSessionId(), err)
		sendExecTunnelClosed(stream, message.GetSessionId())
		return nil
	}
	sessions.set(session)
	sendExecTunnelMessage(stream, &agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindReady})
	go pumpExecOutput(ctx, stream, session, func() {
		sessions.delete(session.id)
	})
	return nil
}

func (tunnel *execTunnelClient) handleStdinMessage(stream *connect.BidiStreamForClient[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse], session *runningExecSession, message *agentv1.OpenExecTunnelResponse) {
	if session == nil {
		return
	}
	if _, err := session.attach.Conn.Write(message.GetPayload()); err != nil {
		sendExecTunnelError(stream, session.id, err)
		closeRunningExecSession(session)
	}
}

func (tunnel *execTunnelClient) handleResizeMessage(ctx context.Context, stream *connect.BidiStreamForClient[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse], session *runningExecSession, message *agentv1.OpenExecTunnelResponse) {
	if session == nil {
		return
	}
	if err := resizeRunningExecSession(ctx, session, message.GetRows(), message.GetCols()); err != nil {
		sendExecTunnelError(stream, session.id, err)
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
	sessions.sessions[session.id] = session
}

func (sessions *runningExecSessions) delete(sessionID string) {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	delete(sessions.sessions, sessionID)
}

func (sessions *runningExecSessions) take(sessionID string) *runningExecSession {
	sessions.mu.Lock()
	defer sessions.mu.Unlock()
	session := sessions.sessions[sessionID]
	delete(sessions.sessions, sessionID)
	return session
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
	return &runningExecSession{id: message.GetSessionId(), execID: createResult.ID, attach: attachResult.HijackedResponse}, nil
}

func resizeRunningExecSession(ctx context.Context, session *runningExecSession, rows, cols uint32) error {
	dockerClient, err := NewDockerClient()
	if err != nil {
		return err
	}
	defer dockerClient.Close()
	_, err = dockerClient.cli.ExecResize(ctx, session.execID, client.ExecResizeOptions{Height: uint(rows), Width: uint(cols)})
	if err != nil {
		return fmt.Errorf("resize docker exec: %w", err)
	}
	return nil
}

func pumpExecOutput(ctx context.Context, stream *connect.BidiStreamForClient[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse], session *runningExecSession, onClose func()) {
	defer onClose()
	defer closeRunningExecSession(session)
	buffer := make([]byte, 4096)
	for {
		n, err := session.attach.Reader.Read(buffer)
		if n > 0 {
			payload := append([]byte(nil), buffer[:n]...)
			if sendErr := sendExecTunnelMessage(stream, &agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindOutput, Payload: payload}); sendErr != nil {
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				sendExecTunnelError(stream, session.id, err)
			}
			sendExecTunnelClosed(stream, session.id)
			return
		}
		if ctx.Err() != nil {
			sendExecTunnelClosed(stream, session.id)
			return
		}
	}
}

func sendExecTunnelMessage(stream *connect.BidiStreamForClient[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse], message *agentv1.OpenExecTunnelRequest) error {
	return stream.Send(message)
}

func sendExecTunnelError(stream *connect.BidiStreamForClient[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse], sessionID string, err error) {
	_ = sendExecTunnelMessage(stream, &agentv1.OpenExecTunnelRequest{SessionId: sessionID, Kind: execKindError, Payload: []byte(err.Error())})
}

func sendExecTunnelClosed(stream *connect.BidiStreamForClient[agentv1.OpenExecTunnelRequest, agentv1.OpenExecTunnelResponse], sessionID string) {
	_ = sendExecTunnelMessage(stream, &agentv1.OpenExecTunnelRequest{SessionId: sessionID, Kind: execKindClosed})
}

func closeRunningExecSession(session *runningExecSession) {
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return
	}
	session.closed = true
	session.attach.Close()
}
