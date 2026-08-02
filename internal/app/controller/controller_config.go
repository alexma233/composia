package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"gopkg.in/yaml.v3"
)

type editableControllerConfigFile struct {
	Controller *editableControllerConfig `yaml:"controller"`
}

type editableControllerConfig struct {
	AutoDeploy    *editableAutoDeploy    `yaml:"auto_deploy,omitempty"`
	Backup        *editableBackup        `yaml:"backup,omitempty"`
	Updates       *editableUpdates       `yaml:"updates,omitempty"`
	Rustic        *editableRustic        `yaml:"rustic,omitempty"`
	Nodes         *[]editableNode        `yaml:"nodes,omitempty"`
	Notifications *editableNotifications `yaml:"notifications,omitempty"`
}

type editableAutoDeploy struct {
	Infra    *bool `yaml:"infra,omitempty"`
	Services *bool `yaml:"services,omitempty"`
}

type editableBackup struct {
	DefaultSchedule *string `yaml:"default_schedule,omitempty"`
}

type editableUpdates struct {
	DefaultCheckSchedule *string                `yaml:"default_check_schedule,omitempty"`
	AutoApply            *bool                  `yaml:"auto_apply,omitempty"`
	BackupBeforeUpdate   *bool                  `yaml:"backup_before_update,omitempty"`
	DigestPin            *bool                  `yaml:"digest_pin,omitempty"`
	Semver               *editableUpdatesSemver `yaml:"semver,omitempty"`
}

type editableUpdatesSemver struct {
	DefaultAllow *[]string `yaml:"default_allow,omitempty"`
}

type editableRustic struct {
	Maintenance *editableRusticMaintenance `yaml:"maintenance,omitempty"`
}

type editableRusticMaintenance struct {
	ForgetSchedule *string `yaml:"forget_schedule,omitempty"`
	PruneSchedule  *string `yaml:"prune_schedule,omitempty"`
}

type editableNode struct {
	ID          string  `yaml:"id"`
	DisplayName *string `yaml:"display_name,omitempty"`
	Enabled     *bool   `yaml:"enabled,omitempty"`
	PublicIPv4  *string `yaml:"public_ipv4,omitempty"`
	PublicIPv6  *string `yaml:"public_ipv6,omitempty"`
}

type editableNotifications struct {
	Alertmanager *editableNotification `yaml:"alertmanager,omitempty"`
	SMTP         *editableNotification `yaml:"smtp,omitempty"`
	Telegram     *editableNotification `yaml:"telegram,omitempty"`
}

type editableNotification struct {
	Enabled *bool `yaml:"enabled,omitempty"`
}

type controllerConfigEditor struct {
	configPath     string
	cfg            *config.ControllerConfig
	reload         func(context.Context) error
	reloadRevision func(context.Context, string) error
}

var errControllerConfigChanged = errors.New("controller config changed while reload was in progress")

func (editor *controllerConfigEditor) GetEditableConfig(_ context.Context, _ *connect.Request[controllerv1.GetEditableConfigRequest]) (*connect.Response[controllerv1.GetEditableConfigResponse], error) {
	if err := editor.ensureAvailable(); err != nil {
		return nil, err
	}
	unlock, err := lockControllerConfig(editor.configPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	defer unlock()
	raw, err := os.ReadFile(editor.configPath) //nolint:gosec // The path is the configured controller config.
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("read controller config: %w", err))
	}
	current, err := decodeControllerConfigBytes(raw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := ensureRemoteConfigEnabled(current); err != nil {
		return nil, err
	}
	content, err := marshalEditableControllerConfig(current)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&controllerv1.GetEditableConfigResponse{Yaml: content, Revision: configRevision(raw)}), nil
}

func (editor *controllerConfigEditor) UpdateEditableConfig(ctx context.Context, req *connect.Request[controllerv1.UpdateEditableConfigRequest]) (*connect.Response[controllerv1.UpdateEditableConfigResponse], error) {
	if err := editor.ensureAvailable(); err != nil {
		return nil, err
	}
	if req.Msg == nil || strings.TrimSpace(req.Msg.GetYaml()) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("yaml is required"))
	}
	if strings.TrimSpace(req.Msg.GetBaseRevision()) == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("base_revision is required"))
	}
	if len(req.Msg.GetYaml()) > 1<<20 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("yaml is too large"))
	}
	unlock, err := lockControllerConfig(editor.configPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	locked := true
	defer func() {
		if locked {
			unlock()
		}
	}()

	raw, err := os.ReadFile(editor.configPath) //nolint:gosec // The path is the configured controller config.
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("read controller config: %w", err))
	}
	currentRevision := configRevision(raw)
	if currentRevision != req.Msg.GetBaseRevision() {
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("base_revision %q does not match current config revision %q", req.Msg.GetBaseRevision(), currentRevision))
	}
	current, err := decodeControllerConfigBytes(raw)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := ensureRemoteConfigEnabled(current); err != nil {
		return nil, err
	}

	edited, err := decodeEditableControllerConfig(req.Msg.GetYaml())
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	candidate, err := applyEditableControllerConfig(raw, edited)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	mode, err := controllerConfigMode(editor.configPath)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	candidateCfg, err := validateControllerConfigBytes(editor.configPath, candidate, mode)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	if err := validateControllerReload(editor.cfg, candidateCfg); err != nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, err)
	}
	if string(candidate) == string(raw) {
		return connect.NewResponse(&controllerv1.UpdateEditableConfigResponse{Revision: currentRevision}), nil
	}
	latestRaw, err := os.ReadFile(editor.configPath) //nolint:gosec // The path is the configured controller config.
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("read controller config before replace: %w", err))
	}
	if configRevision(latestRaw) != currentRevision {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("controller config changed while the edit was being validated"))
	}
	reload := editor.reload
	if editor.reloadRevision != nil {
		reload = func(ctx context.Context) error { return editor.reloadRevision(ctx, configRevision(candidate)) }
	}
	if reload == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errors.New("controller reload is not available"))
	}
	writeErr := atomicWriteControllerConfig(editor.configPath, candidate, mode)
	if writeErr != nil {
		if !controllerConfigWriteInstalled(writeErr) {
			return nil, connect.NewError(connect.CodeInternal, writeErr)
		}
		log.Printf("controller config replaced but directory sync failed: %v", writeErr)
	}
	queueErr := reload(ctx)
	unlock()
	locked = false
	if queueErr != nil {
		if restoreErr := restoreControllerConfigIfCurrent(editor.configPath, configRevision(candidate), raw); restoreErr != nil {
			return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("queue controller config reload: %w; restore failed: %v", queueErr, restoreErr))
		}
		return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("queue controller config reload: %w", queueErr))
	}
	return connect.NewResponse(&controllerv1.UpdateEditableConfigResponse{Revision: configRevision(candidate)}), nil
}

func (editor *controllerConfigEditor) ensureAvailable() error {
	if editor == nil || editor.cfg == nil || editor.configPath == "" || (editor.reload == nil && editor.reloadRevision == nil) {
		return connect.NewError(connect.CodeUnimplemented, errors.New("remote controller config editing is not available"))
	}
	if editor.cfg.RemoteConfig == nil || !editor.cfg.RemoteConfig.Enabled {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("remote controller config editing is disabled"))
	}
	return nil
}

func decodeControllerConfigBytes(raw []byte) (*config.ControllerConfig, error) {
	decoder := yaml.NewDecoder(strings.NewReader(string(raw)))
	decoder.KnownFields(true)
	var file config.File
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode controller config: %w", err)
	}
	if file.Controller == nil {
		return nil, errors.New("controller config is missing a controller section")
	}
	return file.Controller, nil
}

func ensureRemoteConfigEnabled(cfg *config.ControllerConfig) error {
	if cfg == nil || cfg.RemoteConfig == nil || !cfg.RemoteConfig.Enabled {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("remote controller config editing is disabled"))
	}
	return nil
}

func marshalEditableControllerConfig(cfg *config.ControllerConfig) (string, error) {
	if cfg == nil {
		return "", errors.New("controller config is missing")
	}
	edited := editableControllerConfigFile{Controller: &editableControllerConfig{
		Nodes: func() *[]editableNode { nodes := make([]editableNode, 0, len(cfg.Nodes)); return &nodes }(),
	}}
	if cfg.AutoDeploy != nil {
		edited.Controller.AutoDeploy = &editableAutoDeploy{}
		edited.Controller.AutoDeploy.Infra = &cfg.AutoDeploy.Infra
		edited.Controller.AutoDeploy.Services = &cfg.AutoDeploy.Services
	}
	if cfg.Backup != nil {
		edited.Controller.Backup = &editableBackup{DefaultSchedule: stringPtr(cfg.Backup.DefaultSchedule)}
	}
	if cfg.Updates != nil {
		edited.Controller.Updates = &editableUpdates{
			DefaultCheckSchedule: stringPtr(cfg.Updates.DefaultCheckSchedule),
			AutoApply:            cloneBool(cfg.Updates.AutoApply),
			BackupBeforeUpdate:   cloneBool(cfg.Updates.BackupBeforeUpdate),
			DigestPin:            cloneBool(cfg.Updates.DigestPin),
		}
		if cfg.Updates.Semver != nil {
			allow := append([]string(nil), cfg.Updates.Semver.DefaultAllow...)
			edited.Controller.Updates.Semver = &editableUpdatesSemver{DefaultAllow: &allow}
		}
	}
	if cfg.Rustic != nil && cfg.Rustic.Maintenance != nil {
		edited.Controller.Rustic = &editableRustic{Maintenance: &editableRusticMaintenance{
			ForgetSchedule: stringPtr(cfg.Rustic.Maintenance.ForgetSchedule),
			PruneSchedule:  stringPtr(cfg.Rustic.Maintenance.PruneSchedule),
		}}
	}
	for _, node := range cfg.Nodes {
		enabled := true
		if node.Enabled != nil {
			enabled = *node.Enabled
		}
		*edited.Controller.Nodes = append(*edited.Controller.Nodes, editableNode{ID: node.ID, DisplayName: stringPtr(node.DisplayName), Enabled: &enabled, PublicIPv4: stringPtr(node.PublicIPv4), PublicIPv6: stringPtr(node.PublicIPv6)})
	}
	if cfg.Notifications != nil {
		edited.Controller.Notifications = &editableNotifications{
			Alertmanager: editableAlertmanagerNotificationFor(cfg.Notifications.Alertmanager),
			SMTP:         editableSMTPNotificationFor(cfg.Notifications.SMTP),
			Telegram:     editableTelegramNotificationFor(cfg.Notifications.Telegram),
		}
	}
	encoded, err := yaml.Marshal(&edited)
	if err != nil {
		return "", fmt.Errorf("encode editable controller config: %w", err)
	}
	return string(encoded), nil
}

func decodeEditableControllerConfig(content string) (*editableControllerConfig, error) {
	root, err := decodeYAMLDocument(content)
	if err != nil {
		return nil, fmt.Errorf("decode editable controller config: %w", err)
	}
	if err := validateEditableYAMLNode(root); err != nil {
		return nil, err
	}
	decoder := yaml.NewDecoder(strings.NewReader(content))
	decoder.KnownFields(true)
	var file editableControllerConfigFile
	if err := decoder.Decode(&file); err != nil {
		return nil, fmt.Errorf("decode editable controller config: %w", err)
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("editable controller config must contain one YAML document")
		}
		return nil, fmt.Errorf("decode editable controller config: %w", err)
	}
	if file.Controller == nil {
		return nil, errors.New("editable controller config must contain a controller section")
	}
	return file.Controller, nil
}

func applyEditableControllerConfig(raw []byte, edited *editableControllerConfig) ([]byte, error) {
	root, err := decodeYAMLDocument(string(raw))
	if err != nil {
		return nil, fmt.Errorf("decode controller config: %w", err)
	}
	controller, err := mappingValue(root, "controller")
	if err != nil {
		return nil, err
	}
	if edited.AutoDeploy != nil {
		section := ensureMappingValue(controller, "auto_deploy")
		if edited.AutoDeploy.Infra != nil {
			setMappingValue(section, "infra", *edited.AutoDeploy.Infra)
		}
		if edited.AutoDeploy.Services != nil {
			setMappingValue(section, "services", *edited.AutoDeploy.Services)
		}
	}
	if edited.Backup != nil {
		section := ensureMappingValue(controller, "backup")
		if edited.Backup.DefaultSchedule != nil {
			setMappingValue(section, "default_schedule", *edited.Backup.DefaultSchedule)
		}
	}
	if edited.Updates != nil {
		section := ensureMappingValue(controller, "updates")
		if edited.Updates.DefaultCheckSchedule != nil {
			setMappingValue(section, "default_check_schedule", *edited.Updates.DefaultCheckSchedule)
		}
		if edited.Updates.AutoApply != nil {
			setMappingValue(section, "auto_apply", *edited.Updates.AutoApply)
		}
		if edited.Updates.BackupBeforeUpdate != nil {
			setMappingValue(section, "backup_before_update", *edited.Updates.BackupBeforeUpdate)
		}
		if edited.Updates.DigestPin != nil {
			setMappingValue(section, "digest_pin", *edited.Updates.DigestPin)
		}
		if edited.Updates.Semver != nil {
			semver := ensureMappingValue(section, "semver")
			if edited.Updates.Semver.DefaultAllow != nil {
				setMappingValue(semver, "default_allow", append([]string(nil), (*edited.Updates.Semver.DefaultAllow)...))
			}
		}
	}
	if edited.Rustic != nil && edited.Rustic.Maintenance != nil {
		rustic := ensureMappingValue(controller, "rustic")
		maintenance := ensureMappingValue(rustic, "maintenance")
		if edited.Rustic.Maintenance.ForgetSchedule != nil {
			setMappingValue(maintenance, "forget_schedule", *edited.Rustic.Maintenance.ForgetSchedule)
		}
		if edited.Rustic.Maintenance.PruneSchedule != nil {
			setMappingValue(maintenance, "prune_schedule", *edited.Rustic.Maintenance.PruneSchedule)
		}
	}
	if edited.Nodes != nil {
		if err := applyEditableNodeYAML(controller, *edited.Nodes); err != nil {
			return nil, err
		}
	}
	if edited.Notifications != nil {
		if err := applyEditableNotificationYAML(controller, edited.Notifications); err != nil {
			return nil, err
		}
	}
	encoded, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode controller config: %w", err)
	}
	return encoded, nil
}

func decodeYAMLDocument(content string) (*yaml.Node, error) {
	decoder := yaml.NewDecoder(strings.NewReader(content))
	var root yaml.Node
	if err := decoder.Decode(&root); err != nil {
		return nil, err
	}
	var extra yaml.Node
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("configuration must contain one YAML document")
		}
		return nil, err
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("configuration must contain a YAML mapping")
	}
	return &root, nil
}

func validateEditableYAMLNode(root *yaml.Node) error {
	var walk func(*yaml.Node, int) error
	walk = func(node *yaml.Node, depth int) error {
		if depth > 32 {
			return errors.New("editable controller config is too deeply nested")
		}
		if node.Kind == yaml.AliasNode {
			return errors.New("YAML aliases are not allowed in editable controller config")
		}
		if node.Kind == yaml.MappingNode {
			for index := 0; index < len(node.Content); index += 2 {
				if node.Content[index].Value == "<<" {
					return errors.New("YAML merge keys are not allowed in editable controller config")
				}
				if err := walk(node.Content[index], depth+1); err != nil {
					return err
				}
				if err := walk(node.Content[index+1], depth+1); err != nil {
					return err
				}
			}
			return nil
		}
		for _, child := range node.Content {
			if err := walk(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(root, 0)
}

func mappingValue(root *yaml.Node, key string) (*yaml.Node, error) {
	mapping := root
	if mapping.Kind == yaml.DocumentNode {
		mapping = root.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("YAML section %q must be a mapping", key)
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			value := mapping.Content[index+1]
			if value.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("YAML section %q must be a mapping", key)
			}
			return value, nil
		}
	}
	return nil, fmt.Errorf("configuration is missing a %q section", key)
}

func mappingEntry(mapping *yaml.Node, key string) (*yaml.Node, error) {
	if mapping.Kind == yaml.DocumentNode {
		mapping = mapping.Content[0]
	}
	if mapping.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("YAML section %q must be a mapping", key)
	}
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1], nil
		}
	}
	return nil, fmt.Errorf("configuration is missing a %q section", key)
}

func ensureMappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key && mapping.Content[index+1].Kind == yaml.MappingNode {
			return mapping.Content[index+1]
		}
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	mapping.Content = append(mapping.Content, keyNode, valueNode)
	return valueNode
}

func setMappingValue(mapping *yaml.Node, key string, value any) {
	encoded, err := yaml.Marshal(value)
	if err != nil {
		return
	}
	var document yaml.Node
	if err := yaml.Unmarshal(encoded, &document); err != nil || len(document.Content) != 1 {
		return
	}
	valueNode := document.Content[0]
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			mapping.Content[index+1] = valueNode
			return
		}
	}
	mapping.Content = append(mapping.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, valueNode)
}

func applyEditableNodeYAML(controller *yaml.Node, edited []editableNode) error {
	nodes, err := mappingEntry(controller, "nodes")
	if err != nil {
		return err
	}
	if nodes.Kind != yaml.SequenceNode {
		return errors.New("controller.nodes must be a sequence")
	}
	if len(edited) != len(nodes.Content) {
		return errors.New("editable nodes must contain exactly the configured nodes")
	}
	byID := make(map[string]editableNode, len(edited))
	for _, node := range edited {
		if node.ID == "" {
			return errors.New("editable node id is required")
		}
		if _, exists := byID[node.ID]; exists {
			return fmt.Errorf("editable node %q is duplicated", node.ID)
		}
		byID[node.ID] = node
	}
	for _, node := range nodes.Content {
		idNode, err := mappingEntry(node, "id")
		if err != nil {
			return err
		}
		editedNode, ok := byID[idNode.Value]
		if !ok {
			return fmt.Errorf("editable node %q is missing", idNode.Value)
		}
		if editedNode.DisplayName != nil {
			setMappingValue(node, "display_name", *editedNode.DisplayName)
		}
		if editedNode.Enabled != nil {
			setMappingValue(node, "enabled", *editedNode.Enabled)
		}
		if editedNode.PublicIPv4 != nil {
			setMappingValue(node, "public_ipv4", *editedNode.PublicIPv4)
		}
		if editedNode.PublicIPv6 != nil {
			setMappingValue(node, "public_ipv6", *editedNode.PublicIPv6)
		}
	}
	return nil
}

func applyEditableNotificationYAML(controller *yaml.Node, edited *editableNotifications) error {
	notifications, err := mappingValue(controller, "notifications")
	if err != nil {
		return err
	}
	for name, value := range map[string]*editableNotification{
		"alertmanager": edited.Alertmanager,
		"smtp":         edited.SMTP,
		"telegram":     edited.Telegram,
	} {
		if value == nil || value.Enabled == nil {
			continue
		}
		section, err := mappingValue(notifications, name)
		if err != nil {
			return fmt.Errorf("notifications.%s is not configured", name)
		}
		setMappingValue(section, "enabled", *value.Enabled)
	}
	return nil
}

func editableAlertmanagerNotificationFor(value *config.ControllerAlertmanagerNotificationConfig) *editableNotification {
	if value == nil {
		return nil
	}
	enabled := value.IsEnabled()
	return &editableNotification{Enabled: &enabled}
}

func editableSMTPNotificationFor(value *config.ControllerSMTPNotificationConfig) *editableNotification {
	if value == nil {
		return nil
	}
	enabled := value.IsEnabled()
	return &editableNotification{Enabled: &enabled}
}

func editableTelegramNotificationFor(value *config.ControllerTelegramNotificationConfig) *editableNotification {
	if value == nil {
		return nil
	}
	enabled := value.IsEnabled()
	return &editableNotification{Enabled: &enabled}
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func editableBoolPtr(value bool) *bool { return &value }

func stringPtr(value string) *string { return &value }

func configRevision(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func controllerConfigMode(path string) (os.FileMode, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, fmt.Errorf("stat controller config: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return 0, errors.New("controller config must not be a symlink")
	}
	if !info.Mode().IsRegular() {
		return 0, errors.New("controller config is not a regular file")
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o600
	}
	return mode, nil
}

func validateControllerConfigBytes(path string, content []byte, mode os.FileMode) (*config.ControllerConfig, error) {
	dir := filepath.Dir(path)
	temp, err := os.CreateTemp(dir, ".composia-config-validate-*")
	if err != nil {
		return nil, fmt.Errorf("create config validation file: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return nil, fmt.Errorf("set config validation file mode: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return nil, fmt.Errorf("write config validation file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return nil, fmt.Errorf("close config validation file: %w", err)
	}
	candidate, err := config.LoadController(tempPath)
	if err != nil {
		return nil, fmt.Errorf("validate controller config: %w", err)
	}
	return candidate, nil
}

type controllerConfigWriteError struct {
	installed bool
	err       error
}

func (err *controllerConfigWriteError) Error() string {
	return err.err.Error()
}

func (err *controllerConfigWriteError) Unwrap() error {
	return err.err
}

func controllerConfigWriteInstalled(err error) bool {
	var writeErr *controllerConfigWriteError
	return errors.As(err, &writeErr) && writeErr.installed
}

func atomicWriteControllerConfig(path string, content []byte, mode os.FileMode) error {
	return atomicWriteControllerConfigWithDirectorySync(path, content, mode, func(directory *os.File) error {
		return directory.Sync()
	})
}

func atomicWriteControllerConfigWithDirectorySync(path string, content []byte, mode os.FileMode, syncDirectory func(*os.File) error) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".composia-config-write-*")
	if err != nil {
		return fmt.Errorf("create atomic config file: %w", err)
	}
	tempPath := temp.Name()
	cleanup := func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}
	if err := temp.Chmod(mode); err != nil {
		cleanup()
		return fmt.Errorf("set atomic config file mode: %w", err)
	}
	if _, err := temp.Write(content); err != nil {
		cleanup()
		return fmt.Errorf("write atomic config file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync atomic config file: %w", err)
	}
	if err := temp.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close atomic config file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("replace controller config: %w", err)
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return &controllerConfigWriteError{
			installed: true,
			err:       fmt.Errorf("open config directory after replacement: %w", err),
		}
	}
	defer directory.Close()
	if err := syncDirectory(directory); err != nil {
		return &controllerConfigWriteError{
			installed: true,
			err:       fmt.Errorf("sync config directory after replacement: %w", err),
		}
	}
	return nil
}

func restoreControllerConfigIfCurrent(path, expectedRevision string, content []byte) error {
	unlock, err := lockControllerConfig(path)
	if err != nil {
		return err
	}
	defer unlock()
	current, err := os.ReadFile(path) //nolint:gosec // The path is the configured controller config.
	if err != nil {
		return fmt.Errorf("read current controller config: %w", err)
	}
	if configRevision(current) != expectedRevision {
		return errControllerConfigChanged
	}
	mode, err := controllerConfigMode(path)
	if err != nil {
		return err
	}
	err = atomicWriteControllerConfig(path, content, mode)
	if err != nil && controllerConfigWriteInstalled(err) {
		log.Printf("controller config restored but directory sync failed: %v", err)
		return nil
	}
	return err
}
