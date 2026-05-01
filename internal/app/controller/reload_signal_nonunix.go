//go:build !unix

package controller

import "context"

func watchControllerReloadSignals(_ context.Context, _ chan<- reloadRequest) func() {
	return func() {}
}
