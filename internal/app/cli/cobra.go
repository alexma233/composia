package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	"forgejo.alexma.top/alexma233/composia/internal/version"
	"github.com/spf13/cobra"
)

type cobraRuntime struct {
	ctx          context.Context
	out          io.Writer
	errOut       io.Writer
	app          *app
	headerValues headerFlag
	output       string
	finalized    bool
}

type cobraCommandSpec struct {
	use        string
	short      string
	controller bool
	run        func(*app, []string) error
	flags      func(*cobra.Command, *cobraRuntime)
	complete   cobraCompletionFunc
	children   []cobraCommandSpec
}

type cobraCompletionFunc func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective)

func newRootCommand(ctx context.Context, out io.Writer, errOut io.Writer, cfg globalConfig) *cobra.Command {
	if cfg.output == "" {
		cfg.output = outputModeHuman
	}
	runtime := &cobraRuntime{
		ctx:    ctx,
		out:    out,
		errOut: errOut,
		app:    &app{ctx: ctx, out: out, errOut: errOut, cfg: cfg},
		output: string(cfg.output),
	}

	root := &cobra.Command{
		Use:           "composia",
		Short:         "Manage Composia services and controller resources",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return runtime.finalizeGlobalConfig()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			return errors.New("missing command")
		},
	}
	root.SetOut(out)
	root.SetErr(errOut)
	root.CompletionOptions.DisableDefaultCmd = false
	root.CompletionOptions.DisableNoDescFlag = true
	root.CompletionOptions.DisableDescriptions = true
	root.SetHelpCommand(newHelpCommand(root, out))

	flags := root.PersistentFlags()
	flags.StringVar(&runtime.app.cfg.addr, "addr", cfg.addr, "controller base URL")
	flags.StringVar(&runtime.app.cfg.token, "token", cfg.token, "controller access token")
	flags.StringVar(&runtime.app.cfg.tokenFile, "token-file", cfg.tokenFile, "controller access token file")
	flags.Var(&runtime.headerValues, "header", `custom controller header as "Name: value"`)
	flags.StringVar(&runtime.output, "output", string(cfg.output), "output mode: human, json, terse")
	flags.BoolVar(&runtime.app.cfg.json, "json", cfg.json, "print protobuf JSON for unary RPCs")

	root.AddCommand(newVersionCommand(out))
	for _, spec := range cobraCommandSpecs(runtime) {
		root.AddCommand(newCobraCommand(runtime, spec))
	}

	return root
}

func newVersionCommand(out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print CLI version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(out, version.Value)
			return err
		},
	}
}

func cobraCommandSpecs(runtime *cobraRuntime) []cobraCommandSpec {
	return []cobraCommandSpec{
		serviceCommandSpec(runtime),
		groupSpec("task", "Inspect task status, logs, confirmations, and reruns", true, []cobraCommandSpec{
			leafSpec("list", "List tasks", true, (*app).runTaskList, taskListFlags, nil),
			leafSpec("get <task>", "Show one task", true, (*app).runTaskGet, nil, completeTaskIDs(runtime)),
			leafSpec("logs <task>", "Stream task logs", true, (*app).runTaskLogs, nil, completeTaskIDs(runtime)),
			leafSpec("wait <task>", "Wait for a task", true, (*app).runTaskWait, waitFlags, completeTaskIDs(runtime)),
			leafSpec("run-again <task>", "Run a task again", true, (*app).runTaskAgain, waitFlags, completeTaskIDs(runtime)),
			leafSpec("approve <task>", "Approve a task confirmation", true, func(a *app, args []string) error { return a.runTaskResolve("approve", args) }, commentFlag, completeTaskIDs(runtime)),
			leafSpec("reject <task>", "Reject a task confirmation", true, func(a *app, args []string) error { return a.runTaskResolve("reject", args) }, commentFlag, completeTaskIDs(runtime)),
		}),
		groupSpec("node", "Inspect nodes and run node maintenance", true, []cobraCommandSpec{
			leafSpec("list", "List nodes", true, (*app).runNodeList, nil, nil),
			leafSpec("get <node>", "Show one node", true, (*app).runNodeGet, nil, completeNodeIDsArg(runtime)),
			leafSpec("tasks <node>", "List node tasks", true, (*app).runNodeTasks, nodeTasksFlags, completeNodeIDsArg(runtime)),
			leafSpec("stats <node>", "Show node Docker stats", true, (*app).runNodeStats, nil, completeNodeIDsArg(runtime)),
			leafSpec("sync-caddy-files <node>", "Sync node Caddy files", true, (*app).runNodeSyncCaddyFiles, nodeSyncCaddyFilesFlags, completeNodeIDsArg(runtime)),
			leafSpec("reload-caddy <node>", "Reload node Caddy", true, (*app).runNodeReloadCaddy, waitFlags, completeNodeIDsArg(runtime)),
			leafSpec("prune <node>", "Prune Docker resources on a node", true, (*app).runNodePrune, nodePruneFlags, completeNodeIDsArg(runtime)),
		}),
		leafSpec("container <node> <list|get|logs|start|stop|restart|remove|exec>", "Low-level container operations by node and container ID", true, (*app).runContainer, containerFlags, completeNodeIDsArg(runtime)),
		groupSpec("backup", "List and restore backups", true, []cobraCommandSpec{
			leafSpec("list", "List backups", true, (*app).runBackupList, backupListFlags, nil),
			leafSpec("get <backup>", "Show one backup", true, (*app).runBackupGet, nil, nil),
			leafSpec("restore <node> <backup>", "Restore a backup", true, (*app).runBackupRestore, waitFlags, completeNodeIDsArg(runtime)),
		}),
		groupSpec("repo", "Low-level repository file operations", true, []cobraCommandSpec{
			leafSpec("head", "Show repository HEAD", true, (*app).runRepoHead, nil, nil),
			leafSpec("files [path]", "List repository files", true, (*app).runRepoFiles, repoFilesFlags, nil),
			leafSpec("get <path>", "Print a repository file", true, (*app).runRepoGet, nil, nil),
			leafSpec("edit <path>", "Edit a repository file", true, (*app).runRepoEdit, repoEditFlags, nil),
			leafSpec("update <path>", "Update a repository file", true, (*app).runRepoUpdate, fileMessageFlags, nil),
			leafSpec("mkdir <path>", "Create a repository directory", true, (*app).runRepoMkdir, messageFlag, nil),
			leafSpec("mv <source> <destination>", "Move a repository path", true, (*app).runRepoMove, messageFlag, nil),
			leafSpec("rm <path>", "Remove a repository path", true, (*app).runRepoRemove, messageFlag, nil),
			leafSpec("history", "List repository commits", true, (*app).runRepoHistory, repoHistoryFlags, nil),
			leafSpec("sync", "Sync repository remote", true, (*app).runRepoSync, nil, nil),
			leafSpec("validate", "Validate repository files", true, (*app).runRepoValidate, nil, nil),
		}),
		groupSpec("secret", "Low-level encrypted file operations", true, []cobraCommandSpec{
			leafSpec("get <service> <file>", "Print a decrypted secret file", true, (*app).runSecretGet, nil, completeServiceFirstArg(runtime)),
			leafSpec("edit <service> <file>", "Edit a decrypted secret file", true, (*app).runSecretEdit, messageFlag, completeServiceFirstArg(runtime)),
			leafSpec("update <service> <file>", "Update an encrypted secret file", true, (*app).runSecretUpdate, fileMessageFlags, completeServiceFirstArg(runtime)),
		}),
		groupSpec("system", "Controller status, reload, and capability checks", true, []cobraCommandSpec{
			leafSpec("status", "Show controller status", true, func(a *app, args []string) error { return a.runSystem(append([]string{"status"}, args...)) }, nil, nil),
			leafSpec("reload", "Reload controller config", true, func(a *app, args []string) error { return a.runSystem(append([]string{"reload"}, args...)) }, nil, nil),
			leafSpec("capabilities", "Show controller capabilities", true, func(a *app, args []string) error { return a.runSystem(append([]string{"capabilities"}, args...)) }, nil, nil),
		}),
		groupSpec("instance", "Low-level service instance operations by service and node", true, []cobraCommandSpec{
			leafSpec("list <service>", "List service instances", true, (*app).runInstanceList, nil, completeServiceFirstArg(runtime)),
			leafSpec("get <service> <node>", "Show one service instance", true, (*app).runInstanceGet, containersFlag, completeServiceThenNode(runtime)),
			leafSpec("deploy <service> <node>", "Deploy one service instance", true, func(a *app, args []string) error { return a.runInstanceAction("deploy", args) }, instanceActionFlags, completeServiceThenNode(runtime)),
			leafSpec("update <service> <node>", "Update one service instance", true, func(a *app, args []string) error { return a.runInstanceAction(actionUpdate, args) }, instanceActionFlags, completeServiceThenNode(runtime)),
			leafSpec("stop <service> <node>", "Stop one service instance", true, func(a *app, args []string) error { return a.runInstanceAction("stop", args) }, waitFlags, completeServiceThenNode(runtime)),
			leafSpec("restart <service> <node>", "Restart one service instance", true, func(a *app, args []string) error { return a.runInstanceAction(actionRestart, args) }, waitFlags, completeServiceThenNode(runtime)),
			leafSpec("backup <service> <node>", "Back up one service instance", true, (*app).runInstanceBackup, instanceBackupFlags, completeServiceThenNode(runtime)),
		}),
		leafSpec("network <node> <list|get|remove>", "Low-level Docker network operations by node", true, (*app).runNetwork, dockerResourceFlags, completeNodeIDsArg(runtime)),
		leafSpec("volume <node> <list|get|remove>", "Low-level Docker volume operations by node", true, (*app).runVolume, dockerResourceFlags, completeNodeIDsArg(runtime)),
		leafSpec("image <node> <list|get|remove>", "Low-level Docker image operations by node", true, (*app).runImage, imageFlags, completeNodeIDsArg(runtime)),
		groupSpec("rustic", "Rustic repository maintenance", true, []cobraCommandSpec{
			leafSpec("init <node>", "Initialize rustic on a node", true, (*app).runRusticInit, waitFlags, completeNodeIDsArg(runtime)),
			leafSpec("forget <node>", "Run rustic forget", true, func(a *app, args []string) error { return a.runRusticMaintenance("forget", args) }, rusticMaintenanceFlags, completeNodeIDsArg(runtime)),
			leafSpec("prune <node>", "Run rustic prune", true, func(a *app, args []string) error { return a.runRusticMaintenance("prune", args) }, rusticMaintenanceFlags, completeNodeIDsArg(runtime)),
		}),
		groupSpec("config", "Configure controller address and access token", false, []cobraCommandSpec{
			leafSpec("get [key]", "Read CLI config", false, (*app).runConfigGet, nil, completeConfigKeys),
			leafSpec("set <key> <value>", "Set a CLI config value", false, (*app).runConfigSet, nil, completeConfigKeys),
			leafSpec("unset <key>", "Unset a CLI config value", false, (*app).runConfigUnset, nil, completeConfigKeys),
			leafSpec("path", "Print CLI config path", false, (*app).runConfigPath, nil, nil),
			leafSpec("setup", "Interactively configure CLI access", false, (*app).runConfigSetup, configTokenFlags, nil),
			leafSpec("set-token", "Store controller access token", false, (*app).runConfigSetToken, configTokenFlags, nil),
			leafSpec("unset-token", "Remove stored controller access token", false, (*app).runConfigUnsetToken, nil, nil),
		}),
	}
}

func serviceCommandSpec(runtime *cobraRuntime) cobraCommandSpec {
	return cobraCommandSpec{
		use:        "service <list|create|<service> [action]>",
		short:      "List/create services or target one service by name",
		controller: true,
		run:        (*app).runService,
		flags: func(cmd *cobra.Command, runtime *cobraRuntime) {
			cmd.Flags().Bool("containers", false, "include per-instance containers")
			cmd.Flags().String("status", "", "runtime status filter for list")
			addPageCobraFlags(cmd)
			cmd.Flags().String("message", "", "commit message for create/edit")
			addNodeStringArrayFlag(cmd, "target node ID; repeat or comma-separate")
			cmd.Flags().StringArray("data", nil, "data entry name; repeat or comma-separate")
			cmd.Flags().String("recreate", "auto", "compose recreate mode: auto, never, always")
			cmd.Flags().StringArray("image", nil, "configured image update name; repeat or comma-separate")
			cmd.Flags().StringArray("set-image", nil, "configured image update assignment name=tag; repeat or comma-separate")
			cmd.Flags().Bool("use-detected", false, "apply detected candidate for --image entries")
			cmd.Flags().Bool("all-detected", false, "apply all detected image updates")
			cmd.Flags().Bool("backup", false, "force backup before update")
			cmd.Flags().Bool("no-backup", false, "skip backup before update")
			cmd.Flags().Bool("detach", false, "return immediately without waiting")
			cmd.Flags().String("source", "", "source node ID")
			cmd.Flags().String("from", "", "source node ID")
			cmd.Flags().String("target", "", "target node ID")
			cmd.Flags().String("to", "", "target node ID")
			cmd.Flags().String("task", "", "task ID to stream instead of resolving latest service task")
			cmd.Flags().String("container", "", "container ID, name, or compose service")
			cmd.Flags().Bool("no-tty", false, "run without an interactive terminal")
			cmd.Flags().Duration("timeout", 30*time.Second, "timeout duration")
			cmd.Flags().Uint64("max-output", 1024*1024, "maximum bytes per output stream")
			cmd.Flags().Bool("wait", false, "wait for task completion")
			cmd.Flags().Bool("follow", false, "follow task logs while waiting")
		},
		complete: completeServiceArgs(runtime),
	}
}

func groupSpec(use string, short string, controller bool, children []cobraCommandSpec) cobraCommandSpec {
	return cobraCommandSpec{use: use, short: short, controller: controller, children: children}
}

func leafSpec(use string, short string, controller bool, run func(*app, []string) error, flags func(*cobra.Command, *cobraRuntime), complete cobraCompletionFunc) cobraCommandSpec {
	return cobraCommandSpec{use: use, short: short, controller: controller, run: run, flags: flags, complete: complete}
}

func newCobraCommand(runtime *cobraRuntime, spec cobraCommandSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:               spec.use,
		Short:             spec.short,
		ValidArgsFunction: spec.complete,
	}
	if spec.flags != nil {
		spec.flags(cmd, runtime)
	}
	if len(spec.children) > 0 {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			return fmt.Errorf("missing %s command", commandName(spec.use))
		}
		for _, child := range spec.children {
			cmd.AddCommand(newCobraCommand(runtime, child))
		}
		return cmd
	}
	cmd.DisableFlagParsing = true
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if isHelpArg(args) {
			if commandName(spec.use) == "service" { //nolint:goconst
				return PrintCommandUsage(runtime.out, append([]string{"service"}, trimHelpArgs(args)...))
			}
			return cmd.Help()
		}
		if err := runtime.finalizeGlobalConfig(); err != nil {
			return err
		}
		if spec.controller {
			if err := runtime.app.configureClient(); err != nil {
				return err
			}
		}
		return spec.run(runtime.app, args)
	}
	return cmd
}

func commandName(use string) string {
	name, _, _ := strings.Cut(use, " ")
	return name
}

func newHelpCommand(root *cobra.Command, out io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Help about any command",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return root.Help()
			}
			if args[0] == "service" {
				return PrintCommandUsage(out, args)
			}
			found, _, err := root.Find(args)
			if err == nil && found != nil {
				return found.Help()
			}
			return PrintCommandUsage(out, args)
		},
	}
}

func (runtime *cobraRuntime) finalizeGlobalConfig() error {
	if runtime.finalized {
		return nil
	}
	cfg := runtime.app.cfg
	if cfg.token != "" && cfg.tokenFile != "" {
		return errors.New("--token and --token-file are mutually exclusive")
	}
	headers, err := parseHeaderFlagValues(runtime.headerValues)
	if err != nil {
		return err
	}
	cfg.headers, err = mergeStaticHeaders(cfg.headers, headers)
	if err != nil {
		return err
	}
	mode := outputMode(strings.TrimSpace(runtime.output))
	switch mode {
	case "", outputModeHuman:
		mode = outputModeHuman
	case outputModeJSON, outputModeTerse:
	default:
		return fmt.Errorf("unknown output mode %q", runtime.output)
	}
	if cfg.json {
		mode = outputModeJSON
	}
	cfg.output = mode
	cfg.json = mode == outputModeJSON
	runtime.app.cfg = cfg
	runtime.finalized = true
	return nil
}

func addPageCobraFlags(cmd *cobra.Command) {
	cmd.Flags().Uint("page-size", 50, "page size")
	cmd.Flags().Uint("page", 1, "1-based page number")
}

func addWaitCobraFlags(cmd *cobra.Command) {
	cmd.Flags().Bool("wait", false, "wait for task completion")
	cmd.Flags().Bool("follow", false, "follow task logs while waiting")
	cmd.Flags().Duration("timeout", 0, "wait timeout")
	cmd.Flags().Duration("interval", 2*time.Second, "poll interval")
}

func addNodeStringArrayFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().StringArray("node", nil, usage)
}

func waitFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	addWaitCobraFlags(cmd)
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
}

func messageFlag(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().String("message", "", "commit message")
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
}

func commentFlag(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().String("comment", "", "operator comment")
}

func containersFlag(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().Bool("containers", false, "include containers")
}

func fileMessageFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().String("file", "", "file to read; use - for stdin")
	messageFlag(cmd, runtime)
}

func repoFilesFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().Bool("recursive", false, "include descendants")
}

func repoEditFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().Bool("create", false, "create the file when it does not exist")
	messageFlag(cmd, runtime)
}

func repoHistoryFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().Uint("page-size", 20, "page size")
	cmd.Flags().String("cursor", "", "pagination cursor")
}

func configTokenFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().Bool("stdin", false, "read token from stdin")
	cmd.Flags().Bool("inline", false, "store token inline in CLI config")
	cmd.Flags().Bool("file", false, "store token in the default CLI token file")
}

func taskListFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	for _, name := range []string{"status", "service", "node", "type", "exclude-status", "exclude-service", "exclude-node", "exclude-type"} {
		cmd.Flags().StringArray(name, nil, name+" filter; repeat or comma-separate")
	}
	addPageCobraFlags(cmd)
}

func nodeTasksFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().String("status", "", "status filter")
	addPageCobraFlags(cmd)
}

func nodeSyncCaddyFilesFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	addWaitCobraFlags(cmd)
	cmd.Flags().String("service", "", "service name")
	cmd.Flags().Bool("full-rebuild", false, "rebuild all generated files")
}

func nodePruneFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	addWaitCobraFlags(cmd)
	cmd.Flags().String("target", "all", "prune target: all, container, image, network, volume")
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
}

func dockerResourceFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().String("search", "", "search text")
	cmd.Flags().String("sort-by", "", "sort field")
	cmd.Flags().Bool("desc", false, "sort descending")
	addPageCobraFlags(cmd)
	addWaitCobraFlags(cmd)
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
}

func imageFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	dockerResourceFlags(cmd, runtime)
	cmd.Flags().Bool("force", false, "force remove")
}

func containerFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	cmd.Flags().String("search", "", "search text")
	cmd.Flags().String("sort-by", "", "sort field")
	cmd.Flags().Bool("desc", false, "sort descending")
	addPageCobraFlags(cmd)
	cmd.Flags().String("tail", "100", "number of lines or all")
	cmd.Flags().Bool("timestamps", false, "include timestamps")
	addWaitCobraFlags(cmd)
	cmd.Flags().Bool("force", false, "force remove")
	cmd.Flags().Bool("volumes", false, "remove anonymous volumes")
	cmd.Flags().BoolP("yes", "y", false, "skip confirmation prompt")
	cmd.Flags().BoolP("tty", "t", false, "attach an interactive terminal")
	cmd.Flags().String("stdin-file", "", "file to send to stdin; use - for standard input")
	cmd.Flags().Uint64("max-output", 1024*1024, "maximum bytes per output stream")
	cmd.Flags().Uint("rows", 24, "terminal rows for --tty")
	cmd.Flags().Uint("cols", 80, "terminal columns for --tty")
}

func backupListFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	for _, name := range []string{"service", "status", "data", "node", "exclude-service", "exclude-status", "exclude-data", "exclude-node"} {
		cmd.Flags().StringArray(name, nil, name+" filter; repeat or comma-separate")
	}
	addPageCobraFlags(cmd)
}

func instanceActionFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	addWaitCobraFlags(cmd)
	cmd.Flags().String("recreate", "auto", "compose recreate mode: auto, never, always")
}

func instanceBackupFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	addWaitCobraFlags(cmd)
	cmd.Flags().StringArray("data", nil, "data entry name; repeat or comma-separate")
}

func rusticMaintenanceFlags(cmd *cobra.Command, runtime *cobraRuntime) {
	addWaitCobraFlags(cmd)
	cmd.Flags().String("service", "", "service name")
	cmd.Flags().String("data", "", "data entry name")
}

func completeServiceArgs(runtime *cobraRuntime) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			values := append([]string{"create", "list"}, runtime.completeServiceNames()...)
			return filterCompletions(values, toComplete), cobra.ShellCompDirectiveNoFileComp
		case 1:
			if args[0] == "create" || args[0] == "list" { //nolint:goconst
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return filterCompletions(serviceActionCompletions(), toComplete), cobra.ShellCompDirectiveNoFileComp
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

func completeServiceFirstArg(runtime *cobraRuntime) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return filterCompletions(runtime.completeServiceNames(), toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeServiceThenNode(runtime *cobraRuntime) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		switch len(args) {
		case 0:
			return filterCompletions(runtime.completeServiceNames(), toComplete), cobra.ShellCompDirectiveNoFileComp
		case 1:
			return filterCompletions(runtime.completeNodeIDs(), toComplete), cobra.ShellCompDirectiveNoFileComp
		default:
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
}

func completeNodeIDsArg(runtime *cobraRuntime) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return filterCompletions(runtime.completeNodeIDs(), toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeTaskIDs(runtime *cobraRuntime) cobraCompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 0 {
			return filterCompletions(runtime.completeTaskIDs(), toComplete), cobra.ShellCompDirectiveNoFileComp
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func completeConfigKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) == 0 {
		return filterCompletions([]string{cliConfigKeyAddr, cliConfigKeyToken, cliConfigKeyTokenFile, cliConfigKeyTokenKeyring}, toComplete), cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveNoFileComp
}

func serviceActionCompletions() []string {
	return []string{"backup", "caddy-sync", "dns-update", "down", "edit", "exec", "logs", "migrate", "ps", actionRestart, "tunnel-sync", actionUpdate, "updates", "up"}
}

func filterCompletions(values []string, prefix string) []string {
	seen := make(map[string]bool, len(values))
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] || !strings.HasPrefix(value, prefix) {
			continue
		}
		seen[value] = true
		filtered = append(filtered, value)
	}
	sort.Strings(filtered)
	return filtered
}

func (runtime *cobraRuntime) completeServiceNames() []string {
	if err := runtime.finalizeGlobalConfig(); err != nil {
		return nil
	}
	if err := runtime.app.configureClient(); err != nil {
		return nil
	}
	response, err := runtime.app.client.services.ListServices(runtime.ctx, newRequest(&controllerv1.ListServicesRequest{PageSize: 200, Page: 1}))
	if err != nil {
		return nil
	}
	services := response.Msg.GetServices()
	values := make([]string, 0, len(services))
	for _, service := range services {
		values = append(values, service.GetName())
	}
	return values
}

func (runtime *cobraRuntime) completeNodeIDs() []string {
	if err := runtime.finalizeGlobalConfig(); err != nil {
		return nil
	}
	if err := runtime.app.configureClient(); err != nil {
		return nil
	}
	response, err := runtime.app.client.nodes.ListNodes(runtime.ctx, newRequest(&controllerv1.ListNodesRequest{}))
	if err != nil {
		return nil
	}
	nodes := response.Msg.GetNodes()
	values := make([]string, 0, len(nodes))
	for _, node := range nodes {
		values = append(values, node.GetNodeId())
	}
	return values
}

func (runtime *cobraRuntime) completeTaskIDs() []string {
	if err := runtime.finalizeGlobalConfig(); err != nil {
		return nil
	}
	if err := runtime.app.configureClient(); err != nil {
		return nil
	}
	response, err := runtime.app.client.tasks.ListTasks(runtime.ctx, newRequest(&controllerv1.ListTasksRequest{PageSize: 100, Page: 1}))
	if err != nil {
		return nil
	}
	tasks := response.Msg.GetTasks()
	values := make([]string, 0, len(tasks))
	for _, task := range tasks {
		values = append(values, task.GetTaskId())
	}
	return values
}
