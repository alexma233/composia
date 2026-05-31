package rpcutil

import (
	"context"
	"errors"
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

func TestServerBearerAuthInterceptorRejectsMissingHeader(t *testing.T) {
	t.Parallel()

	interceptor := NewServerBearerAuthInterceptor(func(string) (string, error) {
		return "", nil
	})
	req := connect.NewRequest(&emptypb.Empty{})
	wrapped := interceptor.WrapUnary(func(context.Context, connect.AnyRequest) (connect.AnyResponse, error) {
		t.Fatalf("next should not be called")
		return nil, nil
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
		header := header
		t.Run(header, func(t *testing.T) {
			t.Parallel()
			if _, err := bearerToken(header); err == nil {
				t.Fatalf("expected error for %q", header)
			}
		})
	}
}
