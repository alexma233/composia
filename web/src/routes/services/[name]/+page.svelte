<script lang="ts">
  import { FolderPlus, Pencil, Play, RefreshCcw, Save, Square, Trash2, Upload, Wrench } from 'lucide-svelte';

  import type { PageData } from './$types';

  import CodeEditor from '$lib/components/app/code-editor.svelte';
  import ServiceFileTree from '$lib/components/app/service-file-tree.svelte';
  import TaskLogStream from '$lib/components/app/task-log-stream.svelte';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { formatTimestamp, runtimeStatusTone, taskStatusTone } from '$lib/presenters';
  import type {
    BackupSummary,
    RepoWriteResult,
    ServiceActionResult,
    TaskSummary
  } from '$lib/server/controller';
  import type { ServiceFileNode, WorkspaceFile } from '$lib/service-workspace';
  import { findNode, normalizeServiceRelativePath, upsertFileNode } from '$lib/service-workspace';

  export let data: PageData;

  type EditorTab = WorkspaceFile & {
    savedContent: string;
    dirty: boolean;
  };

  let fileTree: ServiceFileNode[] = data.fileTree;
  let collapsedPaths = new Set<string>();
  let selectedNodePath = data.initialFile?.path ?? '';
  let openTabs: EditorTab[] = data.initialFile ? [createTab(data.initialFile)] : [];
  let activePath = data.initialFile?.path ?? '';
  let headRevision = data.repoHead?.headRevision ?? '';
  let syncStatus = data.repoHead?.syncStatus ?? '';
  let syncError = data.repoHead?.lastSyncError ?? '';
  let lastSuccessfulPullAt = data.repoHead?.lastSuccessfulPullAt ?? '';
  let tasks: TaskSummary[] = data.tasks;
  let backups: BackupSummary[] = data.backups;
  let selectedTaskId = data.tasks[0]?.taskId ?? '';
  let logsExpanded = false;
  let actionBusy = '';
  let saving = false;
  let notice = '';
  let errorMessage = data.error ?? '';
  let showNewFile = false;
  let newFilePath = '';
  let showNewFolder = false;
  let newFolderPath = '';
  let showRename = false;
  let renamePath = '';
  let workspace = data.workspace;
  let activeTab: EditorTab | null = openTabs.find((tab) => tab.path === activePath) ?? null;
  let canSave = Boolean(activeTab && activeTab.dirty && !saving);
  let selectedNode = selectedNodePath ? findNode(fileTree, selectedNodePath) : null;

  function createTab(file: WorkspaceFile): EditorTab {
    return {
      ...file,
      savedContent: file.content,
      dirty: false
    };
  }

  $: activeTab = openTabs.find((tab) => tab.path === activePath) ?? null;
  $: canSave = Boolean(activeTab && activeTab.dirty && !saving);
  $: selectedNode = selectedNodePath ? findNode(fileTree, selectedNodePath) : null;

  async function openFile(path: string) {
    try {
      const normalized = normalizeServiceRelativePath(path);
      const existing = openTabs.find((tab) => tab.path === normalized);
      if (existing) {
        selectedNodePath = normalized;
        activePath = normalized;
        return;
      }

      const response = await fetch(`/services/${workspace?.folder}/workspace/file?path=${encodeURIComponent(normalized)}`);
      const payload = await response.json();
      if (!response.ok) {
        errorMessage = payload.error ?? 'Failed to open file.';
        return;
      }

      openTabs = [...openTabs, createTab(payload.file as WorkspaceFile)];
      selectedNodePath = normalized;
      activePath = normalized;
      errorMessage = '';
    } catch (openError) {
      errorMessage = openError instanceof Error ? openError.message : 'Failed to open file.';
    }
  }

  function closeTab(path: string) {
    const nextTabs = openTabs.filter((tab) => tab.path !== path);
    openTabs = nextTabs;
    if (activePath === path) {
      activePath = nextTabs[nextTabs.length - 1]?.path ?? '';
    }
  }

  function selectNode(path: string) {
    selectedNodePath = path;
    showRename = false;
    if (path) {
      renamePath = path;
    }
  }

  function updateCurrentTab(content: string) {
    openTabs = openTabs.map((tab) =>
      tab.path === activePath
        ? {
            ...tab,
            content,
            dirty: content !== tab.savedContent
          }
        : tab
    );
  }

  async function saveCurrentTab() {
    const tab = activeTab;
    if (!tab || !headRevision) {
      return;
    }

    saving = true;
    notice = '';
    errorMessage = '';

    try {
      const response = await fetch(`/services/${workspace?.folder}/workspace/file`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          path: tab.path,
          content: tab.content,
          baseRevision: headRevision
        })
      });
    const payload = (await response.json()) as {
      error?: string;
      file?: WorkspaceFile;
      write?: RepoWriteResult;
      workspace?: PageData['workspace'];
    };
      if (!response.ok || !payload.file || !payload.write) {
        throw new Error(payload.error ?? 'Failed to save file.');
      }

      headRevision = payload.write.commitId;
      syncStatus = payload.write.syncStatus;
      syncError = payload.write.pushError;
      lastSuccessfulPullAt = payload.write.lastSuccessfulPullAt;
      workspace = payload.workspace ?? workspace;
      openTabs = openTabs.map((item) =>
        item.path === tab.path
          ? {
              ...item,
              content: payload.file?.content ?? item.content,
              savedContent: payload.file?.content ?? item.content,
              dirty: false,
              size: payload.file?.size ?? item.size
            }
          : item
      );
      notice = `Saved ${tab.path}`;
    } catch (saveError) {
      errorMessage = saveError instanceof Error ? saveError.message : 'Failed to save file.';
    } finally {
      saving = false;
    }
  }

  async function createFile() {
    if (!newFilePath.trim()) {
      return;
    }

    try {
      const normalized = normalizeServiceRelativePath(newFilePath);
      if (openTabs.some((tab) => tab.path === normalized)) {
        activePath = normalized;
        showNewFile = false;
        newFilePath = '';
        return;
      }

      saving = true;
      errorMessage = '';

      const response = await fetch(`/services/${workspace?.folder}/workspace/file`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          path: normalized,
          content: '',
          baseRevision: headRevision
        })
      });
      const payload = (await response.json()) as {
        error?: string;
        file?: WorkspaceFile;
        write?: RepoWriteResult;
        workspace?: PageData['workspace'];
      };
      if (!response.ok || !payload.file || !payload.write) {
        throw new Error(payload.error ?? 'Failed to create file.');
      }

      headRevision = payload.write.commitId;
      syncStatus = payload.write.syncStatus;
      syncError = payload.write.pushError;
      lastSuccessfulPullAt = payload.write.lastSuccessfulPullAt;
      workspace = payload.workspace ?? workspace;
      fileTree = upsertFileNode(fileTree, normalized);
      openTabs = [...openTabs, createTab(payload.file)];
      selectedNodePath = normalized;
      activePath = normalized;
      showNewFile = false;
      newFilePath = '';
      notice = `Created ${normalized}`;
    } catch (createError) {
      errorMessage = createError instanceof Error ? createError.message : 'Failed to create file.';
    } finally {
      saving = false;
    }
  }

  async function createDirectory() {
    if (!newFolderPath.trim()) {
      return;
    }

    saving = true;
    errorMessage = '';

    try {
      const normalized = normalizeServiceRelativePath(newFolderPath);
      const response = await fetch(`/services/${workspace?.folder}/workspace/fs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action: 'create_directory',
          path: normalized,
          baseRevision: headRevision
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.write) {
        throw new Error(payload.error ?? 'Failed to create folder.');
      }

      applyFsMutation(payload);
      selectedNodePath = normalized;
      showNewFolder = false;
      newFolderPath = '';
      notice = `Created folder ${normalized}`;
    } catch (directoryError) {
      errorMessage = directoryError instanceof Error ? directoryError.message : 'Failed to create folder.';
    } finally {
      saving = false;
    }
  }

  async function renameNode() {
    if (!selectedNodePath || !renamePath.trim()) {
      return;
    }

    saving = true;
    errorMessage = '';

    try {
      const destination = normalizeServiceRelativePath(renamePath);
      const response = await fetch(`/services/${workspace?.folder}/workspace/fs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action: 'move',
          sourcePath: selectedNodePath,
          destinationPath: destination,
          baseRevision: headRevision
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.write) {
        throw new Error(payload.error ?? 'Failed to rename path.');
      }

      applyFsMutation(payload);
      openTabs = openTabs.map((tab) => {
        if (tab.path === selectedNodePath || tab.path.startsWith(`${selectedNodePath}/`)) {
          const nextPath = destination + tab.path.slice(selectedNodePath.length);
          return { ...tab, path: nextPath };
        }
        return tab;
      });
      if (activePath === selectedNodePath || activePath.startsWith(`${selectedNodePath}/`)) {
        activePath = destination + activePath.slice(selectedNodePath.length);
      }
      selectedNodePath = destination;
      renamePath = destination;
      showRename = false;
      notice = `Renamed to ${destination}`;
    } catch (renameError) {
      errorMessage = renameError instanceof Error ? renameError.message : 'Failed to rename path.';
    } finally {
      saving = false;
    }
  }

  async function deleteNode() {
    if (!selectedNodePath || !confirm(`Delete ${selectedNode?.isDir ? 'folder' : 'file'} ${selectedNodePath}?`)) {
      return;
    }

    saving = true;
    errorMessage = '';

    try {
      const deletedPath = selectedNodePath;
      const response = await fetch(`/services/${workspace?.folder}/workspace/fs`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action: 'delete',
          path: deletedPath,
          baseRevision: headRevision
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.write) {
        throw new Error(payload.error ?? 'Failed to delete path.');
      }

      applyFsMutation(payload);
      const nextTabs = openTabs.filter(
        (tab) => tab.path !== deletedPath && !tab.path.startsWith(`${deletedPath}/`)
      );
      openTabs = nextTabs;
      if (activePath === deletedPath || activePath.startsWith(`${deletedPath}/`)) {
        activePath = nextTabs[0]?.path ?? '';
      }
      selectedNodePath = '';
      showRename = false;
      notice = `Deleted ${deletedPath}`;
    } catch (deleteError) {
      errorMessage = deleteError instanceof Error ? deleteError.message : 'Failed to delete path.';
    } finally {
      saving = false;
    }
  }

  function applyFsMutation(payload: {
    write: RepoWriteResult;
    workspace?: PageData['workspace'];
    fileTree?: ServiceFileNode[];
  }) {
    headRevision = payload.write.commitId;
    syncStatus = payload.write.syncStatus;
    syncError = payload.write.pushError;
    lastSuccessfulPullAt = payload.write.lastSuccessfulPullAt;
    workspace = payload.workspace ?? workspace;
    fileTree = payload.fileTree ?? fileTree;
  }

  async function triggerAction(action: 'deploy' | 'update' | 'stop' | 'restart' | 'backup') {
    actionBusy = action;
    errorMessage = '';

    try {
      const response = await fetch(`/services/${workspace?.folder}/workspace/action`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action })
      });
      const payload = (await response.json()) as ServiceActionResult & { error?: string };
      if (!response.ok || !payload.taskId) {
        throw new Error(payload.error ?? `Failed to run ${action}.`);
      }

      const newTask: TaskSummary = {
        taskId: payload.taskId,
        type: action,
        status: payload.status,
        serviceName: workspace?.serviceName ?? '',
        nodeId: workspace?.node ?? '',
        createdAt: new Date().toISOString()
      };
      tasks = [newTask, ...tasks].slice(0, 12);
      selectedTaskId = payload.taskId;
      logsExpanded = true;
      notice = `${action} queued as ${payload.taskId}`;
    } catch (actionError) {
      errorMessage = actionError instanceof Error ? actionError.message : `Failed to run ${action}.`;
    } finally {
      actionBusy = '';
    }
  }

  function toggleDirectory(path: string) {
    const next = new Set(collapsedPaths);
    if (next.has(path)) {
      next.delete(path);
    } else {
      next.add(path);
    }
    collapsedPaths = next;
  }
</script>

<div class="mx-auto flex min-h-[calc(100vh-72px)] max-w-[1600px] flex-col px-4 py-6 sm:px-6 lg:px-8">
  <div class="mb-4 flex flex-wrap items-center justify-between gap-4 rounded-lg border bg-card px-4 py-3 shadow-xs">
    <div>
      <div class="text-sm text-muted-foreground">Service workspace</div>
      <h1 class="text-2xl font-semibold tracking-tight">{workspace?.displayName ?? 'Service'}</h1>
    </div>

    <div class="flex flex-wrap items-center gap-3 text-sm text-muted-foreground">
      <span>Folder <span class="text-foreground">{workspace?.folder ?? 'n/a'}</span></span>
      <span>Node <span class="text-foreground">{workspace?.node || 'n/a'}</span></span>
      <span>Revision <span class="text-foreground">{headRevision ? headRevision.slice(0, 12) : 'n/a'}</span></span>
      <Badge variant={runtimeStatusTone(workspace?.runtimeStatus ?? 'unknown')}>
        {workspace?.runtimeStatus ?? 'unknown'}
      </Badge>
    </div>
  </div>

  {#if errorMessage}
    <div class="mb-4 rounded-lg border border-destructive/20 bg-destructive/10 p-4 text-sm text-destructive">
      {errorMessage}
    </div>
  {/if}

  {#if notice}
    <div class="mb-4 rounded-lg border border-primary/20 bg-primary/10 p-4 text-sm text-primary">
      {notice}
    </div>
  {/if}

  <div class="grid min-h-0 flex-1 gap-4 xl:grid-cols-[280px_minmax(0,1fr)_320px]">
    <section class="flex min-h-0 flex-col rounded-lg border bg-card shadow-xs">
      <div class="flex items-center justify-between border-b px-4 py-3">
        <div>
          <div class="text-sm font-medium">Files</div>
          <div class="text-xs text-muted-foreground">Scoped to `{workspace?.folder}`</div>
        </div>
        <div class="flex items-center gap-2">
          <Button type="button" variant="outline" size="sm" on:click={() => (showNewFile = !showNewFile)}>
            New file
          </Button>
          <Button type="button" variant="outline" size="sm" on:click={() => (showNewFolder = !showNewFolder)}>
            <FolderPlus class="mr-2 size-4" />Folder
          </Button>
        </div>
      </div>

      {#if showNewFile}
        <div class="space-y-3 border-b px-4 py-3">
          <input
            class="h-9 w-full rounded-md border bg-background px-3 text-sm outline-none"
            bind:value={newFilePath}
            placeholder="config/new-file.yaml"
          />
          <div class="flex items-center justify-between gap-3">
            <p class="text-xs text-muted-foreground">Nested paths create parent folders automatically.</p>
            <Button type="button" size="sm" on:click={createFile} disabled={saving}>Create</Button>
          </div>
        </div>
      {/if}

      {#if showNewFolder}
        <div class="space-y-3 border-b px-4 py-3">
          <input
            class="h-9 w-full rounded-md border bg-background px-3 text-sm outline-none"
            bind:value={newFolderPath}
            placeholder="config/snippets"
          />
          <div class="flex items-center justify-between gap-3">
            <p class="text-xs text-muted-foreground">Empty folders are tracked with a hidden `.gitkeep` file.</p>
            <Button type="button" size="sm" on:click={createDirectory} disabled={saving}>Create</Button>
          </div>
        </div>
      {/if}

      <div class="border-b px-4 py-3 text-sm">
        <div class="flex items-center justify-between gap-3">
          <div>
            <div class="font-medium">Selection</div>
            <div class="text-xs text-muted-foreground">{selectedNodePath || 'Nothing selected'}</div>
          </div>
          <div class="flex items-center gap-2">
            <Button type="button" variant="outline" size="sm" on:click={() => { showRename = !showRename; renamePath = selectedNodePath; }} disabled={!selectedNodePath || saving}>
              <Pencil class="mr-2 size-4" />Rename
            </Button>
            <Button type="button" variant="outline" size="sm" on:click={deleteNode} disabled={!selectedNodePath || saving}>
              <Trash2 class="mr-2 size-4" />Delete
            </Button>
          </div>
        </div>

        {#if showRename}
          <div class="mt-3 space-y-3">
            <input
              class="h-9 w-full rounded-md border bg-background px-3 text-sm outline-none"
              bind:value={renamePath}
              placeholder="new/path.yaml"
            />
            <div class="flex justify-end">
              <Button type="button" size="sm" on:click={renameNode} disabled={!selectedNodePath || saving}>Apply</Button>
            </div>
          </div>
        {/if}
      </div>

      <div class="min-h-0 flex-1 overflow-auto px-2 py-3">
        <ServiceFileTree
          nodes={fileTree}
          {activePath}
          selectedPath={selectedNodePath}
          {collapsedPaths}
          onOpenFile={openFile}
          onSelectNode={selectNode}
          onToggle={toggleDirectory}
        />
      </div>
    </section>

    <section class="flex min-h-0 flex-col rounded-lg border bg-card shadow-xs">
      <div class="border-b px-3 py-3">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
          <div>
            <div class="text-sm font-medium">Editor</div>
            <div class="text-xs text-muted-foreground">CodeMirror workspace with automatic commit messages.</div>
          </div>
          <Button type="button" size="sm" on:click={saveCurrentTab} disabled={!canSave}>
            <Save class="mr-2 size-4" />
            Save
          </Button>
        </div>

        <div class="flex flex-wrap gap-2">
          {#each openTabs as tab}
            <div class="inline-flex items-center gap-2 rounded-md border px-3 py-1.5 text-sm" class:bg-secondary={tab.path === activePath}>
              <button type="button" class="max-w-48 truncate" on:click={() => (activePath = tab.path)}>
                {tab.path}
                {#if tab.dirty}*{/if}
              </button>
              <button type="button" class="text-xs text-muted-foreground" on:click={() => closeTab(tab.path)}>x</button>
            </div>
          {/each}
        </div>
      </div>

      {#if activeTab}
        <div class="min-h-0 flex-1">
          {#key activePath}
            <CodeEditor path={activeTab.path} value={activeTab.content} on:change={(event) => updateCurrentTab(event.detail.value)} on:save={saveCurrentTab} />
          {/key}
        </div>
      {:else}
        <div class="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
          Select or create a file to start editing.
        </div>
      {/if}
    </section>

    <section class="flex min-h-0 flex-col gap-4">
      <article class="rounded-lg border bg-card p-4 shadow-xs">
        <div class="mb-4">
          <h2 class="text-base font-medium">Operations</h2>
          <p class="text-sm text-muted-foreground">Runtime actions and repo sync state for this service.</p>
        </div>

        <div class="grid gap-2">
          <Button type="button" on:click={() => triggerAction('deploy')} disabled={!!actionBusy || !workspace?.isDeclared}>
            <Play class="mr-2 size-4" />Deploy
          </Button>
          <Button type="button" variant="outline" on:click={() => triggerAction('update')} disabled={!!actionBusy || !workspace?.isDeclared}>
            <Upload class="mr-2 size-4" />Update
          </Button>
          <Button type="button" variant="outline" on:click={() => triggerAction('restart')} disabled={!!actionBusy || !workspace?.isDeclared}>
            <RefreshCcw class="mr-2 size-4" />Restart
          </Button>
          <Button type="button" variant="outline" on:click={() => triggerAction('stop')} disabled={!!actionBusy || !workspace?.isDeclared}>
            <Square class="mr-2 size-4" />Stop
          </Button>
          <Button type="button" variant="outline" on:click={() => triggerAction('backup')} disabled={!!actionBusy || !workspace?.isDeclared}>
            <Wrench class="mr-2 size-4" />Backup
          </Button>
          <a href={`/services/${workspace?.folder}/secret`} class="inline-flex h-10 items-center justify-center rounded-md border bg-background px-4 text-sm transition-colors hover:bg-accent hover:text-accent-foreground pointer-events-none opacity-50" class:pointer-events-auto={!!workspace?.isDeclared} class:opacity-100={!!workspace?.isDeclared}>
            Edit secret
          </a>
        </div>

        {#if !workspace?.hasMeta}
          <div class="mt-4 rounded-lg border border-dashed bg-muted/20 p-3 text-sm text-muted-foreground">
            This folder has no `composia-meta.yaml` yet. You can edit files now, then add the meta file to enable runtime actions.
          </div>
        {:else if !workspace?.isDeclared}
          <div class="mt-4 rounded-lg border border-dashed bg-muted/20 p-3 text-sm text-muted-foreground">
            `composia-meta.yaml` exists, but the controller has not accepted this service yet. Fix the meta file until the folder becomes a declared service.
          </div>
        {/if}

        <dl class="mt-4 space-y-2 text-sm text-muted-foreground">
          <div>
            <dt>Sync status</dt>
            <dd class="text-foreground">{syncStatus || 'unknown'}</dd>
          </div>
          {#if lastSuccessfulPullAt}
            <div>
              <dt>Last pull</dt>
              <dd class="text-foreground">{lastSuccessfulPullAt}</dd>
            </div>
          {/if}
          {#if syncError}
            <div class="rounded-lg border border-destructive/20 bg-destructive/10 p-3 text-destructive">
              {syncError}
            </div>
          {/if}
        </dl>
      </article>

      <article class="rounded-lg border bg-card p-4 shadow-xs">
        <div class="mb-3 flex items-center justify-between gap-3">
          <h2 class="text-base font-medium">Recent tasks</h2>
          <button type="button" class="text-xs text-muted-foreground" on:click={() => (logsExpanded = !logsExpanded)}>
            {logsExpanded ? 'Hide logs' : 'Show logs'}
          </button>
        </div>
        <div class="space-y-2">
          {#each tasks.slice(0, 8) as task}
            <button type="button" class="block w-full rounded-lg border bg-background px-3 py-3 text-left transition-colors hover:bg-muted/40" on:click={() => { selectedTaskId = task.taskId; logsExpanded = true; }}>
              <div class="flex items-center justify-between gap-3">
                <div class="min-w-0">
                  <div class="truncate text-sm font-medium">{task.type}</div>
                  <div class="truncate text-xs text-muted-foreground">{task.taskId}</div>
                </div>
                <Badge variant={taskStatusTone(task.status)}>{task.status}</Badge>
              </div>
              <div class="mt-2 text-xs text-muted-foreground">{formatTimestamp(task.createdAt)}</div>
            </button>
          {/each}
          {#if !tasks.length}
            <div class="rounded-lg border border-dashed bg-muted/20 px-3 py-6 text-sm text-muted-foreground">
              No tasks loaded.
            </div>
          {/if}
        </div>
      </article>

      <article class="rounded-lg border bg-card p-4 shadow-xs">
        <h2 class="mb-3 text-base font-medium">Recent backups</h2>
        <div class="space-y-2">
          {#each backups.slice(0, 6) as backup}
            <div class="rounded-lg border bg-background px-3 py-3 text-sm">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <div class="font-medium">{backup.dataName}</div>
                  <div class="text-xs text-muted-foreground">{backup.backupId}</div>
                </div>
                <Badge variant={taskStatusTone(backup.status)}>{backup.status}</Badge>
              </div>
              <div class="mt-2 text-xs text-muted-foreground">{formatTimestamp(backup.finishedAt || backup.startedAt)}</div>
            </div>
          {/each}
          {#if !backups.length}
            <div class="rounded-lg border border-dashed bg-muted/20 px-3 py-6 text-sm text-muted-foreground">
              No backups loaded.
            </div>
          {/if}
        </div>
      </article>
    </section>
  </div>

  <section class="mt-4 rounded-lg border bg-card shadow-xs">
    <button type="button" class="flex w-full items-center justify-between px-4 py-3 text-left" on:click={() => (logsExpanded = !logsExpanded)}>
      <div>
        <div class="text-sm font-medium">Logs</div>
        <div class="text-xs text-muted-foreground">Real-time task output for the selected service task.</div>
      </div>
      <span class="text-xs text-muted-foreground">{logsExpanded ? 'Collapse' : 'Expand'}</span>
    </button>

    {#if logsExpanded}
      <div class="h-80 border-t px-4 py-4">
        <TaskLogStream taskId={selectedTaskId} />
      </div>
    {/if}
  </section>
</div>
