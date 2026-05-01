package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1/controllerv1connect"
	"forgejo.alexma.top/alexma233/composia/internal/platform/rpcutil"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	envControllerAddr = "COMPOSIA_CONTROLLER_ADDR"
	envAccessToken    = "COMPOSIA_ACCESS_TOKEN"
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
	output    outputMode
	json      bool
	terse     bool
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
	backups         controllerv1connect.BackupRecordServiceClient
	nodes           controllerv1connect.NodeQueryServiceClient
	nodeCommands    controllerv1connect.NodeMaintenanceServiceClient
	docker          controllerv1connect.DockerQueryServiceClient
	containers      controllerv1connect.ContainerServiceClient
	repos           controllerv1connect.RepoQueryServiceClient
	repoCommands    controllerv1connect.RepoCommandServiceClient
	secrets         controllerv1connect.SecretServiceClient
}

// Run executes the user-facing CLI command surface.
func Run(ctx context.Context, args []string, out io.Writer, errOut io.Writer) error {
	cfg, rest, err := parseGlobalFlags(args)
	if err != nil {
		return err
	}
	if cfg.help {
		PrintUsage(out)
		return nil
	}
	if len(rest) == 0 {
		PrintUsage(errOut)
		return errors.New("missing command")
	}
	if rest[0] == "help" {
		PrintCommandUsage(out, rest[1:])
		return nil
	}
	if isHelpArg(rest) {
		PrintCommandUsage(out, trimHelpArgs(rest))
		return nil
	}
	if rest[0] == "version" {
		_, err := fmt.Fprintln(out, version.Value)
		return err
	}
	if rest[0] == "completion" {
		application := &app{ctx: ctx, out: out, errOut: errOut, cfg: cfg}
		return application.runCompletion(rest[1:])
	}
	if rest[0] == "skills" {
		application := &app{ctx: ctx, out: out, errOut: errOut, cfg: cfg}
		return application.runSkills(rest[1:])
	}
	if rest[0] == "config" {
		application := &app{ctx: ctx, out: out, errOut: errOut, cfg: cfg}
		return application.runConfig(rest[1:])
	}
	if !isControllerCommand(rest[0]) {
		PrintUsage(errOut)
		return fmt.Errorf("unknown command %q", rest[0])
	}

	application := &app{ctx: ctx, out: out, errOut: errOut, cfg: cfg}
	if err := application.configureClient(); err != nil {
		return err
	}

	switch rest[0] {
	case "system":
		return application.runSystem(rest[1:])
	case "service":
		return application.runService(rest[1:])
	case "instance":
		return application.runInstance(rest[1:])
	case "task":
		return application.runTask(rest[1:])
	case "backup":
		return application.runBackup(rest[1:])
	case "node":
		return application.runNode(rest[1:])
	case "container":
		return application.runContainer(rest[1:])
	case "network":
		return application.runNetwork(rest[1:])
	case "volume":
		return application.runVolume(rest[1:])
	case "image":
		return application.runImage(rest[1:])
	case "rustic":
		return application.runRustic(rest[1:])
	case "repo":
		return application.runRepo(rest[1:])
	case "secret":
		return application.runSecret(rest[1:])
	default:
		return fmt.Errorf("unknown command %q", rest[0])
	}
}

func isControllerCommand(command string) bool {
	switch command {
	case "system", "service", "instance", "task", "backup", "node", "container", "network", "volume", "image", "rustic", "repo", "secret", "config":
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
  --addr string        controller base URL (or COMPOSIA_CONTROLLER_ADDR)
  --token string       controller access token (or COMPOSIA_ACCESS_TOKEN)
  --token-file string  file containing the controller access token
  --output mode        output mode: human, json, terse (default human)
  --json              print protobuf JSON for unary RPCs
  --terse             print compact text for coding agents and scripts

Commands:
  system status|reload
  service list|get|deploy|update|stop|restart|backup|dns-update|caddy-sync|migrate
  instance list|get|deploy|update|stop|restart|backup
  task list|get|logs|wait|run-again|approve|reject
  backup list|get|restore
  node list|get|tasks|stats|reload-caddy|prune
  container list|get|logs|start|stop|restart|remove|exec
  network list|get|remove
  volume list|get|remove
  image list|get|remove
  rustic init|forget|prune
  repo head|files|get|edit|update|history|sync|validate
  secret get|edit|update
  skills list|show
  config get|set|unset|path
  completion bash|zsh|fish
  version
`)
}

var commandUsages = map[string]string{
	"system":             "usage: composia system <status|reload>\n",
	"service":            "usage: composia service <list|get|deploy|update|stop|restart|backup|dns-update|caddy-sync|migrate>\n",
	"system status":      "usage: composia system status\n",
	"system reload":      "usage: composia system reload\n",
	"service list":       "usage: composia service list [--status status] [--page-size n] [--page n]\n",
	"service get":        "usage: composia service get [--containers] <service>\n",
	"service deploy":     "usage: composia service deploy [--wait] [--follow] [--timeout duration] [--node node] <service>\n",
	"service update":     "usage: composia service update [--wait] [--follow] [--timeout duration] [--node node] <service>\n",
	"service stop":       "usage: composia service stop [--wait] [--follow] [--timeout duration] [--node node] <service>\n",
	"service restart":    "usage: composia service restart [--wait] [--follow] [--timeout duration] [--node node] <service>\n",
	"service backup":     "usage: composia service backup [--wait] [--follow] [--timeout duration] [--node node] [--data name] <service>\n",
	"service dns-update": "usage: composia service dns-update [--wait] [--follow] [--timeout duration] [--node node] <service>\n",
	"service caddy-sync": "usage: composia service caddy-sync [--wait] [--follow] [--timeout duration] [--node node] <service>\n",
	"service migrate":    "usage: composia service migrate [--wait] [--follow] [--timeout duration] --source node --target node <service>\n",
	"instance":           "usage: composia instance <list|get|deploy|update|stop|restart|backup>\n",
	"instance list":      "usage: composia instance list <service>\n",
	"instance get":       "usage: composia instance get [--containers] <service> <node>\n",
	"instance deploy":    "usage: composia instance deploy [--wait] [--follow] [--timeout duration] <service> <node>\n",
	"instance update":    "usage: composia instance update [--wait] [--follow] [--timeout duration] <service> <node>\n",
	"instance stop":      "usage: composia instance stop [--wait] [--follow] [--timeout duration] <service> <node>\n",
	"instance restart":   "usage: composia instance restart [--wait] [--follow] [--timeout duration] <service> <node>\n",
	"instance backup":    "usage: composia instance backup [--wait] [--follow] [--timeout duration] [--data name] <service> <node>\n",
	"task":               "usage: composia task <list|get|logs|wait|run-again|approve|reject>\n",
	"task list":          "usage: composia task list [filters]\n",
	"task get":           "usage: composia task get <task>\n",
	"task logs":          "usage: composia task logs <task>\n",
	"task wait":          "usage: composia task wait [--follow] [--timeout duration] [--interval duration] <task>\n",
	"task run-again":     "usage: composia task run-again [--wait] [--follow] [--timeout duration] <task>\n",
	"task approve":       "usage: composia task approve [--comment text] <task>\n",
	"task reject":        "usage: composia task reject [--comment text] <task>\n",
	"backup":             "usage: composia backup <list|get|restore>\n",
	"backup list":        "usage: composia backup list [--service name] [--status status] [--data name]\n",
	"backup get":         "usage: composia backup get <backup>\n",
	"backup restore":     "usage: composia backup restore [--wait] [--follow] [--timeout duration] --node node <backup>\n",
	"node":               "usage: composia node <list|get|tasks|stats|reload-caddy|prune>\n",
	"node list":          "usage: composia node list\n",
	"node get":           "usage: composia node get <node>\n",
	"node tasks":         "usage: composia node tasks [--status status] <node>\n",
	"node stats":         "usage: composia node stats <node>\n",
	"node reload-caddy":  "usage: composia node reload-caddy [--wait] [--follow] [--timeout duration] <node>\n",
	"node prune":         "usage: composia node prune [--wait] [--follow] [--timeout duration] [--target all|container|image|network|volume] <node>\n",
	"container":          "usage: composia container <list|get|logs|start|stop|restart|remove|exec>\n",
	"container list":     "usage: composia container list --node node [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]\n",
	"container get":      "usage: composia container get --node node <container>\n",
	"container logs":     "usage: composia container logs --node node [--tail n|all] [--timestamps] <container>\n",
	"container start":    "usage: composia container start [--wait] [--follow] [--timeout duration] --node node <container>\n",
	"container stop":     "usage: composia container stop [--wait] [--follow] [--timeout duration] --node node <container>\n",
	"container restart":  "usage: composia container restart [--wait] [--follow] [--timeout duration] --node node <container>\n",
	"container remove":   "usage: composia container remove [--wait] [--follow] [--timeout duration] --node node [--force] [--volumes] <container>\n",
	"container exec":     "usage: composia container exec [--tty] [--stdin-file file] [--timeout duration] [--max-output bytes] --node node <container> -- <command> [args...]\n",
	"network":            "usage: composia network <list|get|remove>\n",
	"network list":       "usage: composia network list --node node [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]\n",
	"network get":        "usage: composia network get --node node <network>\n",
	"network remove":     "usage: composia network remove [--wait] [--follow] [--timeout duration] --node node <network>\n",
	"volume":             "usage: composia volume <list|get|remove>\n",
	"volume list":        "usage: composia volume list --node node [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]\n",
	"volume get":         "usage: composia volume get --node node <volume>\n",
	"volume remove":      "usage: composia volume remove [--wait] [--follow] [--timeout duration] --node node <volume>\n",
	"image":              "usage: composia image <list|get|remove>\n",
	"image list":         "usage: composia image list --node node [--search text] [--sort-by field] [--desc] [--page-size n] [--page n]\n",
	"image get":          "usage: composia image get --node node <image>\n",
	"image remove":       "usage: composia image remove [--wait] [--follow] [--timeout duration] --node node [--force] <image>\n",
	"rustic":             "usage: composia rustic <init|forget|prune>\n",
	"rustic init":        "usage: composia rustic init [--wait] [--follow] [--timeout duration] <node>\n",
	"rustic forget":      "usage: composia rustic forget [--wait] [--follow] [--timeout duration] [--service name] [--data name] <node>\n",
	"rustic prune":       "usage: composia rustic prune [--wait] [--follow] [--timeout duration] [--service name] [--data name] <node>\n",
	"repo":               "usage: composia repo <head|files|get|edit|update|history|sync|validate>\n",
	"repo head":          "usage: composia repo head\n",
	"repo files":         "usage: composia repo files [--recursive] [path]\n",
	"repo get":           "usage: composia repo get <path>\n",
	"repo edit":          "usage: composia repo edit [--create] [--message text] <path>\n",
	"repo update":        "usage: composia repo update --file file [--message text] <path>\n",
	"repo history":       "usage: composia repo history [--page-size n] [--cursor cursor]\n",
	"repo sync":          "usage: composia repo sync\n",
	"repo validate":      "usage: composia repo validate\n",
	"secret":             "usage: composia secret <get|edit|update>\n",
	"secret get":         "usage: composia secret get <service> <file>\n",
	"secret edit":        "usage: composia secret edit [--message text] <service> <file>\n",
	"secret update":      "usage: composia secret update --file file [--message text] <service> <file>\n",
	"skills":             "usage: composia skills <list|show>\n",
	"skills list":        "usage: composia skills list\n",
	"skills show":        "usage: composia skills show <skill>\n",
	"config":             "usage: composia config <get|set|unset|path>\n",
	"config get":         "usage: composia config get [key]\n",
	"config set":         "usage: composia config set <addr|token_file> <value>\n",
	"config unset":       "usage: composia config unset <addr|token_file>\n",
	"config path":        "usage: composia config path\n",
	"completion":         "usage: composia completion <bash|zsh|fish>\n",
	"completion bash":    "usage: composia completion bash\n",
	"completion zsh":     "usage: composia completion zsh\n",
	"completion fish":    "usage: composia completion fish\n",
}

func PrintCommandUsage(w io.Writer, args []string) {
	if len(args) == 0 {
		PrintUsage(w)
		return
	}
	key := strings.Join(args, " ")
	if usage, ok := commandUsages[key]; ok {
		_, _ = fmt.Fprint(w, usage)
		return
	}
	matches := make([]string, 0)
	prefix := key + " "
	for candidate := range commandUsages {
		if strings.HasPrefix(candidate, prefix) {
			matches = append(matches, candidate)
		}
	}
	if len(matches) == 0 {
		_, _ = fmt.Fprintf(w, "unknown help topic %q\n", key)
		PrintUsage(w)
		return
	}
	sort.Strings(matches)
	for _, match := range matches {
		_, _ = fmt.Fprint(w, commandUsages[match])
	}
}

func parseGlobalFlags(args []string) (globalConfig, []string, error) {
	cfg := globalConfig{output: outputModeHuman}
	fs := flag.NewFlagSet("composia", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.addr, "addr", "", "controller base URL")
	fs.StringVar(&cfg.token, "token", "", "controller access token")
	fs.StringVar(&cfg.tokenFile, "token-file", "", "controller access token file")
	output := fs.String("output", string(outputModeHuman), "output mode: human, json, terse")
	fs.BoolVar(&cfg.json, "json", false, "print JSON")
	fs.BoolVar(&cfg.terse, "terse", false, "print compact text")
	fs.BoolVar(&cfg.help, "help", false, "print usage")
	fs.BoolVar(&cfg.help, "h", false, "print usage")
	if err := fs.Parse(args); err != nil {
		return globalConfig{}, nil, err
	}
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
	if cfg.terse {
		mode = outputModeTerse
	}
	cfg.output = mode
	cfg.json = mode == outputModeJSON
	cfg.terse = mode == outputModeTerse
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
		cfg.addr = os.Getenv(envControllerAddr)
	}
	if cfg.addr == "" {
		cfg.addr = localConfig[cliConfigKeyAddr]
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
	if cfg.token == "" && cfg.tokenFile == "" {
		cfg.tokenFile = localConfig[cliConfigKeyTokenFile]
	}
	if cfg.token == "" && cfg.tokenFile != "" {
		content, err := os.ReadFile(cfg.tokenFile)
		if err != nil {
			return fmt.Errorf("read token file %q: %w", cfg.tokenFile, err)
		}
		cfg.token = strings.TrimSpace(string(content))
	}
	if strings.TrimSpace(cfg.addr) == "" {
		return fmt.Errorf("controller address is required: pass --addr or set %s", envControllerAddr)
	}
	if strings.TrimSpace(cfg.token) == "" {
		return fmt.Errorf("controller access token is required: pass --token, --token-file, or set %s", envAccessToken)
	}

	baseURL := strings.TrimRight(strings.TrimSpace(cfg.addr), "/")
	auth := connect.WithInterceptors(rpcutil.NewStaticBearerAuthInterceptor(strings.TrimSpace(cfg.token)))
	httpClient := http.DefaultClient
	application.cfg = cfg
	application.client = &controllerClient{
		system:          controllerv1connect.NewSystemServiceClient(httpClient, baseURL, auth),
		services:        controllerv1connect.NewServiceQueryServiceClient(httpClient, baseURL, auth),
		serviceCommands: controllerv1connect.NewServiceCommandServiceClient(httpClient, baseURL, auth),
		instances:       controllerv1connect.NewServiceInstanceServiceClient(httpClient, baseURL, auth),
		tasks:           controllerv1connect.NewTaskServiceClient(httpClient, baseURL, auth),
		backups:         controllerv1connect.NewBackupRecordServiceClient(httpClient, baseURL, auth),
		nodes:           controllerv1connect.NewNodeQueryServiceClient(httpClient, baseURL, auth),
		nodeCommands:    controllerv1connect.NewNodeMaintenanceServiceClient(httpClient, baseURL, auth),
		docker:          controllerv1connect.NewDockerQueryServiceClient(httpClient, baseURL, auth),
		containers:      controllerv1connect.NewContainerServiceClient(httpClient, baseURL, auth),
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
		{"status", response.GetStatus()},
		{"repo_revision", response.GetRepoRevision()},
	})
}

func (application *app) isJSONOutput() bool {
	return application.cfg.output == outputModeJSON || application.cfg.json
}

func (application *app) isTerseOutput() bool {
	return application.cfg.output == outputModeTerse || application.cfg.terse
}

func newRequest[T any](message *T) *connect.Request[T] {
	req := connect.NewRequest(message)
	req.Header().Set("X-Composia-Source", "cli")
	return req
}

func parsePageFlags(fs *flag.FlagSet) (func() (uint32, uint32), *uint) {
	pageSize := fs.Uint("page-size", 50, "page size")
	page := fs.Uint("page", 1, "1-based page number")
	return func() (uint32, uint32) {
		return uint32(*pageSize), uint32(*page)
	}, pageSize
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
	return fmt.Sprintf("%d", value)
}

func uint64Text(value uint64) string {
	return fmt.Sprintf("%d", value)
}

func int64Text(value int64) string {
	return fmt.Sprintf("%d", value)
}
