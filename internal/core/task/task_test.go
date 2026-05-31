package task

import "testing"

func TestIsControllerOwnedType(t *testing.T) {
	t.Parallel()

	tests := map[Type]bool{
		TypeDNSUpdate:         true,
		TypeMigrate:           true,
		TypeMigrateRollback:   true,
		TypeDeploy:            false,
		TypeBackup:            false,
		TypeDockerStart:       false,
		TypeDockerRemoveImage: false,
	}

	for taskType, want := range tests {
		t.Run(string(taskType), func(t *testing.T) {
			t.Parallel()
			if got := IsControllerOwnedType(taskType); got != want {
				t.Fatalf("IsControllerOwnedType(%q) = %v, want %v", taskType, got, want)
			}
		})
	}
}

func TestRequiresOnlineNode(t *testing.T) {
	t.Parallel()

	if RequiresOnlineNode(TypeDNSUpdate) {
		t.Fatalf("controller-owned task should not require online node")
	}
	if !RequiresOnlineNode(TypeDeploy) {
		t.Fatalf("agent-owned task should require online node")
	}
}
