package rpcutil

import "testing"

func TestJoinBaseURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		baseURL  string
		basePath string
		want     string
	}{
		{name: "trims slashes", baseURL: " http://127.0.0.1:7001/ ", basePath: " /api/controller/ ", want: "http://127.0.0.1:7001/api/controller"},
		{name: "adds slash", baseURL: "http://127.0.0.1:7001", basePath: "api/agent", want: "http://127.0.0.1:7001/api/agent"},
		{name: "empty path", baseURL: "http://127.0.0.1:7001/", basePath: "", want: "http://127.0.0.1:7001"},
		{name: "root path", baseURL: "http://127.0.0.1:7001/", basePath: "/", want: "http://127.0.0.1:7001"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := JoinBaseURL(tt.baseURL, tt.basePath); got != tt.want {
				t.Fatalf("JoinBaseURL = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPrefixRPCPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		basePath string
		rpcPath  string
		want     string
	}{
		{name: "joins normalized paths", basePath: " /api/controller/ ", rpcPath: " /proto.Service/Method ", want: "/api/controller/proto.Service/Method"},
		{name: "adds leading slashes", basePath: "api/agent", rpcPath: "proto.Service/Method", want: "/api/agent/proto.Service/Method"},
		{name: "empty base", basePath: "", rpcPath: "proto.Service/Method", want: "/proto.Service/Method"},
		{name: "root base", basePath: "/", rpcPath: "/proto.Service/Method", want: "/proto.Service/Method"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := PrefixRPCPath(tt.basePath, tt.rpcPath); got != tt.want {
				t.Fatalf("PrefixRPCPath = %q, want %q", got, tt.want)
			}
		})
	}
}
