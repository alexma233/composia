package configpath

import (
	"fmt"
	"os"
	"strings"
)

// Resolve selects an explicit config path or the first existing default path.
func Resolve(explicit string, defaults []string, service string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	for _, path := range defaults {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		} else if !os.IsNotExist(err) {
			return "", fmt.Errorf("stat %s config %q: %w", service, path, err)
		}
	}
	return "", fmt.Errorf("%s config is required: pass --config, create %s", service, strings.Join(defaults, ", or create "))
}
