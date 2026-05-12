package controller

import (
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
)

func configuredNodeIDs(cfg *config.ControllerConfig) map[string]struct{} {
	result := make(map[string]struct{}, len(cfg.Nodes))
	for _, node := range cfg.Nodes {
		result[node.ID] = struct{}{}
	}
	return result
}
