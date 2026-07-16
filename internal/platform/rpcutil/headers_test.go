package rpcutil

import (
	"context"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestNormalizeStaticHeaders(t *testing.T) {
	t.Parallel()

	headers, err := NormalizeStaticHeaders(map[string]string{" cf-access-client-id ": " id "})
	if err != nil {
		t.Fatalf("NormalizeStaticHeaders returned error: %v", err)
	}
	if got := headers["Cf-Access-Client-Id"]; got != "id" {
		t.Fatalf("header = %q", got)
	}
}

func TestNormalizeStaticHeadersRejectsReservedHeader(t *testing.T) {
	t.Parallel()

	_, err := NormalizeStaticHeaders(map[string]string{"Authorization": "Bearer token"})
	if err == nil {
		t.Fatalf("expected reserved header error")
	}
}

func TestNormalizeStaticHeadersRejectsCanonicalDuplicates(t *testing.T) {
	t.Parallel()

	_, err := NormalizeStaticHeaders(map[string]string{"x-trace-id": "one", "X-Trace-Id": "two"})
	if err == nil {
		t.Fatalf("expected duplicate header error")
	}
}

func TestStaticHeadersInterceptorAddsMissingHeaders(t *testing.T) {
	t.Parallel()

	interceptor, err := NewStaticHeadersInterceptor(map[string]string{"X-Trace-Id": "trace-1"})
	if err != nil {
		t.Fatalf("NewStaticHeadersInterceptor returned error: %v", err)
	}
	req := connect.NewRequest(&emptypb.Empty{})
	wrapped := interceptor.WrapUnary(func(_ context.Context, gotReq connect.AnyRequest) (connect.AnyResponse, error) {
		if got := gotReq.Header().Get("X-Trace-Id"); got != "trace-1" {
			t.Fatalf("X-Trace-Id = %q", got)
		}
		return connect.NewResponse(&emptypb.Empty{}), nil
	})

	if _, err := wrapped(context.Background(), req); err != nil {
		t.Fatalf("wrapped unary returned error: %v", err)
	}
}

func TestStaticHeadersInterceptorPreservesExistingHeader(t *testing.T) {
	t.Parallel()

	interceptor, err := NewStaticHeadersInterceptor(map[string]string{"X-Trace-Id": "configured"})
	if err != nil {
		t.Fatalf("NewStaticHeadersInterceptor returned error: %v", err)
	}
	req := connect.NewRequest(&emptypb.Empty{})
	req.Header().Set("X-Trace-Id", "existing")
	wrapped := interceptor.WrapUnary(func(_ context.Context, gotReq connect.AnyRequest) (connect.AnyResponse, error) {
		if got := gotReq.Header().Get("X-Trace-Id"); got != "existing" {
			t.Fatalf("X-Trace-Id = %q", got)
		}
		return connect.NewResponse(&emptypb.Empty{}), nil
	})

	if _, err := wrapped(context.Background(), req); err != nil {
		t.Fatalf("wrapped unary returned error: %v", err)
	}
}

func TestStaticHeadersInterceptorAddsStreamingClientHeaders(t *testing.T) {
	t.Parallel()

	interceptor, err := NewStaticHeadersInterceptor(map[string]string{"X-Trace-Id": "trace-1"})
	if err != nil {
		t.Fatalf("NewStaticHeadersInterceptor returned error: %v", err)
	}
	wrapped := interceptor.WrapStreamingClient(func(context.Context, connect.Spec) connect.StreamingClientConn {
		return &testStreamingClientConn{requestHeader: make(http.Header)}
	})
	conn := wrapped(context.Background(), connect.Spec{})

	if got := conn.RequestHeader().Get("X-Trace-Id"); got != "trace-1" {
		t.Fatalf("X-Trace-Id = %q", got)
	}
}
