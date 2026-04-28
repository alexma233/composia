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

func FindRusticInfraServiceAtRevision(repoDir, revision string, availableNodeIDs map[string]struct{}) (Service, error) {
	if strings.TrimSpace(revision) == "" {
		return Service{}, fmt.Errorf("rustic infra service revision is required")
	}
	services, err := ListFilesAtRevision(repoDir, revision, "")
	if err != nil {
		return Service{}, err
	}

	var matched *Service
	for _, path := range services {
		if filepath.Base(path) != MetaFileName {
			continue
		}
		content, err := ReadFileAtRevision(repoDir, revision, filepath.ToSlash(path))
		if err != nil {
			return Service{}, err
		}

		var meta ServiceMeta
		decoder := yaml.NewDecoder(strings.NewReader(content))
		decoder.KnownFields(true)
		if err := decoder.Decode(&meta); err != nil {
			return Service{}, fmt.Errorf("decode service meta %q at revision %q: %w", path, revision, err)
		}
		if meta.Infra == nil || meta.Infra.Rustic == nil {
			continue
		}

		service, err := strictServiceFromMeta(filepath.ToSlash(path), meta, availableNodeIDs)
		if err != nil {
			return Service{}, err
		}
		service.Directory = filepath.Join(repoDir, filepath.FromSlash(filepath.Dir(path)))
		service.MetaPath = filepath.Join(service.Directory, MetaFileName)
		if matched != nil {
			return Service{}, fmt.Errorf("rustic infra service is declared more than once: %s and %s", matched.MetaPath, service.MetaPath)
		}
		matched = &service
	}
	if matched != nil {
		return *matched, nil
	}
	return Service{}, fmt.Errorf("rustic infra service is not declared")
}
