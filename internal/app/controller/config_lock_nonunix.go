//go:build !unix

package controller

import "sync"

var controllerConfigProcessLock sync.Mutex

func lockControllerConfig(_ string) (func(), error) {
	controllerConfigProcessLock.Lock()
	return controllerConfigProcessLock.Unlock, nil
}
