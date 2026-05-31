package rpcutil

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestStaticBearerAuthInterceptorAddsAuthorizationHeader(t *testing.T) {
	t.Parallel()

	interceptor := NewStaticBearerAuthInterceptor("secret-token")
	req := connect.NewRequest(&emptypb.Empty{})
	wrapped := interceptor.WrapUnary(func(_ context.Context, gotReq connect.AnyRequest) (connect.AnyResponse, error) {
		if got := gotReq.Header().Get("Authorization"); got != "Bearer secret-token" {
			t.Fatalf("Authorization header = %q", got)
		}
		return connect.NewResponse(&emptypb.Empty{}), nil
	})

	if _, err := wrapped(context.Background(), req); err != nil {
		t.Fatalf("wrapped unary returned error: %v", err)
	}
}

func TestServerBearerAuthInterceptorAuthenticatesSubject(t *testing.T) {
	t.Parallel()

	interceptor := NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "secret-token" {
			return "", errors.New("bad token")
		}
		return "node-1", nil
	})
	req := connect.NewRequest(&emptypb.Empty{})
	req.Header().Set("Authorization", "Bearer secret-token")
	wrapped := interceptor.WrapUnary(func(ctx context.Context, _ connect.AnyRequest) (connect.AnyResponse, error) {
		subject, ok := BearerSubject(ctx)
		if !ok || subject != "node-1" {
			t.Fatalf("BearerSubject = %q, %v", subject, ok)
		}
		return connect.NewResponse(&emptypb.Empty{}), nil
	})

	if _, err := wrapped(context.Background(), req); err != nil {
		t.Fatalf("wrapped unary returned error: %v", err)
	}
}

func TestStaticBearerAuthInterceptorAddsStreamingClientAuthorizationHeader(t *testing.T) {
	t.Parallel()

	interceptor := NewStaticBearerAuthInterceptor("secret-token")
	wrapped := interceptor.WrapStreamingClient(func(context.Context, connect.Spec) connect.StreamingClientConn {
		return &testStreamingClientConn{requestHeader: make(http.Header)}
	})
	conn := wrapped(context.Background(), connect.Spec{})

	if got := conn.RequestHeader().Get("Authorization"); got != "Bearer secret-token" {
		t.Fatalf("Authorization header = %q", got)
	}
}

func TestServerBearerAuthInterceptorAuthenticatesStreamingSubject(t *testing.T) {
	t.Parallel()

	interceptor := NewServerBearerAuthInterceptor(func(token string) (string, error) {
		if token != "secret-token" {
			return "", errors.New("bad token")
		}
		return "node-1", nil
	})
	conn := &testStreamingHandlerConn{requestHeader: http.Header{"Authorization": []string{"Bearer secret-token"}}}
	wrapped := interceptor.WrapStreamingHandler(func(ctx context.Context, _ connect.StreamingHandlerConn) error {
		subject, ok := BearerSubject(ctx)
		if !ok || subject != "node-1" {
			t.Fatalf("BearerSubject = %q, %v", subject, ok)
		}
		return nil
	})

	if err := wrapped(context.Background(), conn); err != nil {
		t.Fatalf("wrapped streaming handler returned error: %v", err)
	}
}

func TestServerBearerAuthInterceptorRejectsStreamingMissingHeader(t *testing.T) {
	t.Parallel()

	interceptor := NewServerBearerAuthInterceptor(func(string) (string, error) {
		return "", nil
	})
	wrapped := interceptor.WrapStreamingHandler(func(context.Context, connect.StreamingHandlerConn) error {
		t.Fatalf("next should not be called")
		return nil
	})

	err := wrapped(context.Background(), &testStreamingHandlerConn{requestHeader: make(http.Header)})
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("error code = %v, want unauthenticated, err=%v", connect.CodeOf(err), err)
	}
}

func TestServerBearerAuthInterceptorRejectsMissingHeader(t *testing.T) {
	t.Parallel()

	interceptor := NewServerBearerAuthInterceptor(func(string) (string, error) {
		return "", errors.New("unexpected validate call")
	})
	req := connect.NewRequest(&emptypb.Empty{})
	wrapped := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatalf("next should not be called")
		return nil, errors.New("next should not be called")
	})

	_, err := wrapped(context.Background(), req)
	if connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("error code = %v, want unauthenticated, err=%v", connect.CodeOf(err), err)
	}
}

func TestBearerToken(t *testing.T) {
	t.Parallel()

	token, err := bearerToken(" Bearer secret-token ")
	if err != nil {
		t.Fatalf("bearerToken returned error: %v", err)
	}
	if token != "secret-token" {
		t.Fatalf("token = %q", token)
	}

	for _, header := range []string{"", "Basic token", "Bearer   "} {
		t.Run(header, func(t *testing.T) {
			t.Parallel()
			if _, err := bearerToken(header); err == nil {
				t.Fatalf("expected error for %q", header)
			}
		})
	}
}

type testStreamingClientConn struct {
	requestHeader   http.Header
	responseHeader  http.Header
	responseTrailer http.Header
}

func (conn *testStreamingClientConn) Spec() connect.Spec { return connect.Spec{} }

func (conn *testStreamingClientConn) Peer() connect.Peer { return connect.Peer{} }

func (conn *testStreamingClientConn) Send(any) error { return nil }

func (conn *testStreamingClientConn) RequestHeader() http.Header { return conn.requestHeader }

func (conn *testStreamingClientConn) CloseRequest() error { return nil }

func (conn *testStreamingClientConn) Receive(any) error { return nil }

func (conn *testStreamingClientConn) ResponseHeader() http.Header { return conn.responseHeader }

func (conn *testStreamingClientConn) ResponseTrailer() http.Header { return conn.responseTrailer }

func (conn *testStreamingClientConn) CloseResponse() error { return nil }

type testStreamingHandlerConn struct {
	requestHeader   http.Header
	responseHeader  http.Header
	responseTrailer http.Header
}

func (conn *testStreamingHandlerConn) Spec() connect.Spec { return connect.Spec{} }

func (conn *testStreamingHandlerConn) Peer() connect.Peer { return connect.Peer{} }

func (conn *testStreamingHandlerConn) Receive(any) error { return nil }

func (conn *testStreamingHandlerConn) RequestHeader() http.Header { return conn.requestHeader }

func (conn *testStreamingHandlerConn) Send(any) error { return nil }

func (conn *testStreamingHandlerConn) ResponseHeader() http.Header { return conn.responseHeader }

func (conn *testStreamingHandlerConn) ResponseTrailer() http.Header { return conn.responseTrailer }
