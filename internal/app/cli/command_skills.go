package cli

import (
	"fmt"
	"sort"
)

type cliSkill struct {
	name        string
	description string
	content     string
}

var cliSkills = map[string]cliSkill{
	"coding-agent": {
		name:        "coding-agent",
		description: "Use Composia from coding agents with low-token CLI output.",
		content: `name: coding-agent
description: Use Composia from coding agents with low-token CLI output.

Rules:
- Use --terse for normal inspection and mutation commands.
- Use --json only when exact protobuf field names are required.
- Prefer service and instance commands for lifecycle operations.
- Use container commands for runtime diagnosis only.
- Use --wait for mutations when the caller needs completion.
- Use --follow only when streaming logs is explicitly needed.
- Keep logs bounded with --tail.

Common commands:
composia --terse system status
composia --terse service list
composia --terse service get <service>
composia --terse instance list <service>
composia --terse task get <task>
composia --terse task logs <task>
composia --terse container list --node <node>
composia --terse container logs --node <node> --tail 100 <container>
`,
	},
	"service-ops": {
		name:        "service-ops",
		description: "Deploy, update, stop, restart, back up, and migrate services safely.",
		content: `name: service-ops
description: Deploy, update, stop, restart, back up, and migrate services safely.

Rules:
- Inspect the service before mutating it.
- Prefer service commands for all target-node fanout operations.
- Prefer instance commands only when the user names a specific node.
- Always pass --wait when the next step depends on task completion.
- On failure, inspect the task and then read bounded task logs.

Common commands:
composia --terse service get <service>
composia --terse service deploy --wait --timeout 10m <service>
composia --terse service update --wait --timeout 10m <service>
composia --terse instance deploy --wait --timeout 10m <service> <node>
composia --terse service backup --wait --timeout 30m <service>
composia --terse service migrate --wait --timeout 30m --source <node> --target <node> <service>
composia --terse task logs <task>
`,
	},
	"repo-secrets": {
		name:        "repo-secrets",
		description: "Read and update Composia repo files and encrypted secrets.",
		content: `name: repo-secrets
description: Read and update Composia repo files and encrypted secrets.

Rules:
- Use repo commands instead of editing the controller repo directly.
- Use secret commands for secret files; never print secrets unless explicitly requested.
- Provide a commit message for intentional writes when possible.
- On base revision conflicts, re-read the file and retry only after checking the new content.

Common commands:
composia --terse repo head
composia --terse repo files --recursive
composia repo get <path>
composia repo update --file <local-file> --message <message> <path>
composia secret get <service> <file>
composia secret update --file <local-file> --message <message> <service> <file>
composia --terse repo validate
`,
	},
}

func (application *app) runSkills(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: composia skills <list|show>")
	}
	switch args[0] {
	case "list":
		return application.runSkillsList(args[1:])
	case "show":
		return application.runSkillsShow(args[1:])
	default:
		return fmt.Errorf("unknown skills command %q", args[0])
	}
}

func (application *app) runSkillsList(args []string) error {
	if err := requireArgs(args, 0, "composia skills list"); err != nil {
		return err
	}
	names := make([]string, 0, len(cliSkills))
	for name := range cliSkills {
		names = append(names, name)
	}
	sort.Strings(names)
	rows := make([][]string, 0, len(names))
	for _, name := range names {
		skill := cliSkills[name]
		rows = append(rows, []string{skill.name, skill.description})
	}
	return application.writeTable([]string{"SKILL", "DESCRIPTION"}, rows)
}

func (application *app) runSkillsShow(args []string) error {
	if err := requireArgs(args, 1, "composia skills show <skill>"); err != nil {
		return err
	}
	skill, ok := cliSkills[args[0]]
	if !ok {
		return fmt.Errorf("unknown skill %q", args[0])
	}
	_, err := fmt.Fprint(application.out, skill.content)
	return err
}
