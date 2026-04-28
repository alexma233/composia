package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
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

type globalConfig struct {
	addr      string
	token     string
	tokenFile string
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
	backups         controllerv1connect.BackupRecordServiceClient
	nodes           controllerv1connect.NodeQueryServiceClient
	nodeCommands    controllerv1connect.NodeMaintenanceServiceClient
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
		PrintUsage(out)
		return nil
	}
	if rest[0] == "version" {
		fmt.Fprintln(out, version.Value)
		return nil
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
	case "system", "service", "instance", "task", "backup", "node", "repo", "secret":
		return true
	default:
		return false
	}
}

func PrintUsage(w io.Writer) {
	fmt.Fprint(w, `usage: composia [global flags] <command> [args]

Global flags:
  --addr string        controller base URL (or COMPOSIA_CONTROLLER_ADDR)
  --token string       controller access token (or COMPOSIA_ACCESS_TOKEN)
  --token-file string  file containing the controller access token
  --json              print protobuf JSON for unary RPCs

Commands:
  system status
  service list|get|deploy|update|stop|restart|backup|dns-update|caddy-sync|migrate
  instance list|get|deploy|update|stop|restart|backup
  task list|get|logs|run-again|approve|reject
  backup list|get|restore
  node list|get|tasks|reload-caddy|prune
  repo head|files|get|edit|update|history|sync|validate
  secret get|edit|update
  version
`)
}

func parseGlobalFlags(args []string) (globalConfig, []string, error) {
	var cfg globalConfig
	fs := flag.NewFlagSet("composia", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.addr, "addr", "", "controller base URL")
	fs.StringVar(&cfg.token, "token", "", "controller access token")
	fs.StringVar(&cfg.tokenFile, "token-file", "", "controller access token file")
	fs.BoolVar(&cfg.json, "json", false, "print JSON")
	fs.BoolVar(&cfg.help, "help", false, "print usage")
	fs.BoolVar(&cfg.help, "h", false, "print usage")
	if err := fs.Parse(args); err != nil {
		return globalConfig{}, nil, err
	}
	return cfg, fs.Args(), nil
}

func newCommandFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

func (application *app) configureClient() error {
	cfg := application.cfg
	if cfg.addr == "" {
		cfg.addr = os.Getenv(envControllerAddr)
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
		repos:           controllerv1connect.NewRepoQueryServiceClient(httpClient, baseURL, auth),
		repoCommands:    controllerv1connect.NewRepoCommandServiceClient(httpClient, baseURL, auth),
		secrets:         controllerv1connect.NewSecretServiceClient(httpClient, baseURL, auth),
	}
	return nil
}

func (application *app) printMessage(message proto.Message) error {
	if !application.cfg.json {
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
	if application.cfg.json {
		return application.printMessage(response)
	}
	return writeKV(application.out, [][2]string{
		{"task_id", response.GetTaskId()},
		{"status", response.GetStatus()},
		{"repo_revision", response.GetRepoRevision()},
	})
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

func requireAtLeastArgs(args []string, count int, usage string) error {
	if len(args) < count {
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
