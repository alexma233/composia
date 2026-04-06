<script lang="ts">
  import { FilePlus, FolderPlus, Pencil, Play, RefreshCcw, Save, Square, Trash2, Upload, Wrench } from 'lucide-svelte';

  import type { PageData } from './$types';

  import CodeEditor from '$lib/components/app/code-editor.svelte';
  import ServiceFileTree from '$lib/components/app/service-file-tree.svelte';
  import TaskItem from '$lib/components/app/task-item.svelte';
  import TaskLogStream from '$lib/components/app/task-log-stream.svelte';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Input } from '$lib/components/ui/input';
  import { toast } from 'svelte-sonner';
  import { formatTimestamp, runtimeStatusTone, taskStatusTone } from '$lib/presenters';
  import type {
    BackupSummary,
    RepoWriteResult,
    ServiceActionResult,
    TaskSummary
  } from '$lib/server/controller';
  import type { ServiceFileNode, WorkspaceFile } from '$lib/service-workspace';
  import { findNode, normalizeServiceRelativePath, upsertFileNode } from '$lib/service-workspace';

  let { data }: { data: PageData } = $props();

  type EditorTab = WorkspaceFile & {
    savedContent: string;
    dirty: boolean;
  };

  let fileTree = $state<ServiceFileNode[]>(data.fileTree);
  let collapsedPaths = $state(new Set<string>());
  let selectedNodePath = $state(data.initialFile?.path ?? '');
  let openTabs = $state<EditorTab[]>(data.initialFile ? [createTab(data.initialFile)] : []);
  let activePath = $state(data.initialFile?.path ?? '');
  let headRevision = $state(data.repoHead?.headRevision ?? '');
  let syncStatus = $state(data.repoHead?.syncStatus ?? '');
  let syncError = $state(data.repoHead?.lastSyncError ?? '');
  let lastSuccessfulPullAt = $state(data.repoHead?.lastSuccessfulPullAt ?? '');
  let tasks = $state<TaskSummary[]>(data.tasks);
  let backups = $state<BackupSummary[]>(data.backups);
  let selectedTaskId = $state(data.tasks[0]?.taskId ?? '');
  let logsExpanded = $state(false);
  let actionBusy = $state('');
  let saving = $state(false);
  let errorMessage = $state(data.error ?? '');
  let showNewFile = $state(false);
  let newFilePath = $state('');
  let showNewFolder = $state(false);
  let newFolderPath = $state('');
  let showRename = $state(false);
  let renamePath = $state('');
  let showServiceRename = $state(false);
  let renameServiceFolder = $state(data.workspace?.folder ?? '');
  let workspace = $state(data.workspace);
  let refreshTimer = $state<ReturnType<typeof setTimeout> | null>(null);

  function createTab(file: WorkspaceFile): EditorTab {
    return {
      ...file,
      savedContent: file.content,
      dirty: false
    };
  }

  let activeTab = $derived(openTabs.find((tab) => tab.path === activePath) ?? null);
  let canSave = $derived(Boolean(activeTab && activeTab.dirty && !saving));
  let selectedNode = $derived(selectedNodePath ? findNode(fileTree, selectedNodePath) : null);
  let recentTasks = $derived(tasks.filter((task) => isTaskRecent(task.createdAt)).slice(0, 4));

  $effect(() => {
    return () => stopActionRefresh();
  });

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
      toast.success(`Saved ${tab.path}`);
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
      toast.success(`Created ${normalized}`);
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
      toast.success(`Created folder ${normalized}`);
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
      toast.success(`Renamed to ${destination}`);
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
      toast.success(`Deleted ${deletedPath}`);
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

  async function refreshServiceSummary() {
    const response = await fetch(`/services/${workspace?.folder}/workspace/summary`);
    const payload = await response.json();
    if (!response.ok) {
      throw new Error(payload.error ?? 'Failed to refresh service summary.');
    }

    workspace = payload.workspace ?? workspace;
    tasks = payload.tasks ?? tasks;
    backups = payload.backups ?? backups;
    return payload as {
      workspace?: PageData['workspace'];
      tasks?: TaskSummary[];
      backups?: BackupSummary[];
    };
  }

  function startActionRefresh(taskId: string) {
    stopActionRefresh();

    const tick = async () => {
      try {
        const payload = await refreshServiceSummary();
        const task = (payload.tasks ?? []).find((entry) => entry.taskId === taskId);
        if (!task) {
          refreshTimer = setTimeout(tick, 2500);
          return;
        }
        if (isTerminalTaskStatus(task.status)) {
          stopActionRefresh();
          return;
        }
        refreshTimer = setTimeout(tick, 2500);
      } catch {
        refreshTimer = setTimeout(tick, 4000);
      }
    };

    refreshTimer = setTimeout(tick, 1200);
  }

  function stopActionRefresh() {
    if (refreshTimer) {
      clearTimeout(refreshTimer);
      refreshTimer = null;
    }
  }

  function isTaskRecent(createdAt: string) {
    const createdAtMs = Date.parse(createdAt);
    if (Number.isNaN(createdAtMs)) {
      return false;
    }
    return Date.now() - createdAtMs <= 24 * 60 * 60 * 1000;
  }

  function isTerminalTaskStatus(status: string) {
    return status === 'succeeded' || status === 'failed' || status === 'cancelled';
  }

  async function triggerAction(action: 'deploy' | 'update' | 'stop' | 'restart' | 'backup' | 'dns_update') {
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
      toast.success(`${action} queued as ${payload.taskId}`);
      startActionRefresh(payload.taskId);
    } catch (actionError) {
      errorMessage = actionError instanceof Error ? actionError.message : `Failed to run ${action}.`;
    } finally {
      actionBusy = '';
    }
  }

  async function renameServiceRoot() {
    if (!renameServiceFolder.trim()) {
      return;
    }

    saving = true;
    errorMessage = '';

    try {
      const response = await fetch(`/services/${workspace?.folder}/workspace/service`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action: 'rename',
          folder: renameServiceFolder,
          baseRevision: headRevision
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.redirectTo) {
        throw new Error(payload.error ?? 'Failed to rename service folder.');
      }

      window.location.href = payload.redirectTo;
    } catch (renameError) {
      errorMessage = renameError instanceof Error ? renameError.message : 'Failed to rename service folder.';
      saving = false;
    }
  }

  async function deleteServiceRoot() {
    if (!workspace?.folder || !confirm(`Delete service folder ${workspace.folder}?`)) {
      return;
    }

    saving = true;
    errorMessage = '';

    try {
      const response = await fetch(`/services/${workspace.folder}/workspace/service`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action: 'delete',
          baseRevision: headRevision
        })
      });
      const payload = await response.json();
      if (!response.ok || !payload.redirectTo) {
        throw new Error(payload.error ?? 'Failed to delete service folder.');
      }

      window.location.href = payload.redirectTo;
    } catch (deleteError) {
      errorMessage = deleteError instanceof Error ? deleteError.message : 'Failed to delete service folder.';
      saving = false;
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
  <Card class="mb-4 border-border/70 bg-card/95">
    <CardHeader class="gap-4 p-4 sm:p-5">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="space-y-1">
          <CardTitle class="page-title">{workspace?.displayName ?? 'Service'}</CardTitle>
          <CardDescription class="page-description">{workspace?.folder ?? 'n/a'}</CardDescription>
        </div>

        <div class="flex flex-wrap items-center gap-2 text-sm">
          <Badge variant="outline">Node {workspace?.node || 'n/a'}</Badge>
          <Badge variant="outline">Rev {headRevision ? headRevision.slice(0, 12) : 'n/a'}</Badge>
          <Badge variant={runtimeStatusTone(workspace?.runtimeStatus ?? 'unknown')}>
            {workspace?.runtimeStatus ?? 'unknown'}
          </Badge>
        </div>
      </div>
    </CardHeader>
  </Card>

  {#if errorMessage}
    <Alert variant="destructive" class="mb-4">
      <AlertTitle>Workspace error</AlertTitle>
      <AlertDescription>{errorMessage}</AlertDescription>
    </Alert>
  {/if}

  <div class="grid min-h-0 flex-1 gap-4 xl:grid-cols-[280px_minmax(0,1fr)_320px]">
    <Card class="flex min-h-0 flex-col border-border/70 bg-card/95">
      <CardHeader class="border-b px-4 py-3">
        <CardTitle class="section-title">Files</CardTitle>
<div class="flex flex-wrap items-center gap-2">
           <Button type="button" variant="outline" size="sm" onclick={() => (showNewFile = !showNewFile)}>
             <FilePlus class="mr-2 size-4" />New file
           </Button>
           <Button type="button" variant="outline" size="sm" onclick={() => (showNewFolder = !showNewFolder)}>
             <FolderPlus class="mr-2 size-4" />New folder
           </Button>
           <Button type="button" variant="outline" size="sm" onclick={() => { showRename = !showRename; renamePath = selectedNodePath; }} disabled={!selectedNodePath || saving}>
             <Pencil class="mr-2 size-4" />Rename
           </Button>
           <Button type="button" variant="outline" size="sm" onclick={deleteNode} disabled={!selectedNodePath || saving}>
             <Trash2 class="mr-2 size-4" />Delete
           </Button>
         </div>
      </CardHeader>

      {#if showNewFile}
        <div class="space-y-3 border-b px-4 py-3">
          <Input bind:value={newFilePath} placeholder="config/new-file.yaml" />
          <div class="flex items-center justify-between gap-3">
            <p class="text-xs text-muted-foreground">Parents are created automatically.</p>
            <Button type="button" size="sm" onclick={createFile} disabled={saving}>Create</Button>
          </div>
        </div>
      {/if}

      {#if showNewFolder}
        <div class="space-y-3 border-b px-4 py-3">
          <Input bind:value={newFolderPath} placeholder="config/snippets" />
          <div class="flex items-center justify-between gap-3">
            <p class="text-xs text-muted-foreground">Tracked with `.gitkeep`.</p>
            <Button type="button" size="sm" onclick={createDirectory} disabled={saving}>Create</Button>
          </div>
        </div>
      {/if}

      {#if showRename}
        <div class="border-b px-4 py-3 text-sm">
          <div class="space-y-3">
            <Input bind:value={renamePath} placeholder="new/path.yaml" />
            <div class="flex justify-end">
              <Button type="button" size="sm" onclick={renameNode} disabled={!selectedNodePath || saving}>Apply</Button>
            </div>
          </div>
        </div>
      {/if}

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
    </Card>

    <Card class="flex min-h-0 flex-col border-border/70 bg-card/95">
      <CardHeader class="border-b px-3 py-3">
        <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
          <CardTitle class="section-title">Editor</CardTitle>
          <Button type="button" size="sm" onclick={saveCurrentTab} disabled={!canSave}>
            <Save class="mr-2 size-4" />
            Save
          </Button>
        </div>

        <div class="flex flex-wrap gap-2">
          {#each openTabs as tab}
            <div class="inline-flex items-center gap-2 rounded-md border border-border/70 bg-background/80 px-3 py-1.5 text-sm" class:bg-secondary={tab.path === activePath}>
              <button type="button" class="max-w-48 truncate" onclick={() => (activePath = tab.path)}>
                {tab.path}
                {#if tab.dirty}*{/if}
              </button>
              <button type="button" class="text-xs text-muted-foreground hover:text-foreground" onclick={() => closeTab(tab.path)}>x</button>
            </div>
          {/each}
        </div>
      </CardHeader>

      {#if activeTab}
        <div class="min-h-0 flex-1">
          {#key activePath}
            <CodeEditor path={activeTab.path} value={activeTab.content} onchange={({ value }) => updateCurrentTab(value)} onsave={saveCurrentTab} />
          {/key}
        </div>
      {:else}
        <div class="flex min-h-0 flex-1 items-center justify-center text-sm text-muted-foreground">
          Open a file to start editing.
        </div>
      {/if}
    </Card>

    <section class="flex min-h-0 flex-col gap-4">
      <Card class="border-border/70 bg-card/95">
        <CardHeader class="flex items-center justify-between gap-3 p-4">
          <CardTitle class="section-title">Operations</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4 p-4 pt-0">
          <div class="grid gap-2">
            <Button type="button" onclick={() => triggerAction('deploy')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <Play class="mr-2 size-4" />Deploy (Up)
            </Button>
            <Button type="button" variant="outline" onclick={() => triggerAction('update')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <Upload class="mr-2 size-4" />Update (Pull + Up)
            </Button>
            <Button type="button" variant="outline" onclick={() => triggerAction('restart')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <RefreshCcw class="mr-2 size-4" />Restart (Down + Up)
            </Button>
            <Button type="button" variant="outline" onclick={() => triggerAction('stop')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <Square class="mr-2 size-4" />Stop (Down)
            </Button>
            <Button type="button" variant="outline" onclick={() => triggerAction('backup')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <Wrench class="mr-2 size-4" />Backup
            </Button>
            <Button type="button" variant="outline" onclick={() => triggerAction('dns_update')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <Upload class="mr-2 size-4" />DNS update
            </Button>
            <a href={`/services/${workspace?.folder}/secret`} class="inline-flex h-10 items-center justify-center rounded-md border border-border bg-background px-4 text-sm transition-colors hover:bg-accent hover:text-accent-foreground pointer-events-none opacity-50" class:pointer-events-auto={!!workspace?.isDeclared} class:opacity-100={!!workspace?.isDeclared}>
              Edit secret
            </a>
            <Button type="button" variant="outline" onclick={() => { showServiceRename = !showServiceRename; renameServiceFolder = workspace?.folder ?? ''; }} disabled={saving}>
              <Pencil class="mr-2 size-4" />Rename folder
            </Button>
            <Button type="button" variant="outline" class="border-destructive text-destructive hover:bg-destructive/10 hover:text-destructive" onclick={deleteServiceRoot} disabled={saving}>
              <Trash2 class="mr-2 size-4" />Delete service
            </Button>
          </div>

          {#if showServiceRename}
            <div class="space-y-3 border-t pt-4">
              <Input bind:value={renameServiceFolder} placeholder="new-service-folder" />
              <div class="flex justify-end">
                <Button type="button" size="sm" onclick={renameServiceRoot} disabled={saving}>Apply</Button>
              </div>
            </div>
          {/if}

          {#if !workspace?.hasMeta}
            <div class="empty-state px-3 py-4">Add `composia-meta.yaml` to declare this service.</div>
          {:else if !workspace?.isDeclared}
            <div class="empty-state px-3 py-4">Fix `composia-meta.yaml` until the controller accepts it.</div>
          {/if}

          <dl class="kv-grid">
            <div>
              <dt>Sync status</dt>
              <dd>{syncStatus || 'unknown'}</dd>
            </div>
            {#if lastSuccessfulPullAt}
              <div>
                <dt>Last pull</dt>
                <dd>{lastSuccessfulPullAt}</dd>
              </div>
            {/if}
          </dl>

          {#if syncError}
            <Alert variant="destructive">
              <AlertTitle>Sync error</AlertTitle>
              <AlertDescription>{syncError}</AlertDescription>
            </Alert>
          {/if}
        </CardContent>
      </Card>

      <Card class="border-border/70 bg-card/95">
        <CardHeader class="flex items-center justify-between gap-3 p-4">
          <div class="space-y-1">
            <CardTitle class="section-title">Recent tasks</CardTitle>
          </div>
          <button type="button" class="text-xs text-muted-foreground hover:text-foreground" onclick={() => (logsExpanded = !logsExpanded)}>
            {logsExpanded ? 'Hide logs' : 'Show logs'}
          </button>
        </CardHeader>
        <CardContent class="space-y-3 p-4 pt-0">
          {#each recentTasks as task}
            <TaskItem {task} showService={false} />
          {/each}
          {#if !recentTasks.length}
            <div class="empty-state px-3 py-6">No recent tasks.</div>
          {/if}
        </CardContent>
      </Card>

      <Card class="border-border/70 bg-card/95">
        <CardHeader class="p-4">
          <CardTitle class="section-title">Recent backups</CardTitle>
        </CardHeader>
        <CardContent class="space-y-2 p-4 pt-0">
          {#each backups.slice(0, 6) as backup}
            <div class="rounded-lg border border-border/70 bg-background/80 px-3 py-3 text-sm">
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
            <div class="empty-state px-3 py-6">No backups loaded.</div>
          {/if}
        </CardContent>
      </Card>
    </section>
  </div>

  <Card class="mt-4 border-border/70 bg-card/95">
    <button type="button" class="flex w-full items-center justify-between px-4 py-3 text-left" onclick={() => (logsExpanded = !logsExpanded)}>
      <CardTitle class="section-title">Logs</CardTitle>
      <span class="text-xs text-muted-foreground">{logsExpanded ? 'Collapse' : 'Expand'}</span>
    </button>

    {#if logsExpanded}
      <div class="h-80 border-t px-4 py-4">
        <TaskLogStream taskId={selectedTaskId} />
      </div>
    {/if}
  </Card>
</div>
