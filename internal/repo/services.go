package repo

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const MetaFileName = "composia-meta.yaml"

type Service struct {
	Name      string
	Directory string
	MetaPath  string
	Node      string
	Enabled   bool
	Meta      ServiceMeta
}

type ServiceMeta struct {
	Name        string             `yaml:"name"`
	ProjectName string             `yaml:"project_name"`
	Enabled     *bool              `yaml:"enabled"`
	Node        string             `yaml:"node"`
	Network     *NetworkConfig     `yaml:"network"`
	Update      *UpdateConfig      `yaml:"update"`
	DataProtect *DataProtectConfig `yaml:"data_protect"`
	Backup      *BackupConfig      `yaml:"backup"`
	Migrate     *MigrateConfig     `yaml:"migrate"`
}

type NetworkConfig struct {
	Caddy *CaddyConfig `yaml:"caddy"`
	DNS   *DNSConfig   `yaml:"dns"`
}

type CaddyConfig struct {
	Enabled *bool  `yaml:"enabled"`
	Source  string `yaml:"source"`
}

type DNSConfig struct {
	Provider   string  `yaml:"provider"`
	Hostname   string  `yaml:"hostname"`
	RecordType string  `yaml:"record_type"`
	Value      string  `yaml:"value"`
	Proxied    *bool   `yaml:"proxied"`
	TTL        *uint32 `yaml:"ttl"`
	Comment    string  `yaml:"comment"`
}

type UpdateConfig struct {
	Enabled            *bool  `yaml:"enabled"`
	Strategy           string `yaml:"strategy"`
	Schedule           string `yaml:"schedule"`
	BackupBeforeUpdate *bool  `yaml:"backup_before_update"`
}

type DataProtectConfig struct {
	Data []DataProtectItem `yaml:"data"`
}

type DataProtectItem struct {
	Name    string            `yaml:"name"`
	Backup  *DataActionConfig `yaml:"backup"`
	Restore *DataActionConfig `yaml:"restore"`
}

type DataActionConfig struct {
	Strategy string   `yaml:"strategy"`
	Service  string   `yaml:"service"`
	Include  []string `yaml:"include"`
}

type BackupConfig struct {
	Data []BackupItem `yaml:"data"`
}

type BackupItem struct {
	Name     string `yaml:"name"`
	Provider string `yaml:"provider"`
	Enabled  *bool  `yaml:"enabled"`
	Schedule string `yaml:"schedule"`
	Retain   string `yaml:"retain"`
}

type MigrateConfig struct {
	Data []MigrateItem `yaml:"data"`
}

type MigrateItem struct {
	Name    string `yaml:"name"`
	Enabled *bool  `yaml:"enabled"`
}

func DiscoverServices(repoDir string, availableNodeIDs map[string]struct{}) ([]Service, error) {
	services := make([]Service, 0)
	seenServiceNames := make(map[string]string)

	err := filepath.WalkDir(repoDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() != MetaFileName {
			return nil
		}

		meta, err := loadServiceMeta(path)
		if err != nil {
			return err
		}
		if err := validateServiceMeta(path, &meta, availableNodeIDs); err != nil {
			return err
		}

		if previousPath, exists := seenServiceNames[meta.Name]; exists {
			return fmt.Errorf("service %q is declared more than once: %s and %s", meta.Name, previousPath, path)
		}
		seenServiceNames[meta.Name] = path

		targetNode := meta.Node
		if targetNode == "" {
			targetNode = "main"
		}

		services = append(services, Service{
			Name:      meta.Name,
			Directory: filepath.Dir(path),
			MetaPath:  path,
			Node:      targetNode,
			Enabled:   boolValue(meta.Enabled, true),
			Meta:      meta,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover services in %q: %w", repoDir, err)
	}

	return services, nil
}

func FindService(repoDir string, availableNodeIDs map[string]struct{}, serviceName string) (Service, error) {
	services, err := DiscoverServices(repoDir, availableNodeIDs)
	if err != nil {
		return Service{}, err
	}
	for _, service := range services {
		if service.Name == serviceName {
			return service, nil
		}
	}
	return Service{}, fmt.Errorf("service %q is not declared", serviceName)
}

func loadServiceMeta(path string) (ServiceMeta, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return ServiceMeta{}, fmt.Errorf("read service meta %q: %w", path, err)
	}

	var meta ServiceMeta
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&meta); err != nil {
		return ServiceMeta{}, fmt.Errorf("decode service meta %q: %w", path, err)
	}
	return meta, nil
}

func validateServiceMeta(path string, meta *ServiceMeta, availableNodeIDs map[string]struct{}) error {
	if meta.Name == "" {
		return fmt.Errorf("service meta %q: name is required", path)
	}

	targetNode := meta.Node
	if targetNode == "" {
		targetNode = "main"
	}
	if _, ok := availableNodeIDs[targetNode]; !ok {
		return fmt.Errorf("service meta %q: node %q is not configured", path, targetNode)
	}

	if meta.Network != nil {
		if err := validateNetwork(path, meta.Network); err != nil {
			return err
		}
	}
	if meta.Update != nil {
		if err := validateUpdate(path, meta.Update); err != nil {
			return err
		}
	}

	dataItems := make(map[string]DataProtectItem)
	if meta.DataProtect != nil {
		for _, item := range meta.DataProtect.Data {
			if item.Name == "" {
				return fmt.Errorf("service meta %q: data_protect.data[].name is required", path)
			}
			if _, exists := dataItems[item.Name]; exists {
				return fmt.Errorf("service meta %q: data_protect.data[%q] is duplicated", path, item.Name)
			}
			if err := validateDataProtectItem(path, item); err != nil {
				return err
			}
			dataItems[item.Name] = item
		}
	}

	if meta.Backup != nil {
		for _, item := range meta.Backup.Data {
			if item.Name == "" {
				return fmt.Errorf("service meta %q: backup.data[].name is required", path)
			}
			protectedData, ok := dataItems[item.Name]
			if !ok {
				return fmt.Errorf("service meta %q: backup.data[%q] references an unknown data_protect item", path, item.Name)
			}
			if protectedData.Backup == nil {
				return fmt.Errorf("service meta %q: backup.data[%q] requires data_protect.data[%q].backup", path, item.Name, item.Name)
			}
		}
	}

	if meta.Migrate != nil {
		for _, item := range meta.Migrate.Data {
			if item.Name == "" {
				return fmt.Errorf("service meta %q: migrate.data[].name is required", path)
			}
			protectedData, ok := dataItems[item.Name]
			if !ok {
				return fmt.Errorf("service meta %q: migrate.data[%q] references an unknown data_protect item", path, item.Name)
			}
			if protectedData.Backup == nil || protectedData.Restore == nil {
				return fmt.Errorf("service meta %q: migrate.data[%q] requires both backup and restore definitions", path, item.Name)
			}
		}
	}

	return nil
}

func validateNetwork(path string, network *NetworkConfig) error {
	if network.Caddy != nil && boolValue(network.Caddy.Enabled, false) && network.Caddy.Source == "" {
		return fmt.Errorf("service meta %q: network.caddy.source is required when caddy is enabled", path)
	}

	if network.DNS != nil {
		if network.DNS.Provider != "cloudflare" {
			return fmt.Errorf("service meta %q: network.dns.provider must be cloudflare", path)
		}
		if network.DNS.Hostname == "" {
			return fmt.Errorf("service meta %q: network.dns.hostname is required", path)
		}
		switch network.DNS.RecordType {
		case "", "A", "AAAA", "CNAME":
		default:
			return fmt.Errorf("service meta %q: network.dns.record_type must be A, AAAA, or CNAME", path)
		}
	}

	return nil
}

func validateUpdate(path string, update *UpdateConfig) error {
	if update.Strategy == "" {
		return fmt.Errorf("service meta %q: update.strategy is required", path)
	}
	if update.Strategy != "pull_and_recreate" {
		return fmt.Errorf("service meta %q: update.strategy must be pull_and_recreate", path)
	}
	return nil
}

func validateDataProtectItem(path string, item DataProtectItem) error {
	if item.Backup != nil {
		if err := validateDataAction(path, item.Name, "backup", item.Backup); err != nil {
			return err
		}
	}
	if item.Restore != nil {
		if err := validateDataAction(path, item.Name, "restore", item.Restore); err != nil {
			return err
		}
	}
	return nil
}

func validateDataAction(path, dataName, actionName string, action *DataActionConfig) error {
	if action.Strategy == "" {
		return fmt.Errorf("service meta %q: data_protect.data[%q].%s.strategy is required", path, dataName, actionName)
	}

	if strings.HasPrefix(action.Strategy, "database.") {
		if action.Service == "" {
			return fmt.Errorf("service meta %q: data_protect.data[%q].%s.service is required for %s", path, dataName, actionName, action.Strategy)
		}
		switch action.Strategy {
		case "database.pgdumpall", "database.pgimport":
			return nil
		default:
			return fmt.Errorf("service meta %q: unsupported %s strategy %q", path, actionName, action.Strategy)
		}
	}

	if strings.HasPrefix(action.Strategy, "files.") {
		if len(action.Include) == 0 {
			return fmt.Errorf("service meta %q: data_protect.data[%q].%s.include is required for %s", path, dataName, actionName, action.Strategy)
		}
		switch action.Strategy {
		case "files.tar_after_stop", "files.untar", "files.copy":
			return nil
		default:
			return fmt.Errorf("service meta %q: unsupported %s strategy %q", path, actionName, action.Strategy)
		}
	}

	return fmt.Errorf("service meta %q: unsupported %s strategy %q", path, actionName, action.Strategy)
}

func boolValue(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func EnabledBackupDataNames(service Service) []string {
	if service.Meta.Backup == nil {
		return nil
	}
	names := make([]string, 0, len(service.Meta.Backup.Data))
	for _, item := range service.Meta.Backup.Data {
		if item.Name == "" {
			continue
		}
		if item.Enabled != nil && !*item.Enabled {
			continue
		}
		names = append(names, item.Name)
	}
	sort.Strings(names)
	return names
}

func ValidateRequestedBackupDataNames(service Service, requested []string) ([]string, error) {
	enabled := EnabledBackupDataNames(service)
	if len(requested) == 0 {
		if len(enabled) == 0 {
			return nil, fmt.Errorf("service %q does not have any enabled backup data items", service.Name)
		}
		return enabled, nil
	}

	allowed := make(map[string]struct{}, len(enabled))
	for _, name := range enabled {
		allowed[name] = struct{}{}
	}
	validated := make([]string, 0, len(requested))
	seen := make(map[string]struct{}, len(requested))
	for _, name := range requested {
		if _, ok := allowed[name]; !ok {
			return nil, fmt.Errorf("service %q backup data %q is not enabled", service.Name, name)
		}
		if _, duplicate := seen[name]; duplicate {
			continue
		}
		seen[name] = struct{}{}
		validated = append(validated, name)
	}
	sort.Strings(validated)
	return validated, nil
}
