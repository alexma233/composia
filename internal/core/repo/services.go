package repo

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"forgejo.alexma.top/alexma233/composia/internal/core/schedule"
	"gopkg.in/yaml.v3"
)

const MetaFileName = "composia-meta.yaml"

func walkServiceMetaFiles(repoDir string, visit func(path string) error) error {
	return filepath.WalkDir(repoDir, func(path string, entry fs.DirEntry, walkErr error) error {
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
		return visit(path)
	})
}

type Service struct {
	Name        string
	Directory   string
	MetaPath    string
	TargetNodes []string
	Enabled     bool
	Meta        ServiceMeta
}

type ServiceMeta struct {
	Name         string             `yaml:"name"`
	ProjectName  string             `yaml:"project_name"`
	ComposeFiles []string           `yaml:"compose_files"`
	Enabled      *bool              `yaml:"enabled"`
	Nodes        []string           `yaml:"nodes"`
	Infra        *InfraConfig       `yaml:"infra"`
	Network      *NetworkConfig     `yaml:"network"`
	Update       *UpdateConfig      `yaml:"update"`
	DataProtect  *DataProtectConfig `yaml:"data_protect"`
	Backup       *BackupConfig      `yaml:"backup"`
	Migrate      *MigrateConfig     `yaml:"migrate"`
	AutoDeploy   *bool              `yaml:"auto_deploy"`
}

func (meta ServiceMeta) IsInfra() bool {
	return meta.Infra != nil
}

func (meta ServiceMeta) AutoDeployEnabled() bool {
	return meta.AutoDeploy != nil && *meta.AutoDeploy
}

type InfraConfig struct {
	Caddy  *InfraCaddyConfig  `yaml:"caddy"`
	Rustic *InfraRusticConfig `yaml:"rustic"`
	Config *InfraConfigConfig `yaml:"config"`
}

type InfraConfigConfig struct{}

type InfraCaddyConfig struct {
	ComposeService string `yaml:"compose_service"`
	ConfigDir      string `yaml:"config_dir"`
}

type InfraRusticConfig struct {
	ComposeService string   `yaml:"compose_service"`
	Profile        string   `yaml:"profile"`
	DataProtectDir string   `yaml:"data_protect_dir"`
	InitArgs       []string `yaml:"init_args"`
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
	Enabled            *bool                           `yaml:"enabled"`
	AutoApply          *bool                           `yaml:"auto_apply"`
	CheckSchedule      string                          `yaml:"check_schedule"`
	BackupBeforeUpdate *bool                           `yaml:"backup_before_update"`
	BackupData         []UpdateBackupDataItem          `yaml:"backup_data"`
	DigestPin          *bool                           `yaml:"digest_pin"`
	DiscoverySources   map[string]ImageUpdateDiscovery `yaml:"discovery_sources"`
	Images             map[string]ImageUpdateConfig    `yaml:"images"`
}

type UpdateBackupDataItem struct {
	Name    string `yaml:"name"`
	Enabled *bool  `yaml:"enabled"`
}

type ImageUpdateConfig struct {
	Image              string               `yaml:"image"`
	AutoApply          *bool                `yaml:"auto_apply"`
	CheckSchedule      string               `yaml:"check_schedule"`
	BackupBeforeUpdate *bool                `yaml:"backup_before_update"`
	DigestPin          *bool                `yaml:"digest_pin"`
	Current            ImageUpdateCurrent   `yaml:"current"`
	Discovery          ImageUpdateDiscovery `yaml:"discovery"`
	Filter             *ImageUpdateFilter   `yaml:"filter"`
}

type ImageUpdateCurrent struct {
	Tag  string                  `yaml:"tag"`
	Env  *ImageUpdateCurrentEnv  `yaml:"env"`
	YAML *ImageUpdateCurrentYAML `yaml:"yaml"`
}

type ImageUpdateCurrentEnv struct {
	File string `yaml:"file"`
	Key  string `yaml:"key"`
}

type ImageUpdateCurrentYAML struct {
	File string `yaml:"file"`
	Path string `yaml:"path"`
}

type ImageUpdateDiscovery struct {
	Ref               string                       `yaml:"-"`
	Sources           []ImageUpdateDiscoverySource `yaml:"sources"`
	Combine           string                       `yaml:"combine"`
	IncludePrerelease *bool                        `yaml:"include_prerelease"`
}

type ImageUpdateDiscoverySource struct {
	Type    string `yaml:"type"`
	Repo    string `yaml:"repo"`
	RepoURL string `yaml:"repo_url"`
	Project string `yaml:"project"`
	APIURL  string `yaml:"api_url"`
}

type ImageUpdateFilter struct {
	Type    string   `yaml:"type"`
	Format  string   `yaml:"format"`
	Pattern string   `yaml:"pattern"`
	Order   string   `yaml:"order"`
	Allow   []string `yaml:"allow"`
}

func (discovery *ImageUpdateDiscovery) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.ScalarNode {
		discovery.Ref = strings.TrimSpace(value.Value)
		return nil
	}
	type rawImageUpdateDiscovery ImageUpdateDiscovery
	var raw rawImageUpdateDiscovery
	if err := value.Decode(&raw); err != nil {
		return err
	}
	*discovery = ImageUpdateDiscovery(raw)
	return nil
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
}

type MigrateConfig struct {
	Data []MigrateItem `yaml:"data"`
}

type MigrateItem struct {
	Name    string `yaml:"name"`
	Enabled *bool  `yaml:"enabled"`
}

func DiscoverServices(repoDir string, availableNodeIDs map[string]struct{}) ([]Service, error) {
	servicesByName := make(map[string]Service)
	duplicateNames := make(map[string]struct{})

	err := walkServiceMetaFiles(repoDir, func(path string) error {
		service, err := strictServiceFromMetaPath(path, availableNodeIDs)
		if err != nil {
			return nil
		}

		if _, duplicated := duplicateNames[service.Name]; duplicated {
			return nil
		}
		if _, exists := servicesByName[service.Name]; exists {
			delete(servicesByName, service.Name)
			duplicateNames[service.Name] = struct{}{}
			return nil
		}
		servicesByName[service.Name] = service
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("discover services in %q: %w", repoDir, err)
	}

	services := make([]Service, 0, len(servicesByName))
	for _, service := range servicesByName {
		services = append(services, service)
	}
	sort.Slice(services, func(left, right int) bool {
		return services[left].Name < services[right].Name
	})
	return services, nil
}

func FindService(repoDir string, availableNodeIDs map[string]struct{}, serviceName string) (Service, error) {
	var matched *Service
	err := walkServiceMetaFiles(repoDir, func(path string) error {
		meta, err := loadServiceMeta(path)
		if err != nil {
			return nil
		}
		if meta.Name != serviceName {
			return nil
		}
		service, err := strictServiceFromMeta(path, meta, availableNodeIDs)
		if err != nil {
			return err
		}
		if matched != nil {
			return fmt.Errorf("service %q is declared more than once: %s and %s", serviceName, matched.MetaPath, path)
		}
		matched = &service
		return nil
	})
	if err != nil {
		return Service{}, err
	}
	if matched != nil {
		return *matched, nil
	}
	return Service{}, fmt.Errorf("service %q is not declared", serviceName)
}

func FindCaddyInfraService(repoDir string, availableNodeIDs map[string]struct{}) (Service, error) {
	var matched *Service
	err := walkServiceMetaFiles(repoDir, func(path string) error {
		meta, err := loadServiceMeta(path)
		if err != nil {
			return nil
		}
		if meta.Infra == nil || meta.Infra.Caddy == nil {
			return nil
		}
		service, err := strictServiceFromMeta(path, meta, availableNodeIDs)
		if err != nil {
			return err
		}
		if matched != nil {
			return fmt.Errorf("caddy infra service is declared more than once: %s and %s", matched.MetaPath, path)
		}
		matched = &service
		return nil
	})
	if err != nil {
		return Service{}, err
	}
	if matched != nil {
		return *matched, nil
	}
	return Service{}, fmt.Errorf("caddy infra service is not declared")
}

func FindRusticInfraService(repoDir string, availableNodeIDs map[string]struct{}) (Service, error) {
	var matched *Service
	err := walkServiceMetaFiles(repoDir, func(path string) error {
		meta, err := loadServiceMeta(path)
		if err != nil {
			return nil
		}
		if meta.Infra == nil || meta.Infra.Rustic == nil {
			return nil
		}
		service, err := strictServiceFromMeta(path, meta, availableNodeIDs)
		if err != nil {
			return err
		}
		if matched != nil {
			return fmt.Errorf("rustic infra service is declared more than once: %s and %s", matched.MetaPath, path)
		}
		matched = &service
		return nil
	})
	if err != nil {
		return Service{}, err
	}
	if matched != nil {
		return *matched, nil
	}
	return Service{}, fmt.Errorf("rustic infra service is not declared")
}

func strictServiceFromMetaPath(path string, availableNodeIDs map[string]struct{}) (Service, error) {
	meta, err := loadServiceMeta(path)
	if err != nil {
		return Service{}, err
	}
	return strictServiceFromMeta(path, meta, availableNodeIDs)
}

func strictServiceFromMeta(path string, meta ServiceMeta, availableNodeIDs map[string]struct{}) (Service, error) {
	if err := validateServiceMeta(path, &meta, availableNodeIDs); err != nil {
		return Service{}, err
	}
	targetNodes := normalizedTargetNodes(meta)
	return Service{
		Name:        meta.Name,
		Directory:   filepath.Dir(path),
		MetaPath:    path,
		TargetNodes: targetNodes,
		Enabled:     boolValue(meta.Enabled, true),
		Meta:        meta,
	}, nil
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

func LoadServiceMeta(path string) (ServiceMeta, error) {
	return loadServiceMeta(path)
}

func LoadServiceFromMetaPath(path string, availableNodeIDs map[string]struct{}) (Service, error) {
	return strictServiceFromMetaPath(path, availableNodeIDs)
}

func RewriteServiceTargetNodes(path string, nodeIDs []string, availableNodeIDs map[string]struct{}) (string, error) {
	meta, err := loadServiceMeta(path)
	if err != nil {
		return "", err
	}
	normalizedNodes := normalizeNodeIDs(nodeIDs)
	meta.Nodes = append([]string(nil), normalizedNodes...)
	if err := validateServiceMeta(path, &meta, availableNodeIDs); err != nil {
		return "", err
	}
	rawContent, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read service meta %q: %w", path, err)
	}
	var document yaml.Node
	decoder := yaml.NewDecoder(strings.NewReader(string(rawContent)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return "", fmt.Errorf("decode service meta %q: %w", path, err)
	}
	updateServiceTargetNodesNode(&document, normalizedNodes)
	var builder strings.Builder
	encoder := yaml.NewEncoder(&builder)
	encoder.SetIndent(2)
	err = encoder.Encode(&document)
	_ = encoder.Close()
	if err != nil {
		return "", fmt.Errorf("encode service meta %q: %w", path, err)
	}
	return builder.String(), nil
}

func updateServiceTargetNodesNode(document *yaml.Node, nodeIDs []string) {
	if document == nil || len(document.Content) == 0 {
		return
	}
	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return
	}
	removeMappingKey(mapping, "node")
	setMappingSequence(mapping, "nodes", nodeIDs)
}

func removeMappingKey(mapping *yaml.Node, key string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value != key {
			continue
		}
		mapping.Content = append(mapping.Content[:index], mapping.Content[index+2:]...)
		return
	}
}

func setMappingSequence(mapping *yaml.Node, key string, values []string) {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return
	}
	sequence := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
	for _, value := range values {
		sequence.Content = append(sequence.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value != key {
			continue
		}
		mapping.Content[index+1] = sequence
		return
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, sequence)
}

func validateServiceMeta(path string, meta *ServiceMeta, availableNodeIDs map[string]struct{}) error {
	if meta.Name == "" {
		return fmt.Errorf("service meta %q: name is required", path)
	}

	targetNodes := normalizedTargetNodes(*meta)
	if len(targetNodes) == 0 {
		return fmt.Errorf("service meta %q: at least one target node is required", path)
	}
	seenNodes := make(map[string]struct{}, len(targetNodes))
	for _, nodeID := range targetNodes {
		if _, ok := availableNodeIDs[nodeID]; !ok {
			return fmt.Errorf("service meta %q: node %q is not configured", path, nodeID)
		}
		if _, exists := seenNodes[nodeID]; exists {
			return fmt.Errorf("service meta %q: node %q is duplicated", path, nodeID)
		}
		seenNodes[nodeID] = struct{}{}
	}

	if meta.Network != nil {
		if err := validateNetwork(path, meta.Network); err != nil {
			return err
		}
	}
	composeFiles, err := meta.NormalizedComposeFiles()
	if err != nil {
		return fmt.Errorf("service meta %q: %w", path, err)
	}
	meta.ComposeFiles = composeFiles
	if meta.IsConfigInfra() {
		if err := validateConfigInfra(path, meta); err != nil {
			return err
		}
	}
	if meta.Infra != nil {
		if err := validateInfra(path, meta.Infra); err != nil {
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
			if err := schedule.Validate(item.Schedule); err != nil {
				return fmt.Errorf("service meta %q: backup.data[%q].schedule: %w", path, item.Name, err)
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

func validateConfigInfra(path string, meta *ServiceMeta) error {
	if meta.Infra != nil && meta.Infra.Caddy != nil {
		return fmt.Errorf("service meta %q: infra.config cannot be combined with infra.caddy because caddy reload requires docker compose", path)
	}
	if meta.Infra != nil && meta.Infra.Rustic != nil {
		return fmt.Errorf("service meta %q: infra.config cannot be combined with infra.rustic because rustic requires docker compose", path)
	}
	if meta.DataProtect == nil {
		return nil
	}
	for _, item := range meta.DataProtect.Data {
		for _, action := range []struct {
			name   string
			config *DataActionConfig
		}{
			{name: "backup", config: item.Backup},
			{name: "restore", config: item.Restore},
		} {
			if action.config == nil {
				continue
			}
			if action.config.Strategy != "files.copy" {
				return fmt.Errorf("service meta %q: infra.config services only support data_protect.data[%q].%s.strategy files.copy", path, item.Name, action.name)
			}
			for _, include := range action.config.Include {
				if _, _, err := ClassifyDataInclude(include); err != nil {
					return fmt.Errorf("service meta %q: data_protect.data[%q].%s.include %q is invalid: %w", path, item.Name, action.name, include, err)
				}
			}
		}
	}
	return nil
}

func (meta ServiceMeta) IsConfigInfra() bool {
	return meta.Infra != nil && meta.Infra.Config != nil
}

func validateNetwork(path string, network *NetworkConfig) error {
	if network.Caddy != nil && boolValue(network.Caddy.Enabled, false) && network.Caddy.Source == "" {
		return fmt.Errorf("service meta %q: network.caddy.source is required when caddy is enabled", path)
	}

	if network.DNS != nil {
		switch network.DNS.Provider {
		case "cloudflare", "alidns", "dnspod", "route53", "huaweicloud":
		default:
			return fmt.Errorf("service meta %q: network.dns.provider must be cloudflare, alidns, dnspod, route53, or huaweicloud", path)
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

func validateInfra(path string, infra *InfraConfig) error {
	if infra.Caddy != nil && strings.TrimSpace(infra.Caddy.ConfigDir) == "" && infra.Caddy.ConfigDir != "" {
		return fmt.Errorf("service meta %q: infra.caddy.config_dir must not be empty when set", path)
	}
	if infra.Rustic != nil && strings.TrimSpace(infra.Rustic.ComposeService) == "" && infra.Rustic.ComposeService != "" {
		return fmt.Errorf("service meta %q: infra.rustic.compose_service must not be empty when set", path)
	}
	if infra.Rustic != nil && strings.TrimSpace(infra.Rustic.DataProtectDir) == "" && infra.Rustic.DataProtectDir != "" {
		return fmt.Errorf("service meta %q: infra.rustic.data_protect_dir must not be empty when set", path)
	}
	if infra.Rustic != nil {
		for index, arg := range infra.Rustic.InitArgs {
			if strings.TrimSpace(arg) == "" {
				return fmt.Errorf("service meta %q: infra.rustic.init_args[%d] must not be empty", path, index)
			}
		}
	}
	return nil
}

func (meta ServiceMeta) CaddyComposeService() string {
	if meta.Infra != nil && meta.Infra.Caddy != nil && strings.TrimSpace(meta.Infra.Caddy.ComposeService) != "" {
		return strings.TrimSpace(meta.Infra.Caddy.ComposeService)
	}
	return "caddy"
}

func (meta ServiceMeta) NormalizedComposeFiles() ([]string, error) {
	if len(meta.ComposeFiles) == 0 {
		return nil, nil
	}
	normalized := make([]string, 0, len(meta.ComposeFiles))
	seen := make(map[string]struct{}, len(meta.ComposeFiles))
	for index, file := range meta.ComposeFiles {
		trimmed := strings.TrimSpace(file)
		if trimmed == "" {
			return nil, fmt.Errorf("compose_files[%d] must not be empty", index)
		}
		if filepath.IsAbs(trimmed) {
			return nil, fmt.Errorf("compose_files[%d] must be a relative path", index)
		}
		cleaned := filepath.Clean(trimmed)
		if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("compose_files[%d] must stay within the service directory", index)
		}
		if _, exists := seen[cleaned]; exists {
			return nil, fmt.Errorf("compose_files[%d] duplicates %q", index, cleaned)
		}
		seen[cleaned] = struct{}{}
		normalized = append(normalized, cleaned)
	}
	return normalized, nil
}

func ComposeProjectName(projectName, fallback string) string {
	trimmedProjectName := strings.TrimSpace(projectName)
	if trimmedProjectName != "" {
		return trimmedProjectName
	}

	normalized := normalizeComposeProjectName(fallback)
	if normalized == "" {
		return "service"
	}
	return normalized
}

func normalizeComposeProjectName(name string) string {
	var builder strings.Builder
	pendingHyphen := false
	lastWasSeparator := false

	for _, char := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			if pendingHyphen && builder.Len() > 0 && !lastWasSeparator {
				builder.WriteByte('-')
			}
			builder.WriteRune(char)
			pendingHyphen = false
			lastWasSeparator = false
		case char == '-', char == '_':
			pendingHyphen = false
			if builder.Len() == 0 || lastWasSeparator {
				continue
			}
			builder.WriteRune(char)
			lastWasSeparator = true
		default:
			if builder.Len() > 0 {
				pendingHyphen = true
			}
		}
	}

	return strings.TrimRight(builder.String(), "-_")
}

func (meta ServiceMeta) CaddyConfigDir() string {
	if meta.Infra != nil && meta.Infra.Caddy != nil && strings.TrimSpace(meta.Infra.Caddy.ConfigDir) != "" {
		return strings.TrimSpace(meta.Infra.Caddy.ConfigDir)
	}
	return "/etc/caddy"
}

func (meta ServiceMeta) RusticComposeService() string {
	if meta.Infra != nil && meta.Infra.Rustic != nil && strings.TrimSpace(meta.Infra.Rustic.ComposeService) != "" {
		return strings.TrimSpace(meta.Infra.Rustic.ComposeService)
	}
	return "rustic"
}

func (meta ServiceMeta) RusticProfile() string {
	if meta.Infra != nil && meta.Infra.Rustic != nil && strings.TrimSpace(meta.Infra.Rustic.Profile) != "" {
		return strings.TrimSpace(meta.Infra.Rustic.Profile)
	}
	return ""
}

func (meta ServiceMeta) RusticDataProtectDir() string {
	if meta.Infra != nil && meta.Infra.Rustic != nil && strings.TrimSpace(meta.Infra.Rustic.DataProtectDir) != "" {
		return strings.TrimSpace(meta.Infra.Rustic.DataProtectDir)
	}
	return ""
}

func (meta ServiceMeta) RusticInitArgs() []string {
	if meta.Infra == nil || meta.Infra.Rustic == nil || len(meta.Infra.Rustic.InitArgs) == 0 {
		return nil
	}
	args := make([]string, 0, len(meta.Infra.Rustic.InitArgs))
	for _, arg := range meta.Infra.Rustic.InitArgs {
		trimmed := strings.TrimSpace(arg)
		if trimmed == "" {
			continue
		}
		args = append(args, trimmed)
	}
	if len(args) == 0 {
		return nil
	}
	return args
}

func validateUpdate(path string, update *UpdateConfig) error {
	if err := schedule.Validate(update.CheckSchedule); err != nil {
		return fmt.Errorf("service meta %q: update.check_schedule: %w", path, err)
	}
	for name, discovery := range update.DiscoverySources {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			return fmt.Errorf("service meta %q: update.discovery_sources[] name must not be empty", path)
		}
		if discovery.Ref != "" {
			return fmt.Errorf("service meta %q: update.discovery_sources[%q] cannot reference another discovery source", path, trimmedName)
		}
		if err := validateImageUpdateDiscovery(fmt.Sprintf("service meta %q: update.discovery_sources[%q]", path, trimmedName), discovery, nil, nil); err != nil {
			return err
		}
	}
	seenImageNames := make(map[string]struct{}, len(update.Images))
	for name, image := range update.Images {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			return fmt.Errorf("service meta %q: update.images[] name must not be empty", path)
		}
		if _, exists := seenImageNames[trimmedName]; exists {
			return fmt.Errorf("service meta %q: update.images[%q] is duplicated after trimming", path, trimmedName)
		}
		seenImageNames[trimmedName] = struct{}{}
		if err := validateImageUpdate(path, trimmedName, image, update.DiscoverySources); err != nil {
			return err
		}
	}
	return nil
}

func validateImageUpdate(path, name string, image ImageUpdateConfig, sources map[string]ImageUpdateDiscovery) error {
	fieldPrefix := fmt.Sprintf("service meta %q: update.images[%q]", path, name)
	if strings.TrimSpace(image.Image) == "" {
		return fmt.Errorf("%s.image is required", fieldPrefix)
	}
	if err := schedule.Validate(image.CheckSchedule); err != nil {
		return fmt.Errorf("%s.check_schedule: %w", fieldPrefix, err)
	}
	if err := validateImageUpdateCurrent(fieldPrefix, image.Current); err != nil {
		return err
	}
	if err := validateImageUpdateDiscovery(fieldPrefix+".discovery", image.Discovery, image.Filter, sources); err != nil {
		return err
	}
	return validateImageUpdateFilter(fieldPrefix+".filter", image.Discovery, image.Filter)
}

func validateImageUpdateCurrent(fieldPrefix string, current ImageUpdateCurrent) error {
	modeCount := 0
	if strings.TrimSpace(current.Tag) != "" {
		modeCount++
	}
	if current.Env != nil {
		modeCount++
		if err := validateImageUpdateCurrentFile(fieldPrefix+".current.env.file", current.Env.File); err != nil {
			return err
		}
		if strings.TrimSpace(current.Env.Key) == "" {
			return fmt.Errorf("%s.current.env.key is required", fieldPrefix)
		}
	}
	if current.YAML != nil {
		modeCount++
		if err := validateImageUpdateCurrentFile(fieldPrefix+".current.yaml.file", current.YAML.File); err != nil {
			return err
		}
		if strings.TrimSpace(current.YAML.Path) == "" {
			return fmt.Errorf("%s.current.yaml.path is required", fieldPrefix)
		}
	}
	if modeCount != 1 {
		return fmt.Errorf("%s.current must specify exactly one of tag, env, or yaml", fieldPrefix)
	}
	return nil
}

func validateImageUpdateCurrentFile(fieldPrefix, file string) error {
	file = strings.TrimSpace(file)
	if file == "" {
		return fmt.Errorf("%s is required", fieldPrefix)
	}
	if filepath.IsAbs(file) {
		return fmt.Errorf("%s must be a relative path", fieldPrefix)
	}
	cleaned := filepath.Clean(file)
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("%s must stay within the service directory", fieldPrefix)
	}
	return nil
}

func validateImageUpdateDiscovery(fieldPrefix string, discovery ImageUpdateDiscovery, filter *ImageUpdateFilter, sources map[string]ImageUpdateDiscovery) error {
	if discovery.Ref != "" {
		if sources != nil {
			if _, ok := sources[discovery.Ref]; !ok {
				return fmt.Errorf("%s references unknown discovery source %q", fieldPrefix, discovery.Ref)
			}
		}
		return nil
	}
	if discovery.Combine != "" && discovery.Combine != "merge" && discovery.Combine != "first_success" {
		return fmt.Errorf("%s.combine must be merge or first_success", fieldPrefix)
	}
	if len(discovery.Sources) == 0 {
		return fmt.Errorf("%s.sources must contain at least one source", fieldPrefix)
	}
	hasAuto := false
	hasDigest := false
	for index, source := range discovery.Sources {
		if err := validateImageUpdateDiscoverySource(fmt.Sprintf("%s.sources[%d]", fieldPrefix, index), source, filter); err != nil {
			return err
		}
		switch strings.TrimSpace(source.Type) {
		case "auto":
			hasAuto = true
		case "digest":
			hasDigest = true
		}
	}
	if hasAuto && len(discovery.Sources) != 1 {
		return fmt.Errorf("%s.sources with type auto must be exclusive", fieldPrefix)
	}
	if hasDigest && len(discovery.Sources) != 1 {
		return fmt.Errorf("%s.sources with type digest must be exclusive", fieldPrefix)
	}
	return nil
}

func validateImageUpdateDiscoverySource(fieldPrefix string, source ImageUpdateDiscoverySource, filter *ImageUpdateFilter) error {
	switch strings.TrimSpace(source.Type) {
	case "auto":
		if strings.TrimSpace(source.RepoURL) != "" {
			if _, err := url.ParseRequestURI(strings.TrimSpace(source.RepoURL)); err != nil {
				return fmt.Errorf("%s.repo_url is invalid: %w", fieldPrefix, err)
			}
		}
		return nil
	case "probe":
		if filter != nil && strings.TrimSpace(filter.Type) != "semver" {
			return fmt.Errorf("%s.type probe requires semver filter", fieldPrefix)
		}
		return nil
	case "registry", "digest":
		return nil
	case "github", "forgejo":
		if strings.TrimSpace(source.Repo) == "" {
			return fmt.Errorf("%s.repo is required", fieldPrefix)
		}
		return nil
	case "gitlab":
		if strings.TrimSpace(source.Project) == "" {
			return fmt.Errorf("%s.project is required", fieldPrefix)
		}
		return nil
	case "":
		return fmt.Errorf("%s.type is required", fieldPrefix)
	default:
		return fmt.Errorf("%s.type must be auto, probe, registry, digest, github, gitlab, or forgejo", fieldPrefix)
	}
}

func validateImageUpdateFilter(fieldPrefix string, discovery ImageUpdateDiscovery, filter *ImageUpdateFilter) error {
	if isDigestDiscovery(discovery) {
		if filter != nil {
			return fmt.Errorf("%s must be omitted for digest discovery", fieldPrefix)
		}
		return nil
	}
	if filter == nil {
		return fmt.Errorf("%s.type is required", fieldPrefix)
	}
	switch strings.TrimSpace(filter.Type) {
	case "semver":
		return validateSemverAllow(fieldPrefix, filter.Allow)
	case "date":
		if strings.TrimSpace(filter.Format) == "" {
			return fmt.Errorf("%s.format is required for date filter", fieldPrefix)
		}
		return nil
	case "regex":
		if strings.TrimSpace(filter.Pattern) == "" {
			return fmt.Errorf("%s.pattern is required for regex filter", fieldPrefix)
		}
		if _, err := regexp.Compile(filter.Pattern); err != nil {
			return fmt.Errorf("%s.pattern is invalid: %w", fieldPrefix, err)
		}
		switch strings.TrimSpace(filter.Order) {
		case "numeric", "lexicographic":
			return nil
		default:
			return fmt.Errorf("%s.order must be numeric or lexicographic for regex filter", fieldPrefix)
		}
	case "latest":
		return nil
	case "":
		return fmt.Errorf("%s.type is required", fieldPrefix)
	default:
		return fmt.Errorf("%s.type must be semver, date, regex, or latest", fieldPrefix)
	}
}

func isDigestDiscovery(discovery ImageUpdateDiscovery) bool {
	return len(discovery.Sources) == 1 && strings.TrimSpace(discovery.Sources[0].Type) == "digest"
}

func IsDigestImageDiscovery(discovery ImageUpdateDiscovery, sources map[string]ImageUpdateDiscovery) bool {
	return isDigestDiscovery(ResolveImageUpdateDiscovery(discovery, sources))
}

func ResolveImageUpdateDiscovery(discovery ImageUpdateDiscovery, sources map[string]ImageUpdateDiscovery) ImageUpdateDiscovery {
	if discovery.Ref == "" {
		return discovery
	}
	if source, ok := sources[discovery.Ref]; ok {
		return source
	}
	return discovery
}

func ImageUpdateCurrentFile(current ImageUpdateCurrent) string {
	if current.Env != nil {
		return current.Env.File
	}
	if current.YAML != nil {
		return current.YAML.File
	}
	return ""
}

func validateSemverAllow(fieldPrefix string, allow []string) error {
	seen := make(map[string]struct{}, len(allow))
	for _, value := range allow {
		value = strings.TrimSpace(value)
		switch value {
		case "patch", "minor", "major":
			if _, exists := seen[value]; exists {
				return fmt.Errorf("%s.allow[%q] is duplicated", fieldPrefix, value)
			}
			seen[value] = struct{}{}
		default:
			return fmt.Errorf("%s.allow must contain only patch, minor, or major", fieldPrefix)
		}
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
		for _, include := range action.Include {
			if _, _, err := ClassifyDataInclude(include); err != nil {
				return fmt.Errorf("service meta %q: data_protect.data[%q].%s.include %q is invalid: %w", path, dataName, actionName, include, err)
			}
		}
		switch action.Strategy {
		case "files.copy_after_stop", "files.copy":
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

func normalizedTargetNodes(meta ServiceMeta) []string {
	return normalizeNodeIDs(meta.Nodes)
}

func normalizeNodeIDs(nodeIDs []string) []string {
	nodes := make([]string, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		nodeID = strings.TrimSpace(nodeID)
		if nodeID == "" {
			continue
		}
		nodes = append(nodes, nodeID)
	}
	return nodes
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

func EnabledUpdateBackupDataNames(service Service) []string {
	if service.Meta.Update == nil {
		return nil
	}
	names := make([]string, 0, len(service.Meta.Update.BackupData))
	for _, item := range service.Meta.Update.BackupData {
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

func CaddyManaged(service Service) bool {
	return service.Meta.Network != nil && service.Meta.Network.Caddy != nil && boolValue(service.Meta.Network.Caddy.Enabled, false)
}

func CaddySource(service Service) string {
	if !CaddyManaged(service) {
		return ""
	}
	return strings.TrimSpace(service.Meta.Network.Caddy.Source)
}

func AffectedServicesFromChangedFiles(repoDir string, changedFiles []string) ([]string, error) {
	affected := make(map[string]struct{})
	for _, changedFile := range changedFiles {
		dir := filepath.Dir(filepath.Join(repoDir, changedFile))
		for {
			metaPath := filepath.Join(dir, MetaFileName)
			if _, err := os.Stat(metaPath); err == nil {
				meta, err := loadServiceMeta(metaPath)
				if err == nil && meta.Name != "" {
					affected[meta.Name] = struct{}{}
				}
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir || parent == repoDir {
				break
			}
			dir = parent
		}
	}
	names := make([]string, 0, len(affected))
	for name := range affected {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}
