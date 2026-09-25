package controller

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect"
	controllerv1 "forgejo.alexma.top/alexma233/composia/gen/go/proto/composia/controller/v1"
	backupcfg "forgejo.alexma.top/alexma233/composia/internal/core/backup"
	"forgejo.alexma.top/alexma233/composia/internal/core/config"
	"forgejo.alexma.top/alexma233/composia/internal/core/repo"
	"forgejo.alexma.top/alexma233/composia/internal/core/task"
	"forgejo.alexma.top/alexma233/composia/internal/platform/store"
)

func TestMigratePhysicalDataSelection(t *testing.T) {
	for _, withBackup := range []bool{true, false} {
		name := "migration_only"
		if withBackup {
			name = "separate_daily_backup"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			meta := `name: app
nodes: [main]
data_protect:
  data:
    - name: dump
      backup:
        strategy: database.pgdumpall
        service: postgres
      restore:
        strategy: database.pgimport
        service: postgres
    - name: files
      backup:
        strategy: files.copy
        include: [./config]
    - name: physical
      backup:
        strategy: files.copy_after_stop
        include: [./pgdata]
      restore:
        strategy: files.copy
        include: [./pgdata]
migrate:
  data:
    - name: physical
    - name: dump
      enabled: false
`
			if withBackup {
				meta += "backup:\n  data:\n    - name: dump\n    - name: files\n"
			}
			cfg := &config.ControllerConfig{
				RepoDir: filepath.Join(root, "repo"), LogDir: filepath.Join(root, "logs"),
				Nodes:  []config.NodeConfig{{ID: "main"}, {ID: "edge"}},
				Backup: &config.ControllerBackupConfig{DefaultSchedule: "* * * * *"},
			}
			createGitRepoWithContent(t, cfg.RepoDir, map[string]string{
				"app/composia-meta.yaml":    meta,
				"rustic/composia-meta.yaml": "name: rustic\nnodes: [main, edge]\ninfra:\n  rustic:\n    compose_service: rustic\n",
			})
			if err := os.MkdirAll(filepath.Join(cfg.LogDir, "tasks"), 0o750); err != nil {
				t.Fatal(err)
			}
			db := openControllerTestDB(t)
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Errorf("close database: %v", err)
				}
			})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := db.SyncConfiguredNodes(ctx, []string{"main", "edge"}); err != nil {
				t.Fatal(err)
			}
			if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main"}, "rustic": {"main", "edge"}}); err != nil {
				t.Fatal(err)
			}
			for _, node := range []string{"main", "edge"} {
				if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: node, HeartbeatAt: time.Now().UTC()}); err != nil {
					t.Fatal(err)
				}
			}
			server := &serviceCommandServer{db: db, cfg: cfg, availableNodeIDs: configuredNodeIDs(cfg)}
			backupRequest := &controllerv1.RunServiceActionRequest{ServiceName: "app", NodeIds: []string{"main"}, Action: controllerv1.ServiceAction_SERVICE_ACTION_BACKUP, DataNames: []string{"physical"}}
			if _, err := server.RunServiceAction(ctx, connect.NewRequest(backupRequest)); connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Fatalf("ordinary backup must reject migration-only data: %v", err)
			}
			backupRequest.DataNames = nil
			_, err := server.RunServiceAction(ctx, connect.NewRequest(backupRequest))
			if withBackup && err != nil || !withBackup && connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Fatalf("default backup: %v", err)
			}
			// ListTasks has no source filter, so the source is matched on the loaded records.
			checkDailyTasks := func(source task.Source) {
				t.Helper()
				records, _, err := db.ListTasks(ctx, nil, []string{"app"}, nil, []string{string(task.TypeBackup)}, nil, nil, nil, nil, 1, 50)
				if err != nil {
					t.Fatal(err)
				}
				matched := 0
				for _, summary := range records {
					detail, err := db.GetTask(ctx, summary.TaskID)
					if err != nil {
						t.Fatal(err)
					}
					record := detail.Record
					if record.Source != source {
						continue
					}
					matched++
					params := mustTaskParams(t, record.ParamsJSON)
					if !slices.Equal(params.DataNames, []string{"dump", "files"}) {
						t.Fatalf("%s selected %v", source, params.DataNames)
					}
					if err := db.CompleteTask(ctx, record.TaskID, task.StatusSucceeded, time.Now().UTC(), ""); err != nil {
						t.Fatal(err)
					}
				}
				wantCount := 0
				if withBackup {
					wantCount = 1
				}
				if matched != wantCount {
					t.Fatalf("%s backup count = %d, want %d", source, matched, wantCount)
				}
			}
			checkDailyTasks(task.SourceCLI)
			if err := runScheduledTasksPass(ctx, db, cfg, configuredNodeIDs(cfg), newTaskQueueNotifier(), time.Now().UTC()); err != nil {
				t.Fatal(err)
			}
			checkDailyTasks(task.SourceSchedule)
			response, err := server.MigrateService(ctx, connect.NewRequest(&controllerv1.MigrateServiceRequest{ServiceName: "app", SourceNodeId: "main", TargetNodeId: "edge"}))
			if err != nil {
				t.Fatal(err)
			}
			instance, err := db.GetServiceInstanceSnapshot(ctx, "app", "edge")
			if err != nil {
				t.Fatalf("migration admission must prepare the undeclared target instance: %v", err)
			}
			if instance.IsDeclared || instance.RuntimeStatus != store.ServiceRuntimeUnknown {
				t.Fatalf("target instance must stay an undeclared placeholder: %+v", instance)
			}
			declaredService, err := repo.FindService(cfg.RepoDir, configuredNodeIDs(cfg), "app")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(declaredService.TargetNodes, []string{"main"}) {
				t.Fatalf("migration admission changed declared nodes: %v", declaredService.TargetNodes)
			}
			detail, err := db.GetTask(ctx, response.Msg.GetTaskId())
			if err != nil {
				t.Fatal(err)
			}
			if names := mustTaskParams(t, detail.Record.ParamsJSON).DataNames; !slices.Equal(names, []string{"physical"}) {
				t.Fatalf("migration selected %v", names)
			}
			runGit(t, cfg.RepoDir, "commit", "--allow-empty", "-m", "advance HEAD after migration admission")
			if err := db.TransitionTaskStatus(ctx, detail.Record.TaskID, task.StatusPending, task.StatusRunning, ""); err != nil {
				t.Fatal(err)
			}
			for _, step := range []task.StepName{task.StepComposeDown} {
				if err := db.UpsertTaskStep(ctx, task.StepRecord{TaskID: detail.Record.TaskID, StepName: step, Status: task.StatusSucceeded}); err != nil {
					t.Fatal(err)
				}
			}
			executor := &controllerTaskExecutor{db: db, cfg: cfg, availableNodeIDs: configuredNodeIDs(cfg), taskResults: newTaskResultNotifier()}
			done := make(chan struct{})
			var executionErr error
			go func() {
				executionErr = executor.executeMigrateTask(ctx, detail.Record)
				close(done)
			}()
			defer func() { cancel(); <-done }()
			seen := map[task.Type]bool{}
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for len(seen) < 3 {
				select {
				case <-ctx.Done():
					t.Fatal("migration did not create backup, restore, and deploy children")
				case <-ticker.C:
				}
				records, _, err := db.ListTasks(ctx, []string{string(task.StatusPending)}, []string{"app"}, nil, nil, nil, nil, nil, nil, 1, 10)
				if err != nil {
					t.Fatal(err)
				}
				for _, summary := range records {
					childDetail, err := db.GetTask(ctx, summary.TaskID)
					if err != nil {
						t.Fatal(err)
					}
					child := childDetail.Record
					if child.RepoRevision != detail.Record.RepoRevision {
						t.Fatalf("child revision %s != pinned %s", child.RepoRevision, detail.Record.RepoRevision)
					}
					params := mustTaskParams(t, child.ParamsJSON)
					extra, err := bundleExtraFiles(cfg, child, params, true)
					if err != nil {
						t.Fatal(err)
					}
					switch child.Type {
					case task.TypeBackup:
						var payload backupcfg.RuntimeConfig
						if err := json.Unmarshal([]byte(extra["app/.composia-backup.json"]), &payload); err != nil {
							t.Fatal(err)
						}
						if !slices.Equal(params.DataNames, []string{"physical"}) || len(payload.Items) != 1 || payload.Items[0].Name != "physical" || payload.Items[0].Strategy != "files.copy_after_stop" || payload.Items[0].Provider != backupProviderRustic || !slices.Equal(payload.Items[0].Include, []string{"./pgdata"}) {
							t.Fatalf("unexpected migration backup: params=%+v payload=%+v", params, payload)
						}
						if err := db.UpsertBackupRecord(ctx, store.BackupDetail{BackupID: child.TaskID, TaskID: child.TaskID, ServiceName: "app", NodeID: "main", DataName: "physical", Status: string(task.StatusSucceeded), ArtifactRef: "snapshot-physical"}); err != nil {
							t.Fatal(err)
						}
					case task.TypeRestore:
						var payload backupcfg.RestoreConfig
						if err := json.Unmarshal([]byte(extra["app/.composia-restore.json"]), &payload); err != nil {
							t.Fatal(err)
						}
						if len(payload.Items) != 1 || payload.Items[0].Name != "physical" || payload.Items[0].Strategy != "files.copy" || payload.Items[0].ArtifactRef != "snapshot-physical" {
							t.Fatalf("unexpected migration restore: %+v", payload)
						}
					case task.TypeDeploy:
						if child.NodeID != "edge" || !seen[task.TypeRestore] {
							t.Fatalf("target deployment must follow restore: %+v", child)
						}
					default:
						t.Fatalf("unexpected child type: %s", child.Type)
					}
					seen[child.Type] = true
					if err := db.CompleteTask(ctx, child.TaskID, task.StatusSucceeded, time.Now().UTC(), ""); err != nil {
						t.Fatal(err)
					}
					notifyTaskResult(executor.taskResults, child.TaskID)
				}
			}
			<-done
			if executionErr != nil {
				t.Fatal(executionErr)
			}
			finished, err := db.GetTask(ctx, detail.Record.TaskID)
			if err != nil || finished.Record.Status != task.StatusAwaitingConfirmation {
				t.Fatalf("migration did not reach confirmation: %+v, %v", finished.Record, err)
			}
		})
	}
}

// TestMigrateServiceTargetInstancePreparation covers the migration admission contract for a
// target node that the service is not declared on yet.
func TestMigrateServiceTargetInstancePreparation(t *testing.T) {
	t.Run("fresh target is prepared as undeclared instance", func(t *testing.T) {
		db, cfg := newMigrateAdmissionFixture(t)
		ctx := context.Background()
		server := &serviceCommandServer{db: db, cfg: cfg, availableNodeIDs: configuredNodeIDs(cfg)}

		response, err := server.MigrateService(ctx, connect.NewRequest(&controllerv1.MigrateServiceRequest{ServiceName: "app", SourceNodeId: "main", TargetNodeId: "edge"}))
		if err != nil {
			t.Fatal(err)
		}
		if response.Msg.GetTaskId() == "" {
			t.Fatal("migration task was not created")
		}
		instance, err := db.GetServiceInstanceSnapshot(ctx, "app", "edge")
		if err != nil {
			t.Fatalf("target instance was not prepared: %v", err)
		}
		if instance.IsDeclared || instance.RuntimeStatus != store.ServiceRuntimeUnknown {
			t.Fatalf("prepared target instance must stay undeclared and unknown: %+v", instance)
		}
		service, err := repo.FindService(cfg.RepoDir, configuredNodeIDs(cfg), "app")
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(service.TargetNodes, []string{"main"}) {
			t.Fatalf("admission changed declared nodes: %v", service.TargetNodes)
		}
		childTasks, _, err := db.ListTasks(ctx, nil, []string{"app"}, []string{"edge"}, nil, nil, nil, nil, nil, 1, 50)
		if err != nil {
			t.Fatal(err)
		}
		if len(childTasks) != 0 {
			t.Fatalf("admission created target node tasks: %+v", childTasks)
		}
	})

	t.Run("existing target instance state is preserved", func(t *testing.T) {
		db, cfg := newMigrateAdmissionFixture(t)
		ctx := context.Background()
		if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main", "edge"}}); err != nil {
			t.Fatal(err)
		}
		if err := db.UpdateServiceInstanceRuntimeStatus(ctx, "app", "edge", store.ServiceRuntimeStopped, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
		// The node is dropped from the declaration again: a known, undeclared instance remains.
		if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main"}}); err != nil {
			t.Fatal(err)
		}
		before, err := db.GetServiceInstanceSnapshot(ctx, "app", "edge")
		if err != nil {
			t.Fatal(err)
		}
		if before.IsDeclared || before.RuntimeStatus != store.ServiceRuntimeStopped {
			t.Fatalf("fixture is not in the expected state: %+v", before)
		}

		server := &serviceCommandServer{db: db, cfg: cfg, availableNodeIDs: configuredNodeIDs(cfg)}
		if _, err := server.MigrateService(ctx, connect.NewRequest(&controllerv1.MigrateServiceRequest{ServiceName: "app", SourceNodeId: "main", TargetNodeId: "edge"})); err != nil {
			t.Fatal(err)
		}
		after, err := db.GetServiceInstanceSnapshot(ctx, "app", "edge")
		if err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Fatalf("admission overwrote the existing target instance: %+v -> %+v", before, after)
		}
	})

	t.Run("conflicting admission leaves no target instance", func(t *testing.T) {
		db, cfg := newMigrateAdmissionFixture(t)
		ctx := context.Background()
		if _, err := db.CreateTaskIfNoActiveServiceInstanceTask(ctx, task.Record{
			Type: task.TypeDeploy, Source: task.SourceCLI, ServiceName: "app", NodeID: "main", Status: task.StatusPending,
		}); err != nil {
			t.Fatal(err)
		}

		server := &serviceCommandServer{db: db, cfg: cfg, availableNodeIDs: configuredNodeIDs(cfg)}
		if _, err := server.MigrateService(ctx, connect.NewRequest(&controllerv1.MigrateServiceRequest{ServiceName: "app", SourceNodeId: "main", TargetNodeId: "edge"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
			t.Fatalf("expected admission conflict, got %v", err)
		}
		if _, err := db.GetServiceInstanceSnapshot(ctx, "app", "edge"); !errors.Is(err, store.ErrServiceNotFound) {
			t.Fatalf("rejected admission leaked a target instance: %v", err)
		}
	})
}

func newMigrateAdmissionFixture(t *testing.T) (*store.DB, *config.ControllerConfig) {
	t.Helper()
	root := t.TempDir()
	cfg := &config.ControllerConfig{
		RepoDir: filepath.Join(root, "repo"), LogDir: filepath.Join(root, "logs"),
		Nodes: []config.NodeConfig{{ID: "main"}, {ID: "edge"}},
	}
	createGitRepoWithContent(t, cfg.RepoDir, map[string]string{
		"app/composia-meta.yaml": "name: app\nnodes: [main]\n",
	})
	if err := os.MkdirAll(filepath.Join(cfg.LogDir, "tasks"), 0o750); err != nil {
		t.Fatal(err)
	}
	db := openControllerTestDB(t)
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	ctx := context.Background()
	if err := db.SyncConfiguredNodes(ctx, []string{"main", "edge"}); err != nil {
		t.Fatal(err)
	}
	if err := db.SyncDeclaredServices(ctx, map[string][]string{"app": {"main"}}); err != nil {
		t.Fatal(err)
	}
	for _, nodeID := range []string{"main", "edge"} {
		if err := db.RecordHeartbeat(ctx, store.NodeHeartbeat{NodeID: nodeID, HeartbeatAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	return db, cfg
}
