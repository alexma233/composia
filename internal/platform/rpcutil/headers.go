package rpcutil

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"connectrpc.com/connect"
)

var reservedStaticHeaderNames = map[string]struct{}{
	"authorization":            {},
	"connect-protocol-version": {},
	"content-type":             {},
	"grpc-accept-encoding":     {},
	"grpc-encoding":            {},
	"grpc-timeout":             {},
	"te":                       {},
}

func NewStaticHeadersInterceptor(headers map[string]string) (connect.Interceptor, error) {
	normalized, err := NormalizeStaticHeaders(headers)
	if err != nil {
		return nil, err
	}
	return staticHeadersInterceptor{headers: normalized}, nil
}

func NormalizeStaticHeaders(headers map[string]string) (map[string]string, error) {
	normalized := make(map[string]string, len(headers))
	for name, value := range headers {
		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)
		if name == "" {
			return nil, fmt.Errorf("custom header name must not be empty")
		}
		if value == "" {
			return nil, fmt.Errorf("custom header %q value must not be empty", name)
		}
		if !validHeaderName(name) {
			return nil, fmt.Errorf("custom header %q is invalid", name)
		}
		if _, reserved := reservedStaticHeaderNames[strings.ToLower(name)]; reserved {
			return nil, fmt.Errorf("custom header %q is reserved", name)
		}
		normalized[http.CanonicalHeaderKey(name)] = value
	}
	return normalized, nil
}

type staticHeadersInterceptor struct {
	headers map[string]string
}

func (interceptor staticHeadersInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		setStaticHeaders(req.Header(), interceptor.headers)
		return next(ctx, req)
	}
}

func (interceptor staticHeadersInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		setStaticHeaders(conn.RequestHeader(), interceptor.headers)
		return conn
	}
}

func (interceptor staticHeadersInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}

func setStaticHeaders(header http.Header, headers map[string]string) {
	for name, value := range headers {
		if header.Get(name) == "" {
			header.Set(name, value)
		}
	}
}

func validHeaderName(name string) bool {
	for _, char := range name {
		if char > 127 || !isTokenChar(byte(char)) {
			return false
		}
	}
	return true
}

func isTokenChar(char byte) bool {
	switch {
	case char >= '0' && char <= '9':
		return true
	case char >= 'A' && char <= 'Z':
		return true
	case char >= 'a' && char <= 'z':
		return true
	case strings.ContainsRune("!#$%&'*+-.^_`|~", rune(char)):
		return true
	default:
		return false
	}
}
