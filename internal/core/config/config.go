package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"forgejo.alexma.top/alexma233/composia/internal/core/schedule"
	"gopkg.in/yaml.v3"
)

const LocalMainNodeID = "main"

type File struct {
	Controller *ControllerConfig `yaml:"controller"`
	Agent      *AgentConfig      `yaml:"agent"`
}

type ControllerConfig struct {
	ListenAddr   string                   `yaml:"listen_addr"`
	RepoDir      string                   `yaml:"repo_dir"`
	StateDir     string                   `yaml:"state_dir"`
	LogDir       string                   `yaml:"log_dir"`
	Backup       *ControllerBackupConfig  `yaml:"backup"`
	Git          *ControllerGitConfig     `yaml:"git"`
	Nodes        []NodeConfig             `yaml:"nodes"`
	AccessTokens []AccessTokenConfig      `yaml:"access_tokens"`
	DNS          *ControllerDNSConfig     `yaml:"dns"`
	Rustic       *ControllerRusticConfig  `yaml:"rustic"`
	Secrets      *ControllerSecretsConfig `yaml:"secrets"`
}

type ControllerBackupConfig struct {
	DefaultSchedule string `yaml:"default_schedule"`
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
	TokenFile string `yaml:"token_file"`
}

type NodeConfig struct {
	ID          string `yaml:"id"`
	DisplayName string `yaml:"display_name"`
	Enabled     *bool  `yaml:"enabled"`
	PublicIPv4  string `yaml:"public_ipv4"`
	PublicIPv6  string `yaml:"public_ipv6"`
	Token       string `yaml:"token"`
}

type AccessTokenConfig struct {
	Name    string `yaml:"name"`
	Token   string `yaml:"token"`
	Enabled *bool  `yaml:"enabled"`
	Comment string `yaml:"comment"`
}

type ControllerDNSConfig struct {
	Cloudflare *CloudflareDNSConfig `yaml:"cloudflare"`
}

type CloudflareDNSConfig struct {
	APITokenFile string `yaml:"api_token_file"`
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
	Provider      string `yaml:"provider"`
	IdentityFile  string `yaml:"identity_file"`
	RecipientFile string `yaml:"recipient_file"`
	Armor         *bool  `yaml:"armor"`
}

type AgentConfig struct {
	ControllerAddr string            `yaml:"controller_addr"`
	ControllerGRPC bool              `yaml:"controller_grpc"`
	NodeID         string            `yaml:"node_id"`
	Token          string            `yaml:"token"`
	RepoDir        string            `yaml:"repo_dir"`
	StateDir       string            `yaml:"state_dir"`
	Caddy          *AgentCaddyConfig `yaml:"caddy"`
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
	content, err := os.ReadFile(path)
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
	return &file, nil
}

func validateController(file *File) error {
	controller := file.Controller
	if controller.ListenAddr == "" {
		return fmt.Errorf("controller.listen_addr is required")
	}
	if controller.RepoDir == "" {
		return fmt.Errorf("controller.repo_dir is required")
	}
	if controller.StateDir == "" {
		return fmt.Errorf("controller.state_dir is required")
	}
	if controller.LogDir == "" {
		return fmt.Errorf("controller.log_dir is required")
	}
	if controller.Nodes == nil {
		return fmt.Errorf("controller.nodes must be present, even if it is empty")
	}

	seenNodeIDs := make(map[string]struct{}, len(controller.Nodes))
	seenNodeTokens := make(map[string]string, len(controller.Nodes))
	for _, node := range controller.Nodes {
		if node.ID == "" {
			return fmt.Errorf("controller.nodes[].id is required")
		}
		if node.Token == "" {
			return fmt.Errorf("controller.nodes[%q].token is required", node.ID)
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
				return fmt.Errorf("controller.rustic.main_nodes[] must not be empty")
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

	if controller.Backup != nil {
		if err := schedule.Validate(controller.Backup.DefaultSchedule); err != nil {
			return fmt.Errorf("controller.backup.default_schedule: %w", err)
		}
	}

	if controller.Git != nil && controller.Git.RemoteURL != "" && controller.Git.PullInterval == "" {
		return fmt.Errorf("controller.git.pull_interval is required when controller.git.remote_url is set")
	}

	seenAccessTokens := make(map[string]string, len(controller.AccessTokens))
	for _, token := range controller.AccessTokens {
		if token.Name == "" {
			return fmt.Errorf("controller.access_tokens[].name is required")
		}
		if token.Token == "" {
			return fmt.Errorf("controller.access_tokens[%q].token is required", token.Name)
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
		if controller.Secrets.Provider != "age" {
			return fmt.Errorf("controller.secrets.provider must be age")
		}
		if controller.Secrets.IdentityFile == "" {
			return fmt.Errorf("controller.secrets.identity_file is required")
		}
	}

	if file.Agent != nil {
		if err := validateSharedControllerAgentConfig(controller, file.Agent); err != nil {
			return err
		}
	}

	return nil
}

func validateAgent(file *File) error {
	agent := file.Agent
	if agent.ControllerAddr == "" {
		return fmt.Errorf("agent.controller_addr is required")
	}
	if agent.NodeID == "" {
		return fmt.Errorf("agent.node_id is required")
	}
	if agent.Token == "" {
		return fmt.Errorf("agent.token is required")
	}
	if agent.RepoDir == "" {
		return fmt.Errorf("agent.repo_dir is required")
	}
	if agent.StateDir == "" {
		return fmt.Errorf("agent.state_dir is required")
	}
	if file.Controller != nil {
		if err := validateSharedControllerAgentConfig(file.Controller, agent); err != nil {
			return err
		}
	}
	return nil
}

func validateSharedControllerAgentConfig(controller *ControllerConfig, agent *AgentConfig) error {
	if agent.NodeID != LocalMainNodeID {
		return fmt.Errorf("agent.node_id must be %q when controller and agent share one config file", LocalMainNodeID)
	}
	if samePath(controller.RepoDir, agent.RepoDir) {
		return fmt.Errorf("controller.repo_dir and agent.repo_dir must not use the same path")
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
	leftClean := filepath.Clean(left)
	rightClean := filepath.Clean(right)
	return leftClean == rightClean
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
