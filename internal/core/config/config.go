package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"forgejo.alexma.top/alexma233/composia/internal/core/schedule"
	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

const LocalMainNodeID = "main"

var configValidator = validator.New(validator.WithRequiredStructEnabled())

type File struct {
	Controller *ControllerConfig `yaml:"controller"`
	Agent      *AgentConfig      `yaml:"agent"`
}

type ControllerConfig struct {
	ListenAddr       string                            `yaml:"listen_addr" validate:"required"`
	RepoDir          string                            `yaml:"repo_dir" validate:"required"`
	StateDir         string                            `yaml:"state_dir" validate:"required"`
	LogDir           string                            `yaml:"log_dir" validate:"required"`
	Backup           *ControllerBackupConfig           `yaml:"backup"`
	Git              *ControllerGitConfig              `yaml:"git"`
	Notifications    *ControllerNotificationsConfig    `yaml:"notifications"`
	Nodes            []NodeConfig                      `yaml:"nodes" validate:"required"`
	AccessTokens     []AccessTokenConfig               `yaml:"access_tokens"`
	DNS              *ControllerDNSConfig              `yaml:"dns"`
	CloudflareTunnel *ControllerCloudflareTunnelConfig `yaml:"cloudflare_tunnel"`
	Rustic           *ControllerRusticConfig           `yaml:"rustic"`
	Secrets          *ControllerSecretsConfig          `yaml:"secrets"`
	Updates          *ControllerUpdatesConfig          `yaml:"updates"`
	AutoDeploy       *ControllerAutoDeployConfig       `yaml:"auto_deploy"`
}

type ControllerAutoDeployConfig struct {
	Infra    bool `yaml:"infra"`
	Services bool `yaml:"services"`
}

type ControllerBackupConfig struct {
	DefaultSchedule string `yaml:"default_schedule"`
}

type ControllerUpdatesConfig struct {
	DefaultCheckSchedule string                         `yaml:"default_check_schedule"`
	AutoApply            *bool                          `yaml:"auto_apply"`
	BackupBeforeUpdate   *bool                          `yaml:"backup_before_update"`
	DigestPin            *bool                          `yaml:"digest_pin"`
	Semver               *ControllerUpdatesSemverConfig `yaml:"semver"`
	ForgeAuth            *ControllerUpdatesForgeAuth    `yaml:"forge_auth"`
}

type ControllerUpdatesSemverConfig struct {
	DefaultAllow []string `yaml:"default_allow" validate:"dive,oneof=patch minor major"`
}

type ControllerUpdatesForgeAuth struct {
	GitHub  ForgeAuthConfigs `yaml:"github"`
	GitLab  ForgeAuthConfigs `yaml:"gitlab"`
	Forgejo ForgeAuthConfigs `yaml:"forgejo"`
}

type ForgeAuthConfig struct {
	URL       string `yaml:"url"`
	Token     string `yaml:"token"`
	TokenFile string `yaml:"token_file"`
	APIURL    string `yaml:"api_url"`
}

type ForgeAuthConfigs []ForgeAuthConfig

func (configs *ForgeAuthConfigs) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		var items []ForgeAuthConfig
		if err := value.Decode(&items); err != nil {
			return err
		}
		*configs = items
		return nil
	}
	var item ForgeAuthConfig
	if err := value.Decode(&item); err != nil {
		return err
	}
	*configs = []ForgeAuthConfig{item}
	return nil
}

type ControllerGitConfig struct {
	RemoteURL    string                   `yaml:"remote_url"`
	Branch       string                   `yaml:"branch"`
	PullInterval string                   `yaml:"pull_interval"`
	Auth         *ControllerGitAuthConfig `yaml:"auth"`
	AuthorName   string                   `yaml:"author_name"`
	AuthorEmail  string                   `yaml:"author_email"`
}

type ControllerGitAuthConfig struct {
	Username  string `yaml:"username"`
	Token     string `yaml:"token"`
	TokenFile string `yaml:"token_file"`
}

type NodeConfig struct {
	ID          string `yaml:"id" validate:"required"`
	DisplayName string `yaml:"display_name"`
	Enabled     *bool  `yaml:"enabled"`
	PublicIPv4  string `yaml:"public_ipv4"`
	PublicIPv6  string `yaml:"public_ipv6"`
	Token       string `yaml:"token" validate:"required"`
	TokenFile   string `yaml:"token_file"`
}

type AccessTokenConfig struct {
	Name      string `yaml:"name" validate:"required"`
	Token     string `yaml:"token" validate:"required"`
	TokenFile string `yaml:"token_file"`
	Enabled   *bool  `yaml:"enabled"`
	Comment   string `yaml:"comment"`
}

type ControllerDNSConfig struct {
	Cloudflare  *CloudflareDNSConfig  `yaml:"cloudflare"`
	AliDNS      *AliDNSConfig         `yaml:"alidns"`
	DNSPod      *DNSPodConfig         `yaml:"dnspod"`
	Route53     *Route53DNSConfig     `yaml:"route53"`
	HuaweiCloud *HuaweiCloudDNSConfig `yaml:"huaweicloud"`
}

type CloudflareDNSConfig struct {
	APIToken     string   `yaml:"api_token"`
	APITokenFile string   `yaml:"api_token_file"`
	Zones        []string `yaml:"zones"`
}

type ControllerCloudflareTunnelConfig struct {
	AccountID    string                                       `yaml:"account_id"`
	APIToken     string                                       `yaml:"api_token"`
	APITokenFile string                                       `yaml:"api_token_file"`
	Zones        []string                                     `yaml:"zones"`
	Tunnels      map[string]ControllerCloudflareTunnel        `yaml:"tunnels"`
	Nodes        map[string]ControllerCloudflareTunnelNodeMap `yaml:"nodes"`
}

type ControllerCloudflareTunnel struct {
	TunnelID        string `yaml:"tunnel_id"`
	FallbackService string `yaml:"fallback_service"`
}

type ControllerCloudflareTunnelNodeMap struct {
	Tunnel string `yaml:"tunnel"`
}

type AliDNSConfig struct {
	AccessKeyID         string   `yaml:"access_key_id"`
	AccessKeyIDFile     string   `yaml:"access_key_id_file"`
	AccessKeySecret     string   `yaml:"access_key_secret"`
	AccessKeySecretFile string   `yaml:"access_key_secret_file"`
	SecurityToken       string   `yaml:"security_token"`
	SecurityTokenFile   string   `yaml:"security_token_file"`
	RegionID            string   `yaml:"region_id"`
	Zones               []string `yaml:"zones"`
}

type DNSPodConfig struct {
	SecretID         string   `yaml:"secret_id"`
	SecretIDFile     string   `yaml:"secret_id_file"`
	SecretKey        string   `yaml:"secret_key"`
	SecretKeyFile    string   `yaml:"secret_key_file"`
	SessionToken     string   `yaml:"session_token"`
	SessionTokenFile string   `yaml:"session_token_file"`
	Region           string   `yaml:"region"`
	Zones            []string `yaml:"zones"`
}

type Route53DNSConfig struct {
	AccessKeyID         string   `yaml:"access_key_id"`
	AccessKeyIDFile     string   `yaml:"access_key_id_file"`
	SecretAccessKey     string   `yaml:"secret_access_key"`
	SecretAccessKeyFile string   `yaml:"secret_access_key_file"`
	SessionToken        string   `yaml:"session_token"`
	SessionTokenFile    string   `yaml:"session_token_file"`
	Region              string   `yaml:"region"`
	Profile             string   `yaml:"profile"`
	HostedZoneID        string   `yaml:"hosted_zone_id"`
	Zones               []string `yaml:"zones"`
}

type HuaweiCloudDNSConfig struct {
	AccessKeyID         string   `yaml:"access_key_id"`
	AccessKeyIDFile     string   `yaml:"access_key_id_file"`
	SecretAccessKey     string   `yaml:"secret_access_key"`
	SecretAccessKeyFile string   `yaml:"secret_access_key_file"`
	RegionID            string   `yaml:"region_id"`
	Zones               []string `yaml:"zones"`
}

type ControllerRusticConfig struct {
	MainNodes   []string                           `yaml:"main_nodes"`
	Maintenance *ControllerRusticMaintenanceConfig `yaml:"maintenance"`
}

type ControllerRusticMaintenanceConfig struct {
	ForgetSchedule string `yaml:"forget_schedule"`
	PruneSchedule  string `yaml:"prune_schedule"`
}

type ControllerSecretsConfig struct {
	Provider      string `yaml:"provider" validate:"required,oneof=age"`
	IdentityFile  string `yaml:"identity_file" validate:"required"`
	RecipientFile string `yaml:"recipient_file"`
	Armor         *bool  `yaml:"armor"`
}

type AgentConfig struct {
	ControllerAddr    string                        `yaml:"controller_addr" validate:"required"`
	ControllerGRPC    bool                          `yaml:"controller_grpc"`
	ControllerHeaders []AgentControllerHeaderConfig `yaml:"controller_headers"`
	NodeID            string                        `yaml:"node_id" validate:"required"`
	Token             string                        `yaml:"token" validate:"required"`
	TokenFile         string                        `yaml:"token_file"`
	RepoDir           string                        `yaml:"repo_dir" validate:"required"`
	StateDir          string                        `yaml:"state_dir" validate:"required"`
	Caddy             *AgentCaddyConfig             `yaml:"caddy"`
}

type AgentControllerHeaderConfig struct {
	Name      string `yaml:"name" validate:"required"`
	Value     string `yaml:"value" validate:"required"`
	ValueFile string `yaml:"value_file"`
}

type AgentCaddyConfig struct {
	GeneratedDir string `yaml:"generated_dir"`
}

func LoadController(path string) (*ControllerConfig, error) {
	file, err := load(path)
	if err != nil {
		return nil, err
	}
	if file.Controller == nil {
		return nil, fmt.Errorf("config %q does not contain a controller section", path)
	}
	if err := validateController(file); err != nil {
		return nil, err
	}
	return file.Controller, nil
}

func LoadAgent(path string) (*AgentConfig, error) {
	file, err := load(path)
	if err != nil {
		return nil, err
	}
	if file.Agent == nil {
		return nil, fmt.Errorf("config %q does not contain an agent section", path)
	}
	if err := validateAgent(file); err != nil {
		return nil, err
	}
	return file.Agent, nil
}

func load(path string) (*File, error) {
	content, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	var file File
	decoder := yaml.NewDecoder(strings.NewReader(string(content)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode config %q: %w", path, err)
	}
	if file.Controller == nil && file.Agent == nil {
		return nil, fmt.Errorf("config %q must contain at least one of controller or agent", path)
	}
	if err := resolveInlineOrFileConfig(&file); err != nil {
		return nil, err
	}
	return &file, nil
}

func validateController(file *File) error {
	controller := file.Controller
	if err := validationError(configValidator.StructPartial(controller, "ListenAddr", "RepoDir", "StateDir", "LogDir", "Nodes"), func(field validator.FieldError) error {
		switch field.StructField() {
		case "ListenAddr":
			return errors.New("controller.listen_addr is required")
		case "RepoDir":
			return errors.New("controller.repo_dir is required")
		case "StateDir":
			return errors.New("controller.state_dir is required")
		case "LogDir":
			return errors.New("controller.log_dir is required")
		case "Nodes":
			return errors.New("controller.nodes must be present, even if it is empty")
		default:
			return nil
		}
	}); err != nil {
		return err
	}

	seenNodeIDs := make(map[string]struct{}, len(controller.Nodes))
	seenNodeTokens := make(map[string]string, len(controller.Nodes))
	for _, node := range controller.Nodes {
		if err := validationError(configValidator.StructPartial(node, "ID", "Token"), func(field validator.FieldError) error {
			switch field.StructField() {
			case "ID":
				return errors.New("controller.nodes[].id is required")
			case "Token":
				return fmt.Errorf("controller.nodes[%q].token is required", node.ID)
			default:
				return nil
			}
		}); err != nil {
			return err
		}
		if _, exists := seenNodeIDs[node.ID]; exists {
			return fmt.Errorf("controller.nodes[%q] is duplicated", node.ID)
		}
		if previousNodeID, exists := seenNodeTokens[node.Token]; exists {
			return fmt.Errorf("controller.nodes[%q].token duplicates controller.nodes[%q].token", node.ID, previousNodeID)
		}
		seenNodeIDs[node.ID] = struct{}{}
		seenNodeTokens[node.Token] = node.ID
	}

	if controller.Rustic != nil {
		for _, nodeID := range controller.Rustic.MainNodes {
			nodeID = strings.TrimSpace(nodeID)
			if nodeID == "" {
				return errors.New("controller.rustic.main_nodes[] must not be empty")
			}
			if _, exists := seenNodeIDs[nodeID]; !exists {
				return fmt.Errorf("controller.rustic.main_nodes[%q] must reference a configured controller.nodes entry", nodeID)
			}
		}
		if controller.Rustic.Maintenance != nil {
			if err := schedule.Validate(controller.Rustic.Maintenance.ForgetSchedule); err != nil {
				return fmt.Errorf("controller.rustic.maintenance.forget_schedule: %w", err)
			}
			if err := schedule.Validate(controller.Rustic.Maintenance.PruneSchedule); err != nil {
				return fmt.Errorf("controller.rustic.maintenance.prune_schedule: %w", err)
			}
		}
	}
	if err := validateControllerCloudflareTunnel(controller.CloudflareTunnel, seenNodeIDs); err != nil {
		return err
	}

	if controller.Backup != nil {
		if err := schedule.Validate(controller.Backup.DefaultSchedule); err != nil {
			return fmt.Errorf("controller.backup.default_schedule: %w", err)
		}
	}
	if err := validateControllerNotifications(controller.Notifications); err != nil {
		return err
	}
	if controller.Updates != nil {
		if err := schedule.Validate(controller.Updates.DefaultCheckSchedule); err != nil {
			return fmt.Errorf("controller.updates.default_check_schedule: %w", err)
		}
		if controller.Updates.Semver != nil {
			if err := validateControllerUpdatesSemverAllow(controller.Updates.Semver.DefaultAllow); err != nil {
				return err
			}
		}
	}

	if controller.Git != nil && controller.Git.RemoteURL != "" && controller.Git.PullInterval == "" {
		return errors.New("controller.git.pull_interval is required when controller.git.remote_url is set")
	}

	seenAccessTokens := make(map[string]string, len(controller.AccessTokens))
	for _, token := range controller.AccessTokens {
		if err := validationError(configValidator.StructPartial(token, "Name", "Token"), func(field validator.FieldError) error {
			switch field.StructField() {
			case "Name":
				return errors.New("controller.access_tokens[].name is required")
			case "Token":
				return fmt.Errorf("controller.access_tokens[%q].token is required", token.Name)
			default:
				return nil
			}
		}); err != nil {
			return err
		}
		if nodeID, exists := seenNodeTokens[token.Token]; exists {
			return fmt.Errorf("controller.access_tokens[%q].token duplicates controller.nodes[%q].token", token.Name, nodeID)
		}
		if previousName, exists := seenAccessTokens[token.Token]; exists {
			return fmt.Errorf("controller.access_tokens[%q].token duplicates controller.access_tokens[%q].token", token.Name, previousName)
		}
		seenAccessTokens[token.Token] = token.Name
	}

	if controller.Secrets != nil {
		if err := validationError(configValidator.StructPartial(controller.Secrets, "Provider", "IdentityFile"), func(field validator.FieldError) error {
			switch field.StructField() {
			case "Provider":
				return errors.New("controller.secrets.provider must be age")
			case "IdentityFile":
				return errors.New("controller.secrets.identity_file is required")
			default:
				return nil
			}
		}); err != nil {
			return err
		}
	}

	if file.Agent != nil {
		if err := validateSharedControllerAgentConfig(controller, file.Agent); err != nil {
			return err
		}
	}

	return nil
}

func validateControllerCloudflareTunnel(cfg *ControllerCloudflareTunnelConfig, configuredNodeIDs map[string]struct{}) error {
	if cfg == nil {
		return nil
	}
	if strings.TrimSpace(cfg.AccountID) == "" {
		return errors.New("controller.cloudflare_tunnel.account_id is required")
	}
	if strings.TrimSpace(cfg.APIToken) == "" {
		return errors.New("controller.cloudflare_tunnel.api_token or controller.cloudflare_tunnel.api_token_file is required")
	}
	if len(cfg.Tunnels) == 0 {
		return errors.New("controller.cloudflare_tunnel.tunnels must contain at least one tunnel")
	}
	for name, tunnel := range cfg.Tunnels {
		if strings.TrimSpace(name) == "" {
			return errors.New("controller.cloudflare_tunnel.tunnels keys must not be empty")
		}
		if strings.TrimSpace(tunnel.TunnelID) == "" {
			return fmt.Errorf("controller.cloudflare_tunnel.tunnels[%q].tunnel_id is required", name)
		}
	}
	for nodeID, mapping := range cfg.Nodes {
		if strings.TrimSpace(nodeID) == "" {
			return errors.New("controller.cloudflare_tunnel.nodes keys must not be empty")
		}
		if _, exists := configuredNodeIDs[nodeID]; !exists {
			return fmt.Errorf("controller.cloudflare_tunnel.nodes[%q] must reference a configured controller.nodes entry", nodeID)
		}
		if strings.TrimSpace(mapping.Tunnel) == "" {
			return fmt.Errorf("controller.cloudflare_tunnel.nodes[%q].tunnel is required", nodeID)
		}
		if _, exists := cfg.Tunnels[mapping.Tunnel]; !exists {
			return fmt.Errorf("controller.cloudflare_tunnel.nodes[%q].tunnel references unknown tunnel %q", nodeID, mapping.Tunnel)
		}
	}
	return nil
}

func validateControllerUpdatesSemverAllow(allow []string) error {
	normalizedAllow := make([]string, 0, len(allow))
	for _, value := range allow {
		normalizedAllow = append(normalizedAllow, strings.TrimSpace(value))
	}
	semverConfig := ControllerUpdatesSemverConfig{DefaultAllow: normalizedAllow}
	if err := validationError(configValidator.StructPartial(semverConfig, "DefaultAllow"), func(field validator.FieldError) error {
		return errors.New("controller.updates.semver.default_allow must contain only patch, minor, or major")
	}); err != nil {
		return err
	}
	seen := make(map[string]struct{}, len(normalizedAllow))
	for _, value := range normalizedAllow {
		if _, exists := seen[value]; exists {
			return fmt.Errorf("controller.updates.semver.default_allow[%q] is duplicated", value)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func validateAgent(file *File) error {
	agent := file.Agent
	if err := validationError(configValidator.StructPartial(agent, "ControllerAddr", "NodeID", "Token", "RepoDir", "StateDir"), func(field validator.FieldError) error {
		switch field.StructField() {
		case "ControllerAddr":
			return errors.New("agent.controller_addr is required")
		case "NodeID":
			return errors.New("agent.node_id is required")
		case "Token":
			return errors.New("agent.token is required")
		case "RepoDir":
			return errors.New("agent.repo_dir is required")
		case "StateDir":
			return errors.New("agent.state_dir is required")
		default:
			return nil
		}
	}); err != nil {
		return err
	}
	seenHeaders := make(map[string]struct{}, len(agent.ControllerHeaders))
	for _, header := range agent.ControllerHeaders {
		name := strings.TrimSpace(header.Name)
		header = AgentControllerHeaderConfig{Name: name, Value: strings.TrimSpace(header.Value)}
		if err := validationError(configValidator.StructPartial(header, "Name", "Value"), func(field validator.FieldError) error {
			switch field.StructField() {
			case "Name":
				return errors.New("agent.controller_headers[].name is required")
			case "Value":
				return fmt.Errorf("agent.controller_headers[%q].value or agent.controller_headers[%q].value_file is required", name, name)
			default:
				return nil
			}
		}); err != nil {
			return err
		}
		key := strings.ToLower(name)
		if _, exists := seenHeaders[key]; exists {
			return fmt.Errorf("agent.controller_headers[%q] is duplicated", name)
		}
		seenHeaders[key] = struct{}{}
	}
	if file.Controller != nil {
		if err := validateSharedControllerAgentConfig(file.Controller, agent); err != nil {
			return err
		}
	}
	return nil
}

func validationError(err error, format func(validator.FieldError) error) error {
	if err == nil {
		return nil
	}
	var validationErrors validator.ValidationErrors
	ok := errors.As(err, &validationErrors)
	if !ok || len(validationErrors) == 0 {
		return err
	}
	if formatted := format(validationErrors[0]); formatted != nil {
		return formatted
	}
	return err
}

func validateSharedControllerAgentConfig(controller *ControllerConfig, agent *AgentConfig) error {
	if agent.NodeID != LocalMainNodeID {
		return fmt.Errorf("agent.node_id must be %q when controller and agent share one config file", LocalMainNodeID)
	}
	if samePath(controller.RepoDir, agent.RepoDir) {
		return errors.New("controller.repo_dir and agent.repo_dir must not use the same path")
	}
	if !hasNode(controller.Nodes, LocalMainNodeID) {
		return fmt.Errorf("controller.nodes must include %q when a local agent is configured", LocalMainNodeID)
	}
	return nil
}

func hasNode(nodes []NodeConfig, nodeID string) bool {
	for _, node := range nodes {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

func samePath(left, right string) bool {
	if left == "" || right == "" {
		return false
	}
	return physicalPath(left) == physicalPath(right)
}

func physicalPath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return filepath.Clean(absPath)
	}
	return filepath.Clean(resolvedPath)
}

func (controller *ControllerConfig) NodeTokenMap() map[string]string {
	tokens := make(map[string]string, len(controller.Nodes))
	for _, node := range controller.Nodes {
		tokens[node.Token] = node.ID
	}
	return tokens
}

func (controller *ControllerConfig) EnabledAccessTokenMap() map[string]string {
	tokens := make(map[string]string)
	for _, token := range controller.AccessTokens {
		if token.Enabled != nil && !*token.Enabled {
			continue
		}
		tokens[token.Token] = token.Name
	}
	return tokens
}

func (agent *AgentConfig) CaddyGeneratedDir() string {
	if agent.Caddy != nil && agent.Caddy.GeneratedDir != "" {
		return agent.Caddy.GeneratedDir
	}
	return filepath.Join(agent.StateDir, "caddy", "generated")
}
