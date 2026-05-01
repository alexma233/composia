package cli

import (
	"fmt"
	"sort"
	"strings"
)

func (application *app) runCompletion(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: composia completion <bash|zsh|fish>")
	}
	commands := completionCommands()
	switch args[0] {
	case "bash":
		return application.printBashCompletion(commands)
	case "zsh":
		return application.printZshCompletion(commands)
	case "fish":
		return application.printFishCompletion(commands)
	default:
		return fmt.Errorf("unknown completion shell %q", args[0])
	}
}

func completionCommands() []string {
	commands := make([]string, 0, len(commandUsages)+1)
	seen := map[string]bool{"version": true}
	commands = append(commands, "version")
	for command := range commandUsages {
		if strings.Contains(command, " ") {
			continue
		}
		if seen[command] {
			continue
		}
		seen[command] = true
		commands = append(commands, command)
	}
	sort.Strings(commands)
	return commands
}

func (application *app) printBashCompletion(commands []string) error {
	_, err := fmt.Fprintf(application.out, `# bash completion for composia
_composia_completion() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local commands="%s"
  COMPREPLY=( $(compgen -W "$commands" -- "$cur") )
}
complete -F _composia_completion composia
`, strings.Join(commands, " "))
	return err
}

func (application *app) printZshCompletion(commands []string) error {
	_, err := fmt.Fprintf(application.out, `#compdef composia
_composia() {
  local -a commands
  commands=(%s)
  compadd -- $commands
}
compdef _composia composia
`, strings.Join(commands, " "))
	return err
}

func (application *app) printFishCompletion(commands []string) error {
	for _, command := range commands {
		if strings.Contains(command, " ") {
			continue
		}
		if _, err := fmt.Fprintf(application.out, "complete -c composia -f -n '__fish_use_subcommand' -a '%s'\n", command); err != nil {
			return err
		}
	}
	return nil
}
