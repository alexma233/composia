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
  import { formatBytes, formatDockerTimestamp } from '$lib/presenters';
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

  type DockerVolumeSummary = {
    name: string;
    driver: string;
    mountpoint: string;
    scope: string;
    created: string;
    labels: Record<string, string>;
    sizeBytes: number;
    containersCount: number;
    inUse: boolean;
  };

  type DockerVolumeSortField = 'name' | 'driver' | 'created';

  const defaultSortField: DockerVolumeSortField = 'name';

  let searchQuery = $state('');
  let debouncedSearchQuery = $state('');
  let searchDebounceTimer = $state<ReturnType<typeof setTimeout> | null>(null);
  let sortField = $state<DockerVolumeSortField>(defaultSortField);
  let sortDirection = $state<DockerListSortDirection>('asc');
  let currentPage = $state(1);
  let showSizes = $state(false);
  let refreshing = $state(false);
  let removeBusyId = $state('');
  let removeDialogOpen = $state(false);
  let removeTarget = $state<DockerVolumeSummary | null>(null);

  let loading = $derived(!data.ready || refreshing);
  let loadError = $derived(data.error ?? null);
  let volumes = $derived((data.volumes ?? []) as DockerVolumeSummary[]);
  let totalPages = $derived(
    data.totalCount > 0 ? Math.ceil(data.totalCount / data.pageSize) : 0,
  );
  let currentPath = $derived($page.url.pathname);

  $effect(() => {
    refreshing = false;
    currentPage = data.page;
    searchQuery = data.search;
    debouncedSearchQuery = data.search;
    sortField = data.sortBy as DockerVolumeSortField;
    sortDirection = data.sortDirection as DockerListSortDirection;
    showSizes = data.showSizes;
  });

  $effect(() => {
    if (!data.ready) {
      return;
    }

    if (
      currentPage === data.page &&
      debouncedSearchQuery === data.search &&
      sortField === data.sortBy &&
      sortDirection === data.sortDirection &&
      showSizes === data.showSizes
    ) {
      return;
    }

    refreshing = true;
    void goto(pageUrl(currentPage, debouncedSearchQuery, sortField, sortDirection, showSizes), {
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
    nextSortField: DockerVolumeSortField,
    nextSortDirection: DockerListSortDirection,
    nextShowSizes = showSizes,
  ) {
    const url = buildDockerListPageUrl(
      currentPath,
      {
        page: pageNumber,
        search,
        sortBy: nextSortField,
        sortDirection: nextSortDirection,
      },
      defaultSortField,
    );
    if (!nextShowSizes) return url;
    return `${url}${url.includes('?') ? '&' : '?'}showSizes=true`;
  }

  async function refreshVolumes() {
    if (!data.ready) {
      return;
    }

    refreshing = true;
    try {
      await invalidate('app:docker-volumes');
    } finally {
      refreshing = false;
    }
  }

  function openRemoveDialog(volume: DockerVolumeSummary) {
    removeTarget = volume;
    removeDialogOpen = true;
  }

  async function queueVolumeRemove() {
    if (!removeTarget) {
      return;
    }

    const volume = removeTarget;
    removeBusyId = volume.name;
    removeDialogOpen = false;
    try {
      const response = await fetch(`/nodes/${encodeURIComponent(data.nodeId)}/docker/volumes/${encodeURIComponent(volume.name)}/remove`, {
        method: 'POST'
      });
      const payload = await response.json();
      if (!response.ok) {
        throw new Error(actionErrorMessage(payload, $messages, $messages.docker.volumes.removeFailed));
      }
      toast.success($messages.docker.volumes.removeQueued.replace('{taskId}', payload.taskId?.slice(0, 12) ?? 'task'));
      await refreshVolumes();
    } catch (error) {
      toast.error(error instanceof Error ? error.message : $messages.docker.volumes.removeFailed);
    } finally {
      removeBusyId = '';
      removeTarget = null;
    }
  }

  let removeDescription = $derived(
    removeTarget ? $messages.docker.volumes.removeConfirm.replace('{name}', removeTarget.name) : '',
  );

  let removeActionLabel = $derived($messages.common.delete);

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
      sortField = field as DockerVolumeSortField;
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

  function toggleSizes() {
    showSizes = !showSizes;
    currentPage = 1;
  }
</script>

{#snippet toolbarActions()}
  <Button
    variant={showSizes ? 'secondary' : 'outline'}
    size="sm"
    aria-pressed={showSizes}
    onclick={toggleSizes}
    disabled={loading || !data.ready}
  >
    {$messages.docker.volumes.showSizes}
  </Button>
{/snippet}

<svelte:head>
  <title>{$messages.docker.volumes.title} - {$messages.app.name}</title>
</svelte:head>

<div class="page-shell">
  <div class="page-stack">
    <DockerListShell
      title={$messages.docker.volumes.title}
      subtitle={$messages.docker.volumes.titleOnNode.replace('{nodeId}', data.nodeId)}
      backHref={`/nodes/${data.nodeId}`}
      backLabel={$messages.common.back}
      totalCount={data.totalCount}
      pageSize={data.pageSize}
      itemCount={volumes.length}
      {totalPages}
      ready={data.ready}
      {loading}
      error={loadError}
      searchId="volume-search"
      searchPlaceholder={$messages.docker.volumes.searchPlaceholder}
      loadingText={showSizes
        ? $messages.docker.volumes.loadingWithUsage
        : `${$messages.common.loading} ${$messages.docker.volumes.title}...`}
      emptyText={$messages.docker.volumes.noVolumes}
      noResultsText={$messages.common.noData}
      countSummary={data.totalCount > volumes.length
        ? $messages.docker.volumes.countSummary
            .replace('{shown}', String(volumes.length))
            .replace('{total}', String(data.totalCount))
        : undefined}
      bind:searchQuery
      bind:currentPage
      onSearchInput={handleSearchInput}
      onClearSearch={clearSearch}
      onRefresh={refreshVolumes}
      {toolbarActions}
    >
          <Table>
            <TableCaption class="sr-only">{$messages.docker.volumes.tableCaption}</TableCaption>
            <TableHeader>
              <TableRow>
                <SortableTableHead field="name" label={$messages.common.name} {sortField} {sortDirection} onSort={handleSort} />
                <SortableTableHead field="driver" label={$messages.docker.volumes.driver} {sortField} {sortDirection} onSort={handleSort} />
                {#if showSizes}<TableHead>{$messages.docker.volumes.size}</TableHead>{/if}
                <TableHead>{$messages.docker.volumes.usage}</TableHead>
                <TableHead>{$messages.docker.volumes.mountpoint}</TableHead>
                <TableHead>{$messages.docker.volumes.scope}</TableHead>
                <SortableTableHead field="created" label={$messages.common.created} {sortField} {sortDirection} onSort={handleSort} />
                <TableHead>{$messages.common.actions}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {#each volumes as volume}
                <TableRow class="hover:bg-accent/50">
                  <TableCell>
                    <div class="space-y-0.5">
                        <a
                          href="/nodes/{data.nodeId}/docker/volumes/{encodeURIComponent(volume.name)}"
                          class="font-medium truncate max-w-[180px] block hover:underline"
                          title={volume.name}
                        >
                          {volume.name}
                        </a>
                      {#if volume.labels && Object.keys(volume.labels).length > 0}
                        <div class="flex flex-wrap gap-1">
                          {#each Object.entries(volume.labels).slice(0, 2) as [key, value]}
                            <span class="text-xs text-muted-foreground bg-muted/50 px-1 rounded" aria-label="{key}={value}" title="{key}={value}">
                              {key}
                            </span>
                          {/each}
                          {#if Object.keys(volume.labels).length > 2}
                            <span class="text-xs text-muted-foreground">+{Object.keys(volume.labels).length - 2}</span>
                          {/if}
                        </div>
                      {/if}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Badge variant="outline">{volume.driver}</Badge>
                  </TableCell>
                  {#if showSizes}
                    <TableCell>
                      <span class="text-sm">{formatBytes(volume.sizeBytes)}</span>
                    </TableCell>
                  {/if}
                  <TableCell>
                    {#if volume.inUse}
                      <Badge variant="default">{volume.containersCount} {$messages.docker.containers.title}</Badge>
                    {:else}
                      <Badge variant="secondary">{$messages.docker.volumes.unused}</Badge>
                    {/if}
                  </TableCell>
                  <TableCell>
                    <div class="flex items-center gap-1">
                      <code class="text-xs text-muted-foreground bg-muted px-1 py-0.5 rounded truncate max-w-[200px]" title={volume.mountpoint}>
                        {volume.mountpoint}
                      </code>
                      <CopyButton text={volume.mountpoint} label={$messages.common.copy} />
                    </div>
                  </TableCell>
                  <TableCell>
                    <span class="text-sm">{volume.scope}</span>
                  </TableCell>
                  <TableCell>
                    <div class="text-sm text-muted-foreground" title={volume.created}>
                      {formatDockerTimestamp(volume.created)}
                    </div>
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="destructive"
                      size="sm"
                      onclick={() => openRemoveDialog(volume)}
                      disabled={removeBusyId === volume.name}
                    >
                          {$messages.common.delete}
                    </Button>
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
          <DialogTitle>{$messages.docker.volumes.removeDialogTitle}</DialogTitle>
          <DialogDescription>{removeDescription}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button type="button" variant="outline" onclick={() => (removeDialogOpen = false)}>
            {$messages.common.cancel}
          </Button>
          <Button type="button" variant="destructive" onclick={() => void queueVolumeRemove()} disabled={!removeTarget || removeBusyId === removeTarget.name}>
            {$messages.common.delete}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  </div>
</div>
