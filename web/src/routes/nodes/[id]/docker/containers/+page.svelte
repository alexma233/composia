<script lang="ts">
  import { goto, invalidateAll } from '$app/navigation';
  import { page } from '$app/stores';
  import { toast } from 'svelte-sonner';
  import type { PageData } from './$types';
  import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
  import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '$lib/components/ui/table';
  import { Badge } from '$lib/components/ui/badge';
  import { Input } from '$lib/components/ui/input';
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
    Pagination,
    PaginationContent,
    PaginationEllipsis,
    PaginationItem,
    PaginationLink,
    PaginationNextButton,
    PaginationPrevButton,
  } from '$lib/components/ui/pagination';
  import {
    buildDockerListPageUrl,
    type DockerListSortDirection,
  } from '$lib/docker-list-query';
  import {
  containerStateTone,
  formatDockerTimestamp,
  formatShortId,
} from '$lib/presenters';
  import CopyButton from '$lib/components/app/copy-button.svelte';
  import SortableTableHead from '$lib/components/app/sortable-table-head.svelte';
  import Spinner from '$lib/components/ui/spinner/spinner.svelte';
  import { Search } from 'lucide-svelte';
  import { Alert, AlertDescription } from '$lib/components/ui/alert';
  import { messages } from '$lib/i18n';

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
    sortField = data.sortBy as DockerContainerSortField;
    sortDirection = data.sortDirection as DockerListSortDirection;
  });

  $effect(() => {
    if (!data.ready) {
      return;
    }

    if (
      currentPage === data.page &&
      searchQuery === data.search &&
      sortField === data.sortBy &&
      sortDirection === data.sortDirection
    ) {
      return;
    }

    refreshing = true;
    void goto(pageUrl(currentPage, searchQuery, sortField, sortDirection), {
      keepFocus: true,
      noScroll: true,
      replaceState:
        searchQuery !== data.search ||
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
      await invalidateAll();
    } finally {
      refreshing = false;
    }
  }

  async function queueContainerAction(containerId: string, action: 'start' | 'stop' | 'restart') {
    actionBusyId = `${containerId}:${action}`;
    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/containers/${encodeURIComponent(containerId)}/actions/${action}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(payload.error ?? $messages.docker.containers.actionFailed.replace('{action}', action));
      }
      toast.success($messages.docker.containers.actionQueued.replace('{action}', action).replace('{taskId}', payload.taskId?.slice(0, 12) ?? 'task'));
      await refreshContainers();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.docker.containers.actionFailed.replace('{action}', action));
    } finally {
      actionBusyId = '';
    }
  }

  function isActionBusy(containerId: string, action: 'start' | 'stop' | 'restart') {
    return actionBusyId === `${containerId}:${action}`;
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
        throw new Error(payload.error ?? $messages.docker.containers.removeFailed);
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

  function handleSort(field: string) {
    if (sortField === field) {
      sortDirection = sortDirection === 'asc' ? 'desc' : 'asc';
    } else {
      sortField = field as DockerContainerSortField;
      sortDirection = 'asc';
    }
    currentPage = 1;
  }

  function handleSearchInput() {
    currentPage = 1;
  }

  function clearSearch() {
    searchQuery = '';
    currentPage = 1;
  }
</script>

<svelte:head>
  <title>{$messages.docker.containers.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
		<Card>
			<CardHeader>
        <div class="page-header">
          <div class="page-heading">
            <CardTitle class="page-title" level="1">{$messages.docker.containers.title}</CardTitle>
            <p class="page-description">
              {$messages.docker.containers.titleOnNode.replace('{nodeId}', data.nodeId)}
              {#if !loading}
                <Badge variant="outline" class="ml-2">{data.totalCount}</Badge>
              {/if}
            </p>
          </div>
          <a href="/nodes/{data.nodeId}" class="text-sm text-muted-foreground transition-colors hover:text-foreground">
            {$messages.common.back}
          </a>
        </div>

        <div class="flex items-center gap-3">
          <div class="relative flex-1 max-w-sm">
            <Search class="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
            <Input
              type="text"
              placeholder={$messages.docker.containers.searchPlaceholder}
              aria-label={$messages.docker.containers.searchPlaceholder}
              class="pl-9"
              bind:value={searchQuery}
              oninput={handleSearchInput}
            />
          </div>
          {#if searchQuery}
            <Button variant="ghost" size="sm" onclick={clearSearch}>
              {$messages.common.cancel}
            </Button>
          {/if}
          <Button variant="outline" size="sm" onclick={() => void refreshContainers()} disabled={loading || !data.ready}>
            {#if loading}{$messages.common.loading}...{:else}{$messages.common.refresh}{/if}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {#if loadError}
          <Alert variant="destructive">
            <AlertDescription>{loadError}</AlertDescription>
          </Alert>
        {:else if loading}
          <div class="flex min-h-[320px] items-center justify-center">
            <div class="flex items-center gap-3 text-sm text-muted-foreground">
              <Spinner />
              <span>{$messages.common.loading} {$messages.docker.containers.title}...</span>
            </div>
          </div>
        {:else if containers.length > 0}
          <Table>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label={$messages.common.name} {sortField} {sortDirection} onSort={handleSort} class="w-[30%]" />
                <SortableTableHead field="state" label={$messages.docker.containers.state} {sortField} {sortDirection} onSort={handleSort} class="w-[10%]" />
                <SortableTableHead field="image" label={$messages.docker.containers.image} {sortField} {sortDirection} onSort={handleSort} class="w-[20%]" />
                <TableHead class="w-[15%]">{$messages.docker.containers.ports}</TableHead>
                <TableHead class="w-[15%]">{$messages.docker.containers.networks}</TableHead>
                <SortableTableHead field="created" label={$messages.common.created} {sortField} {sortDirection} onSort={handleSort} class="w-[12%]" />
                <TableHead class="w-[10%]">{$messages.common.actions}</TableHead>
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
                            <span class="text-xs text-muted-foreground bg-muted/50 px-1 rounded" title="{key}={value}">
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
                        disabled={isActionBusy(container.id, 'start') || container.state.toLowerCase() === 'running'}
                      >
                        {$messages.docker.containers.actions.start}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onclick={() => void queueContainerAction(container.id, 'stop')}
                        disabled={isActionBusy(container.id, 'stop') || container.state.toLowerCase() !== 'running'}
                      >
                        {$messages.docker.containers.actions.stop}
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onclick={() => void queueContainerAction(container.id, 'restart')}
                        disabled={isActionBusy(container.id, 'restart')}
                      >
                        {$messages.docker.containers.actions.restart}
                      </Button>
                      <Button
                        variant="destructive"
                        size="sm"
                        onclick={() => openRemoveDialog(container)}
                        disabled={removeBusyId === container.id}
                      >
                        {$messages.docker.containers.actions.remove}
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              {/each}
            </TableBody>
          </Table>
          {#if data.totalCount > containers.length}
            <div class="mt-3 text-xs text-muted-foreground text-center">
              {$messages.docker.containers.countSummary.replace('{shown}', String(containers.length)).replace('{total}', String(data.totalCount))}
            </div>
          {/if}

          {#if totalPages > 1}
            <div class="mt-6">
              <Pagination count={data.totalCount} perPage={data.pageSize} bind:page={currentPage}>
                {#snippet children({ pages, currentPage })}
                  <PaginationContent>
                    <PaginationItem>
                      <PaginationPrevButton />
                    </PaginationItem>

                    {#each pages as page (page.key)}
                      {#if page.type === 'ellipsis'}
                        <PaginationItem>
                          <PaginationEllipsis />
                        </PaginationItem>
                      {:else}
                        <PaginationItem>
                          <PaginationLink {page} isActive={currentPage === page.value} />
                        </PaginationItem>
                      {/if}
                    {/each}

                    <PaginationItem>
                      <PaginationNextButton />
                    </PaginationItem>
                  </PaginationContent>
                {/snippet}
              </Pagination>
            </div>
          {/if}
        {:else if searchQuery}
          <div class="empty-state">
            {$messages.common.noData}
          </div>
        {:else}
          <div class="empty-state">{$messages.docker.containers.noContainers}</div>
        {/if}
      </CardContent>
    </Card>

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
