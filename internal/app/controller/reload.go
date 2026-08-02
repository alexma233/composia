package controller

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

type reloadRequest struct {
	reply            chan error
	expectedRevision string
}

func (request reloadRequest) respond(err error) {
	if request.reply == nil {
		return
	}
	request.reply <- err
}

func requestControllerReload(ctx context.Context, requests chan<- reloadRequest) error {
	return requestControllerReloadRevision(ctx, requests, "")
}

func requestControllerReloadRevision(ctx context.Context, requests chan<- reloadRequest, expectedRevision string) error {
	// Live RPCs must return before the runtime shuts down; waiting here would make shutdown wait on this handler.
	request := reloadRequest{expectedRevision: expectedRevision}
	select {
	case requests <- request:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("controller config reload is already queued")
	}
}

func requestControllerReloadAndWait(ctx context.Context, requests chan<- reloadRequest, expectedRevision string) error {
	reply := make(chan error, 1)
	request := reloadRequest{reply: reply, expectedRevision: expectedRevision}
	select {
	case requests <- request:
	case <-ctx.Done():
		return ctx.Err()
	}
	select {
	case err := <-reply:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func loadReloadControllerConfig(configPath string, current *config.ControllerConfig) (*config.ControllerConfig, error) {
	next, err := config.LoadController(configPath)
	if err != nil {
		return nil, err
	}
	if err := validateControllerReload(current, next); err != nil {
		return nil, err
	}
	return next, nil
}

func validateControllerReload(current, next *config.ControllerConfig) error {
	if current == nil || next == nil {
		return errors.New("controller config is missing")
	}
	immutable := []struct {
		name  string
		left  string
		right string
		path  bool
	}{
		{name: "controller.listen_addr", left: current.ListenAddr, right: next.ListenAddr},
		{name: "controller.repo_dir", left: current.RepoDir, right: next.RepoDir, path: true},
		{name: "controller.state_dir", left: current.StateDir, right: next.StateDir, path: true},
		{name: "controller.log_dir", left: current.LogDir, right: next.LogDir, path: true},
	}
	for _, field := range immutable {
		left := field.left
		right := field.right
		if field.path {
			left = filepath.Clean(left)
			right = filepath.Clean(right)
		}
		if left != right {
			return fmt.Errorf("%s changed and requires process restart", field.name)
		}
	}
	return nil
}
