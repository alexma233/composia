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

func startExecTunnelLoop(ctx context.Context, client agentv1connect.AgentReportServiceClient, nodeID string) {
	_ = nodeID
	tunnel := &execTunnelClient{client: client}
	go tunnel.run(ctx)
}

func (tunnel *execTunnelClient) run(ctx context.Context) {
	stream := tunnel.client.OpenExecTunnel(ctx)
	sessions := make(map[string]*runningExecSession)
	var sessionsMu sync.Mutex

	for {
		message, err := stream.Receive()
		if err != nil {
			return
		}
		switch message.GetKind() {
		case execKindStart:
			session, startErr := startDockerExecSession(ctx, message)
			if startErr != nil {
				_ = stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: message.GetSessionId(), Kind: execKindError, Payload: []byte(startErr.Error())})
				_ = stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: message.GetSessionId(), Kind: execKindClosed})
				continue
			}
			sessionsMu.Lock()
			sessions[session.id] = session
			sessionsMu.Unlock()
			_ = stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindReady})
			go pumpExecOutput(ctx, stream, session, func() {
				sessionsMu.Lock()
				delete(sessions, session.id)
				sessionsMu.Unlock()
			})
		case execKindStdin:
			sessionsMu.Lock()
			session := sessions[message.GetSessionId()]
			sessionsMu.Unlock()
			if session != nil {
				if _, err := session.attach.Conn.Write(message.GetPayload()); err != nil {
					_ = stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindError, Payload: []byte(err.Error())})
					closeRunningExecSession(session)
				}
			}
		case execKindResize:
			sessionsMu.Lock()
			session := sessions[message.GetSessionId()]
			sessionsMu.Unlock()
			if session != nil {
				if err := resizeRunningExecSession(ctx, session, message.GetRows(), message.GetCols()); err != nil {
					_ = stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindError, Payload: []byte(err.Error())})
				}
			}
		case execKindClose:
			sessionsMu.Lock()
			session := sessions[message.GetSessionId()]
			delete(sessions, message.GetSessionId())
			sessionsMu.Unlock()
			if session != nil {
				closeRunningExecSession(session)
			}
		}
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
			if sendErr := stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindOutput, Payload: payload}); sendErr != nil {
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				_ = stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindError, Payload: []byte(err.Error())})
			}
			_ = stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindClosed})
			return
		}
		if ctx.Err() != nil {
			_ = stream.Send(&agentv1.OpenExecTunnelRequest{SessionId: session.id, Kind: execKindClosed})
			return
		}
	}
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
