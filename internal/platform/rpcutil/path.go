package rpcutil

import "strings"

const (
	ControllerAPIBasePath = "/api/controller"
	AgentAPIBasePath      = "/api/agent"
	ControllerExecWSPath  = ControllerAPIBasePath + "/ws/container-exec/"
)

func JoinBaseURL(baseURL, basePath string) string {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return baseURL
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	return baseURL + strings.TrimRight(basePath, "/")
}

func PrefixRPCPath(basePath, rpcPath string) string {
	basePath = strings.TrimSpace(basePath)
	rpcPath = strings.TrimSpace(rpcPath)
	if basePath == "" || basePath == "/" {
		if strings.HasPrefix(rpcPath, "/") {
			return rpcPath
		}
		return "/" + rpcPath
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimRight(basePath, "/")
	if !strings.HasPrefix(rpcPath, "/") {
		rpcPath = "/" + rpcPath
	}
	return basePath + rpcPath
}
