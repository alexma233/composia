package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	envControllerAddr    = "COMPOSIA_CONTROLLER_ADDR"
	envAccessToken       = "COMPOSIA_ACCESS_TOKEN"
	envControllerHeaders = "COMPOSIA_CONTROLLER_HEADERS"
	serviceCommandName   = "service"
)

type outputMode string

const (
	outputModeHuman outputMode = "human"
	outputModeJSON  outputMode = "json"
	outputModeTerse outputMode = "terse"
)

type globalConfig struct {
	addr      string
	token     string
	tokenFile string
	headers   map[string]string
	output    outputMode
	json      bool
	help      bool
}

type app struct {
	ctx    context.Context
	out    io.Writer
	errOut io.Writer
	cfg    globalConfig
	client *controllerClient
}

type controllerClient struct {
	system          controllerv1connect.SystemServiceClient
	services        controllerv1connect.ServiceQueryServiceClient
	serviceCommands controllerv1connect.ServiceCommandServiceClient
	instances       controllerv1connect.ServiceInstanceServiceClient
	tasks           controllerv1connect.TaskServiceClient
	backupQueries   controllerv1connect.BackupQueryServiceClient
	backupCommands  controllerv1connect.BackupCommandServiceClient
	nodes           controllerv1connect.NodeQueryServiceClient
	nodeCommands    controllerv1connect.NodeMaintenanceServiceClient
	docker          controllerv1connect.DockerQueryServiceClient
	dockerCommands  controllerv1connect.DockerCommandServiceClient
	repos           controllerv1connect.RepoQueryServiceClient
	repoCommands    controllerv1connect.RepoCommandServiceClient
	secrets         controllerv1connect.SecretServiceClient
}

// Run executes the user-facing CLI command surface.
func Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	root := newRootCommand(ctx, out, errOut)
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		if strings.Contains(err.Error(), "unknown command") {
			root.SetOut(errOut)
			_ = root.Help()
		}
		return err
	}
	return nil
}

func isControllerCommand(command string) bool {
	switch command {
	case "system", serviceCommandName, "instance", "task", "backup", "node", "container", "network", "volume", "image", "rustic", "repo", "secret", "config":
		return true
	default:
		return false
	}
}

func isHelpArg(args []string) bool {
	if len(args) == 0 {
		return false
	}
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func trimHelpArgs(args []string) []string {
	trimmed := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--help" && arg != "-h" {
			trimmed = append(trimmed, arg)
		}
	}
	return trimmed
}

func PrintUsage(w io.Writer) {
	_, _ = fmt.Fprint(w, `usage: composia [global flags] <command> [args]

Global flags:
  --addr string        controller base URL (overrides CLI config and COMPOSIA_CONTROLLER_ADDR)
  --token string       controller access token (mutually exclusive with --token-file)
  --token-file string  file containing the controller access token (mutually exclusive with --token)
  --header string      custom controller header, repeatable as "Name: value"
  --output mode        output mode: human, json, terse (default human)
  --json              print protobuf JSON for unary RPCs

Commands:
  service     List/create services or target one service by name
  task        Inspect task status, logs, confirmations, and reruns
  node        Inspect nodes and run node maintenance
  container   Low-level container operations by node and container ID
  backup      List and restore backups
  repo        Low-level repository file operations
  secret      Low-level encrypted file operations
  system      Controller status, reload, and capability checks
  instance    Low-level service instance operations by service and node
  network     Low-level Docker network operations by node
  volume      Low-level Docker volume operations by node
  image       Low-level Docker image operations by node
  rustic      Rustic repository maintenance
  config      Configure controller address and access token
  completion  Generate shell completion scripts
  version     Print CLI version

Run 'composia help <command>' for command details.
`)
}

type commandHelpInfo struct {
	description string
	examples    []string
}

var commandUsages = map[string]string{ //nolint:gosec
	"system":                "usage: composia system <status|reload|capabilities>\n",
	"service":               "usage: composia service <list|create|<service> [action]>\n",
	"system status":         "usage: composia system status\n",
	"system reload":         "usage: composia system reload\n",
	"system capabilities":   "usage: composia system capabilities\n",
	"service list":          "usage: composia service list [--status status] [--page-size n] [--page n]\n",
	"service create":        "usage: composia service create [--message text] <name>\n",
	"service edit":          "usage: composia service <service> edit [--message text] <path>\n",
	"service updates":       "usage: composia service <service> updates [--node node]\n",
	"service up":            "usage: composia service <service> up [--detach] [--wait] [--follow] [--timeout duration] [--node node] [--recreate auto|never|always]\n",
	"service down":          "usage: composia service <service> down [--detach] [--wait] [--follow] [--timeout duration] [--node node]\n",
	"service update":        "usage: composia service <service> update [--detach] [--wait] [--follow] [--timeout duration] [--node node] [--image name] [--use-detected] [--all-detected] [--set-image name=tag] [--recreate auto|never|always]\n",
	"service restart":       "usage: composia service <service> restart [--detach] [--wait] [--follow] [--timeout duration] [--node node]\n",
	"service backup":        "usage: composia service <service> backup [--detach] [--wait] [--follow] [--timeout duration] [--node node] [--data name]\n",
	"service dns-update":    "usage: composia service <service> dns-update [--detach] [--wait] [--follow] [--timeout duration] [--node node]\n",
	"service caddy-sync":    "usage: composia service <service> caddy-sync [--detach] [--wait] [--follow] [--timeout duration] [--node node]\n",
	"service tunnel-sync":   "usage: composia service <service> tunnel-sync [--detach] [--wait] [--follow] [--timeout duration]\n",
	"service migrate":       "usage: composia service <service> migrate [--wait] [--follow] [--timeout duration] --source node --target node\n",
	"service logs":          "usage: composia service <service> logs [--task task] [--node node]\n",
	"service ps":            "usage: composia service <service> ps\n",
	"service exec":          "usage: composia service <service> exec [--node node] [--container name] [--no-tty] [command] [args...]\n",
	"instance":              "usage: composia instance <list|get|deploy|update|stop|restart|backup>\n",
	"instance list":         "usage: composia instance list <service>\n",
	"instance get":          "usage: composia instance get [--containers] <service> <node>\n",
	"instance deploy":       "usage: composia instance deploy [--wait] [--follow] [--timeout duration] [--recreate auto|never|always] <service> <node>\n",
	"instance update":       "usage: composia instance update [--wait] [--follow] [--timeout duration] [--recreate auto|never|always] <service> <node>\n",
	"instance stop":         "usage: composia instance stop [--wait] [--follow] [--timeout duration] <service> <node>\n",
	"instance restart":      "usage: composia instance restart [--wait] [--follow] [--timeout duration] <service> <node>\n",
	"instance backup":       "usage: composia instance backup [--wait] [--follow] [--timeout duration] [--data name] <service> <node>\n",
	"task":                  "usage: composia task <list|get|logs|wait|run-again|approve|reject>\n",
	"task list":             "usage: composia task list [filters]\n",
	"task get":              "usage: composia task get <task>\n",
	"task logs":             "usage: composia task logs <task>\n",
	"task wait":             "usage: composia task wait [--follow] [--timeout duration] [--interval duration] <task>\n",
	"task run-again":        "usage: composia task run-again [--wait] [--follow] [--timeout duration] <task>\n",
	"task approve":          "usage: composia task approve [--comment text] <task>\n",
	"task reject":           "usage: composia task reject [--comment text] <task>\n",
	"backup":                "usage: composia backup <list|get|restore>\n",
	"backup list":           "usage: composia backup list [--service name] [--status status] [--data name]\n",
	"backup get":            "usage: composia backup get <backup>\n",
	"backup restore":        "usage: composia backup restore [--wait] [--follow] [--timeout duration] <node> <backup>\n",
	"node":                  "usage: composia node <list|get|tasks|stats|sync-caddy-files|reload-caddy|prune>\n",
	"node list":             "usage: composia node list\n",
	"node get":              "usage: composia node get <node>\n",
	"node tasks":            "usage: composia node tasks [--status status] <node>\n",
	"node stats":            "usage: composia node stats <node>\n",
	"node sync-caddy-files": "usage: composia node sync-caddy-files [--wait] [--follow] [--timeout duration] [--service name] [--full-rebuild] <node>\n",
	"node reload-caddy":     "usage: composia node reload-caddy [--wait] [--follow] [--timeout duration] <node>\n",
	"node prune":            "usage: composia node prune [--wait] [--follow] [--timeout duration] [--target all|container|image|network|volume] <node>\n",
	"container":             "usage: composia container <node> <list|get|logs|start|stop|restart|remove|exec>\n",
	"container list":        "usage: composia container <node> list [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]\n",
	"container get":         "usage: composia container <node> get <container>\n",
	"container logs":        "usage: composia container <node> logs [--tail n|all] [--timestamps] <container>\n",
	"container start":       "usage: composia container <node> start [--wait] [--follow] [--timeout duration] <container>\n",
	"container stop":        "usage: composia container <node> stop [--wait] [--follow] [--timeout duration] <container>\n",
	"container restart":     "usage: composia container <node> restart [--wait] [--follow] [--timeout duration] <container>\n",
	"container remove":      "usage: composia container <node> remove [--wait] [--follow] [--timeout duration] [--force] [--volumes] <container>\n",
	"container exec":        "usage: composia container <node> exec [--tty] [--stdin-file file] [--timeout duration] [--max-output bytes] <container> <command> [args...]\n",
	"network":               "usage: composia network <node> <list|get|remove>\n",
	"network list":          "usage: composia network <node> list [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]\n",
	"network get":           "usage: composia network <node> get <network>\n",
	"network remove":        "usage: composia network <node> remove [--wait] [--follow] [--timeout duration] <network>\n",
	"volume":                "usage: composia volume <node> <list|get|remove>\n",
	"volume list":           "usage: composia volume <node> list [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]\n",
	"volume get":            "usage: composia volume <node> get <volume>\n",
	"volume remove":         "usage: composia volume <node> remove [--wait] [--follow] [--timeout duration] <volume>\n",
	"image":                 "usage: composia image <node> <list|get|remove>\n",
	"image list":            "usage: composia image <node> list [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]\n",
	"image get":             "usage: composia image <node> get <image>\n",
	"image remove":          "usage: composia image <node> remove [--wait] [--follow] [--timeout duration] [--force] <image>\n",
	"rustic":                "usage: composia rustic <init|forget|prune>\n",
	"rustic init":           "usage: composia rustic init [--wait] [--follow] [--timeout duration] <node>\n",
	"rustic forget":         "usage: composia rustic forget [--wait] [--follow] [--timeout duration] [--service name] [--data name] <node>\n",
	"rustic prune":          "usage: composia rustic prune [--wait] [--follow] [--timeout duration] [--service name] [--data name] <node>\n",
	"repo":                  "usage: composia repo <head|files|get|edit|update|mkdir|mv|rm|history|sync|validate>\n",
	"repo head":             "usage: composia repo head\n",
	"repo files":            "usage: composia repo files [--recursive] [path]\n",
	"repo get":              "usage: composia repo get <path>\n",
	"repo edit":             "usage: composia repo edit [--create] [--message text] <path>\n",
	"repo update":           "usage: composia repo update --file file [--message text] <path>\n",
	"repo mkdir":            "usage: composia repo mkdir [--message text] <path>\n",
	"repo mv":               "usage: composia repo mv [--message text] <source> <destination>\n",
	"repo rm":               "usage: composia repo rm [--message text] <path>\n",
	"repo history":          "usage: composia repo history [--page-size n] [--cursor cursor]\n",
	"repo sync":             "usage: composia repo sync\n",
	"repo validate":         "usage: composia repo validate\n",
	"secret":                "usage: composia secret <get|edit|update>\n",
	"secret get":            "usage: composia secret get <service> <file>\n",
	"secret edit":           "usage: composia secret edit [--message text] <service> <file>\n",
	"secret update":         "usage: composia secret update --file file [--message text] <service> <file>\n",
	"config":                "usage: composia config <get|set|unset|path|setup|set-token|unset-token>\n",
	"config get":            "usage: composia config get [key]\n",
	"config set":            "usage: composia config set <addr|token_file|token_keyring> <value>\n",
	"config unset":          "usage: composia config unset <addr|token|token_file|token_keyring>\n",
	"config path":           "usage: composia config path\n",
	"config setup":          "usage: composia config setup [--stdin] [--file|--inline]\n",
	"config set-token":      "usage: composia config set-token [--stdin] [--file|--inline]\n",
	"config unset-token":    "usage: composia config unset-token\n",
	"completion":            "usage: composia completion <bash|zsh|fish>\n",
	"completion bash":       "usage: composia completion bash\n",
	"completion zsh":        "usage: composia completion zsh\n",
	"completion fish":       "usage: composia completion fish\n",
}

var commandHelp = map[string]commandHelpInfo{
	"service": {
		description: "Manage services. Use list/create for the service collection; use 'composia service <name> [action]' for one concrete service.",
		examples: []string{
			"composia service create vaultwarden",
			"composia service vaultwarden",
			"composia service vaultwarden edit docker-compose.yaml",
			"composia service vaultwarden up",
			"composia service vaultwarden exec",
		},
	},
	"service <name>": {
		description: "Show one service. Add an action after the service name to edit, deploy, read logs, or enter a container.",
		examples:    []string{"composia service vaultwarden", "composia service vaultwarden --containers", "composia service vaultwarden up"},
	},
	"service list": {
		description: "List declared and discovered services with runtime status and instance counts.",
		examples:    []string{"composia service list", "composia service list --status running"},
	},
	"service create": {
		description: "Create a new top-level service workspace with empty docker-compose.yaml and composia-meta.yaml files.",
		examples:    []string{"composia service create vaultwarden", "composia service create --message 'create vaultwarden service' vaultwarden"},
	},
	"service edit": {
		description: "Edit a service repo file in your editor. The path is relative to the service directory and is used literally.",
		examples:    []string{"composia service vaultwarden edit docker-compose.yaml", "composia service vaultwarden edit composia-meta.yaml", "composia service vaultwarden edit caddy/Caddyfile"},
	},
	"service up": {
		description: "Deploy a service. By default this waits for the task and follows task logs; use --detach to return immediately with the task ID.",
		examples:    []string{"composia service vaultwarden up", "composia service vaultwarden up --node miniserver", "composia service vaultwarden up --detach"},
	},
	"service down": {
		description: "Stop a service. By default this waits for the stop task; use --detach to return immediately.",
		examples:    []string{"composia service vaultwarden down", "composia service vaultwarden down --node miniserver"},
	},
	"service update": {
		description: "Run an image update for a service. You can select detected updates, force all detected updates, or set explicit image tags.",
		examples:    []string{"composia service vaultwarden update", "composia service vaultwarden update --all-detected", "composia service vaultwarden update --set-image app=1.2.3"},
	},
	"service restart": {
		description: "Restart a service and wait for the task by default.",
		examples:    []string{"composia service vaultwarden restart", "composia service vaultwarden restart --node miniserver"},
	},
	"service backup": {
		description: "Create a backup for a service or one data entry defined in composia-meta.yaml.",
		examples:    []string{"composia service vaultwarden backup", "composia service vaultwarden backup --data app-data"},
	},
	"service dns-update": {
		description: "Run the DNS update task for a service.",
		examples:    []string{"composia service vaultwarden dns-update"},
	},
	"service caddy-sync": {
		description: "Regenerate and sync Caddy files for a service.",
		examples:    []string{"composia service vaultwarden caddy-sync"},
	},
	"service tunnel-sync": {
		description: "Sync Cloudflare Tunnel ingress and DNS for a service.",
		examples:    []string{"composia service vaultwarden tunnel-sync"},
	},
	"service migrate": {
		description: "Migrate a service from one node to another.",
		examples:    []string{"composia service vaultwarden migrate --from old-node --to new-node"},
	},
	"service updates": {
		description: "Show detected image update candidates for a service.",
		examples:    []string{"composia service vaultwarden updates", "composia service vaultwarden updates --node miniserver"},
	},
	"service logs": {
		description: "Stream logs for the latest task of a service. Use --task when you already know the task ID.",
		examples:    []string{"composia service vaultwarden logs", "composia service vaultwarden logs --task task_123"},
	},
	"service ps": {
		description: "List containers that belong to a service, including node, container ID, image, state, and compose service name.",
		examples:    []string{"composia service vaultwarden ps"},
	},
	"service exec": {
		description: "Open a shell or run a command inside a service container. It auto-resolves the node and container when there is only one match.",
		examples:    []string{"composia service vaultwarden exec", "composia service vaultwarden exec /bin/bash", "composia service vaultwarden exec --container app sh", "composia service vaultwarden exec --no-tty env"},
	},
}

func PrintCommandUsage(w io.Writer, args []string) error {
	if len(args) == 0 {
		PrintUsage(w)
		return nil
	}
	key := strings.Join(args, " ")
	if usage, ok := commandUsages[key]; ok {
		return printCommandHelp(w, key, usage)
	}
	if helpKey, usage, ok := servicePatternHelp(args); ok {
		return printCommandHelp(w, helpKey, usage)
	}
	matches := make([]string, 0)
	prefix := key + " "
	for candidate := range commandUsages {
		if strings.HasPrefix(candidate, prefix) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		return fmt.Errorf("unknown help topic %q", key)
	}
	sort.Strings(matches)
	for _, match := range matches {
		if err := printCommandHelp(w, match, commandUsages[match]); err != nil {
			return err
		}
	}
	return nil
}

func printCommandHelp(w io.Writer, key string, usage string) error {
	if key == "service" {
		return printServiceCommandHelp(w)
	}
	if _, err := fmt.Fprint(w, usage); err != nil {
		return err
	}
	info := commandHelp[key]
	if info.description != "" {
		if _, err := fmt.Fprintf(w, "\nDescription:\n  %s\n", info.description); err != nil {
			return err
		}
	}
	children := immediateChildCommandKeys(key)
	if len(children) > 0 {
		if _, err := fmt.Fprintln(w, "\nCommands:"); err != nil {
			return err
		}
		for _, child := range children {
			name := strings.TrimPrefix(child, key+" ")
			description := commandHelp[child].description
			if description == "" {
				description = strings.TrimPrefix(strings.TrimSpace(commandUsages[child]), "usage: composia ")
			}
			if _, err := fmt.Fprintf(w, "  %-14s %s\n", name, description); err != nil {
				return err
			}
		}
	}
	if len(info.examples) > 0 {
		if _, err := fmt.Fprintln(w, "\nExamples:"); err != nil {
			return err
		}
		for _, example := range info.examples {
			if _, err := fmt.Fprintf(w, "  %s\n", example); err != nil {
				return err
			}
		}
	}
	return nil
}

func servicePatternHelp(args []string) (string, string, bool) {
	if len(args) < 2 || args[0] != "service" {
		return "", "", false
	}
	serviceName := args[1]
	if len(args) == 2 {
		return "service <name>", fmt.Sprintf("usage: composia service %s [--containers]\n", serviceName), true
	}
	actionKey := "service " + args[2]
	usage, ok := commandUsages[actionKey]
	if !ok {
		return "", "", false
	}
	return actionKey, strings.Replace(usage, "<service>", serviceName, 1), true
}

func printServiceCommandHelp(w io.Writer) error {
	if _, err := fmt.Fprint(w, commandUsages["service"]); err != nil {
		return err
	}
	info := commandHelp["service"]
	if _, err := fmt.Fprintf(w, "\nDescription:\n  %s\n", info.description); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, `
Service collection commands:
  list           List declared and discovered services with runtime status and instance counts.
  create         Create a new top-level service workspace with empty docker-compose.yaml and composia-meta.yaml files.

Per-service actions:
`); err != nil {
		return err
	}
	for _, action := range []string{"edit", "up", "down", "update", "restart", "backup", "dns-update", "caddy-sync", "tunnel-sync", "migrate", "updates", "logs", "ps", "exec"} {
		key := "service " + action
		if _, err := fmt.Fprintf(w, "  %-14s %s\n", action, commandHelp[key].description); err != nil {
			return err
		}
	}
	if len(info.examples) == 0 {
		return nil
	}
	if _, err := fmt.Fprintln(w, "\nExamples:"); err != nil {
		return err
	}
	for _, example := range info.examples {
		if _, err := fmt.Fprintf(w, "  %s\n", example); err != nil {
			return err
		}
	}
	return nil
}

func immediateChildCommandKeys(key string) []string {
	prefix := key + " "
	childWordCount := strings.Count(key, " ") + 1
	children := make([]string, 0)
	for candidate := range commandUsages {
		if strings.HasPrefix(candidate, prefix) && strings.Count(candidate, " ") == childWordCount {
			children = append(children, candidate)
		}
	}
	sort.Strings(children)
	return children
}

func parseGlobalFlags(args []string) (globalConfig, []string, error) {
	cfg := globalConfig{output: outputModeHuman}
	var headerValues headerFlag
	fs := flag.NewFlagSet("composia", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.addr, "addr", "", "controller base URL")
	fs.StringVar(&cfg.token, "token", "", "controller access token")
	fs.StringVar(&cfg.tokenFile, "token-file", "", "controller access token file")
	fs.Var(&headerValues, "header", `custom controller header as "Name: value"`)
	output := fs.String("output", string(outputModeHuman), "output mode: human, json, terse")
	fs.BoolVar(&cfg.json, "json", false, "print JSON")
	fs.BoolVar(&cfg.help, "help", false, "print usage")
	fs.BoolVar(&cfg.help, "h", false, "print usage")
	if err := fs.Parse(args); err != nil {
		return globalConfig{}, nil, err
	}
	if cfg.token != "" && cfg.tokenFile != "" {
		return globalConfig{}, nil, errors.New("--token and --token-file are mutually exclusive")
	}
	headers, err := parseHeaderFlagValues(headerValues)
	if err != nil {
		return globalConfig{}, nil, err
	}
	cfg.headers = headers
	mode := outputMode(strings.TrimSpace(*output))
	switch mode {
	case "", outputModeHuman:
		mode = outputModeHuman
	case outputModeJSON, outputModeTerse:
	default:
		return globalConfig{}, nil, fmt.Errorf("unknown output mode %q", *output)
	}
	if cfg.json {
		mode = outputModeJSON
	}
	cfg.output = mode
	cfg.json = mode == outputModeJSON
	return cfg, fs.Args(), nil
}

func newCommandFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func (application *app) configureClient() error {
	cfg := application.cfg
	localConfig, err := loadCLIConfig()
	if err != nil {
		return err
	}
	if cfg.addr == "" {
		cfg.addr = localConfig[cliConfigKeyAddr]
	}
	if cfg.addr == "" {
		cfg.addr = os.Getenv(envControllerAddr)
	}
	envHeaders, err := parseHeadersJSON(os.Getenv(envControllerHeaders))
	if err != nil {
		return err
	}
	cfg.headers, err = mergeStaticHeaders(envHeaders, cfg.headers)
	if err != nil {
		return err
	}
	if cfg.token == "" && cfg.tokenFile != "" {
		content, err := os.ReadFile(cfg.tokenFile)
		if err != nil {
			return fmt.Errorf("read token file %q: %w", cfg.tokenFile, err)
		}
		cfg.token = strings.TrimSpace(string(content))
	}
	if cfg.token == "" && cfg.tokenFile == "" {
		cfg.token = localConfig[cliConfigKeyToken]
	}
	if cfg.token == "" && cfg.tokenFile == "" {
		cfg.tokenFile = localConfig[cliConfigKeyTokenFile]
	}
	if cfg.token == "" && cfg.tokenFile == "" && localConfig[cliConfigKeyTokenKeyring] != "" {
		content, err := readCLIKeyringToken(localConfig[cliConfigKeyTokenKeyring])
		if err != nil {
			return fmt.Errorf("read controller access token from system keyring %q: %w", localConfig[cliConfigKeyTokenKeyring], err)
		}
		cfg.token = strings.TrimSpace(content)
	}
	if cfg.token == "" && cfg.tokenFile != "" {
		content, err := os.ReadFile(cfg.tokenFile)
		if err != nil {
			return fmt.Errorf("read token file %q: %w", cfg.tokenFile, err)
		}
		cfg.token = strings.TrimSpace(string(content))
	}
	if cfg.token == "" {
		cfg.token = os.Getenv(envAccessToken)
	}
	if strings.TrimSpace(cfg.addr) == "" {
		path, pathErr := cliConfigPath()
		if pathErr != nil {
			return pathErr
		}
		return fmt.Errorf("controller address is required: pass --addr, set addr in %s, or set %s", path, envControllerAddr)
	}
	if strings.TrimSpace(cfg.token) == "" {
		path, pathErr := cliConfigPath()
		if pathErr != nil {
			return pathErr
		}
		return fmt.Errorf("controller access token is required: pass --token, pass --token-file, run composia config set-token, set token/token_file/token_keyring in %s, or set %s", path, envAccessToken)
	}

	baseURL := rpcutil.JoinBaseURL(cfg.addr, rpcutil.ControllerAPIBasePath)
	customHeaders, err := rpcutil.NewStaticHeadersInterceptor(cfg.headers)
	if err != nil {
		return err
	}
	auth := connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(strings.TrimSpace(cfg.token)), customHeaders)
	httpClient := http.DefaultClient
	application.cfg = cfg
	application.client = &controllerClient{
		system:          controllerv1connect.NewSystemServiceClient(httpClient, baseURL, auth),
		services:        controllerv1connect.NewServiceQueryServiceClient(httpClient, baseURL, auth),
		serviceCommands: controllerv1connect.NewServiceCommandServiceClient(httpClient, baseURL, auth),
		instances:       controllerv1connect.NewServiceInstanceServiceClient(httpClient, baseURL, auth),
		tasks:           controllerv1connect.NewTaskServiceClient(httpClient, baseURL, auth),
		backupQueries:   controllerv1connect.NewBackupQueryServiceClient(httpClient, baseURL, auth),
		backupCommands:  controllerv1connect.NewBackupCommandServiceClient(httpClient, baseURL, auth),
		nodes:           controllerv1connect.NewNodeQueryServiceClient(httpClient, baseURL, auth),
		nodeCommands:    controllerv1connect.NewNodeMaintenanceServiceClient(httpClient, baseURL, auth),
		docker:          controllerv1connect.NewDockerQueryServiceClient(httpClient, baseURL, auth),
		dockerCommands:  controllerv1connect.NewDockerCommandServiceClient(httpClient, baseURL, auth),
		repos:           controllerv1connect.NewRepoQueryServiceClient(httpClient, baseURL, auth),
		repoCommands:    controllerv1connect.NewRepoCommandServiceClient(httpClient, baseURL, auth),
		secrets:         controllerv1connect.NewSecretServiceClient(httpClient, baseURL, auth),
	}
	return nil
}

func (application *app) printMessage(message proto.Message) error {
	if !application.isJSONOutput() {
		return fmt.Errorf("human output is not implemented for %T", message)
	}
	data, err := protojson.MarshalOptions{Multiline: true, Indent: "  ", UseProtoNames: true}.Marshal(message)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(application.out, string(data))
	return err
}

func (application *app) printTaskAction(response *controllerv1.TaskActionResponse) error {
	if application.isJSONOutput() {
		return application.printMessage(response)
	}
	return application.writeKV([][2]string{
		{"task_id", response.GetTaskId()},
		{"status", taskStatusText(response.GetStatus())},
		{"repo_revision", response.GetRepoRevision()},
	})
}

func (application *app) printServiceAction(response *controllerv1.RunServiceActionResponse) error {
	if application.isJSONOutput() {
		return application.printMessage(response)
	}
	tasks := response.GetTasks()
	if len(tasks) == 1 {
		if err := application.printTaskAction(tasks[0]); err != nil {
			return err
		}
	} else {
		if err := application.writeKV([][2]string{{"task_count", strconv.Itoa(len(tasks))}}); err != nil {
			return err
		}
		rows := make([][]string, 0, len(tasks))
		for _, task := range tasks {
			rows = append(rows, []string{task.GetTaskId(), taskStatusText(task.GetStatus()), task.GetRepoRevision()})
		}
		if err := application.writeTable([]string{"TASK ID", "STATUS", "REPO REVISION"}, rows); err != nil {
			return err
		}
	}
	if response.GetRepoWrite() == nil {
		return nil
	}
	return application.writeKV([][2]string{
		{"commit_id", response.GetRepoWrite().GetCommitId()},
		{"sync_status", response.GetRepoWrite().GetSyncStatus()},
		{"push_error", response.GetRepoWrite().GetPushError()},
		{"last_successful_pull_at", response.GetRepoWrite().GetLastSuccessfulPullAt()},
	})
}

func (application *app) isJSONOutput() bool {
	return application.cfg.output == outputModeJSON || application.cfg.json
}

func (application *app) isTerseOutput() bool {
	return application.cfg.output == outputModeTerse
}

func newRequest[T any](message *T) *connect.Request[T] {
	req := connect.NewRequest(message)
	req.Header().Set("X-Composia-Source", "cli")
	return req
}

func parsePageFlags(fs *flag.FlagSet) (func() (uint32, uint32, error), *uint) {
	pageSize := fs.Uint("page-size", 50, "page size")
	page := fs.Uint("page", 1, "1-based page number")
	return func() (uint32, uint32, error) {
		pageSizeValue, err := uint32FlagValue("page-size", *pageSize)
		if err != nil {
			return 0, 0, err
		}
		pageValue, err := uint32FlagValue("page", *page)
		if err != nil {
			return 0, 0, err
		}
		return pageSizeValue, pageValue, nil
	}, pageSize
}

func uint32FlagValue(name string, value uint) (uint32, error) {
	if value > math.MaxUint32 {
		return 0, fmt.Errorf("%s exceeds maximum uint32 value %d", name, uint64(math.MaxUint32))
	}
	return uint32(value), nil
}

func commandUsageText(key string) string {
	return strings.TrimPrefix(strings.TrimSpace(commandUsages[key]), "usage: ")
}

type stringListFlag []string

func (values *stringListFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *stringListFlag) Set(value string) error {
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			*values = append(*values, part)
		}
	}
	return nil
}

func (values *stringListFlag) Type() string {
	return "stringSlice"
}

type headerFlag []string

func (values *headerFlag) String() string {
	return strings.Join(*values, ",")
}

func (values *headerFlag) Set(value string) error {
	value = strings.TrimSpace(value)
	if value != "" {
		*values = append(*values, value)
	}
	return nil
}

func (values *headerFlag) Type() string {
	return "header"
}

func parseHeaderFlagValues(values headerFlag) (map[string]string, error) {
	headers := make(map[string]string, len(values))
	for _, value := range values {
		name, headerValue, ok := strings.Cut(value, ":")
		if !ok {
			return nil, fmt.Errorf("custom header %q must use Name: value", value)
		}
		headers[strings.TrimSpace(name)] = strings.TrimSpace(headerValue)
	}
	return rpcutil.NormalizeStaticHeaders(headers)
}

func parseHeadersJSON(value string) (map[string]string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return map[string]string{}, nil
	}
	var headers map[string]string
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return nil, fmt.Errorf("parse %s: %w", envControllerHeaders, err)
	}
	return rpcutil.NormalizeStaticHeaders(headers)
}

func mergeStaticHeaders(left, right map[string]string) (map[string]string, error) {
	merged := make(map[string]string, len(left)+len(right))
	for name, value := range left {
		merged[name] = value
	}
	for name, value := range right {
		merged[name] = value
	}
	return rpcutil.NormalizeStaticHeaders(merged)
}

func staticHTTPHeaders(headers map[string]string) http.Header {
	result := make(http.Header, len(headers))
	for name, value := range headers {
		result.Set(name, value)
	}
	return result
}

func requireArgs(args []string, count int, usage string) error {
	if len(args) != count {
		return fmt.Errorf("usage: %s", usage)
	}
	return nil
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func uintText(value uint32) string {
	return strconv.FormatUint(uint64(value), 10)
}

func uint64Text(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func int64Text(value int64) string {
	return strconv.FormatInt(value, 10)
}
