package repo

import "testing"

func TestNormalizeBundleExtraPathRejectsEscapingPaths(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", ".", "../secret", `/abs/secret`, `..\secret`} {
		if _, err := normalizeBundleExtraPath(name); err == nil {
			t.Fatalf("expected %q to be rejected", name)
		}
	}
}

func TestNormalizeBundleExtraPathCleansSafePath(t *testing.T) {
	t.Parallel()

	got, err := normalizeBundleExtraPath("./demo/../demo/.composia-backup.json")
	if err != nil {
		t.Fatalf("normalize safe path: %v", err)
	}
	if got != "demo/.composia-backup.json" {
		t.Fatalf("unexpected normalized path %q", got)
	}
}
