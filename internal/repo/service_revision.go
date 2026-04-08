package repo

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

func FindServiceAtRevision(repoDir, revision, serviceDir string, availableNodeIDs map[string]struct{}) (Service, error) {
	if strings.TrimSpace(revision) == "" {
		return Service{}, fmt.Errorf("service revision is required")
	}
	relativeMetaPath := filepath.ToSlash(filepath.Join(serviceDir, MetaFileName))
	content, err := ReadFileAtRevision(repoDir, revision, relativeMetaPath)
	if err != nil {
		return Service{}, err
	}

	var meta ServiceMeta
	decoder := yaml.NewDecoder(strings.NewReader(content))
	decoder.KnownFields(true)
	if err := decoder.Decode(&meta); err != nil {
		return Service{}, fmt.Errorf("decode service meta %q at revision %q: %w", relativeMetaPath, revision, err)
	}

	service, err := strictServiceFromMeta(relativeMetaPath, meta, availableNodeIDs)
	if err != nil {
		return Service{}, err
	}
	service.Directory = filepath.Join(repoDir, filepath.FromSlash(serviceDir))
	service.MetaPath = filepath.Join(service.Directory, MetaFileName)
	return service, nil
}
