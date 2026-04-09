<script lang="ts">
  import { Copy, FilePlus, FolderPlus, Pencil, Play, RefreshCcw, Save, Square, Trash2, Upload, Wrench } from 'lucide-svelte';

  import type { PageData } from './$types';
  import { messages } from '$lib/i18n';

  import CodeEditor from '$lib/components/app/code-editor.svelte';
  import ServiceFileTree from '$lib/components/app/service-file-tree.svelte';
  import TaskItem from '$lib/components/app/task-item.svelte';
  import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '$lib/components/ui/collapsible';
  import { Dialog, DialogTitle, DialogDescription, DialogFooter, DialogHeader } from '$lib/components/ui/dialog';
  import DialogContent from '$lib/components/ui/dialog/dialog-content.svelte';
  import DialogOverlay from '$lib/components/ui/dialog/dialog-overlay.svelte';
  import { Input } from '$lib/components/ui/input';
  import * as Popover from '$lib/components/ui/popover';
  import { Select, SelectContent, SelectItem, SelectTrigger } from '$lib/components/ui/select';
  import { toast } from 'svelte-sonner';
  import { formatTimestamp, runtimeStatusLabel, runtimeStatusTone, taskStatusLabel, taskStatusTone } from '$lib/presenters';
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

  let fileTree = $state<ServiceFileNode[]>([]);
  let collapsedPaths = $state(new Set<string>());
  let selectedNodePath = $state('');
  let openTabs = $state<EditorTab[]>([]);
  let activePath = $state('');
  let headRevision = $state('');
  let syncStatus = $state('');
  let syncError = $state('');
  let lastSuccessfulPullAt = $state('');
  let tasks = $state<TaskSummary[]>([]);
  let backups = $state<BackupSummary[]>([]);
  let actionBusy = $state('');
  let saving = $state(false);
  let errorMessage = $state('');
  let showNewFile = $state(false);
  let newFilePath = $state('');
  let showNewFolder = $state(false);
  let newFolderPath = $state('');
  let showRename = $state(false);
  let renamePath = $state('');
  let showDeleteDialog = $state(false);
  let showServiceRename = $state(false);
  let renameServiceFolder = $state('');
  let advancedOperationsOpen = $state(false);
  let workspace = $state<PageData['workspace']>(null);
  let serviceDetail = $state<PageData['serviceDetail']>(null);
  let nodeContainers = $state<NonNullable<PageData['nodeContainers']>>([]);
  let refreshTimer = $state<ReturnType<typeof setTimeout> | null>(null);
  let migrateSourceNode = $state('');
  let migrateTargetNode = $state('');
  let selectedInstanceNode = $state('__all__');

  $effect(() => {
    fileTree = data.fileTree;
    openTabs = data.initialFile ? [createTab(data.initialFile)] : [];
    selectedNodePath = data.initialFile?.path ?? '';
    activePath = data.initialFile?.path ?? '';
    headRevision = data.repoHead?.headRevision ?? '';
    syncStatus = data.repoHead?.syncStatus ?? '';
    syncError = data.repoHead?.lastSyncError ?? '';
    lastSuccessfulPullAt = data.repoHead?.lastSuccessfulPullAt ?? '';
    tasks = data.tasks;
    backups = data.backups;
    errorMessage = data.error ?? '';
    renameServiceFolder = data.workspace?.folder ?? '';
    workspace = data.workspace;
    serviceDetail = data.serviceDetail;
    nodeContainers = (data.nodeContainers ?? []) as NonNullable<PageData['nodeContainers']>;
    migrateSourceNode = data.serviceDetail?.nodes?.[0] ?? '';
    migrateTargetNode = '';
    selectedInstanceNode = '__all__';
  });

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
  let migrateSourceNodes = $derived(serviceDetail?.nodes ?? []);
  let hasMultipleInstanceNodes = $derived(nodeContainers.length > 1);
  let selectedInstanceEntry = $derived(
    nodeContainers.find((instance) => instance.nodeId === selectedInstanceNode) ?? null
  );
  let visibleNodeContainers = $derived(
    !hasMultipleInstanceNodes || selectedInstanceNode === '__all__'
      ? nodeContainers
      : nodeContainers.filter((instance) => instance.nodeId === selectedInstanceNode)
  );

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

      const response = await fetch(`/services/${workspace?.folder}/workspace?path=${encodeURIComponent(normalized)}`);
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
        method: 'PUT',
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
        method: 'PUT',
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
      const response = await fetch(`/services/${workspace?.folder}/workspace/directories`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
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
      const response = await fetch(`/services/${workspace?.folder}/workspace/path`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
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
      const response = await fetch(`/services/${workspace?.folder}/workspace/path`, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
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
    const response = await fetch(`/services/${workspace?.folder}/workspace`);
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
      const response = await fetch(`/services/${workspace?.folder}/actions/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
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
      toast.success(`${action} queued as ${payload.taskId}`);
      startActionRefresh(payload.taskId);
    } catch (actionError) {
      errorMessage = actionError instanceof Error ? actionError.message : `Failed to run ${action}.`;
    } finally {
      actionBusy = '';
    }
  }

  async function triggerCaddySync() {
    if (!workspace?.isDeclared || !workspace?.node || !workspace?.serviceName) {
      return;
    }
    actionBusy = 'caddy_sync';
    errorMessage = '';

    try {
      const response = await fetch(`/services/${workspace.folder}/actions/caddy-sync`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });
      const payload = (await response.json()) as { taskId?: string; error?: string };
      if (!response.ok || !payload.taskId) {
        throw new Error(payload.error ?? 'Failed to sync Caddy file.');
      }
      const newTask: TaskSummary = {
        taskId: payload.taskId,
        type: 'caddy_sync',
        status: 'pending',
        serviceName: workspace.serviceName,
        nodeId: workspace.node,
        createdAt: new Date().toISOString()
      };
      tasks = [newTask, ...tasks].slice(0, 12);
      toast.success(`caddy_sync queued as ${payload.taskId}`);
      startActionRefresh(payload.taskId);
    } catch (actionError) {
      errorMessage = actionError instanceof Error ? actionError.message : 'Failed to sync Caddy file.';
    } finally {
      actionBusy = '';
    }
  }

  async function triggerMigrate() {
    if (!workspace?.isDeclared || !workspace?.serviceName || !migrateSourceNode || !migrateTargetNode.trim()) {
      errorMessage = 'Select a source node and enter a target node.';
      return;
    }
    actionBusy = 'migrate';
    errorMessage = '';

    try {
      const response = await fetch(`/services/${workspace.folder}/actions/migrate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          sourceNodeId: migrateSourceNode,
          targetNodeId: migrateTargetNode.trim()
        })
      });
      const payload = (await response.json()) as ServiceActionResult & { error?: string };
      if (!response.ok || !payload.taskId) {
        throw new Error(payload.error ?? 'Failed to run migrate.');
      }
      const newTask: TaskSummary = {
        taskId: payload.taskId,
        type: 'migrate',
        status: payload.status,
        serviceName: workspace.serviceName,
        nodeId: migrateSourceNode,
        createdAt: new Date().toISOString()
      };
      tasks = [newTask, ...tasks].slice(0, 12);
      toast.success(`migrate queued as ${payload.taskId}`);
      startActionRefresh(payload.taskId);
    } catch (actionError) {
      errorMessage = actionError instanceof Error ? actionError.message : 'Failed to run migrate.';
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
      const response = await fetch(`/services/${workspace?.folder}/root`, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
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
      const response = await fetch(`/services/${workspace.folder}/root`, {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
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

<div class="page-shell flex min-h-[calc(100vh-72px)] flex-col">
  <div class="page-stack flex min-h-0 flex-1 flex-col">
    <Card>
      <CardHeader class="gap-3 py-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="min-w-0 space-y-1">
            <CardTitle class="page-title">{workspace?.displayName ?? $messages.services.service}</CardTitle>
            <div class="truncate text-sm text-muted-foreground">{workspace?.folder ?? 'n/a'}</div>
          </div>

          <Badge variant={runtimeStatusTone(workspace?.runtimeStatus ?? 'unknown')}>
            {runtimeStatusLabel(workspace?.runtimeStatus ?? '', $messages)}
          </Badge>
        </div>

        <div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-muted-foreground">
          <div class="flex items-center gap-1.5">
            <span>{$messages.nodes.node}</span>
            <span class="font-medium text-foreground">{workspace?.node || $messages.common.na}</span>
          </div>
          <div class="flex items-center gap-1.5">
            <span>{$messages.settings.repoSync.revision}</span>
            <span class="font-medium text-foreground">{headRevision ? headRevision.slice(0, 12) : $messages.common.na}</span>
          </div>
          <div class="flex items-center gap-1.5">
            <span>{$messages.services.syncStatus}</span>
            <span class="font-medium text-foreground">{syncStatus || $messages.status.unknown}</span>
          </div>
          <div class="flex items-center gap-1.5">
            <span>{$messages.services.lastPull}</span>
            <span class="font-medium text-foreground">{lastSuccessfulPullAt || $messages.common.never}</span>
          </div>
        </div>

        {#if syncError}
          <Alert variant="destructive">
            <AlertTitle>{$messages.error.syncFailed}</AlertTitle>
            <AlertDescription>{syncError}</AlertDescription>
          </Alert>
        {/if}
      </CardHeader>
    </Card>

  {#if (nodeContainers ?? []).length > 0}
    <Card>
      <CardHeader class="gap-3">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <CardTitle class="section-title">{$messages.services.instances}</CardTitle>
            <div class="text-sm text-muted-foreground">{$messages.services.containersByNode}</div>
          </div>

          {#if hasMultipleInstanceNodes}
            <Select type="single" bind:value={selectedInstanceNode as any}>
              <SelectTrigger class="w-[240px]">
                {#if selectedInstanceNode === '__all__'}
                  <span>{$messages.services.allNodes}</span>
                {:else}
                  <span>
                    {selectedInstanceNode}
                    {#if selectedInstanceEntry}
                      · {runtimeStatusLabel(selectedInstanceEntry.runtimeStatus, $messages)}
                    {/if}
                  </span>
                {/if}
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">{$messages.services.allNodes}</SelectItem>
                {#each nodeContainers ?? [] as instance}
                  <SelectItem value={instance.nodeId}>
                    {instance.nodeId} · {runtimeStatusLabel(instance.runtimeStatus, $messages)}
                  </SelectItem>
                {/each}
              </SelectContent>
            </Select>
          {/if}
        </div>
      </CardHeader>
      <CardContent class="space-y-4">
        {#each visibleNodeContainers as instance, index}
          <section class="space-y-3">
            {#if hasMultipleInstanceNodes && selectedInstanceNode === '__all__'}
              <div class="flex items-center gap-3">
                {#if index > 0}
                  <div class="h-px flex-1 bg-border/70"></div>
                {/if}
                <div class="flex items-center gap-2 text-sm font-medium text-foreground">
                  <span>{instance.nodeId}</span>
                  <Badge variant={runtimeStatusTone(instance.runtimeStatus)}>{runtimeStatusLabel(instance.runtimeStatus, $messages)}</Badge>
                </div>
                <div class="h-px flex-1 bg-border/70"></div>
              </div>
            {/if}

            {#if instance.containers.length > 0}
              <div class="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {#each instance.containers as container}
                  <a href="/nodes/{instance.nodeId}/docker/containers/{encodeURIComponent(container.containerId)}" class="block rounded-md border border-border/60 bg-background px-3 py-2 transition-colors hover:bg-accent/40">
                    <div class="flex items-center justify-between gap-2">
                      <div class="font-medium">{container.name}</div>
                      <Badge variant={runtimeStatusTone(container.state)}>{runtimeStatusLabel(container.state, $messages)}</Badge>
                    </div>
                    <div class="mt-1 text-xs text-muted-foreground">{container.image}</div>
                    <div class="mt-1 text-[11px] text-muted-foreground/80">
                      {container.composeProject}/{container.composeService}
                    </div>
                  </a>
                {/each}
              </div>
            {:else}
              <div class="rounded-lg border border-dashed border-border/70 px-3 py-4 text-sm text-muted-foreground">
                {$messages.services.noContainersOnNode}
              </div>
            {/if}
          </section>
        {/each}
      </CardContent>
    </Card>
  {/if}

  {#if errorMessage}
    <Alert variant="destructive">
      <AlertTitle>{$messages.error.workspaceError}</AlertTitle>
      <AlertDescription>{errorMessage}</AlertDescription>
    </Alert>
  {/if}

    <div class="grid min-h-0 flex-1 gap-4 xl:grid-cols-[280px_minmax(0,1fr)_320px]">
      <Card class="flex min-h-0 min-w-0 flex-col">
        <CardHeader class="section-header border-b">
          <CardTitle class="section-title">{$messages.services.files.title}</CardTitle>
          <div class="flex flex-wrap items-center gap-2">
           <Popover.Root bind:open={showNewFile}>
             <Popover.Trigger class="inline-flex">
               {#snippet child({ props: triggerProps })}
                 <Button type="button" variant="outline" size="sm" {...triggerProps}>
                   <FilePlus class="mr-2 size-4" />{$messages.common.newFile}
                 </Button>
               {/snippet}
             </Popover.Trigger>
             <Popover.Content class="w-80" sideOffset={8}>
               <div class="space-y-3">
                 <Input bind:value={newFilePath} placeholder="config/new-file.yaml" />
                 <div class="flex items-center justify-between gap-3">
                   <p class="text-xs text-muted-foreground">{$messages.common.parentsAutoCreated}</p>
                   <Button type="button" size="sm" onclick={createFile} disabled={saving}>{$messages.common.create}</Button>
                 </div>
               </div>
             </Popover.Content>
           </Popover.Root>

           <Popover.Root bind:open={showNewFolder}>
             <Popover.Trigger class="inline-flex">
               {#snippet child({ props: triggerProps })}
                 <Button type="button" variant="outline" size="sm" {...triggerProps}>
                   <FolderPlus class="mr-2 size-4" />{$messages.common.newFolder}
                 </Button>
               {/snippet}
             </Popover.Trigger>
             <Popover.Content class="w-80" sideOffset={8}>
               <div class="space-y-3">
                 <Input bind:value={newFolderPath} placeholder="config/snippets" />
                 <div class="flex items-center justify-between gap-3">
                   <p class="text-xs text-muted-foreground">{$messages.common.trackedWithGitkeep}</p>
                   <Button type="button" size="sm" onclick={createDirectory} disabled={saving}>{$messages.common.create}</Button>
                 </div>
               </div>
             </Popover.Content>
           </Popover.Root>

           <Button type="button" variant="outline" size="sm" onclick={() => { showRename = !showRename; renamePath = selectedNodePath; }} disabled={!selectedNodePath || saving}>
             <Pencil class="mr-2 size-4" />{$messages.common.rename}
           </Button>
            <Button type="button" variant="outline" size="sm" onclick={() => (showDeleteDialog = true)} disabled={!selectedNodePath || saving}>
              <Trash2 class="mr-2 size-4" />{$messages.common.delete}
            </Button>
          </div>
        </CardHeader>

      {#if showRename}
        <div class="border-b px-4 py-3 text-sm">
          <div class="space-y-3">
            <Input bind:value={renamePath} placeholder="new/path.yaml" />
            <div class="flex justify-end">
              <Button type="button" size="sm" onclick={renameNode} disabled={!selectedNodePath || saving}>{$messages.common.apply}</Button>
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

      <Card class="flex min-h-0 min-w-0 flex-col">
        <CardHeader class="border-b">
          <div class="section-header">
           <CardTitle class="section-title">{$messages.services.files.editor}</CardTitle>
           <Button type="button" size="sm" onclick={saveCurrentTab} disabled={!canSave}>
             <Save class="mr-2 size-4" />
             {$messages.common.save}
           </Button>
          </div>

          <div class="flex flex-wrap gap-2">
          {#each openTabs as tab}
            <div class="inline-flex items-center gap-2 rounded-md border bg-background px-3 py-1.5 text-sm" class:bg-secondary={tab.path === activePath}>
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
          {$messages.services.files.openFileToEdit}
        </div>
      {/if}
    </Card>


      <section class="flex min-h-0 min-w-0 flex-col gap-4">
        <Card>
        <CardHeader class="section-header">
          <CardTitle class="section-title">{$messages.services.operations.title}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-4">
          <div class="grid gap-2">
            <Button type="button" onclick={() => triggerAction('deploy')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <Play class="mr-2 size-4" />{$messages.services.operations.deploy}
            </Button>
            <Button type="button" variant="outline" onclick={() => triggerAction('update')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <Upload class="mr-2 size-4" />{$messages.services.operations.update}
            </Button>
            <Button type="button" variant="outline" onclick={() => triggerAction('restart')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <RefreshCcw class="mr-2 size-4" />{$messages.services.operations.restart}
            </Button>
            <Button type="button" variant="outline" onclick={() => triggerAction('stop')} disabled={!!actionBusy || !workspace?.isDeclared}>
              <Square class="mr-2 size-4" />{$messages.services.operations.stop}
            </Button>
          </div>

          <Collapsible bind:open={advancedOperationsOpen}>
            <CollapsibleTrigger class="group flex w-full items-center gap-3 py-1 text-xs text-muted-foreground hover:text-foreground">
              <div class="h-px flex-1 bg-border/70 transition-colors group-hover:bg-border"></div>
              <span>{advancedOperationsOpen ? $messages.services.collapse : $messages.services.expand}</span>
              <div class="h-px flex-1 bg-border/70 transition-colors group-hover:bg-border"></div>
            </CollapsibleTrigger>
            <CollapsibleContent>
              <div class="grid gap-2 pt-3">
                <Button type="button" variant="outline" onclick={() => triggerAction('backup')} disabled={!!actionBusy || !workspace?.isDeclared}>
                  <Wrench class="mr-2 size-4" />{$messages.services.operations.backup}
                </Button>
                <Button type="button" variant="outline" onclick={() => triggerAction('dns_update')} disabled={!!actionBusy || !workspace?.isDeclared}>
                  <Upload class="mr-2 size-4" />{$messages.services.operations.dnsUpdate}
                </Button>
                <Button type="button" variant="outline" onclick={() => triggerCaddySync()} disabled={!!actionBusy || !workspace?.isDeclared || !workspace?.node}>
                  <Copy class="mr-2 size-4" />{$messages.services.operations.syncCaddy}
                </Button>
                <div class="space-y-3 rounded-lg border border-border/60 bg-muted/20 p-3">
                  <div class="text-sm font-medium">{$messages.services.operations.migrate.title}</div>
                  <Select type="single" bind:value={migrateSourceNode as any}>
                    <SelectTrigger>
                      <span>{migrateSourceNode || $messages.services.operations.migrate.selectSource}</span>
                    </SelectTrigger>
                    <SelectContent>
                      {#each migrateSourceNodes as nodeId}
                        <SelectItem value={nodeId}>{nodeId}</SelectItem>
                      {/each}
                    </SelectContent>
                  </Select>
                  <Input bind:value={migrateTargetNode} placeholder={$messages.services.operations.migrate.targetNodeId} />
                  <Button type="button" variant="outline" onclick={triggerMigrate} disabled={!!actionBusy || !workspace?.isDeclared || !migrateSourceNode || !migrateTargetNode.trim()}>
                    <RefreshCcw class="mr-2 size-4" />{$messages.services.operations.migrate.migrate}
                  </Button>
                </div>
                <Button type="button" variant="outline" onclick={() => { showServiceRename = !showServiceRename; renameServiceFolder = workspace?.folder ?? ''; }} disabled={saving}>
                  <Pencil class="mr-2 size-4" />{$messages.services.operations.renameFolder}
                </Button>
                <Button type="button" variant="outline" class="border-destructive text-destructive hover:bg-destructive/10 hover:text-destructive" onclick={deleteServiceRoot} disabled={saving}>
                  <Trash2 class="mr-2 size-4" />{$messages.services.operations.deleteService}
                </Button>
              </div>
            </CollapsibleContent>
          </Collapsible>

          {#if showServiceRename}
            <div class="space-y-3 border-t pt-4">
              <Input bind:value={renameServiceFolder} placeholder="new-service-folder" />
              <div class="flex justify-end">
                <Button type="button" size="sm" onclick={renameServiceRoot} disabled={saving}>{$messages.common.apply}</Button>
              </div>
            </div>
          {/if}

          {#if !workspace?.hasMeta}
            <div class="empty-state px-3 py-4">{$messages.services.addMetaToDeclare}</div>
          {:else if !workspace?.isDeclared}
            <div class="empty-state px-3 py-4">{$messages.services.fixMetaUntilAccepted}</div>
          {/if}
        </CardContent>
      </Card>

        <Card>
        <CardHeader class="section-header">
          <div class="section-heading">
            <CardTitle class="section-title">{$messages.services.recentTasks}</CardTitle>
          </div>
          {#if workspace?.serviceName}
            <a class="text-sm text-muted-foreground transition-colors hover:text-foreground" href={`/tasks?serviceName=${encodeURIComponent(workspace.serviceName)}`}>
              {$messages.common.viewAll}
            </a>
          {/if}
        </CardHeader>
        <CardContent class="space-y-3">
          {#each recentTasks as task}
            <TaskItem {task} showService={false} />
          {/each}
          {#if !recentTasks.length}
            <div class="empty-state px-3 py-6">{$messages.tasks.noTasks}</div>
          {/if}
        </CardContent>
      </Card>

        <Card>
        <CardHeader>
          <CardTitle class="section-title">{$messages.services.recentBackups}</CardTitle>
        </CardHeader>
        <CardContent class="space-y-2">
          {#each backups.slice(0, 6) as backup}
            <div class="list-row-compact">
              <div class="flex items-center justify-between gap-3">
                <div>
                  <div class="font-medium">{backup.dataName}</div>
                  <div class="text-xs text-muted-foreground">{backup.backupId}</div>
                </div>
                 <Badge variant={taskStatusTone(backup.status)}>{taskStatusLabel(backup.status, $messages)}</Badge>
              </div>
              <div class="mt-2 text-xs text-muted-foreground">{formatTimestamp(backup.finishedAt || backup.startedAt)}</div>
            </div>
          {/each}
          {#if !backups.length}
            <div class="empty-state px-3 py-6">{$messages.backups.noBackups}</div>
          {/if}
        </CardContent>
      </Card>
      </section>
    </div>

    <Dialog bind:open={showDeleteDialog}>
      <DialogOverlay />
      <DialogContent class="max-w-sm">
        <DialogHeader>
          <DialogTitle>{$messages.common.delete} {selectedNode?.isDir ? $messages.common.folder : $messages.common.file}?</DialogTitle>
          <DialogDescription>
            {selectedNode?.isDir ? $messages.services.deleteFolderConfirm : $messages.services.deleteFileConfirm}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" variant="outline" onclick={() => (showDeleteDialog = false)}>{$messages.common.cancel}</Button>
          <Button type="button" variant="destructive" onclick={() => { showDeleteDialog = false; deleteNode(); }} disabled={saving}>{$messages.common.delete}</Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</div>
