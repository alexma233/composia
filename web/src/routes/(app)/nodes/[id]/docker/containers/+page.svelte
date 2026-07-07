<script lang="ts">
  import { goto, invalidate } from '$app/navigation';
  import { page } from '$app/stores';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Table, TableBody, TableCaption, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge } from '$lib/components/ui/badge';
  import { Button } from '$lib/components/ui/button';
  import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogOverlay,
    DialogTitle,
  } from '$lib/components/ui/dialog';
  import {
    buildDockerListPageUrl,
    debouncedDockerListSearchState,
    dockerSearchDebounceMs,
    type DockerListSortDirection,
  } from '$lib/docker-list-query';
  import {
  containerStateTone,
  formatDockerTimestamp,
  formatShortId,
} from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import DockerListShell from '$lib/components/app/docker-list-shell.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import { actionErrorMessage } from '$lib/capabilities';
  import { getMessages } from '$lib/i18n';

  const messages = getMessages();

  interface Props {
    data: PageData;
  }

  let { data }: Props = $props();

  type DockerContainerSummary = {
    id: string;
    name: string;
    image: string;
    state: string;
    status: string;
    created: string;
    labels: Record<string, string>;
    ports: string[];
    networks: string[];
    imageId: string;
  };

  type DockerContainerSortField = 'name' | 'state' | 'image' | 'created';

  const defaultSortField: DockerContainerSortField = 'name';

  let searchQuery = $state('');
  let debouncedSearchQuery = $state('');
  let searchDebounceTimer = $state<ReturnType<typeof setTimeout> | null>(null);
  let sortField = $state<DockerContainerSortField>(defaultSortField);
  let sortDirection = $state<DockerListSortDirection>('asc');
  let currentPage = $state(1);
  let refreshing = $state(false);
  let actionBusyId = $state('');
  let removeBusyId = $state('');
  let removeDialogOpen = $state(false);
  let removeTarget = $state<DockerContainerSummary | null>(null);

  let loading = $derived(!data.ready || refreshing);
  let loadError = $derived(data.error ?? null);
  let containers = $derived((data.containers ?? []) as DockerContainerSummary[]);
  let totalPages = $derived(
    data.totalCount > 0 ? Math.ceil(data.totalCount / data.pageSize) : 0,
  );
  let currentPath = $derived($page.url.pathname);

  $effect(() => {
    refreshing = false;
    currentPage = data.page;
    searchQuery = data.search;
    debouncedSearchQuery = data.search;
    sortField = data.sortBy as DockerContainerSortField;
    sortDirection = data.sortDirection as DockerListSortDirection;
  });

  $effect(() => {
    if (!data.ready) {
      return;
    }

    if (
      currentPage === data.page &&
      debouncedSearchQuery === data.search &&
      sortField === data.sortBy &&
      sortDirection === data.sortDirection
    ) {
      return;
    }

    refreshing = true;
    void goto(pageUrl(currentPage, debouncedSearchQuery, sortField, sortDirection), {
      keepFocus: true,
      noScroll: true,
      replaceState:
        debouncedSearchQuery !== data.search ||
        sortField !== data.sortBy ||
        sortDirection !== data.sortDirection,
    });
  });

  function pageUrl(
    pageNumber: number,
    search: string,
    nextSortField: DockerContainerSortField,
    nextSortDirection: DockerListSortDirection,
  ) {
    return buildDockerListPageUrl(
      currentPath,
      {
        page: pageNumber,
        search,
        sortBy: nextSortField,
        sortDirection: nextSortDirection,
      },
      defaultSortField,
    );
  }

  async function refreshContainers() {
    if (!data.ready) {
      return;
    }

    refreshing = true;
    try {
      await invalidate('app:docker-containers');
    } finally {
      refreshing = false;
    }
  }

  function actionLabel(action: 'start' | 'stop' | 'restart') {
    switch (action) {
      case 'start':
        return $messages.docker.containers.start;
      case 'stop':
        return $messages.docker.containers.stop;
      case 'restart':
        return $messages.docker.containers.restart;
    }
  }

  async function queueContainerAction(containerId: string, action: 'start' | 'stop' | 'restart') {
    actionBusyId = `${containerId}:${action}`;
    const label = actionLabel(action);
    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/${encodeURIComponent(containerId)}/actions/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(actionErrorMessage(payload, $messages, $messages.docker.containers.actionFailed.replace('{action}', label)));
      }
      toast.success($messages.docker.containers.actionQueued.replace('{action}', label).replace('{taskId}', payload.taskId?.slice(0, 12) ?? 'task'));
      await refreshContainers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.docker.containers.actionFailed.replace('{action}', label));
    } finally {
      actionBusyId = '';
    }
  }

  function isActionBusy(containerId: string) {
    return actionBusyId.startsWith(`${containerId}:`);
  }

  function openRemoveDialog(container: DockerContainerSummary) {
    removeTarget = container;
    removeDialogOpen = true;
  }

  async function queueContainerRemove(removeVolumes: boolean) {
    if (!removeTarget) {
      return;
    }

    const container = removeTarget;
    const force = shouldForceRemove(container.state);

    removeBusyId = container.id;
    removeDialogOpen = false;
    try {
      const response = await fetch(
        `/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/${encodeURIComponent(container.id)}/remove`,
        {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ force, removeVolumes })
        },
      );
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(actionErrorMessage(payload, $messages, $messages.docker.containers.removeFailed));
      }
      toast.success(
        $messages.docker.containers.removeQueued.replace('{taskId}', payload.taskId?.slice(0, 12) ?? 'task'),
      );
      await refreshContainers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.docker.containers.removeFailed);
    } finally {
      removeBusyId = '';
      removeTarget = null;
    }
  }

  function shouldForceRemove(state: string) {
    const normalized = state.toLowerCase();
    return normalized === 'running' || normalized === 'restarting' || normalized === 'paused' || normalized === 'dead';
  }

  let removeDescription = $derived(
    removeTarget
      ? shouldForceRemove(removeTarget.state)
        ? $messages.docker.containers.forceRemoveConfirm.replace('{name}', removeTarget.name || removeTarget.id)
        : $messages.docker.containers.removeConfirm.replace('{name}', removeTarget.name || removeTarget.id)
      : '',
  );

  let removeVolumesDescription = $derived(
    removeTarget
      ? $messages.docker.containers.removeWithVolumesConfirm.replace('{name}', removeTarget.name || removeTarget.id)
      : '',
  );

  let removeActionLabel = $derived(
    removeTarget && shouldForceRemove(removeTarget.state)
      ? $messages.docker.containers.forceRemoveAction
      : $messages.common.delete,
  );

  function flushSearchDebounce() {
    if (searchDebounceTimer) {
      clearTimeout(searchDebounceTimer);
      searchDebounceTimer = null;
    }
    debouncedSearchQuery = searchQuery;
  }

  function handleSort(field: string) {
    flushSearchDebounce();
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as DockerContainerSortField;
      sortDirection = 'asc';
    }
    currentPage = 1;
  }

  function handleSearchInput() {
    if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
    searchDebounceTimer = setTimeout(() => {
      const nextSearch = debouncedDockerListSearchState(searchQuery, debouncedSearchQuery);
      if (nextSearch) {
        currentPage = nextSearch.page;
        debouncedSearchQuery = nextSearch.search;
      }
      searchDebounceTimer = null;
    }, dockerSearchDebounceMs);
  }

  function clearSearch() {
    searchQuery = '';
    currentPage = 1;
    flushSearchDebounce();
  }
</script>

<svelte:head>
  <title>{$messages.docker.containers.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
    <DockerListShell
      title={$messages.docker.containers.title}
      subtitle={$messages.docker.containers.titleOnNode.replace('{nodeId}', data.nodeId)}
      backHref={`/nodes/${data.nodeId}`}
      backLabel={$messages.common.back}
      totalCount={data.totalCount}
      pageSize={data.pageSize}
      itemCount={containers.length}
      {totalPages}
      ready={data.ready}
      {loading}
      error={loadError}
      searchId="container-search"
      searchPlaceholder={$messages.docker.containers.searchPlaceholder}
      loadingText={`${$messages.common.loading} ${$messages.docker.containers.title}...`}
      emptyText={$messages.docker.containers.noContainers}
      noResultsText={$messages.common.noData}
      countSummary={data.totalCount > containers.length
        ? $messages.docker.containers.countSummary
            .replace('{shown}', String(containers.length))
            .replace('{total}', String(data.totalCount))
        : undefined}
      bind:searchQuery
      bind:currentPage
      onSearchInput={handleSearchInput}
      onClearSearch={clearSearch}
      onRefresh={refreshContainers}
    >
          <Table>
            <TableCaption class="sr-only">{$messages.docker.containers.tableCaption}</TableCaption>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label={$messages.common.name} {sortField} {sortDirection} onSort={handleSort} />
                <SortableTableHead field="state" label={$messages.docker.containers.state} {sortField} {sortDirection} onSort={handleSort} />
                <SortableTableHead field="image" label={$messages.docker.containers.image} {sortField} {sortDirection} onSort={handleSort} />
                <TableHead>{$messages.docker.containers.ports}</TableHead>
                <TableHead>{$messages.docker.containers.networks}</TableHead>
                <SortableTableHead field="created" label={$messages.common.created} {sortField} {sortDirection} onSort={handleSort} />
                <TableHead>{$messages.common.actions}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each containers as container}
                <TableRow class="hover:bg-accent/50">
                  <TableCell>
                    <div class="space-y-0.5">
                        <a
                          href="/nodes/{data.nodeId}/docker/containers/{encodeURIComponent(container.id)}"
                          class="font-medium hover:underline"
                        >
                          {container.name}
                        </a>
                      <div class="flex items-center gap-1.5">
                        <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded">
                          {formatShortId(container.id)}
                        </code>
                        <CopyButton text={container.id} label={$messages.common.copy} />
                      </div>
                      {#if container.labels && Object.keys(container.labels).length > 0}
                        <div class="flex flex-wrap gap-1">
                          {#each Object.entries(container.labels).slice(0, 2) as [key, value]}
                            <span class="text-xs text-muted-foreground bg-muted/50 px-1 rounded" aria-label="{key}={value}" title="{key}={value}">
                              {key}
                            </span>
                          {/each}
                          {#if Object.keys(container.labels).length > 2}
                            <span class="text-xs text-muted-foreground">+{Object.keys(container.labels).length - 2}</span>
                          {/if}
                        </div>
                      {/if}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant={containerStateTone(container.state)}>
                      {container.state}
                    </Badge>
                    {#if container.status}
                      <div class="text-xs text-muted-foreground mt-1">{container.status}</div>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="truncate max-w-[200px]" title={container.image}>
                      {container.image}
                    </div>
                  </TableCell>
                  <TableCell>
                    {#if container.ports && container.ports.length > 0}
                      <div class="space-y-0.5">
                        {#each container.ports.slice(0, 3) as port}
                          <code class="text-xs bg-muted px-1 py-0.5 rounded block truncate">{port}</code>
                        {/each}
                        {#if container.ports.length > 3}
                          <span class="text-xs text-muted-foreground">+{container.ports.length - 3}</span>
                        {/if}
                      </div>
                    {:else}
                      <span class="text-muted-foreground">-</span>
                    {/if}
                  </TableCell>
                  <TableCell>
                    {#if container.networks && container.networks.length > 0}
                      <div class="flex flex-wrap gap-1">
                        {#each container.networks.slice(0, 2) as network}
                          <Badge variant="outline" class="text-xs">{network}</Badge>
                        {/each}
                        {#if container.networks.length > 2}
                          <span class="text-xs text-muted-foreground">+{container.networks.length - 2}</span>
                        {/if}
                      </div>
                    {:else}
                      <span class="text-muted-foreground">-</span>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="text-sm text-muted-foreground" title={container.created}>
                      {formatDockerTimestamp(container.created)}
                    </div>
                  </TableCell>
                  <TableCell>
                    <div class="flex flex-wrap gap-2">
                      <Button variant="outline" size="sm" onclick={() => goto(`/nodes/${data.nodeId}/docker/containers/${encodeURIComponent(container.id)}?tab=logs`)}>
                        {$messages.docker.containers.logsLabel}
                      </Button>
                      <Button variant="outline" size="sm" onclick={() => goto(`/nodes/${data.nodeId}/docker/containers/${encodeURIComponent(container.id)}?tab=terminal`)}>
                        {$messages.docker.containers.terminalLabel}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onclick={() => void queueContainerAction(container.id, 'start')}
                        disabled={isActionBusy(container.id) || container.state.toLowerCase() === 'running'}
                      >
                        {$messages.docker.containers.actions.start}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onclick={() => void queueContainerAction(container.id, 'stop')}
                        disabled={isActionBusy(container.id) || container.state.toLowerCase() !== 'running'}
                      >
                        {$messages.docker.containers.actions.stop}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onclick={() => void queueContainerAction(container.id, 'restart')}
                        disabled={isActionBusy(container.id)}
                      >
                        {$messages.docker.containers.actions.restart}
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        onclick={() => openRemoveDialog(container)}
                        disabled={isActionBusy(container.id) || removeBusyId === container.id}
                      >
                        {$messages.docker.containers.actions.remove}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
    </DockerListShell>

    <Dialog bind:open={removeDialogOpen}>
      <DialogOverlay />
      <DialogContent class="max-w-md">
        <DialogHeader>
          <DialogTitle>{$messages.docker.containers.removeDialogTitle}</DialogTitle>
          <DialogDescription>{removeDescription}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" variant="outline" onclick={() => (removeDialogOpen = false)}>
            {$messages.common.cancel}
          </Button>
          <Button type="button" variant="destructive" onclick={() => void queueContainerRemove(false)} disabled={!removeTarget || removeBusyId === removeTarget.id}>
            {removeActionLabel}
          </Button>
          <Button type="button" variant="destructive" onclick={() => void queueContainerRemove(true)} disabled={!removeTarget || removeBusyId === removeTarget.id}>
            {$messages.docker.containers.removeVolumesAction}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</div>
