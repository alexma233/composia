package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agentv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/agent/v1"
)

func testManifestFile(path, content string, mode uint32) *agentv1.ServiceManifestFile {
	return &agentv1.ServiceManifestFile{Path: path, Sha256: fmt.Sprintf("%x", sha256.Sum256([]byte(content))), Mode: mode}
}

func TestVerifyServiceFiles(t *testing.T) {
	for _, scenario := range []string{"match", "umask", "unreadable", "changed", "missing", "permissions", "executable", "setuid", "setgid", "sticky", "symlink", "parent-symlink", "escape", "duplicate", "missing-meta", "untracked-env"} {
		t.Run(scenario, func(t *testing.T) {
			root := t.TempDir()
			writeAgentTestFile(t, filepath.Join(root, "composia-meta.yaml"), "name: demo\n")
			writeAgentTestFile(t, filepath.Join(root, "config", "app.conf"), "original")
			files := []*agentv1.ServiceManifestFile{testManifestFile("composia-meta.yaml", "name: demo\n", 0o600), testManifestFile("config/app.conf", "original", 0o600)}
			writeAgentTestFile(t, filepath.Join(root, "generated", "demo.caddy"), "runtime output")
			switch scenario {
			case "unreadable":
				if os.Geteuid() == 0 {
					t.Skip("root bypasses file read permissions")
				}
				if err := os.Chmod(filepath.Join(root, "config", "app.conf"), 0); err != nil {
					t.Fatal(err)
				}
			case "changed":
				writeAgentTestFile(t, filepath.Join(root, "config", "app.conf"), "modified")
			case "missing":
				if err := os.Remove(filepath.Join(root, "config", "app.conf")); err != nil {
					t.Fatal(err)
				}
			case "umask":
				files[1].Mode = 0o664
			case "permissions":
				files[1].Mode = 0o400
			case "executable":
				files[1].Mode = 0o755
			case "setuid", "setgid", "sticky":
				mode := map[string]os.FileMode{"setuid": os.ModeSetuid, "setgid": os.ModeSetgid, "sticky": os.ModeSticky}[scenario]
				if err := os.Chmod(filepath.Join(root, "config", "app.conf"), mode|0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink", "parent-symlink":
				outside := t.TempDir()
				writeAgentTestFile(t, filepath.Join(outside, "app.conf"), "original")
				if err := os.RemoveAll(filepath.Join(root, "config")); err != nil {
					t.Fatal(err)
				}
				if scenario == "parent-symlink" {
					if err := os.Symlink(outside, filepath.Join(root, "config")); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.Mkdir(filepath.Join(root, "config"), 0o700); err != nil {
						t.Fatal(err)
					}
					if err := os.Symlink(filepath.Join(outside, "app.conf"), filepath.Join(root, "config", "app.conf")); err != nil {
						t.Fatal(err)
					}
				}
			case "escape":
				files[1].Path = "../outside"
			case "duplicate":
				files = append(files, files[0])
			case "missing-meta":
				files = files[1:]
			case "untracked-env":
				writeAgentTestFile(t, filepath.Join(root, ".env"), "TAG=unexpected")
			}
			err := verifyServiceFiles(root, files)
			wantStatus := "drifted"
			switch scenario {
			case "match", "umask":
				wantStatus = "consistent"
			case "escape", "duplicate", "missing-meta", "parent-symlink", "unreadable":
				wantStatus = "error"
			}
			if status := consistencyOutcome(err).GetStatus(); status != wantStatus {
				t.Fatalf("status=%s want=%s: %v", status, wantStatus, err)
			}
			if (scenario == "match" || scenario == "umask") && err != nil {
				t.Fatal(err)
			}
			if scenario != "match" && scenario != "umask" && err == nil {
				t.Fatal("expected verification failure")
			}
		})
	}
}

func testCheckContainer(id, service, hash, image string) serviceCheckContainer {
	value := serviceCheckContainer{ID: id, Image: image}
	value.Config.Labels = map[string]string{"com.docker.compose.project": "demo", "com.docker.compose.service": service, "com.docker.compose.config-hash": hash}
	value.State.Status = "running"
	return value
}

func TestCheckServiceContainers(t *testing.T) {
	const hash = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const digestA = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const digestB = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	for _, scenario := range []string{"running", "replicas", "mixed-replicas", "stale", "missing", "stopped", "orphan", "oneoff", "foreign-project", "optional-profile", "profile-running", "bad-hashes", "missing-hash", "tool-failure", "ambiguous-digest", "missing-digest"} {
		t.Run(scenario, func(t *testing.T) {
			containers := []serviceCheckContainer{testCheckContainer("container-a", "app", hash, "sha256:running-a")}
			configJSON := `{"services":{"app":{"image":"example/app:latest"}}}`
			hashes := "app " + hash + "\n"
			digestsA := `["example/app@` + digestA + `"]`
			switch scenario {
			case "replicas":
				containers = append(containers, testCheckContainer("container-b", "app", hash, "sha256:running-a"))
			case "mixed-replicas":
				containers = append(containers, testCheckContainer("container-b", "app", hash, "sha256:running-b"))
			case "stale":
				containers[0].Config.Labels["com.docker.compose.config-hash"] = "old"
			case "missing":
				containers = nil
			case "stopped":
				containers[0].State.Status = "exited"
			case "orphan":
				containers = append(containers, testCheckContainer("orphan", "removed", hash, "sha256:running-a"))
			case "oneoff":
				other := testCheckContainer("oneoff", "removed", "old", "")
				other.Config.Labels["com.docker.compose.oneoff"] = "True"
				containers = append(containers, other)
			case "foreign-project":
				other := testCheckContainer("foreign", "removed", "old", "")
				other.Config.Labels["com.docker.compose.project"] = "foreign"
				containers = append(containers, other)
			case "optional-profile", "profile-running":
				configJSON = `{"services":{"app":{"image":"example/app:latest"},"optional":{"image":"example/app:latest","profiles":["tools"]}}}`
				hashes += "optional " + hash + "\n"
				if scenario == "profile-running" {
					containers = append(containers, testCheckContainer("optional", "optional", hash, "sha256:running-a"))
				}
			case "bad-hashes":
				hashes = ""
			case "missing-hash":
				hashes = "other " + hash + "\n"
			case "ambiguous-digest":
				digestsA = `["example/app@` + digestA + `","example/app@` + digestB + `"]`
			case "missing-digest":
				digestsA = `[]`
			}
			encoded, err := json.Marshal(containers)
			if err != nil {
				t.Fatal(err)
			}
			dockerFailure := ""
			if scenario == "tool-failure" {
				dockerFailure = "echo permission denied >&2; exit 1"
			}
			logFile := installFakeDockerScript(t, fmt.Sprintf(`#!/bin/sh
printf '%%s\n' "$*" >> "$TEST_DOCKER_LOG_FILE"
%s
case "$*" in
  'compose version --short') printf '5.5.1\n' ;;
  *'config --format json') printf '%%s\n' '%s' ;;
  *'config --hash *') printf '%%s' '%s' ;;
  'container ls '*) printf 'container-a\n' ;;
  'container inspect '*) printf '%%s\n' '%s' ;;
  'image inspect --format {{json .RepoDigests}} sha256:running-a') printf '%%s\n' '%s' ;;
  'image inspect --format {{json .RepoDigests}} sha256:running-b') printf '%%s\n' '["example/app@%s"]' ;;
  *) printf 'unexpected docker command: %%s\n' "$*" >&2; exit 2 ;;
esac
`, dockerFailure, configJSON, hashes, encoded, digestsA, digestB))
			root := t.TempDir()
			checked, err := checkServiceContainers(t.Context(), root, composeCommandConfig{ProjectName: "demo", Files: []string{"compose.yaml"}})
			configStatus := consistencyOutcome(err).GetStatus()
			wantConfigStatus := "consistent"
			switch scenario {
			case "stale", "missing", "orphan":
				wantConfigStatus = "drifted"
			case "bad-hashes", "missing-hash", "tool-failure":
				wantConfigStatus = "error"
			}
			if configStatus != wantConfigStatus {
				t.Fatalf("configuration status=%s want=%s: %v", configStatus, wantConfigStatus, err)
			}
			var observations []serviceImageObservation
			if err == nil {
				observations, err = observeServiceImages(t.Context(), root, checked)
			}
			wantSuccess := scenario == "running" || scenario == "replicas" || scenario == "oneoff" || scenario == "foreign-project" || scenario == "optional-profile" || scenario == "profile-running"
			if wantSuccess {
				if err != nil {
					t.Fatal(err)
				}
				if len(observations) == 0 || observations[0].LocalDigest != digestA || !observations[0].LocalObserved {
					t.Fatalf("unexpected observations: %+v", observations)
				}
			} else if err == nil {
				t.Fatal("expected check failure")
			}
			commands := readAgentTestFile(t, logFile)
			if strings.Contains(commands, "}} example/app:latest") || strings.Contains(commands, " pull") || strings.Contains(commands, " up") {
				t.Fatalf("check must inspect immutable image IDs without pulling or deploying: %s", commands)
			}
		})
	}
}

func TestCheckServiceContainersRejectsOldCompose(t *testing.T) {
	installFakeDockerScript(t, "#!/bin/sh\nprintf '5.1.4\\n'\n")
	if _, err := checkServiceContainers(t.Context(), t.TempDir(), composeCommandConfig{ProjectName: "demo"}); err == nil || !strings.Contains(err.Error(), "5.5.1 or newer") {
		t.Fatalf("expected explicit Compose upgrade requirement, got %v", err)
	}
}

func BenchmarkVerifyServiceFiles(b *testing.B) {
	root := b.TempDir()
	contents := strings.Repeat("x", 4096)
	files := make([]*agentv1.ServiceManifestFile, 0, 128)
	for index := range 128 {
		name := fmt.Sprintf("config-%03d", index)
		if index == 0 {
			name = "composia-meta.yaml"
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(contents), 0o600); err != nil {
			b.Fatal(err)
		}
		files = append(files, testManifestFile(name, contents, 0o600))
	}
	b.SetBytes(int64(len(files) * len(contents)))
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if err := verifyServiceFiles(root, files); err != nil {
			b.Fatal(err)
		}
	}
}

func TestRunningImageDigest(t *testing.T) {
	const a = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const b = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	for _, tc := range []struct {
		ref     string
		digests []string
		want    string
	}{
		{"alpine:3.24", []string{"docker.io/library/alpine@" + a}, a},
		{"alpine@" + a, []string{"alpine@" + b}, ""},
		{"alpine@" + a, []string{"alpine@" + b, "alpine@" + a}, a},
		{"alpine:3.24", []string{"alpine@" + a, "alpine@" + b}, ""},
		{"alpine:3.24", []string{"other@" + a}, ""},
	} {
		got, err := runningImageDigest(tc.ref, tc.digests)
		if got != tc.want || (tc.want == "") != (err != nil) {
			t.Fatalf("runningImageDigest(%q, %v) = %q, %v", tc.ref, tc.digests, got, err)
		}
	}
}
