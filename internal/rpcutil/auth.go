package rpcutil

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"connectrpc.com/connect"
)

type bearerSubjectContextKey struct{}

func NewServerBearerAuthInterceptor(validate func(token string) (string, error)) connect.Interceptor {
	return bearerAuthInterceptor{
		validate: validate,
	}
}

func NewStaticBearerAuthInterceptor(token string) connect.Interceptor {
	return bearerAuthInterceptor{staticToken: token}
}

func BearerSubject(ctx context.Context) (string, bool) {
	subject, ok := ctx.Value(bearerSubjectContextKey{}).(string)
	return subject, ok
}

type bearerAuthInterceptor struct {
	validate    func(token string) (string, error)
	staticToken string
}

func (interceptor bearerAuthInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		if interceptor.validate == nil {
			req.Header().Set("Authorization", "Bearer "+interceptor.staticToken)
			return next(ctx, req)
		}

		authenticatedContext, err := interceptor.authenticatedContext(ctx, req.Header().Get("Authorization"))
		if err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, err)
		}
		return next(authenticatedContext, req)
	}
}

func (interceptor bearerAuthInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	if interceptor.validate != nil {
		return next
	}
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("Authorization", "Bearer "+interceptor.staticToken)
		return conn
	}
}

func (interceptor bearerAuthInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	if interceptor.validate == nil {
		return next
	}
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		authenticatedContext, err := interceptor.authenticatedContext(ctx, conn.RequestHeader().Get("Authorization"))
		if err != nil {
			return connect.NewError(connect.CodeUnauthenticated, err)
		}
		return next(authenticatedContext, conn)
	}
}

func (interceptor bearerAuthInterceptor) authenticatedContext(ctx context.Context, authorizationHeader string) (context.Context, error) {
	token, err := bearerToken(authorizationHeader)
	if err != nil {
		return nil, err
	}
	subject, err := interceptor.validate(token)
	if err != nil {
		return nil, err
	}
	return context.WithValue(ctx, bearerSubjectContextKey{}, subject), nil
}

func bearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return "", errors.New("missing Authorization header")
	}

	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", fmt.Errorf("invalid Authorization header")
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, prefix))
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	return token, nil
}
